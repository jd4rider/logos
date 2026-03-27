package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jd4rider/logos/internal/ai"
	"github.com/jd4rider/logos/internal/api"
	"github.com/jd4rider/logos/internal/biblemeta"
	localdb "github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/precache"
	coretts "github.com/jd4rider/logos/internal/tts"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

// ── State ─────────────────────────────────────────────────────────────────────

type State int

const (
	StateLoading State = iota
	StateBibles
	StateBooks
	StateChapters
	StateReader
	StateSearch
	StateVoicePicker
	StateLanguagePicker
	StateImport
	StateAI
)

// ── Messages ──────────────────────────────────────────────────────────────────

type biblesLoadedMsg struct {
	bibles       []api.Bible
	offline      bool
	offlineFlags []bool
}
type booksLoadedMsg struct{ books []api.Book }
type chaptersLoadedMsg struct{ chapters []api.Chapter }
type chapterLoadedMsg struct{ content api.ChapterContent }
type searchDoneMsg struct{ results api.SearchData }
type errMsg struct{ err error }

// import panel messages
type importProgressMsg struct{ line string }
type importDoneMsg struct {
	err           error
	translationID string
}

// precache messages
type precacheProgressMsg struct {
	ch        <-chan precache.Progress
	bookName  string
	chapterID string
	done      int
	total     int
	err       error
}
type precacheDoneMsg struct{ translationID string }

// AI TTS messages
type aiReadAloudMsg struct{ text string }

// ── List items ────────────────────────────────────────────────────────────────

type bibleItem struct {
	b       api.Bible
	offline bool // true = sourced from local SQLite, not API
}

func (i bibleItem) Title() string {
	prefix := ""
	if i.offline {
		prefix = "📦 "
	}
	return prefix + fmt.Sprintf("%-8s %s", biblemeta.DisplayBibleAbbreviation(i.b.Abbreviation), i.b.Name)
}
func (i bibleItem) Description() string {
	if i.offline {
		return i.b.Language.Name + " [offline]"
	}
	return i.b.Language.Name
}
func (i bibleItem) FilterValue() string { return i.b.Name + " " + i.b.Abbreviation }

type bookItem struct{ b api.Book }

func (i bookItem) Title() string       { return i.b.Name }
func (i bookItem) Description() string { return i.b.NameLong }
func (i bookItem) FilterValue() string { return i.b.Name }

type chapterItem struct{ c api.Chapter }

func (i chapterItem) Title() string       { return "Chapter " + i.c.Number }
func (i chapterItem) Description() string { return "" }
func (i chapterItem) FilterValue() string { return i.c.Number }

type searchItem struct{ v api.SearchVerse }

func (i searchItem) Title() string {
	return i.v.Reference
}
func (i searchItem) Description() string {
	if len(i.v.Text) > 120 {
		return i.v.Text[:120] + "…"
	}
	return i.v.Text
}
func (i searchItem) FilterValue() string { return i.v.Reference + " " + i.v.Text }

type languageItem struct {
	code string
	name string
}

func (i languageItem) Title() string { return i.name }
func (i languageItem) Description() string {
	if i.code == "" {
		return "Show every available translation."
	}
	return "Filter the library to " + i.name + " translations."
}
func (i languageItem) FilterValue() string { return i.name + " " + i.code }

type voiceItem struct{ entry coretts.VoiceEntry }

func (i voiceItem) Title() string { return i.entry.Name }
func (i voiceItem) Description() string {
	switch i.entry.Engine {
	case "piper":
		return "Piper TTS — high quality neural"
	case "kokoro":
		return "Kokoro TTS — natural neural"
	case "say":
		return "macOS built-in"
	case "windows":
		return "Windows built-in speech"
	case "espeak":
		return "Linux fallback speech"
	case "speechd":
		return "Speech Dispatcher fallback"
	}
	return ""
}
func (i voiceItem) FilterValue() string { return i.entry.Name }

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	state         State
	prevState     State
	width, height int

	client   *api.Client
	tts      *coretts.Engine
	styles   *Styles
	keys     KeyMap
	localDB  *localdb.DB // nil if db not available
	aiClient *ai.Client

	// Navigation
	selectedBible        api.Bible
	selectedBibleOffline bool
	selectedBook         api.Book
	currentChapter       api.ChapterContent
	languageFilter       string

	// Bubbles
	bibleList    list.Model
	bookList     list.Model
	chapterList  list.Model
	searchList   list.Model
	voiceList    list.Model
	languageList list.Model
	viewport     viewport.Model
	searchInput  textinput.Model
	spinner      spinner.Model

	// Import panel
	importPanel *ImportPanel

	// AI panel
	aiPanel *AIPanel

	// TTS word highlighting
	ttsWords     []string
	ttsDurations []time.Duration // per-word durations calibrated to actual audio
	ttsWordIndex int
	ttsSpeaking  bool
	ttsPaused    bool   // audio paused (SIGSTOP); word tick also halted
	ttsCleanText string // CleanForTTS output; used for jump-to-word re-synthesis
	ttsGen       int    // session generation; stale WordAdvanceMsg are discarded

	// Search overlay
	inSearch bool

	// Misc
	loading   bool
	err       error
	statusMsg string // transient status bar message (e.g. precache progress)
}

func NewModel(client *api.Client, ttsEngine *coretts.Engine, initialBibleID string) Model {
	styles := NewStyles()
	keys := DefaultKeyMap()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorGold)

	si := textinput.New()
	si.Placeholder = "Search the Bible…"
	si.CharLimit = 120
	si.Width = 50
	si.PromptStyle = lipgloss.NewStyle().Foreground(ColorPurple)
	si.TextStyle = lipgloss.NewStyle().Foreground(ColorText)

	m := Model{
		state:          StateLoading,
		client:         client,
		tts:            ttsEngine,
		styles:         styles,
		keys:           keys,
		spinner:        sp,
		searchInput:    si,
		loading:        true,
		languageFilter: "eng",
		selectedBible:  api.Bible{ID: initialBibleID},
		ttsWordIndex:   -1,
		// Pre-init all lists so SetSize never hits a zero-value struct
		bibleList:    newStyledList([]list.Item{}, "✝  Select Translation", 80, 24),
		bookList:     newStyledList([]list.Item{}, "✝  Select Book", 80, 24),
		chapterList:  newStyledList([]list.Item{}, "✝  Select Chapter", 80, 24),
		searchList:   newStyledList([]list.Item{}, "✝  Search Results", 80, 24),
		voiceList:    newStyledList([]list.Item{}, "✝  Select Voice", 80, 24),
		languageList: newStyledList([]list.Item{}, "✝  Filter Language", 80, 24),
	}
	// Open local SQLite db (best-effort — TUI still works without it)
	if ldb, err := localdb.Open(localdb.DefaultDBPath()); err == nil {
		m.localDB = ldb
	}
	// Initialise Ollama AI client (works even if Ollama is not running)
	m.aiClient = ai.NewClient()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.cmdLoadBibles())
}

// ── Commands ──────────────────────────────────────────────────────────────────

func (m Model) cmdLoadBibles() tea.Cmd {
	filter := m.languageFilter
	return func() tea.Msg {
		apiBibles, apiErr := m.client.GetBibles(filter)

		// Build a merged, alphabetically-sorted list.
		// Local (offline) entries win over API entries with the same abbreviation
		// so that licensed translations that require a higher API tier don't 403.
		seenID := map[string]bool{}
		seenAbbr := map[string]bool{} // normalised abbreviation dedup
		var items []bibleItem

		// Add local SQLite translations FIRST so they take dedup priority
		if m.localDB != nil {
			if locals, err := m.localDB.ListTranslations(); err == nil {
				for _, t := range locals {
					if !biblemeta.MatchesLanguage(t.Language, filter) {
						continue
					}
					normAbbr := strings.ToUpper(stripLangPrefix(t.Abbreviation))
					items = append(items, bibleItem{
						b: api.Bible{
							ID:           t.ID,
							Name:         t.Name,
							Abbreviation: biblemeta.DisplayBibleAbbreviation(t.Abbreviation),
							Language: api.Language{
								ID:        t.Language,
								Name:      displayLanguageName(t.Language),
								NameLocal: displayLanguageName(t.Language),
							},
						},
						offline: true,
					})
					seenID[t.ID] = true
					seenAbbr[normAbbr] = true
				}
			}
		}

		// Add API translations, skipping any whose abbreviation matches a local one
		if apiErr == nil {
			for _, b := range apiBibles {
				if seenID[b.ID] {
					continue
				}
				normAbbr := strings.ToUpper(stripLangPrefix(b.Abbreviation))
				if seenAbbr[normAbbr] {
					// local version exists — skip the API copy to avoid 403
					continue
				}
				b.Abbreviation = biblemeta.DisplayBibleAbbreviation(b.Abbreviation)
				items = append(items, bibleItem{b: b, offline: false})
				seenID[b.ID] = true
				seenAbbr[normAbbr] = true
			}
		}

		// Sort alphabetically by stripped abbreviation
		sort.Slice(items, func(i, j int) bool {
			ai := strings.ToUpper(stripLangPrefix(items[i].b.Abbreviation))
			aj := strings.ToUpper(stripLangPrefix(items[j].b.Abbreviation))
			return ai < aj
		})

		bibles := make([]api.Bible, len(items))
		offlineFlags := make([]bool, len(items))
		for i, it := range items {
			bibles[i] = it.b
			offlineFlags[i] = it.offline
		}

		if apiErr != nil && len(bibles) == 0 {
			return errMsg{apiErr}
		}
		return biblesLoadedMsg{bibles: bibles, offline: apiErr != nil, offlineFlags: offlineFlags}
	}
}

func (m Model) cmdLoadBooks() tea.Cmd {
	id := m.selectedBible.ID
	return func() tea.Msg {
		if m.selectedBibleOffline && m.localDB != nil {
			books, err := m.localDB.ListBooks(id)
			if err != nil {
				return errMsg{err}
			}

			out := make([]api.Book, len(books))
			for i, b := range books {
				abbr := b.ShortName
				if abbr == "" {
					abbr = b.ID
				}
				out[i] = api.Book{
					ID:           b.ID,
					BibleID:      id,
					Abbreviation: abbr,
					Name:         b.Name,
					NameLong:     b.Name,
				}
			}
			return booksLoadedMsg{out}
		}
		books, err := m.client.GetBooks(id)
		if err != nil {
			return errMsg{err}
		}
		return booksLoadedMsg{books}
	}
}

func (m Model) cmdLoadChapters() tea.Cmd {
	bid, bookID := m.selectedBible.ID, m.selectedBook.ID
	return func() tea.Msg {
		if m.selectedBibleOffline && m.localDB != nil {
			chapters, err := m.localDB.ListChapters(bookID, bid)
			if err != nil {
				return errMsg{err}
			}

			out := make([]api.Chapter, len(chapters))
			for i, c := range chapters {
				num := strconv.Itoa(c.Number)
				out[i] = api.Chapter{
					ID:       c.ID,
					BibleID:  bid,
					BookID:   bookID,
					Number:   num,
					Position: c.Number,
				}
			}
			return chaptersLoadedMsg{out}
		}
		chapters, err := m.client.GetChapters(bid, bookID)
		if err != nil {
			return errMsg{err}
		}
		return chaptersLoadedMsg{chapters}
	}
}

func (m Model) cmdLoadChapter(chapterID string) tea.Cmd {
	bid := m.selectedBible.ID
	return func() tea.Msg {
		if m.selectedBibleOffline && m.localDB != nil {
			ch, err := m.loadOfflineChapter(chapterID)
			if err != nil {
				return errMsg{err}
			}
			return chapterLoadedMsg{ch}
		}
		ch, err := m.client.GetChapter(bid, chapterID)
		if err != nil {
			return errMsg{err}
		}
		return chapterLoadedMsg{ch}
	}
}

func (m Model) cmdSearch(query string) tea.Cmd {
	bid := m.selectedBible.ID
	return func() tea.Msg {
		if m.selectedBibleOffline && m.localDB != nil {
			results, err := m.localDB.Search(bid, query, 30)
			if err != nil {
				return errMsg{err}
			}

			verses := make([]api.SearchVerse, len(results))
			for i, r := range results {
				verses[i] = api.SearchVerse{
					ID:        r.VerseID,
					BookID:    r.BookID,
					BibleID:   bid,
					ChapterID: r.ChapterID,
					Reference: formatLocalReference(r.VerseID),
					Text:      r.Text,
				}
			}
			return searchDoneMsg{api.SearchData{
				Query:      query,
				Limit:      30,
				Offset:     0,
				Total:      len(verses),
				VerseCount: len(verses),
				Verses:     verses,
			}}
		}
		results, err := m.client.Search(bid, query, 30)
		if err != nil {
			return errMsg{err}
		}
		return searchDoneMsg{results}
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

// cmdStartPrecache kicks off a background TTS pre-cache job and returns the
// first progress message via the BubbleTea event loop.
func (m Model) cmdStartPrecache(translationID string) tea.Cmd {
	job := precache.NewJob(m.localDB, m.tts, translationID)
	job.Start()
	ch := job.Progress()
	return m.cmdWaitPrecache(ch)
}

// cmdWaitPrecache reads the next event from a running precache job channel.
func (m Model) cmdWaitPrecache(ch <-chan precache.Progress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok || p.Finished {
			tid := ""
			if ok {
				tid = p.TranslationID
			}
			return precacheDoneMsg{translationID: tid}
		}
		return precacheProgressMsg{
			ch:        ch,
			bookName:  p.BookName,
			chapterID: p.ChapterID,
			done:      p.Done,
			total:     p.Total,
			err:       p.Err,
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		h := m.contentHeight()
		m.bibleList.SetSize(msg.Width, h)
		m.bookList.SetSize(msg.Width, h)
		m.chapterList.SetSize(msg.Width, h)
		m.searchList.SetSize(msg.Width, h)
		m.voiceList.SetSize(msg.Width, h)
		m.languageList.SetSize(msg.Width, h)
		if m.state == StateReader {
			m.viewport.Width = msg.Width
			m.viewport.Height = h
			if m.currentChapter.ID != "" {
				m.setReaderContent(m.ttsWordIndex)
			}
		}
		if m.importPanel != nil {
			m.importPanel.SetSize(msg.Width, h)
		}
		if m.aiPanel != nil {
			m.aiPanel.SetSize(msg.Width, h)
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case biblesLoadedMsg:
		m.loading = false
		items := make([]list.Item, len(msg.bibles))
		for i, b := range msg.bibles {
			offline := i < len(msg.offlineFlags) && msg.offlineFlags[i]
			items[i] = bibleItem{b: b, offline: offline}
		}
		title := "✝  Select Translation  [" + m.languageFilterLabel() + "]"
		if msg.offline {
			title += "  [offline mode]"
		}
		m.bibleList = newStyledList(items, title, m.width, m.contentHeight())
		foundSelected := false
		for idx, b := range msg.bibles {
			if b.ID == m.selectedBible.ID {
				m.selectedBibleOffline = idx < len(msg.offlineFlags) && msg.offlineFlags[idx]
				m.bibleList.Select(idx)
				foundSelected = true
				break
			}
		}
		if !foundSelected {
			m.selectedBible = api.Bible{}
			m.selectedBibleOffline = false
			m.selectedBook = api.Book{}
			m.currentChapter = api.ChapterContent{}
		}
		m.state = StateBibles
		return m, nil

	case booksLoadedMsg:
		m.loading = false
		items := make([]list.Item, len(msg.books))
		for i, b := range msg.books {
			items[i] = bookItem{b}
		}
		m.bookList = newStyledList(items, "✝  "+biblemeta.DisplayBibleAbbreviation(m.selectedBible.Abbreviation)+"  ›  Select Book", m.width, m.contentHeight())
		m.state = StateBooks
		return m, nil

	case chaptersLoadedMsg:
		m.loading = false
		items := make([]list.Item, len(msg.chapters))
		for i, c := range msg.chapters {
			items[i] = chapterItem{c}
		}
		m.chapterList = newStyledList(items, "✝  "+biblemeta.DisplayBibleAbbreviation(m.selectedBible.Abbreviation)+"  ›  "+m.selectedBook.Name, m.width, m.contentHeight())
		m.state = StateChapters
		return m, nil

	case chapterLoadedMsg:
		m.loading = false
		m.currentChapter = msg.content
		m.ttsSpeaking = false
		m.ttsPaused = false
		m.ttsWordIndex = -1
		m.ttsCleanText = coretts.CleanForTTS(msg.content.Content)
		m.ttsWords = coretts.SplitWords(m.ttsCleanText)
		m.viewport = viewport.New(m.width, m.contentHeight())
		m.setReaderContent(-1)
		m.viewport.GotoTop()
		m.state = StateReader
		return m, nil

	case searchDoneMsg:
		m.loading = false
		m.inSearch = false
		items := make([]list.Item, len(msg.results.Verses))
		for i, v := range msg.results.Verses {
			items[i] = searchItem{v}
		}
		title := fmt.Sprintf("✝  \"%s\"  (%d results)", msg.results.Query, msg.results.Total)
		m.searchList = newStyledList(items, title, m.width, m.contentHeight())
		m.state = StateSearch
		return m, nil

	case coretts.TTSStartedMsg:
		// Discard stale started messages from a previous session
		if msg.Gen != m.ttsGen || !m.ttsSpeaking {
			return m, nil
		}
		return m, coretts.SyncedWordTickCmd(m.ttsDurations, 1, m.ttsGen)

	case coretts.WordAdvanceMsg:
		// Discard stale ticks from superseded TTS sessions
		if msg.Gen != m.ttsGen || !m.ttsSpeaking {
			return m, nil
		}
		m.ttsWordIndex = msg.Index
		// Update highlighted word in the appropriate content view
		if m.state == StateReader {
			m.setReaderContent(m.ttsWordIndex)
			if msg.Index > 0 && msg.Index%40 == 0 {
				m.viewport.LineDown(2)
			}
		} else if m.state == StateAI {
			m.setAIContent(m.ttsWordIndex)
			if msg.Index > 0 && msg.Index%40 == 0 && m.aiPanel != nil {
				m.aiPanel.vp.LineDown(2)
			}
		}
		if msg.Index < len(m.ttsWords) {
			if m.ttsPaused {
				return m, nil // don't schedule next tick while paused
			}
			return m, coretts.SyncedWordTickCmd(m.ttsDurations, msg.Index+1, m.ttsGen)
		}
		// All words highlighted — playback finished
		m.ttsSpeaking = false
		m.ttsPaused = false
		m.ttsWordIndex = -1
		if m.state == StateReader {
			m.setReaderContent(-1)
		} else if m.state == StateAI {
			m.setAIContent(-1)
		}
		return m, nil

	case aiReadAloudMsg:
		if m.tts == nil || !m.tts.Available() {
			return m, nil
		}
		m.tts.Stop()
		m.ttsPaused = false
		m.ttsGen++ // invalidate any in-flight ticks from previous session
		clean := coretts.CleanForTTS(msg.text)
		words := coretts.SplitWords(clean)
		m.ttsCleanText = clean
		m.ttsWords = words
		if synced, err := m.tts.SpeakSynced(clean, words); err == nil {
			m.ttsSpeaking = true
			m.ttsWordIndex = 0
			m.ttsDurations = synced.WordDurations
			return m, coretts.WaitForTTSStart(synced.Started, m.ttsGen)
		}
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	// ── AI panel messages ────────────────────────────────────────────────────

	case aiTokenMsg, aiDoneMsg, aiSavedMsg, libraryLoadedMsg:
		if m.aiPanel != nil {
			var cmd tea.Cmd
			m.aiPanel, cmd = m.aiPanel.Update(msg)
			return m, cmd
		}
		return m, nil

	// fullResultMsg: complete text returned from StartStream
	case struct {
		text string
		err  error
	}:
		if m.aiPanel != nil {
			if msg.err != nil {
				m.aiPanel, _ = m.aiPanel.Update(aiDoneMsg{err: msg.err})
			} else {
				m.aiPanel.streamed.Reset()
				m.aiPanel.streamed.WriteString(msg.text)
				m.aiPanel.vp.SetContent(wrapText(msg.text, m.aiPanel.vp.Width))
				m.aiPanel.vp.GotoBottom()
				m.aiPanel, _ = m.aiPanel.Update(aiDoneMsg{})
			}
		}
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	// ── Precache progress messages ────────────────────────────────────────────

	case precacheProgressMsg:
		if msg.err == nil {
			m.statusMsg = fmt.Sprintf("🔊 Pre-caching %s ch.%s (%d/%d)", msg.bookName, msg.chapterID, msg.done, msg.total)
		}
		return m, m.cmdWaitPrecache(msg.ch)

	case precacheDoneMsg:
		m.statusMsg = fmt.Sprintf("✓ TTS pre-cache complete for %s", msg.translationID)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m.updateActiveComponent(msg)
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.state == StateReader {
			m.viewport.LineUp(3)
		}
	case tea.MouseButtonWheelDown:
		if m.state == StateReader {
			m.viewport.LineDown(3)
		}
	case tea.MouseButtonLeft:
		// Left-click in reader: jump TTS playback to the clicked word
		if msg.Action == tea.MouseActionRelease && m.state == StateReader && m.ttsSpeaking {
			wordIdx := m.wordIdxAtClick(msg.X, msg.Y)
			return m.jumpToWord(wordIdx)
		}
	}
	return m.updateActiveComponent(msg)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.inSearch {
		switch msg.String() {
		case "q", "ctrl+c":
			if m.tts != nil {
				m.tts.Stop()
			}
			return m, tea.Quit
		}
	}

	// Search input mode
	if m.inSearch {
		switch msg.Type {
		case tea.KeyEsc:
			m.inSearch = false
			m.searchInput.Blur()
			return m, nil
		case tea.KeyEnter:
			q := strings.TrimSpace(m.searchInput.Value())
			if q != "" && m.selectedBible.ID != "" {
				m.loading = true
				m.state = StateLoading
				return m, tea.Batch(m.spinner.Tick, m.cmdSearch(q))
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	switch m.state {

	case StateBibles:
		switch msg.String() {
		case "enter", "right", "l":
			if item, ok := m.bibleList.SelectedItem().(bibleItem); ok {
				m.selectedBible = item.b
				m.selectedBibleOffline = item.offline
				m.loading = true
				m.state = StateLoading
				return m, tea.Batch(m.spinner.Tick, m.cmdLoadBooks())
			}
		case "f", "F":
			return m.openLanguagePicker()
		case "i", "I":
			// Open import panel
			if m.importPanel == nil {
				m.importPanel = NewImportPanel(m.localDB, m.aiClient)
			}
			m.importPanel.Reset()
			m.state = StateImport
			return m, m.importPanel.Init()
		case "/":
			return m.openSearch()
		}

	case StateBooks:
		switch msg.String() {
		case "enter", "right", "l":
			if item, ok := m.bookList.SelectedItem().(bookItem); ok {
				m.selectedBook = item.b
				m.loading = true
				m.state = StateLoading
				return m, tea.Batch(m.spinner.Tick, m.cmdLoadChapters())
			}
		case "esc", "backspace":
			m.loading = true
			m.state = StateLoading
			return m, tea.Batch(m.spinner.Tick, m.cmdLoadBibles())
		case "f", "F":
			return m.openLanguagePicker()
		case "/":
			return m.openSearch()
		}

	case StateChapters:
		switch msg.String() {
		case "enter", "right", "l":
			if item, ok := m.chapterList.SelectedItem().(chapterItem); ok {
				m.loading = true
				m.state = StateLoading
				return m, tea.Batch(m.spinner.Tick, m.cmdLoadChapter(item.c.ID))
			}
		case "esc", "backspace":
			m.state = StateBooks
			return m, nil
		case "f", "F":
			return m.openLanguagePicker()
		case "/":
			return m.openSearch()
		}

	case StateReader:
		switch msg.String() {
		case "esc", "backspace":
			if m.tts != nil {
				m.tts.Stop()
			}
			m.ttsSpeaking = false
			m.ttsPaused = false
			m.state = StateChapters
			return m, nil
		case "s":
			if m.tts != nil && m.tts.Available() {
				m.tts.Stop()
				m.ttsPaused = false
				m.ttsGen++ // new session — invalidate any queued ticks
				clean := m.ttsCleanText
				if clean == "" {
					clean = coretts.CleanForTTS(m.currentChapter.Content)
					m.ttsCleanText = clean
					m.ttsWords = coretts.SplitWords(clean)
				}
				if synced, err := m.tts.SpeakSynced(clean, m.ttsWords); err == nil {
					m.ttsSpeaking = true
					m.ttsWordIndex = 0
					m.ttsDurations = synced.WordDurations
					return m, coretts.WaitForTTSStart(synced.Started, m.ttsGen)
				}
			}
		case "S":
			// Always kill audio regardless of ttsSpeaking state — a stale tick
			// may have cleared ttsSpeaking while sox is still running.
			if m.tts != nil {
				m.tts.Stop()
			}
			m.ttsSpeaking = false
			m.ttsPaused = false
			m.ttsWordIndex = -1
			m.ttsGen++ // prevent any queued ticks from re-enabling speak
			m.setReaderContent(-1)
		case " ":
			return m.toggleTTSPause()
		case "v":
			return m.openVoicePicker()
		case "n", "]":
			if m.currentChapter.Next != nil {
				if m.tts != nil {
					m.tts.Stop()
				}
				m.ttsSpeaking = false
				m.ttsPaused = false
				m.loading = true
				m.state = StateLoading
				return m, tea.Batch(m.spinner.Tick, m.cmdLoadChapter(m.currentChapter.Next.ID))
			}
		case "p", "[":
			if m.currentChapter.Previous != nil && m.currentChapter.Previous.Number != "intro" {
				if m.tts != nil {
					m.tts.Stop()
				}
				m.ttsSpeaking = false
				m.ttsPaused = false
				m.loading = true
				m.state = StateLoading
				return m, tea.Batch(m.spinner.Tick, m.cmdLoadChapter(m.currentChapter.Previous.ID))
			}
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		case "/":
			return m.openSearch()
		case "a":
			return m.openAIPanel()
		}

	case StateSearch:
		switch msg.String() {
		case "enter", "right", "l":
			if item, ok := m.searchList.SelectedItem().(searchItem); ok {
				m.loading = true
				m.state = StateLoading
				return m, tea.Batch(m.spinner.Tick, m.cmdLoadChapter(item.v.ChapterID))
			}
		case "esc", "backspace":
			if m.currentChapter.ID != "" {
				m.state = StateReader
			} else {
				m.state = StateBooks
			}
			return m, nil
		case "/":
			return m.openSearch()
		}

	case StateVoicePicker:
		switch msg.String() {
		case "esc", "backspace":
			m.state = m.prevState
			return m, nil
		case "enter", "right", "l":
			if item, ok := m.voiceList.SelectedItem().(voiceItem); ok {
				if m.tts != nil {
					m.tts.SetVoiceEntry(item.entry)
				}
				m.state = m.prevState
			}
		}

	case StateLanguagePicker:
		switch msg.String() {
		case "esc", "backspace":
			m.state = m.prevState
			return m, nil
		case "enter", "right", "l":
			if item, ok := m.languageList.SelectedItem().(languageItem); ok {
				m.languageFilter = item.code
				m.loading = true
				m.selectedBook = api.Book{}
				m.currentChapter = api.ChapterContent{}
				m.state = StateLoading
				return m, tea.Batch(m.spinner.Tick, m.cmdLoadBibles())
			}
		}

	case StateImport:
		if m.importPanel != nil {
			var cmd tea.Cmd
			m.importPanel, cmd = m.importPanel.Update(msg)
			if m.importPanel.Done() {
				tid := m.importPanel.lastTranslationID
				// Refresh the bible list after a successful import
				m.state = StateLoading
				m.loading = true
				// Auto-start TTS pre-cache in background
				var precacheCmd tea.Cmd
				if tid != "" && m.localDB != nil && m.tts != nil {
					precacheCmd = m.cmdStartPrecache(tid)
				}
				return m, tea.Batch(cmd, m.spinner.Tick, m.cmdLoadBibles(), precacheCmd)
			}
			return m, cmd
		}

	case StateAI:
		if msg.String() == "esc" && m.aiPanel != nil && m.aiPanel.sub == aiMenu {
			m.state = m.prevState
			return m, nil
		}
		// Stop TTS when closing the AI result (going back to menu)
		if m.ttsSpeaking && m.aiPanel != nil && m.aiPanel.sub == aiResult &&
			(msg.String() == "esc" || msg.String() == "backspace") {
			if m.tts != nil {
				m.tts.Stop()
			}
			m.ttsSpeaking = false
			m.ttsPaused = false
			m.ttsWordIndex = -1
		}
		// TTS controls in the AI result view (intercept before passing to panel)
		if m.aiPanel != nil && m.aiPanel.sub == aiResult {
			switch msg.String() {
			case "S":
				if m.tts != nil {
					m.tts.Stop()
				}
				m.ttsSpeaking = false
				m.ttsPaused = false
				m.ttsWordIndex = -1
				m.ttsGen++ // invalidate queued ticks
				m.setAIContent(-1)
				return m, nil
			case " ":
				return m.toggleTTSPause()
			}
		}
		if m.aiPanel != nil {
			var cmd tea.Cmd
			m.aiPanel, cmd = m.aiPanel.Update(msg)
			return m, cmd
		}
	}

	return m.updateActiveComponent(msg)
}

// setReaderContent renders the current chapter with word-wrapping and sets it
// on the viewport. wordIdx = -1 means no TTS highlighting.
func (m *Model) setReaderContent(wordIdx int) {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	raw := renderChapterContent(m.currentChapter, wordIdx, m.styles, m.width)
	m.viewport.SetContent(wordwrap.String(raw, w))
}

// setAIContent renders the AI panel's streamed content with optional TTS word
// highlighting. wordIdx = -1 means no highlighting.
//
// When TTS is active (wordIdx >= 0) we render m.ttsCleanText (the markdown-
// stripped text that was actually synthesised) so that word indices from
// m.ttsWords align exactly with what's on screen. When TTS is off we render
// the raw streamed text so formatting / markdown shows through normally.
func (m *Model) setAIContent(wordIdx int) {
	if m.aiPanel == nil {
		return
	}
	w := m.aiPanel.vp.Width
	if w <= 10 {
		w = 80
	}
	if wordIdx < 0 {
		// TTS off — show formatted raw text
		text := m.aiPanel.streamed.String()
		if text == "" {
			return
		}
		m.aiPanel.vp.SetContent(wordwrap.String(text, w))
		return
	}
	// TTS active — render the same clean text that was synthesised so that
	// ttsWordIndex maps to exactly the right word on screen.
	text := m.ttsCleanText
	if text == "" {
		return
	}
	raw := wordwrap.String(text, w)
	wordCounter := 0
	var sb strings.Builder
	for i, line := range strings.Split(raw, "\n") {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(renderWords(line, &wordCounter, wordIdx, m.styles))
	}
	m.aiPanel.vp.SetContent(sb.String())
}

// toggleTTSPause pauses or resumes TTS playback and the word-advance ticker.
// Works even if m.ttsSpeaking was prematurely cleared by a stale tick.
func (m Model) toggleTTSPause() (Model, tea.Cmd) {
	if m.tts == nil {
		return m, nil
	}
	if m.ttsPaused || m.tts.IsPaused() {
		m.tts.Resume()
		m.ttsPaused = false
		if m.ttsSpeaking {
			// Restart the word-advance ticker from the current position
			return m, coretts.SyncedWordTickCmd(m.ttsDurations, m.ttsWordIndex+1, m.ttsGen)
		}
		return m, nil
	}
	if m.ttsSpeaking || m.tts.IsPlaying() {
		m.tts.Pause()
		m.ttsPaused = true
	}
	return m, nil
}

// wordIdxAtClick maps a terminal click position (inside the reader viewport)
// to the nearest word index in m.ttsWords.  It simulates the same word-wrap
// that setReaderContent uses so the mapping is exact for ASCII text.
func (m *Model) wordIdxAtClick(clickX, clickY int) int {
	const headerLines = 1 // renderHeader produces one line
	vpLine := m.viewport.YOffset + (clickY - headerLines)
	if vpLine < 0 {
		vpLine = 0
	}
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	lineIdx := 0
	lineLen := 0
	for i, word := range m.ttsWords {
		wl := len(word)
		if lineLen > 0 && lineLen+1+wl > w {
			lineIdx++
			lineLen = 0
		}
		if lineIdx > vpLine {
			return i
		}
		if lineIdx == vpLine && lineLen+wl >= clickX {
			return i
		}
		if lineLen > 0 {
			lineLen++
		}
		lineLen += wl
	}
	return len(m.ttsWords) - 1
}

// jumpToWord stops the current TTS session and re-synthesises from wordIdx.
func (m Model) jumpToWord(wordIdx int) (Model, tea.Cmd) {
	if m.tts == nil || !m.tts.Available() || wordIdx < 0 || wordIdx >= len(m.ttsWords) {
		return m, nil
	}
	m.tts.Stop()
	m.ttsPaused = false
	m.ttsGen++ // invalidate any in-flight ticks from the old session
	remainingWords := m.ttsWords[wordIdx:]
	remainingText := strings.Join(remainingWords, " ")
	if synced, err := m.tts.SpeakSynced(remainingText, remainingWords); err == nil {
		m.ttsSpeaking = true
		m.ttsWordIndex = wordIdx
		m.ttsDurations = synced.WordDurations
		if m.state == StateReader {
			m.setReaderContent(wordIdx)
		} else if m.state == StateAI {
			m.setAIContent(wordIdx)
		}
		return m, coretts.WaitForTTSStart(synced.Started, m.ttsGen)
	}
	return m, nil
}

func (m Model) openSearch() (Model, tea.Cmd) {
	m.inSearch = true
	m.searchInput.SetValue("")
	m.searchInput.Focus()
	return m, textinput.Blink
}

func (m Model) openVoicePicker() (Model, tea.Cmd) {
	var voices []coretts.VoiceEntry
	if m.tts != nil {
		voices = m.tts.ListVoices()
	}
	items := make([]list.Item, len(voices))
	for i, v := range voices {
		items[i] = voiceItem{v}
	}
	m.voiceList = newStyledList(items, "✝  Select Voice  (enter to apply)", m.width, m.contentHeight())
	// Pre-select current voice
	if m.tts != nil {
		cur := m.tts.ActiveVoice()
		for idx, v := range voices {
			if v.ID == cur.ID {
				m.voiceList.Select(idx)
				break
			}
		}
	}
	m.prevState = m.state
	m.state = StateVoicePicker
	return m, nil
}

func (m Model) openLanguagePicker() (Model, tea.Cmd) {
	options := biblemeta.CommonLanguageOptions()
	items := make([]list.Item, len(options))
	for i, option := range options {
		items[i] = languageItem{code: option.Code, name: option.Name}
	}
	m.languageList = newStyledList(items, "✝  Filter Translation Language", m.width, m.contentHeight())
	for idx, option := range options {
		if option.Code == m.languageFilter {
			m.languageList.Select(idx)
			break
		}
	}
	m.prevState = m.state
	m.state = StateLanguagePicker
	return m, nil
}

func (m Model) openAIPanel() (Model, tea.Cmd) {
	if m.aiPanel == nil {
		m.aiPanel = NewAIPanel(m.localDB, m.aiClient)
		m.aiPanel.SetSize(m.width, m.contentHeight())
		m.aiPanel.SetTTSEngine(m.tts)
	}
	m.aiPanel.Reset()

	// Pass current passage context
	verseRef := m.currentChapter.Reference
	verseText := ""
	// extract first verse as the "current verse" context
	if m.currentChapter.Content != "" {
		// find first [N]text pattern
		re := regexp.MustCompile(`\[(\d+)\]([^\[]+)`)
		if match := re.FindStringSubmatch(m.currentChapter.Content); len(match) >= 3 {
			verseRef = m.currentChapter.Reference + ":" + match[1]
			verseText = strings.TrimSpace(match[2])
		}
	}
	trans := m.selectedBible.Abbreviation
	if trans == "" {
		trans = m.selectedBible.ID
	}
	m.aiPanel.SetContext(
		verseRef,
		verseText,
		m.currentChapter.Content,
		m.selectedBook.Name,
		m.currentChapter.Number,
		trans,
	)

	m.prevState = m.state
	m.state = StateAI
	return m, m.aiPanel.Init()
}

func (m Model) languageFilterLabel() string {
	if strings.TrimSpace(m.languageFilter) == "" {
		return "All languages"
	}
	return biblemeta.DisplayLanguageName(m.languageFilter)
}

func (m Model) updateActiveComponent(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case StateBibles:
		m.bibleList, cmd = m.bibleList.Update(msg)
	case StateBooks:
		m.bookList, cmd = m.bookList.Update(msg)
	case StateChapters:
		m.chapterList, cmd = m.chapterList.Update(msg)
	case StateReader:
		m.viewport, cmd = m.viewport.Update(msg)
	case StateSearch:
		m.searchList, cmd = m.searchList.Update(msg)
	case StateVoicePicker:
		m.voiceList, cmd = m.voiceList.Update(msg)
	case StateLanguagePicker:
		m.languageList, cmd = m.languageList.Update(msg)
		// StateImport is handled directly in the key handler above
	}
	return m, cmd
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	h := m.contentHeight()

	var body string
	switch m.state {
	case StateLoading:
		body = lipgloss.Place(m.width, h, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+"  Loading…")
	case StateBibles:
		m.bibleList.SetSize(m.width, h)
		body = m.bibleList.View()
	case StateBooks:
		m.bookList.SetSize(m.width, h)
		body = m.bookList.View()
	case StateChapters:
		m.chapterList.SetSize(m.width, h)
		body = m.chapterList.View()
	case StateReader:
		m.viewport.Width = m.width
		m.viewport.Height = h
		body = m.viewport.View()
	case StateSearch:
		m.searchList.SetSize(m.width, h)
		body = m.searchList.View()
	case StateVoicePicker:
		m.voiceList.SetSize(m.width, h)
		body = m.voiceList.View()
	case StateLanguagePicker:
		m.languageList.SetSize(m.width, h)
		body = m.languageList.View()
	case StateImport:
		if m.importPanel != nil {
			m.importPanel.SetSize(m.width, h)
			body = m.importPanel.View()
		}
	case StateAI:
		if m.aiPanel != nil {
			m.aiPanel.SetSize(m.width, h)
			body = m.aiPanel.View()
		}
	}

	if m.err != nil {
		errMsg := m.styles.Error.Render("✗  " + m.err.Error())
		body = lipgloss.Place(m.width, h, lipgloss.Center, lipgloss.Center, errMsg)
	}

	if m.inSearch {
		overlay := m.styles.SearchBox.Render("  / " + m.searchInput.View() + "  ")
		centered := lipgloss.Place(m.width, 3, lipgloss.Center, lipgloss.Center, overlay)
		return lipgloss.JoinVertical(lipgloss.Left, header, body, centered, footer)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) contentHeight() int {
	h := m.height - 2
	if h < 1 {
		return 1
	}
	return h
}

func (m Model) renderHeader() string {
	title := m.styles.HeaderTitle.Render("✝  LOGOS AI")

	var crumbs []string
	if m.selectedBible.Abbreviation == "" && m.languageFilterLabel() != "" {
		crumbs = append(crumbs, m.languageFilterLabel())
	}
	if m.selectedBible.Abbreviation != "" {
		crumbs = append(crumbs, stripEngPrefix(m.selectedBible.Abbreviation))
	}
	if m.selectedBook.Name != "" {
		crumbs = append(crumbs, m.selectedBook.Name)
	}
	if m.currentChapter.Number != "" && (m.state == StateReader || m.state == StateVoicePicker) {
		crumbs = append(crumbs, "Ch. "+m.currentChapter.Number)
	}
	crumb := ""
	if len(crumbs) > 0 {
		crumb = m.styles.HeaderCrumb.Render("  ›  " + strings.Join(crumbs, " › "))
	}

	ttsStatus := ""
	if m.tts != nil {
		voice := m.tts.ActiveVoice()
		label := shortVoiceLabel(voice)
		if m.ttsSpeaking && m.ttsPaused {
			ttsStatus = m.styles.TTSOn.Render("  ⏸ " + label)
		} else if m.ttsSpeaking {
			ttsStatus = m.styles.TTSOn.Render("  🔊 " + label)
		} else if m.tts.Available() {
			ttsStatus = m.styles.TTSOff.Render("  🔇 " + label)
		}
	}

	left := title + crumb
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(ttsStatus)
	if gap < 0 {
		gap = 0
	}
	row := left + strings.Repeat(" ", gap) + ttsStatus

	return m.styles.Header.Width(m.width).Render(row)
}

func (m Model) renderFooter() string {
	var hints string
	switch m.state {
	case StateLoading:
		hints = "loading…"
	case StateBibles:
		hints = "↑↓ navigate  •  enter select  •  f language  •  i import  •  / search  •  q quit"
	case StateBooks:
		hints = "↑↓ navigate  •  enter select  •  f language  •  esc back  •  / search  •  q quit"
	case StateChapters:
		hints = "↑↓ navigate  •  enter select  •  f language  •  esc back  •  / search  •  q quit"
	case StateReader:
		if m.ttsSpeaking && m.ttsPaused {
			hints = "↑↓ scroll  •  space resume  •  S stop  •  click jump  •  a AI  •  v voice  •  n/[ ch  •  esc back  •  q quit"
		} else if m.ttsSpeaking {
			hints = "↑↓ scroll  •  space pause  •  S stop  •  click jump  •  a AI  •  v voice  •  n/[ ch  •  esc back  •  q quit"
		} else {
			hints = "↑↓ scroll  •  s speak  •  a AI  •  v voice  •  n/[ ch  •  / search  •  esc back  •  q quit"
		}
	case StateSearch:
		hints = "↑↓ navigate  •  enter open  •  / new search  •  esc back  •  q quit"
	case StateVoicePicker:
		hints = "↑↓ navigate  •  enter select voice  •  esc cancel"
	case StateLanguagePicker:
		hints = "↑↓ navigate  •  enter choose language  •  esc cancel"
	case StateImport:
		if m.importPanel != nil {
			hints = m.importPanel.Hints()
		}
	case StateAI:
		if m.aiPanel != nil {
			hints = m.aiPanel.Hints()
		}
	}
	// Show precache/transient status when available
	if m.statusMsg != "" {
		hints = m.statusMsg
	}
	return m.styles.Footer.Width(m.width).Render(hints)
}

// ── Chapter rendering ─────────────────────────────────────────────────────────

var verseTagRe = regexp.MustCompile(`\[(\d+)\]`)

func renderChapterContent(ch api.ChapterContent, wordIdx int, styles *Styles, width int) string {
	var sb strings.Builder
	sb.WriteString(styles.HeaderTitle.Render(ch.Reference))
	sb.WriteString("\n\n")

	wordCounter := 0
	for _, para := range strings.Split(ch.Content, "\n") {
		para = strings.TrimSpace(strings.ReplaceAll(para, "¶", ""))
		if para == "" {
			continue
		}
		sb.WriteString(renderParagraph(para, &wordCounter, wordIdx, styles))
		sb.WriteString("\n\n")
	}

	if ch.Copyright != "" {
		sb.WriteString("\n")
		sb.WriteString(styles.Copyright.Render("© " + ch.Copyright))
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderParagraph(para string, wordCounter *int, wordIdx int, styles *Styles) string {
	var sb strings.Builder
	remaining := para
	for len(remaining) > 0 {
		loc := verseTagRe.FindStringIndex(remaining)
		if loc == nil {
			sb.WriteString(renderWords(remaining, wordCounter, wordIdx, styles))
			break
		}
		if loc[0] > 0 {
			sb.WriteString(renderWords(remaining[:loc[0]], wordCounter, wordIdx, styles))
		}
		num := verseTagRe.FindStringSubmatch(remaining[loc[0]:loc[1]])[1]
		sb.WriteString(styles.VerseNum.Render("[" + num + "]"))
		sb.WriteString(" ")
		remaining = remaining[loc[1]:]
	}
	return sb.String()
}

func renderWords(text string, wordCounter *int, wordIdx int, styles *Styles) string {
	var sb strings.Builder
	fields := strings.FieldsFunc(text, func(r rune) bool { return unicode.IsSpace(r) })
	for i, word := range fields {
		if i > 0 {
			sb.WriteString(" ")
		}
		if wordIdx >= 0 && *wordCounter == wordIdx {
			sb.WriteString(styles.WordHL.Render(word))
		} else {
			sb.WriteString(styles.VerseText.Render(word))
		}
		*wordCounter++
	}
	return sb.String()
}

// ── List factory ──────────────────────────────────────────────────────────────

func newStyledList(items []list.Item, title string, width, height int) list.Model {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(ColorGold).Background(ColorHL).Bold(true).
		Padding(0, 0, 0, 2).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(ColorGold)
	d.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(ColorAccent).Background(ColorHL).Padding(0, 0, 0, 3)
	d.Styles.NormalTitle = lipgloss.NewStyle().Foreground(ColorText).Padding(0, 0, 0, 2)
	d.Styles.NormalDesc = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 0, 0, 2)

	l := list.New(items, d, width, height)
	l.Title = title
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(ColorGold).Background(ColorHeaderBg).Bold(true).Padding(0, 1)
	l.Styles.TitleBar = lipgloss.NewStyle().
		Background(ColorHeaderBg).Padding(0, 0, 1, 0)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(ColorPurple)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(ColorGold)
	return l
}

// Run starts the BubbleTea program with the given API client and TTS engine.
func Run(client *api.Client, engine *coretts.Engine) error {
	m := NewModel(client, engine, "")
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}

func (m Model) loadOfflineChapter(chapterID string) (api.ChapterContent, error) {
	if m.localDB == nil {
		return api.ChapterContent{}, fmt.Errorf("local bible database unavailable")
	}

	bookID, _, found := strings.Cut(chapterID, ".")
	if !found || bookID == "" {
		bookID = m.selectedBook.ID
	}

	bookName := m.selectedBook.Name
	if books, err := m.localDB.ListBooks(m.selectedBible.ID); err == nil {
		for _, b := range books {
			if b.ID == bookID {
				bookName = b.Name
				break
			}
		}
	}

	content, err := m.localDB.GetChapterContent(chapterID, m.selectedBible.ID)
	if err != nil {
		return api.ChapterContent{}, err
	}

	chapters, err := m.localDB.ListChapters(bookID, m.selectedBible.ID)
	if err != nil {
		return api.ChapterContent{}, err
	}

	var (
		number string
		prev   *api.ChapterRef
		next   *api.ChapterRef
	)
	for i, ch := range chapters {
		if ch.ID != chapterID {
			continue
		}
		number = strconv.Itoa(ch.Number)
		if i > 0 {
			prev = &api.ChapterRef{
				ID:     chapters[i-1].ID,
				Number: strconv.Itoa(chapters[i-1].Number),
				BookID: bookID,
			}
		}
		if i+1 < len(chapters) {
			next = &api.ChapterRef{
				ID:     chapters[i+1].ID,
				Number: strconv.Itoa(chapters[i+1].Number),
				BookID: bookID,
			}
		}
		break
	}
	if number == "" {
		return api.ChapterContent{}, fmt.Errorf("local chapter not found: %s", chapterID)
	}

	return api.ChapterContent{
		ID:         chapterID,
		BibleID:    m.selectedBible.ID,
		BookID:     bookID,
		Number:     number,
		Reference:  fmt.Sprintf("%s %s", bookName, number),
		Content:    strings.TrimSpace(content),
		VerseCount: strings.Count(content, "["),
		Copyright:  fmt.Sprintf("%s (offline import)", m.selectedBible.Name),
		Next:       next,
		Previous:   prev,
	}, nil
}

// stripLangPrefix removes any leading ISO-639-3 or ISO-639-1 language code from
// a Bible abbreviation. Handles 3-letter codes (eng, spa, fra, deu, por, zho,
// hin, ara, rus, kor, jpn, vie, ind, nld, ita, pol, tur, heb, grc) and 2-letter
// codes (en, es, fr, de, pt). Examples:
//
//	"engKJV"  → "KJV"
//	"spaRVR"  → "RVR"
//	"espBLA"  → "BLA"  (esp treated same as spa)
//	"KJV"     → "KJV"  (unchanged)
//	"NLV"     → "NLV"  (not touched — result would be single char)
func stripLangPrefix(abbr string) string {
	return biblemeta.StripLangPrefix(abbr)
}

// stripEngPrefix is kept for backwards compat; delegates to stripLangPrefix.
func stripEngPrefix(abbr string) string { return stripLangPrefix(abbr) }

// displayLanguageName maps ISO-639-3/1 codes to English display names.
func displayLanguageName(code string) string {
	return biblemeta.DisplayLanguageName(code)
}

func formatLocalReference(verseID string) string {
	parts := strings.Split(verseID, ".")
	if len(parts) != 3 {
		return verseID
	}
	return fmt.Sprintf("%s %s:%s", parts[0], parts[1], parts[2])
}

// shortVoiceLabel produces a compact label for the header bar.
// "Piper: en_US-lessac-medium" → "Piper·lessac"
// "Kokoro: Michael (US Male)"  → "Kokoro·Michael"
// "Say: Samantha"              → "Say·Samantha"
func shortVoiceLabel(v coretts.VoiceEntry) string {
	if v.Name == "" {
		return ttsDisplayName(v.Engine)
	}
	// Strip engine prefix ("Piper: ", "Kokoro: ", "Say: ")
	name := v.Name
	if idx := strings.Index(name, ": "); idx >= 0 {
		name = name[idx+2:]
	}
	// For piper model names like "en_US-lessac-medium", keep only last segment
	if v.Engine == "piper" {
		parts := strings.Split(name, "-")
		if len(parts) >= 2 {
			name = strings.Join(parts[1:], "-")
		}
	}
	// For kokoro, keep only first word of display name
	if v.Engine == "kokoro" {
		if idx := strings.Index(name, " "); idx > 0 {
			name = name[:idx]
		}
	}
	engine := ttsDisplayName(v.Engine)
	return engine + "·" + name
}

func ttsDisplayName(engine string) string {
	switch engine {
	case "speechd":
		return "Speech Dispatcher"
	case "espeak":
		return "eSpeak"
	default:
		return strings.Title(engine) //nolint:staticcheck
	}
}
