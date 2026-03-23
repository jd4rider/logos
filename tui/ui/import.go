package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/jd4rider/logos/internal/crawler"
	localdb "github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/importer"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Import panel sub-states ───────────────────────────────────────────────────

type importStep int

const (
	importStepPickType importStep = iota // choose source type
	importStepEnterSource                // type URL or file path
	importStepEnterAbbr                  // abbreviation
	importStepEnterName                  // full name
	importStepRunning                    // import in progress
	importStepDone                       // finished (success or error)
)

// importSourceType represents the kind of data to import.
type importSourceType int

const (
	importSrcBibleGateway importSourceType = iota
	importSrcCSV
	importSrcSQLiteFile
	importSrcGenericURL
)

var importSourceLabels = []string{
	"🌐  BibleGateway.com (paste a /versions/ or /passage/ URL)",
	"📄  CSV file          (book, chapter, verse, text columns)",
	"🗄️   SQLite file       (Scrollmapper, BibleSuperSearch, etc.)",
	"🔗  Other website     (generic crawler, follows next-chapter links)",
}

// ── ImportPanel ───────────────────────────────────────────────────────────────

// ImportPanel drives a multi-step import wizard inside the TUI.
type ImportPanel struct {
	localDB *localdb.DB

	step     importStep
	srcType  importSourceType
	cursor   int // for type picker

	// Text inputs
	sourceInput textinput.Model
	abbrInput   textinput.Model
	nameInput   textinput.Model

	// Progress log
	log    []string
	done   bool
	runErr error

	// Layout
	width  int
	height int

	// progress channel (written by goroutine, read by WaitForProgress cmd)
	progressCh chan string
}

// NewImportPanel creates a ready-to-use import panel.
func NewImportPanel(db *localdb.DB) *ImportPanel {
	src := textinput.New()
	src.Placeholder = "URL or file path…"
	src.CharLimit = 512
	src.Width = 72

	abbr := textinput.New()
	abbr.Placeholder = "e.g. KJV  (max 8 chars)"
	abbr.CharLimit = 8
	abbr.Width = 20

	name := textinput.New()
	name.Placeholder = "e.g. King James Version"
	name.CharLimit = 80
	name.Width = 60

	for _, ti := range []*textinput.Model{&src, &abbr, &name} {
		ti.PromptStyle = lipgloss.NewStyle().Foreground(ColorPurple)
		ti.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	}

	return &ImportPanel{
		localDB:     db,
		sourceInput: src,
		abbrInput:   abbr,
		nameInput:   name,
		progressCh:  make(chan string, 256),
	}
}

// Reset brings the panel back to the first step.
func (p *ImportPanel) Reset() {
	p.step = importStepPickType
	p.cursor = 0
	p.log = nil
	p.done = false
	p.runErr = nil
	p.sourceInput.Reset()
	p.abbrInput.Reset()
	p.nameInput.Reset()
	p.progressCh = make(chan string, 256)
}

// Done returns true once the import finished AND the user pressed a key.
func (p *ImportPanel) Done() bool { return p.done }

// Init starts blinking on the first focused input (none yet at type-picker).
func (p *ImportPanel) Init() tea.Cmd { return nil }

// SetSize resizes the panel.
func (p *ImportPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.sourceInput.Width = w - 10
	p.nameInput.Width = w - 10
}

// Hints returns the footer hint string for the current step.
func (p *ImportPanel) Hints() string {
	switch p.step {
	case importStepPickType:
		return "↑↓ select type  •  enter confirm  •  esc cancel"
	case importStepEnterSource, importStepEnterAbbr, importStepEnterName:
		return "enter next  •  esc cancel"
	case importStepRunning:
		return "import running…  please wait"
	case importStepDone:
		return "enter / esc  return to translation list"
	}
	return ""
}

// ── Update ────────────────────────────────────────────────────────────────────

func (p *ImportPanel) Update(msg tea.Msg) (*ImportPanel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch p.step {

		case importStepPickType:
			switch msg.String() {
			case "up", "k":
				if p.cursor > 0 { p.cursor-- }
			case "down", "j":
				if p.cursor < len(importSourceLabels)-1 { p.cursor++ }
			case "enter":
				p.srcType = importSourceType(p.cursor)
				p.step = importStepEnterSource
				p.sourceInput.Focus()
				return p, textinput.Blink
			case "esc":
				p.done = true
			}

		case importStepEnterSource:
			switch msg.String() {
			case "enter":
				if strings.TrimSpace(p.sourceInput.Value()) == "" {
					break
				}
				p.step = importStepEnterAbbr
				p.sourceInput.Blur()
				p.abbrInput.Focus()
				return p, textinput.Blink
			case "esc":
				p.step = importStepPickType
				p.sourceInput.Blur()
			default:
				var cmd tea.Cmd
				p.sourceInput, cmd = p.sourceInput.Update(msg)
				return p, cmd
			}

		case importStepEnterAbbr:
			switch msg.String() {
			case "enter":
				p.step = importStepEnterName
				p.abbrInput.Blur()
				p.nameInput.Focus()
				return p, textinput.Blink
			case "esc":
				p.step = importStepEnterSource
				p.abbrInput.Blur()
				p.sourceInput.Focus()
				return p, textinput.Blink
			default:
				var cmd tea.Cmd
				p.abbrInput, cmd = p.abbrInput.Update(msg)
				return p, cmd
			}

		case importStepEnterName:
			switch msg.String() {
			case "enter":
				p.step = importStepRunning
				p.nameInput.Blur()
				return p, p.cmdRunImport()
			case "esc":
				p.step = importStepEnterAbbr
				p.nameInput.Blur()
				p.abbrInput.Focus()
				return p, textinput.Blink
			default:
				var cmd tea.Cmd
				p.nameInput, cmd = p.nameInput.Update(msg)
				return p, cmd
			}

		case importStepRunning:
			// ignore key presses while running

		case importStepDone:
			// any key returns to the translation list
			if msg.String() == "enter" || msg.String() == "esc" {
				p.done = true
			}
		}

	case importProgressMsg:
		p.log = append(p.log, msg.line)
		if p.step == importStepRunning {
			return p, p.waitForProgress()
		}

	case importDoneMsg:
		p.step = importStepDone
		p.runErr = msg.err
		if msg.err != nil {
			p.log = append(p.log, fmt.Sprintf("✗ Error: %v", msg.err))
		} else {
			p.log = append(p.log, "")
			p.log = append(p.log, "✓ Import complete! Press enter to return.")
		}
	}

	return p, nil
}

// cmdRunImport launches the import goroutine and returns a Cmd to stream progress.
func (p *ImportPanel) cmdRunImport() tea.Cmd {
	src := strings.TrimSpace(p.sourceInput.Value())
	abbr := strings.TrimSpace(p.abbrInput.Value())
	name := strings.TrimSpace(p.nameInput.Value())
	srcType := p.srcType
	ch := p.progressCh
	db := p.localDB

	go func() {
		progress := func(msg string) { ch <- msg }

		var err error
		switch srcType {
		case importSrcBibleGateway, importSrcGenericURL:
			err = crawler.Crawl(db, src, crawler.Options{
				Abbreviation: abbr,
				Name:         name,
				MaxChapters:  0,
				Delay:        1200 * time.Millisecond,
				Progress:     progress,
			})
		case importSrcCSV:
			err = importer.ImportCSV(db, src, importer.ImportOptions{
				Abbreviation: abbr,
				Name:         name,
				Progress:     progress,
			})
		case importSrcSQLiteFile:
			err = importer.ImportSQLiteFile(db, src, importer.ImportOptions{
				Abbreviation: abbr,
				Name:         name,
				Progress:     progress,
			})
		}
		ch <- "\x00DONE\x00" + fmt.Sprint(err)
	}()

	return p.waitForProgress()
}

// waitForProgress reads one message from the progress channel.
func (p *ImportPanel) waitForProgress() tea.Cmd {
	ch := p.progressCh
	return func() tea.Msg {
		line := <-ch
		if strings.HasPrefix(line, "\x00DONE\x00") {
			errStr := strings.TrimPrefix(line, "\x00DONE\x00")
			if errStr == "<nil>" || errStr == "" {
				return importDoneMsg{nil}
			}
			return importDoneMsg{fmt.Errorf("%s", errStr)}
		}
		return importProgressMsg{line}
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

var (
	importTitleStyle = lipgloss.NewStyle().
				Foreground(ColorGold).
				Bold(true).
				MarginBottom(1)

	importBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPurple).
			Padding(1, 2)

	importSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorGold).
				Bold(true)

	importNormalStyle = lipgloss.NewStyle().
				Foreground(ColorText)

	importLabelStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Width(14)

	importLogStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(1)

	importErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF5555"))
)

func (p *ImportPanel) View() string {
	var sb strings.Builder

	sb.WriteString(importTitleStyle.Render("✝  Import Bible Translation"))
	sb.WriteString("\n\n")

	switch p.step {

	case importStepPickType:
		sb.WriteString(importLabelStyle.Render("Source type:") + "\n\n")
		for i, label := range importSourceLabels {
			if i == p.cursor {
				sb.WriteString("  " + importSelectedStyle.Render("▶ "+label) + "\n")
			} else {
				sb.WriteString("    " + importNormalStyle.Render(label) + "\n")
			}
		}

	case importStepEnterSource:
		typeLabel := importSourceLabels[int(p.srcType)]
		sb.WriteString(importLabelStyle.Render("Type:") + " " + typeLabel + "\n\n")
		sb.WriteString(importLabelStyle.Render("Source:") + " " + p.sourceInput.View() + "\n")
		sb.WriteString("\n")
		if p.srcType == importSrcBibleGateway {
			sb.WriteString(importNormalStyle.Render(
				"  Tip: paste a BibleGateway URL like:\n" +
					"  https://www.biblegateway.com/versions/New-Life-Version-NLV-Bible/#booklist\n" +
					"  or: https://www.biblegateway.com/passage/?search=Genesis+1&version=KJV"))
		} else if p.srcType == importSrcCSV {
			sb.WriteString(importNormalStyle.Render(
				"  CSV columns (auto-detected): book, chapter, verse, text\n" +
					"  Scrollmapper format (b, c, v, t) is also supported."))
		} else if p.srcType == importSrcSQLiteFile {
			sb.WriteString(importNormalStyle.Render(
				"  Accepts: Scrollmapper .db, BibleSuperSearch exports,\n" +
					"  or another logos database."))
		}

	case importStepEnterAbbr:
		sb.WriteString(importLabelStyle.Render("Source:") + " " + p.sourceInput.Value() + "\n\n")
		sb.WriteString(importLabelStyle.Render("Abbreviation:") + " " + p.abbrInput.View() + "\n")
		sb.WriteString("\n" + importNormalStyle.Render("  Short code shown in the translation picker (e.g. KJV, NLV, NIV)."))

	case importStepEnterName:
		sb.WriteString(importLabelStyle.Render("Source:") + " " + p.sourceInput.Value() + "\n")
		sb.WriteString(importLabelStyle.Render("Abbreviation:") + " " + p.abbrInput.Value() + "\n\n")
		sb.WriteString(importLabelStyle.Render("Full name:") + " " + p.nameInput.View() + "\n")
		sb.WriteString("\n" + importNormalStyle.Render("  Full translation name shown in the picker list."))

	case importStepRunning, importStepDone:
		sb.WriteString(importLabelStyle.Render("Source:") + " " + p.sourceInput.Value() + "\n")
		sb.WriteString(importLabelStyle.Render("Translation:") + " " +
			p.abbrInput.Value() + " — " + p.nameInput.Value() + "\n\n")
		if p.step == importStepRunning {
			sb.WriteString(importNormalStyle.Render("  ⏳ Importing…\n\n"))
		}
		// Show last N lines of log
		lines := p.log
		maxLines := p.height - 12
		if maxLines < 5 { maxLines = 5 }
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
		for _, l := range lines {
			if strings.HasPrefix(l, "✗") {
				sb.WriteString("  " + importErrorStyle.Render(l) + "\n")
			} else {
				sb.WriteString("  " + importLogStyle.Render(l) + "\n")
			}
		}
	}

	content := sb.String()
	box := importBoxStyle.Width(p.width - 4).Render(content)
	return box
}
