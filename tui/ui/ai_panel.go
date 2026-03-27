package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jd4rider/logos/internal/ai"
	localdb "github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/pdf"
	coretts "github.com/jd4rider/logos/internal/tts"
)

// ── Sub-states ────────────────────────────────────────────────────────────────

type aiSubState int

const (
	aiMenu      aiSubState = iota
	aiTyping               // user entering title / topic / question
	aiStreaming            // receiving tokens from Ollama
	aiResult               // response complete
	aiSaving               // showing save/export confirmation
)

type aiAction int

const (
	aiActionExplainVerse aiAction = iota
	aiActionExplainChapter
	aiActionDevotional
	aiActionSermon
	aiActionStudyPlan
	aiActionAsk
	aiActionLibrary
)

var aiMenuItems = []struct {
	action aiAction
	label  string
	hint   string
}{
	{aiActionExplainVerse, "✦  Explain This Verse", "AI commentary on the current verse"},
	{aiActionExplainChapter, "✦  Explain This Chapter", "Overview and context for the chapter"},
	{aiActionDevotional, "✦  Generate Devotional", "Daily devotional from this passage"},
	{aiActionSermon, "✦  Generate Sermon", "Full sermon based on this passage"},
	{aiActionStudyPlan, "✦  Create Study Plan", "Multi-week Bible study plan"},
	{aiActionAsk, "✦  Ask a Question", "Ask anything about this passage"},
	{aiActionLibrary, "✦  My Library", "Browse saved devotionals, sermons & notes"},
}

// ── Messages ──────────────────────────────────────────────────────────────────

type aiTokenMsg struct{ token string }
type aiDoneMsg struct{ err error }
type aiSavedMsg struct {
	path string
	err  error
}
type libraryLoadedMsg struct{ entries []libraryEntry }

type libraryEntry struct {
	kind    string // "devotional" | "sermon" | "note"
	id      int64
	title   string
	ref     string
	content string
	model   string
	date    time.Time
}

// ── AIPanel ───────────────────────────────────────────────────────────────────

type AIPanel struct {
	sub     aiSubState
	action  aiAction
	menuIdx int

	// passage context
	verseRef    string
	verseText   string
	chapterText string
	bookName    string
	chapterNum  string
	translation string

	// input widget (title, topic, question)
	input     textarea.Model
	inputHint string

	// streaming state
	streamed strings.Builder
	tokenCh  chan string // goroutine → BubbleTea
	cancel   context.CancelFunc

	// background TTS precache (runs while/after AI generation)
	ttsEngine      *coretts.Engine
	precacheCancel context.CancelFunc

	vp   viewport.Model
	spin spinner.Model

	// library
	libraryItems []libraryEntry
	libraryIdx   int

	db       *localdb.DB
	aiClient *ai.Client
	width    int
	height   int
	err      error
	saved    string
}

// NewAIPanel creates a new AIPanel.
func NewAIPanel(db *localdb.DB, aiClient *ai.Client) *AIPanel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	ta := textarea.New()
	ta.Placeholder = "Type here…"
	ta.ShowLineNumbers = false
	ta.CharLimit = 400

	vp := viewport.New(80, 20)

	return &AIPanel{
		db:       db,
		aiClient: aiClient,
		spin:     sp,
		input:    ta,
		vp:       vp,
	}
}

func (p *AIPanel) SetContext(verseRef, verseText, chapterText, bookName, chapterNum, translation string) {
	p.verseRef = verseRef
	p.verseText = verseText
	p.chapterText = chapterText
	p.bookName = bookName
	p.chapterNum = chapterNum
	p.translation = translation
}

func (p *AIPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.vp.Width = w - 4
	p.vp.Height = h - 12
	p.input.SetWidth(w - 8)
}

// SetTTSEngine attaches the TTS engine so the panel can pre-synthesise audio
// in the background while (or after) AI content is generated.
func (p *AIPanel) SetTTSEngine(engine *coretts.Engine) {
	p.ttsEngine = engine
}

func (p *AIPanel) Reset() {
	p.sub = aiMenu
	p.menuIdx = 0
	p.streamed.Reset()
	p.err = nil
	p.saved = ""
	p.input.Reset()
	p.stopStream()
	p.cancelPrecache()
}

// cancelPrecache stops any in-progress background TTS pre-synthesis.
func (p *AIPanel) cancelPrecache() {
	if p.precacheCancel != nil {
		p.precacheCancel()
		p.precacheCancel = nil
	}
}

// startPrecache launches a background goroutine that synthesises TTS audio for
// the current streamed content and stores it in the engine's disk cache.
// When the user later presses "s" to read aloud, SpeakSynced will find the
// audio already cached and begin playing instantly.
func (p *AIPanel) startPrecache() {
	p.cancelPrecache() // cancel any previous run

	engine := p.ttsEngine
	if engine == nil || !engine.Available() {
		return
	}
	text := strings.TrimSpace(p.streamed.String())
	if text == "" {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.precacheCancel = cancel

	go func() {
		defer cancel()
		clean := coretts.CleanForTTS(text)
		words := coretts.SplitWords(clean)
		if len(words) == 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		// Precache returns immediately if already cached, so this is a no-op
		// on the happy path where the user presses "s" quickly after generation.
		_ = engine.Precache(clean, words)
	}()
}

func (p *AIPanel) stopStream() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	if p.tokenCh != nil {
		// drain
		for range p.tokenCh {
		}
		p.tokenCh = nil
	}
	p.cancelPrecache()
}

func (p *AIPanel) Init() tea.Cmd {
	return p.spin.Tick
}

func (p *AIPanel) Hints() string {
	switch p.sub {
	case aiMenu:
		return "↑↓ navigate  •  enter select  •  esc back"
	case aiTyping:
		return "ctrl+s submit  •  esc cancel"
	case aiStreaming:
		return "esc stop generation"
	case aiResult:
		return "↑↓ scroll  •  s read aloud  •  space pause  •  S stop  •  enter save  •  e export PDF  •  esc back"
	case aiSaving:
		return "any key to continue"
	}
	return ""
}

// ── Update ────────────────────────────────────────────────────────────────────

func (p *AIPanel) Update(msg tea.Msg) (*AIPanel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		p.spin, cmd = p.spin.Update(msg)
		cmds = append(cmds, cmd)

	case aiTokenMsg:
		p.streamed.WriteString(msg.token)
		p.vp.SetContent(wrapText(p.streamed.String(), p.vp.Width))
		p.vp.GotoBottom()
		// schedule next read
		cmds = append(cmds, p.readNextToken())

	case aiDoneMsg:
		p.sub = aiResult
		if msg.err != nil {
			p.err = msg.err
		}
		p.vp.SetContent(wrapText(p.streamed.String(), p.vp.Width))
		// Kick off background TTS pre-synthesis so audio is cached and ready
		// instantly when the user presses "s" to read aloud.
		if msg.err == nil {
			p.startPrecache()
		}

	case aiSavedMsg:
		p.sub = aiSaving
		if msg.err != nil {
			p.err = msg.err
		} else {
			p.saved = msg.path
		}

	case libraryLoadedMsg:
		p.libraryItems = msg.entries
		p.libraryIdx = 0
		p.sub = aiResult
		p.renderLibrary()
		// Precache the first entry so "s" is instant
		if len(msg.entries) > 0 && p.ttsEngine != nil && p.ttsEngine.Available() {
			first := msg.entries[0]
			go func() {
				clean := coretts.CleanForTTS(first.content)
				words := coretts.SplitWords(clean)
				_ = p.ttsEngine.Precache(clean, words)
			}()
		}

	case tea.KeyMsg:
		switch p.sub {
		case aiMenu:
			return p.handleMenuKey(msg)
		case aiTyping:
			return p.handleTypingKey(msg)
		case aiStreaming:
			if msg.String() == "esc" {
				p.stopStream()
				p.sub = aiResult
			}
		case aiResult:
			return p.handleResultKey(msg)
		case aiSaving:
			p.sub = aiResult
		}
	}

	if p.sub == aiTyping {
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		cmds = append(cmds, cmd)
	}
	if p.sub == aiResult || p.sub == aiStreaming {
		var cmd tea.Cmd
		p.vp, cmd = p.vp.Update(msg)
		cmds = append(cmds, cmd)
	}

	return p, tea.Batch(cmds...)
}

func (p *AIPanel) handleMenuKey(msg tea.KeyMsg) (*AIPanel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if p.menuIdx > 0 {
			p.menuIdx--
		}
	case "down", "j":
		if p.menuIdx < len(aiMenuItems)-1 {
			p.menuIdx++
		}
	case "enter", "right", "l":
		return p.selectAction(aiMenuItems[p.menuIdx].action)
	}
	return p, nil
}

func (p *AIPanel) handleTypingKey(msg tea.KeyMsg) (*AIPanel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+s":
		return p.submitInput()
	case "esc":
		p.sub = aiMenu
		p.input.Reset()
	}
	return p, nil
}

func (p *AIPanel) handleResultKey(msg tea.KeyMsg) (*AIPanel, tea.Cmd) {
	// Library navigation mode
	if len(p.libraryItems) > 0 && p.streamed.Len() == 0 {
		switch msg.String() {
		case "esc", "backspace":
			p.libraryItems = nil
			p.sub = aiMenu
			return p, nil
		case "up", "k":
			if p.libraryIdx > 0 {
				p.libraryIdx--
				p.renderLibrary()
				// Precache selected entry in background for instant read
				if p.ttsEngine != nil && p.ttsEngine.Available() {
					entry := p.libraryItems[p.libraryIdx]
					clean := coretts.CleanForTTS(entry.content)
					words := coretts.SplitWords(clean)
					go func() { _ = p.ttsEngine.Precache(clean, words) }()
				}
			}
			return p, nil
		case "down", "j":
			if p.libraryIdx < len(p.libraryItems)-1 {
				p.libraryIdx++
				p.renderLibrary()
				// Precache selected entry in background for instant read
				if p.ttsEngine != nil && p.ttsEngine.Available() {
					entry := p.libraryItems[p.libraryIdx]
					clean := coretts.CleanForTTS(entry.content)
					words := coretts.SplitWords(clean)
					go func() { _ = p.ttsEngine.Precache(clean, words) }()
				}
			}
			return p, nil
		case "enter":
			// Open entry: load its content into the streamed viewport
			entry := p.libraryItems[p.libraryIdx]
			p.libraryItems = nil
			p.streamed.Reset()
			p.streamed.WriteString(entry.content)
			p.vp.SetContent(wrapText(entry.content, p.vp.Width))
			return p, nil
		case "s":
			// Read selected library entry aloud
			entry := p.libraryItems[p.libraryIdx]
			text := strings.TrimSpace(entry.content)
			if text != "" {
				return p, func() tea.Msg { return aiReadAloudMsg{text: text} }
			}
		}
		return p, nil
	}

	switch msg.String() {
	case "esc", "backspace":
		p.sub = aiMenu
		p.streamed.Reset()
		p.err = nil
	case "enter":
		return p, p.saveToLibraryCmd()
	case "e":
		return p, p.exportPDFCmd()
	case "s":
		// Start TTS reading of the generated content
		text := strings.TrimSpace(p.streamed.String())
		if text != "" {
			return p, func() tea.Msg { return aiReadAloudMsg{text: text} }
		}
	}
	return p, nil
}

func (p *AIPanel) selectAction(action aiAction) (*AIPanel, tea.Cmd) {
	p.action = action
	p.streamed.Reset()
	p.err = nil

	switch action {
	case aiActionExplainVerse, aiActionExplainChapter:
		return p, p.beginStream("")
	case aiActionAsk:
		p.inputHint = "Ask your question about this passage:"
		p.input.Placeholder = "e.g. What does this mean for daily life?"
		p.sub = aiTyping
		p.input.Focus()
		return p, textarea.Blink
	case aiActionDevotional:
		p.inputHint = "Optional theme (leave blank for default) — Ctrl+S to generate:"
		p.input.Placeholder = "e.g. Faith, Courage, Hope"
		p.sub = aiTyping
		p.input.Focus()
		return p, textarea.Blink
	case aiActionSermon:
		p.inputHint = "Sermon title — Ctrl+S to generate:"
		p.input.Placeholder = "e.g. Walking in the Light"
		p.sub = aiTyping
		p.input.Focus()
		return p, textarea.Blink
	case aiActionStudyPlan:
		p.inputHint = "Topic and weeks, e.g. 'Grace, 4 weeks' — Ctrl+S to generate:"
		p.input.Placeholder = "e.g. Prayer and Fasting, 6 weeks"
		p.sub = aiTyping
		p.input.Focus()
		return p, textarea.Blink
	case aiActionLibrary:
		p.sub = aiStreaming
		return p, p.loadLibraryCmd()
	}
	return p, nil
}

func (p *AIPanel) submitInput() (*AIPanel, tea.Cmd) {
	userInput := strings.TrimSpace(p.input.Value())
	p.input.Reset()
	return p, p.beginStream(userInput)
}

// ── Streaming ─────────────────────────────────────────────────────────────────

// beginStream starts an Ollama generation in a background goroutine,
// feeding tokens into p.tokenCh, then returns the first readNextToken cmd.
func (p *AIPanel) beginStream(userInput string) tea.Cmd {
	p.sub = aiStreaming
	p.streamed.Reset()
	p.stopStream()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	p.cancel = cancel
	p.tokenCh = make(chan string, 128)

	vc := ai.VerseContext{
		Reference:   p.verseRef,
		Text:        p.verseText,
		Translation: p.translation,
	}

	tokenCh := p.tokenCh
	action := p.action
	aiCl := p.aiClient
	bookName := p.bookName
	chapterNum := p.chapterNum
	chapterText := p.chapterText
	translation := p.translation

	go func() {
		defer close(tokenCh)

		var tokens <-chan string
		var errc <-chan error

		switch action {
		case aiActionExplainVerse:
			tokens, errc = aiCl.ExplainVerse(ctx, vc)
		case aiActionExplainChapter:
			tokens, errc = aiCl.ExplainChapter(ctx, bookName, chapterNum, translation, chapterText)
		case aiActionDevotional:
			tokens, errc = aiCl.GenerateDevotional(ctx, vc, userInput)
		case aiActionSermon:
			tokens, errc = aiCl.GenerateSermon(ctx, vc, userInput)
		case aiActionStudyPlan:
			topic, weeksStr := parseTopicWeeks(userInput)
			weeks := 4
			fmt.Sscanf(weeksStr, "%d", &weeks)
			plan, err := aiCl.GenerateStudyPlan(ctx, topic, weeks, translation)
			if err != nil {
				tokenCh <- "\n[Error: " + err.Error() + "]"
				return
			}
			tokenCh <- renderStudyPlanText(plan)
			return
		case aiActionAsk:
			tokens, errc = aiCl.AskAboutVerse(ctx, userInput, vc)
		default:
			return
		}

		for t := range tokens {
			select {
			case tokenCh <- t:
			case <-ctx.Done():
				return
			}
		}
		if errc != nil {
			if err := <-errc; err != nil && err != context.Canceled {
				tokenCh <- "\n[Error: " + err.Error() + "]"
			}
		}
	}()

	return p.readNextToken()
}

// readNextToken returns a Cmd that reads one token from the channel.
func (p *AIPanel) readNextToken() tea.Cmd {
	ch := p.tokenCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		token, ok := <-ch
		if !ok {
			return aiDoneMsg{}
		}
		return aiTokenMsg{token: token}
	}
}

// ── Save / Export ─────────────────────────────────────────────────────────────

func (p *AIPanel) saveToLibraryCmd() tea.Cmd {
	content := p.streamed.String()
	db := p.db
	action := p.action
	vRef := p.verseRef
	trans := p.translation
	model := ""
	if p.aiClient != nil {
		model = p.aiClient.Model()
	}
	return func() tea.Msg {
		now := time.Now()
		switch action {
		case aiActionDevotional:
			title := extractFirstLine(content)
			_, err := db.SaveDevotional(localdb.Devotional{
				Title: title, VerseRef: vRef, Content: content,
				AIModel: model, CreatedAt: now,
			})
			return aiSavedMsg{err: err}
		case aiActionSermon:
			title := extractFirstLine(content)
			_, err := db.SaveSermon(localdb.Sermon{
				Title: title, ScriptureRef: vRef, Content: content,
				AIModel: model, CreatedAt: now,
			})
			return aiSavedMsg{err: err}
		case aiActionExplainVerse, aiActionAsk, aiActionExplainChapter:
			_, err := db.SaveNote(localdb.AINote{
				VerseID: vRef, TranslationID: trans, Content: content,
				AIModel: model, CreatedAt: now,
			})
			return aiSavedMsg{err: err}
		}
		return aiSavedMsg{err: fmt.Errorf("saving not supported for this type")}
	}
}

func (p *AIPanel) exportPDFCmd() tea.Cmd {
	content := p.streamed.String()
	action := p.action
	vRef := p.verseRef
	home, _ := os.UserHomeDir()
	outDir := filepath.Join(home, "Desktop")
	_ = os.MkdirAll(outDir, 0o755)

	return func() tea.Msg {
		ts := time.Now().Format("20060102-150405")
		title := extractFirstLine(content)
		var outPath string
		var err error

		switch action {
		case aiActionDevotional:
			outPath = filepath.Join(outDir, "logos-devotional-"+ts+".pdf")
			err = pdf.ExportDevotional(title, vRef, "", content, outPath)
		case aiActionSermon:
			outPath = filepath.Join(outDir, "logos-sermon-"+ts+".pdf")
			err = pdf.ExportSermon(title, vRef, content, outPath)
		case aiActionExplainVerse, aiActionAsk, aiActionExplainChapter:
			outPath = filepath.Join(outDir, "logos-note-"+ts+".pdf")
			err = pdf.ExportNote(vRef, content, outPath)
		default:
			return aiSavedMsg{err: fmt.Errorf("export not supported for this type")}
		}
		return aiSavedMsg{path: outPath, err: err}
	}
}

func (p *AIPanel) loadLibraryCmd() tea.Cmd {
	db := p.db
	return func() tea.Msg {
		var entries []libraryEntry
		if db == nil {
			return libraryLoadedMsg{entries: entries}
		}
		if devs, err := db.ListDevotionals(20); err == nil {
			for _, d := range devs {
				entries = append(entries, libraryEntry{
					kind: "devotional", id: d.ID, title: d.Title,
					ref: d.VerseRef, content: d.Content, model: d.AIModel, date: d.CreatedAt,
				})
			}
		}
		if serms, err := db.ListSermons(20); err == nil {
			for _, s := range serms {
				entries = append(entries, libraryEntry{
					kind: "sermon", id: s.ID, title: s.Title,
					ref: s.ScriptureRef, content: s.Content, model: s.AIModel, date: s.CreatedAt,
				})
			}
		}
		if notes, err := db.ListAllNotes(20); err == nil {
			for _, n := range notes {
				title := truncate(n.Content, 60)
				entries = append(entries, libraryEntry{
					kind: "note", id: n.ID, title: title,
					ref: n.VerseID, content: n.Content, model: n.AIModel, date: n.CreatedAt,
				})
			}
		}
		// Sort combined list by date descending
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].date.After(entries[j].date)
		})
		return libraryLoadedMsg{entries: entries}
	}
}

func (p *AIPanel) renderLibrary() {
	if len(p.libraryItems) == 0 {
		p.vp.SetContent("No saved content yet.\n\nGenerate devotionals, sermons, or notes and press Enter to save them.")
		return
	}
	var sb strings.Builder
	for i, e := range p.libraryItems {
		prefix := "  "
		if i == p.libraryIdx {
			prefix = "▶ "
		}
		sb.WriteString(fmt.Sprintf("%s[%s] %s\n", prefix, e.kind, e.title))
		sb.WriteString(fmt.Sprintf("   %s  •  %s\n\n", e.ref, e.date.Format("Jan 2, 2006")))
	}
	p.vp.SetContent(sb.String())
}

// ── View ──────────────────────────────────────────────────────────────────────

func (p *AIPanel) View() string {
	st := p.panelStyles()
	switch p.sub {
	case aiMenu:
		return p.viewMenu(st)
	case aiTyping:
		return p.viewTyping(st)
	case aiStreaming:
		return p.viewStreaming(st)
	case aiResult:
		return p.viewResult(st)
	case aiSaving:
		return p.viewSaving(st)
	}
	return ""
}

type panelStyleSet struct {
	title    lipgloss.Style
	selected lipgloss.Style
	normal   lipgloss.Style
	hint     lipgloss.Style
	gold     lipgloss.Style
	red      lipgloss.Style
}

func (p *AIPanel) panelStyles() panelStyleSet {
	return panelStyleSet{
		title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#D4AF37")).Padding(0, 1),
		selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#D4AF37")).Background(lipgloss.Color("#0F172A")).Padding(0, 2),
		normal:   lipgloss.NewStyle().Foreground(lipgloss.Color("#C9B8E8")).Padding(0, 2),
		hint:     lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true),
		gold:     lipgloss.NewStyle().Foreground(lipgloss.Color("#D4AF37")),
		red:      lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")),
	}
}

func (p *AIPanel) panelHeader(st panelStyleSet, subtitle string) string {
	title := st.title.Render("✝  Logos AI Assistant")
	sub := st.hint.Render(subtitle)
	ctx := ""
	if p.verseRef != "" {
		ctx = st.gold.Render("  Context: ") + st.hint.Render(p.verseRef)
	}
	if p.aiClient != nil {
		ctx += st.hint.Render("  •  model: " + p.aiClient.Model())
	}
	sep := strings.Repeat("─", max(p.width-4, 20))
	return lipgloss.JoinVertical(lipgloss.Left, title, sub, ctx, st.hint.Render(sep), "")
}

func (p *AIPanel) viewMenu(st panelStyleSet) string {
	rows := []string{p.panelHeader(st, "What would you like to do?")}
	for i, item := range aiMenuItems {
		if i == p.menuIdx {
			rows = append(rows, st.selected.Render("▶  "+item.label))
			rows = append(rows, st.hint.Render("     "+item.hint))
		} else {
			rows = append(rows, st.normal.Render("   "+item.label))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (p *AIPanel) viewTyping(st panelStyleSet) string {
	idx := p.menuIdx
	if idx >= len(aiMenuItems) {
		idx = 0
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		p.panelHeader(st, aiMenuItems[idx].hint),
		st.gold.Render(p.inputHint),
		"",
		p.input.View(),
	)
}

func (p *AIPanel) viewStreaming(st panelStyleSet) string {
	return lipgloss.JoinVertical(lipgloss.Left,
		p.panelHeader(st, p.spin.View()+" Generating…  Esc to stop"),
		p.vp.View(),
	)
}

func (p *AIPanel) viewResult(st panelStyleSet) string {
	if p.err != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			p.panelHeader(st, "Error"),
			st.red.Render(p.err.Error()),
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		p.panelHeader(st, aiMenuItems[p.menuIdx].label),
		p.vp.View(),
	)
}

func (p *AIPanel) viewSaving(st panelStyleSet) string {
	var msg string
	if p.err != nil {
		msg = st.red.Render("Error: " + p.err.Error())
	} else if p.saved != "" {
		msg = st.gold.Render("✓ Exported: ") + st.hint.Render(p.saved)
	} else {
		msg = st.gold.Render("✓ Saved to library!")
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		p.panelHeader(st, "Done"),
		"",
		msg,
	)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func wrapText(s string, width int) string {
	if width <= 10 {
		width = 80
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		for len(line) > width {
			out = append(out, line[:width])
			line = line[width:]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func extractFirstLine(content string) string {
	for _, l := range strings.Split(strings.TrimSpace(content), "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			if len(l) > 80 {
				l = l[:80] + "…"
			}
			return l
		}
	}
	return "Untitled"
}

func parseTopicWeeks(s string) (topic, weeks string) {
	parts := strings.SplitN(s, ",", 2)
	if len(parts) == 2 {
		topic = strings.TrimSpace(parts[0])
		w := strings.ToLower(strings.TrimSpace(parts[1]))
		w = strings.ReplaceAll(w, "weeks", "")
		w = strings.ReplaceAll(w, "week", "")
		return topic, strings.TrimSpace(w)
	}
	return strings.TrimSpace(s), "4"
}

func renderStudyPlanText(plan *ai.StudyPlan) string {
	if plan == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(plan.Title + "\n\n")
	sb.WriteString(plan.Description + "\n\n")
	for _, w := range plan.Weeks {
		sb.WriteString(fmt.Sprintf("━━ Week %d — %s ━━\n", w.Week, w.Theme))
		sb.WriteString("Reading: " + w.Reading + "\n")
		if len(w.Verses) > 0 {
			sb.WriteString("Key Verses: " + strings.Join(w.Verses, " • ") + "\n")
		}
		sb.WriteString(w.Notes + "\n\n")
	}
	return sb.String()
}

// truncate returns the first maxLen runes of s, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(strings.SplitN(s, "\n", 2)[0]) // first line only
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

// savePlanToLibrary saves a structured study plan to the database.
func savePlanToLibrary(db *localdb.DB, plan *ai.StudyPlan, model string) error {
	if db == nil || plan == nil {
		return nil
	}
	weeks := make([]localdb.StudyPlanWeekRecord, 0, len(plan.Weeks))
	for _, w := range plan.Weeks {
		vj, _ := json.Marshal(w.Verses)
		weeks = append(weeks, localdb.StudyPlanWeekRecord{
			WeekNumber: w.Week,
			Theme:      w.Theme,
			Reading:    w.Reading,
			VersesJSON: string(vj),
			Notes:      w.Notes,
		})
	}
	_, err := db.SaveStudyPlan(localdb.StudyPlanRecord{
		Title:       plan.Title,
		Description: plan.Description,
		Topic:       plan.Title,
		WeeksCount:  len(plan.Weeks),
		AIModel:     model,
		CreatedAt:   time.Now(),
	}, weeks)
	return err
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
