package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	localdb "github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/ai"
	"github.com/jd4rider/logos/internal/pdf"
)

// ── AI Panel state machine ────────────────────────────────────────────────────

type aiSubState int

const (
	aiMenu aiSubState = iota // top-level menu
	aiTyping                 // user is typing a question or title
	aiStreaming              // streaming response from Ollama
	aiResult                 // showing completed result
	aiSaving                 // showing "saved" confirmation
)

type aiAction int

const (
	aiActionExplainVerse aiAction = iota
	aiActionExplainChapter
	aiActionDevotional
	aiActionSermon
	aiActionStudyPlan
	aiActionAsk
	aiActionLibrary // browse saved content
)

var aiMenuItems = []struct {
	action aiAction
	label  string
	hint   string
}{
	{aiActionExplainVerse, "✦  Explain This Verse", "AI commentary on the selected verse"},
	{aiActionExplainChapter, "✦  Explain This Chapter", "Overview and context for the full chapter"},
	{aiActionDevotional, "✦  Generate Devotional", "Create a daily devotional from this verse"},
	{aiActionSermon, "✦  Generate Sermon", "Write a full sermon based on this verse"},
	{aiActionStudyPlan, "✦  Create Study Plan", "Build a multi-week study plan"},
	{aiActionAsk, "✦  Ask a Question", "Ask anything about this passage"},
	{aiActionLibrary, "✦  My Library", "Browse saved devotionals, sermons, notes"},
}

// ── Messages ─────────────────────────────────────────────────────────────────

type aiTokenMsg struct{ token string }
type aiDoneMsg struct{ err error }
type aiSavedMsg struct{ path string; err error }

// ── AIPanel ──────────────────────────────────────────────────────────────────

// AIPanel is the full AI assistant panel rendered in StateAI.
type AIPanel struct {
	sub       aiSubState
	action    aiAction
	menuIdx   int

	// context passed from reader
	verseRef    string
	verseText   string
	chapterText string
	bookName    string
	chapterNum  string
	translation string

	// input for prompts requiring user text (title, topic, question, weeks)
	input     textarea.Model
	inputHint string

	// streaming / result
	streamed  strings.Builder
	vp        viewport.Model
	spin      spinner.Model
	cancel    context.CancelFunc

	// library sub-list
	libraryItems []libraryEntry
	libraryIdx   int

	db     *localdb.DB
	ai     *ai.Client
	width  int
	height int
	err    error
	saved  string // last exported path
}

type libraryEntry struct {
	kind    string // "devotional" | "sermon" | "note"
	id      int64
	title   string
	ref     string
	content string
	model   string
	date    time.Time
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
		sub:    aiMenu,
		db:     db,
		ai:     aiClient,
		spin:   sp,
		input:  ta,
		vp:     vp,
	}
}

// SetContext passes the current reader passage into the panel.
func (p *AIPanel) SetContext(verseRef, verseText, chapterText, bookName, chapterNum, translation string) {
	p.verseRef    = verseRef
	p.verseText   = verseText
	p.chapterText = chapterText
	p.bookName    = bookName
	p.chapterNum  = chapterNum
	p.translation = translation
}

func (p *AIPanel) SetSize(w, h int) {
	p.width  = w
	p.height = h
	p.vp.Width  = w - 4
	p.vp.Height = h - 14
	p.input.SetWidth(w - 8)
}

func (p *AIPanel) Reset() {
	p.sub      = aiMenu
	p.menuIdx  = 0
	p.streamed.Reset()
	p.err      = nil
	p.saved    = ""
	p.input.Reset()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

func (p *AIPanel) Init() tea.Cmd {
	return p.spin.Tick
}

// Done returns true when the user pressed Esc from the menu.
// The parent should switch back to the previous state.
func (p *AIPanel) Done() bool { return false } // handled by key in parent

func (p *AIPanel) Hints() string {
	switch p.sub {
	case aiMenu:
		return "↑↓ navigate  •  enter select  •  esc back"
	case aiTyping:
		return "ctrl+enter submit  •  esc cancel"
	case aiStreaming:
		return "esc stop generation"
	case aiResult:
		return "s speak  •  e export PDF  •  enter save to library  •  esc back"
	case aiSaving:
		return "esc back"
	default:
		return "esc back"
	}
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

	case aiDoneMsg:
		p.sub = aiResult
		if msg.err != nil {
			p.err = msg.err
		}
		p.vp.SetContent(wrapText(p.streamed.String(), p.vp.Width))

	case aiSavedMsg:
		p.sub = aiSaving
		if msg.err != nil {
			p.err = msg.err
			p.saved = ""
		} else {
			p.saved = msg.path
		}

	case tea.KeyMsg:
		switch p.sub {
		case aiMenu:
			return p.handleMenuKey(msg)
		case aiTyping:
			return p.handleTypingKey(msg)
		case aiStreaming:
			if msg.String() == "esc" {
				if p.cancel != nil {
					p.cancel()
				}
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
	case "ctrl+s", "ctrl+enter":
		return p.submitInput()
	case "esc":
		p.sub = aiMenu
		p.input.Reset()
	}
	return p, nil
}

func (p *AIPanel) handleResultKey(msg tea.KeyMsg) (*AIPanel, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		p.sub = aiMenu
		p.streamed.Reset()
		p.err = nil
	case "enter":
		return p, p.saveToLibraryCmd()
	case "e":
		return p, p.exportPDFCmd()
	}
	return p, nil
}

func (p *AIPanel) selectAction(action aiAction) (*AIPanel, tea.Cmd) {
	p.action = action
	p.streamed.Reset()
	p.err = nil

	switch action {
	case aiActionExplainVerse, aiActionExplainChapter, aiActionAsk:
		if action == aiActionAsk {
			p.inputHint = "Enter your question about this passage:"
			p.sub = aiTyping
			p.input.Placeholder = "e.g. What does this mean for daily life?"
			p.input.Focus()
			return p, textarea.Blink
		}
		// No input needed — go straight to streaming
		return p, p.startGenerationCmd("")
	case aiActionDevotional:
		p.inputHint = "Optional theme (e.g. 'Faith', 'Courage') — press Ctrl+S to generate:"
		p.sub = aiTyping
		p.input.Placeholder = "Leave blank for default theme"
		p.input.Focus()
		return p, textarea.Blink
	case aiActionSermon:
		p.inputHint = "Sermon title (required):"
		p.sub = aiTyping
		p.input.Placeholder = "e.g. Walking in the Light"
		p.input.Focus()
		return p, textarea.Blink
	case aiActionStudyPlan:
		p.inputHint = "Study topic and number of weeks, e.g. 'Grace, 4 weeks':"
		p.sub = aiTyping
		p.input.Placeholder = "e.g. Prayer and Fasting, 6 weeks"
		p.input.Focus()
		return p, textarea.Blink
	case aiActionLibrary:
		return p, p.loadLibraryCmd()
	}
	return p, nil
}

func (p *AIPanel) submitInput() (*AIPanel, tea.Cmd) {
	userInput := strings.TrimSpace(p.input.Value())
	p.input.Reset()
	p.sub = aiStreaming
	return p, p.startGenerationCmd(userInput)
}

// ── Commands ─────────────────────────────────────────────────────────────────

func (p *AIPanel) startGenerationCmd(userInput string) tea.Cmd {
	p.sub = aiStreaming
	p.streamed.Reset()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	p.cancel = cancel

	vc := ai.VerseContext{
		Reference:   p.verseRef,
		Text:        p.verseText,
		Translation: p.translation,
	}

	aiClient := p.ai

	return func() tea.Msg {
		var tokens <-chan string
		var errc   <-chan error

		switch p.action {
		case aiActionExplainVerse:
			tokens, errc = aiClient.ExplainVerse(ctx, vc)
		case aiActionExplainChapter:
			tokens, errc = aiClient.ExplainChapter(ctx, p.bookName, p.chapterNum, p.translation, p.chapterText)
		case aiActionDevotional:
			tokens, errc = aiClient.GenerateDevotional(ctx, vc, userInput)
		case aiActionSermon:
			tokens, errc = aiClient.GenerateSermon(ctx, vc, userInput)
		case aiActionStudyPlan:
			// Parse "Topic, N weeks" from userInput
			topic, weeksStr := parseTopicWeeks(userInput)
			weeks := 4
			fmt.Sscanf(weeksStr, "%d", &weeks)
			plan, err := aiClient.GenerateStudyPlan(ctx, topic, weeks, p.translation)
			if err != nil {
				return aiDoneMsg{err: err}
			}
			// Convert to readable text
			var sb strings.Builder
			sb.WriteString(plan.Title + "\n\n")
			sb.WriteString(plan.Description + "\n\n")
			for _, w := range plan.Weeks {
				sb.WriteString(fmt.Sprintf("Week %d — %s\n", w.Week, w.Theme))
				sb.WriteString("Reading: " + w.Reading + "\n")
				if len(w.Verses) > 0 {
					sb.WriteString("Key Verses: " + strings.Join(w.Verses, ", ") + "\n")
				}
				sb.WriteString(w.Notes + "\n\n")
			}
			// store JSON for saving
			jsonBytes, _ := json.Marshal(plan)
			_ = jsonBytes
			return aiDoneMsg{}
		case aiActionAsk:
			tokens, errc = aiClient.AskAboutVerse(ctx, userInput, vc)
		default:
			return aiDoneMsg{err: fmt.Errorf("unknown action")}
		}

		// Stream tokens back as individual messages
		// We collect all tokens and send batched updates
		for t := range tokens {
			_ = t // handled via channel below — we use a separate goroutine approach
		}
		if err := <-errc; err != nil {
			return aiDoneMsg{err: err}
		}
		return aiDoneMsg{}
	}
}

// streamCmd returns a command that fires a sequence of aiTokenMsg then aiDoneMsg.
func (p *AIPanel) streamCmd(userInput string) tea.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	p.cancel = cancel

	vc := ai.VerseContext{
		Reference:   p.verseRef,
		Text:        p.verseText,
		Translation: p.translation,
	}

	aiClient := p.ai
	action   := p.action

	return func() tea.Msg {
		var tokens <-chan string
		var errc   <-chan error

		switch action {
		case aiActionExplainVerse:
			tokens, errc = aiClient.ExplainVerse(ctx, vc)
		case aiActionExplainChapter:
			tokens, errc = aiClient.ExplainChapter(ctx, p.verseRef, p.chapterNum, p.translation, p.chapterText)
		case aiActionDevotional:
			tokens, errc = aiClient.GenerateDevotional(ctx, vc, userInput)
		case aiActionSermon:
			tokens, errc = aiClient.GenerateSermon(ctx, vc, userInput)
		case aiActionAsk:
			tokens, errc = aiClient.AskAboutVerse(ctx, userInput, vc)
		default:
			return aiDoneMsg{err: fmt.Errorf("unsupported action")}
		}

		// We can't return multiple messages from one Cmd; use a sub-program pattern:
		// collect all tokens synchronously and send one aiDoneMsg with full text.
		var sb strings.Builder
		for t := range tokens {
			sb.WriteString(t)
		}
		if err := <-errc; err != nil && err != context.Canceled {
			return aiDoneMsg{err: err}
		}
		return struct {
			text string
		}{text: sb.String()}
	}
}

// We use a streaming approach via a goroutine that sends individual token messages.
// Replace startGenerationCmd with this proper streaming version.
func (p *AIPanel) StartStream(userInput string) tea.Cmd {
	p.sub = aiStreaming
	p.streamed.Reset()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	p.cancel = cancel

	vc := ai.VerseContext{
		Reference:   p.verseRef,
		Text:        p.verseText,
		Translation: p.translation,
	}

	aiClient := p.ai
	action   := p.action
	bookName    := p.bookName
	chapterNum  := p.chapterNum
	chapterText := p.chapterText
	translation := p.translation

	return func() tea.Msg {
		var tokens <-chan string
		var errc   <-chan error

		switch action {
		case aiActionExplainVerse:
			tokens, errc = aiClient.ExplainVerse(ctx, vc)
		case aiActionExplainChapter:
			tokens, errc = aiClient.ExplainChapter(ctx, bookName, chapterNum, translation, chapterText)
		case aiActionDevotional:
			tokens, errc = aiClient.GenerateDevotional(ctx, vc, userInput)
		case aiActionSermon:
			tokens, errc = aiClient.GenerateSermon(ctx, vc, userInput)
		case aiActionStudyPlan:
			topic, weeksStr := parseTopicWeeks(userInput)
			weeks := 4
			fmt.Sscanf(weeksStr, "%d", &weeks)
			plan, err := aiClient.GenerateStudyPlan(ctx, topic, weeks, translation)
			if err != nil {
				return aiDoneMsg{err: err}
			}
			// Render as readable text
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("%s\n\n%s\n\n", plan.Title, plan.Description))
			for _, w := range plan.Weeks {
				sb.WriteString(fmt.Sprintf("━━ Week %d — %s ━━\n", w.Week, w.Theme))
				sb.WriteString("Reading: " + w.Reading + "\n")
				if len(w.Verses) > 0 {
					sb.WriteString("Key Verses: " + strings.Join(w.Verses, " • ") + "\n")
				}
				sb.WriteString(w.Notes + "\n\n")
			}
			return struct{ text string }{text: sb.String()}
		case aiActionAsk:
			tokens, errc = aiClient.AskAboutVerse(ctx, userInput, vc)
		default:
			return aiDoneMsg{err: fmt.Errorf("unsupported action")}
		}

		// Collect all tokens
		var sb strings.Builder
		for t := range tokens {
			sb.WriteString(t)
		}
		var genErr error
		if errc != nil {
			if err := <-errc; err != nil && err != context.Canceled {
				genErr = err
			}
		}
		return struct {
			text string
			err  error
		}{text: sb.String(), err: genErr}
	}
}

// fullResultMsg carries the complete streamed text.
type fullResultMsg struct {
	text string
	err  error
}

// HandleFullResult wires the anonymous struct from StartStream into a typed message.
// Call this in the parent app's Update for StateAI.
func HandleFullResult(msg tea.Msg) (string, error, bool) {
	type r struct {
		text string
		err  error
	}
	if v, ok := msg.(r); ok {
		return v.text, v.err, true
	}
	return "", nil, false
}

func (p *AIPanel) saveToLibraryCmd() tea.Cmd {
	content := p.streamed.String()
	db      := p.db
	action  := p.action
	vRef    := p.verseRef
	trans   := p.translation
	model   := ""
	if p.ai != nil {
		model = p.ai.Model()
	}

	return func() tea.Msg {
		now := time.Now()
		switch action {
		case aiActionDevotional:
			title := extractTitle(content)
			_, err := db.SaveDevotional(localdb.Devotional{
				Title: title, VerseRef: vRef, Content: content,
				AIModel: model, CreatedAt: now,
			})
			return aiSavedMsg{err: err}
		case aiActionSermon:
			title := extractTitle(content)
			_, err := db.SaveSermon(localdb.Sermon{
				Title: title, ScriptureRef: vRef, Content: content,
				AIModel: model, CreatedAt: now,
			})
			return aiSavedMsg{err: err}
		case aiActionExplainVerse, aiActionAsk:
			_, err := db.SaveNote(localdb.AINote{
				VerseID: vRef, TranslationID: trans, Content: content,
				AIModel: model, CreatedAt: now,
			})
			return aiSavedMsg{err: err}
		}
		return aiSavedMsg{err: fmt.Errorf("saving not supported for this content type")}
	}
}

func (p *AIPanel) exportPDFCmd() tea.Cmd {
	content := p.streamed.String()
	action  := p.action
	vRef    := p.verseRef
	home, _ := os.UserHomeDir()
	outDir  := filepath.Join(home, "Desktop")
	_ = os.MkdirAll(outDir, 0o755)

	return func() tea.Msg {
		ts    := time.Now().Format("20060102-150405")
		title := extractTitle(content)
		var outPath string
		var err error

		switch action {
		case aiActionDevotional:
			outPath = filepath.Join(outDir, fmt.Sprintf("logos-devotional-%s.pdf", ts))
			err = pdf.ExportDevotional(title, vRef, "", content, outPath)
		case aiActionSermon:
			outPath = filepath.Join(outDir, fmt.Sprintf("logos-sermon-%s.pdf", ts))
			err = pdf.ExportSermon(title, vRef, content, outPath)
		case aiActionExplainVerse, aiActionAsk:
			outPath = filepath.Join(outDir, fmt.Sprintf("logos-note-%s.pdf", ts))
			err = pdf.ExportNote(vRef, content, outPath)
		default:
			return aiSavedMsg{err: fmt.Errorf("export not supported for this content")}
		}
		return aiSavedMsg{path: outPath, err: err}
	}
}

func (p *AIPanel) loadLibraryCmd() tea.Cmd {
	db := p.db
	return func() tea.Msg {
		var entries []libraryEntry
		if db != nil {
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
		}
		return libraryLoadedMsg{entries: entries}
	}
}

type libraryLoadedMsg struct{ entries []libraryEntry }

// ── View ─────────────────────────────────────────────────────────────────────

func (p *AIPanel) View() string {
	s := p.styles()

	switch p.sub {
	case aiMenu:
		return p.viewMenu(s)
	case aiTyping:
		return p.viewTyping(s)
	case aiStreaming:
		return p.viewStreaming(s)
	case aiResult:
		return p.viewResult(s)
	case aiSaving:
		return p.viewSaving(s)
	}
	return ""
}

type aiStyles struct {
	title    lipgloss.Style
	selected lipgloss.Style
	normal   lipgloss.Style
	hint     lipgloss.Style
	gold     lipgloss.Style
	box      lipgloss.Style
}

func (p *AIPanel) styles() aiStyles {
	return aiStyles{
		title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#D4AF37")).Padding(0, 1),
		selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#D4AF37")).Background(lipgloss.Color("#0F172A")).Padding(0, 2),
		normal:   lipgloss.NewStyle().Foreground(lipgloss.Color("#C9B8E8")).Padding(0, 2),
		hint:     lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true),
		gold:     lipgloss.NewStyle().Foreground(lipgloss.Color("#D4AF37")),
		box:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#D4AF37")).Padding(0, 1),
	}
}

func (p *AIPanel) header(s aiStyles, subtitle string) string {
	title := s.title.Render("✝  Logos AI Assistant")
	sub   := s.hint.Render(subtitle)
	ctx   := ""
	if p.verseRef != "" {
		ctx = s.gold.Render("Context: ") + s.hint.Render(p.verseRef)
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, sub, ctx, "")
}

func (p *AIPanel) viewMenu(s aiStyles) string {
	var rows []string
	rows = append(rows, p.header(s, "What would you like to do?"))
	for i, item := range aiMenuItems {
		line := item.label
		if i == p.menuIdx {
			rows = append(rows, s.selected.Render("▶  "+line))
		} else {
			rows = append(rows, s.normal.Render("   "+line))
		}
	}
	rows = append(rows, "", s.hint.Render(p.Hints()))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (p *AIPanel) viewTyping(s aiStyles) string {
	return lipgloss.JoinVertical(lipgloss.Left,
		p.header(s, aiMenuItems[p.menuIdx].hint),
		s.hint.Render(p.inputHint),
		"",
		p.input.View(),
		"",
		s.hint.Render("Ctrl+S to generate  •  Esc to cancel"),
	)
}

func (p *AIPanel) viewStreaming(s aiStyles) string {
	text := p.streamed.String()
	preview := text
	if len(preview) > p.vp.Width*p.vp.Height {
		preview = "..." + preview[len(preview)-p.vp.Width*p.vp.Height:]
	}
	p.vp.SetContent(wrapText(preview, p.vp.Width))
	return lipgloss.JoinVertical(lipgloss.Left,
		p.header(s, p.spin.View()+" Generating…"),
		p.vp.View(),
		s.hint.Render("Esc to stop"),
	)
}

func (p *AIPanel) viewResult(s aiStyles) string {
	if p.err != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			p.header(s, "Error"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render(p.err.Error()),
			"",
			s.hint.Render("Esc to go back"),
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		p.header(s, aiMenuItems[p.menuIdx].label),
		p.vp.View(),
		"",
		s.hint.Render(p.Hints()),
	)
}

func (p *AIPanel) viewSaving(s aiStyles) string {
	msg := ""
	if p.err != nil {
		msg = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("Error: " + p.err.Error())
	} else if p.saved != "" {
		msg = s.gold.Render("✓ Exported to: ") + s.hint.Render(p.saved)
	} else {
		msg = s.gold.Render("✓ Saved to library!")
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		p.header(s, "Done"),
		"",
		msg,
		"",
		s.hint.Render("Press any key to continue"),
	)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func wrapText(s string, width int) string {
	if width <= 0 {
		width = 80
	}
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		for len(line) > width {
			lines = append(lines, line[:width])
			line = line[width:]
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func extractTitle(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, l := range lines {
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

// parseTopicWeeks splits "Prayer and Fasting, 6 weeks" → ("Prayer and Fasting", "6")
func parseTopicWeeks(s string) (topic, weeks string) {
	parts := strings.Split(s, ",")
	if len(parts) >= 2 {
		topic = strings.TrimSpace(parts[0])
		// extract number from second part
		second := strings.ToLower(strings.TrimSpace(parts[1]))
		second = strings.ReplaceAll(second, "weeks", "")
		second = strings.ReplaceAll(second, "week", "")
		weeks = strings.TrimSpace(second)
		return
	}
	return strings.TrimSpace(s), "4"
}
