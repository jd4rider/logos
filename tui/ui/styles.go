package ui

import "github.com/charmbracelet/lipgloss"

const (
ColorBg       = lipgloss.Color("#0d0d1a")
ColorHeaderBg = lipgloss.Color("#12102a")
ColorBorder   = lipgloss.Color("#2d2d4e")
ColorGold     = lipgloss.Color("#d4af37")
ColorPurple   = lipgloss.Color("#7c3aed")
ColorText     = lipgloss.Color("#e2e8f0")
ColorMuted    = lipgloss.Color("#64748b")
ColorAccent   = lipgloss.Color("#a78bfa")
ColorHL       = lipgloss.Color("#4c1d95") // selected list item bg
ColorWordBg   = lipgloss.Color("#1a3a1a") // currently spoken word bg
ColorWordFg   = lipgloss.Color("#86efac") // currently spoken word fg
ColorGreen    = lipgloss.Color("#10b981")
ColorRed      = lipgloss.Color("#ef4444")
)

type Styles struct {
Header      lipgloss.Style
HeaderTitle lipgloss.Style
HeaderCrumb lipgloss.Style
Footer      lipgloss.Style
VerseNum    lipgloss.Style
VerseText   lipgloss.Style
WordHL      lipgloss.Style
Error       lipgloss.Style
Muted       lipgloss.Style
Success     lipgloss.Style
TTSOn       lipgloss.Style
TTSOff      lipgloss.Style
Copyright   lipgloss.Style
ListTitle   lipgloss.Style
SearchBox   lipgloss.Style
}

func NewStyles() *Styles {
s := &Styles{}

s.Header = lipgloss.NewStyle().
Background(ColorHeaderBg).
Foreground(ColorText).
Padding(0, 2)

s.HeaderTitle = lipgloss.NewStyle().
Foreground(ColorGold).
Bold(true)

s.HeaderCrumb = lipgloss.NewStyle().
Foreground(ColorAccent)

s.Footer = lipgloss.NewStyle().
Background(ColorHeaderBg).
Foreground(ColorMuted).
Padding(0, 2)

s.VerseNum = lipgloss.NewStyle().
Foreground(ColorGold).
Bold(true)

s.VerseText = lipgloss.NewStyle().
Foreground(ColorText)

s.WordHL = lipgloss.NewStyle().
Background(ColorWordBg).
Foreground(ColorWordFg).
Bold(true)

s.Error = lipgloss.NewStyle().
Foreground(ColorRed).
Bold(true)

s.Muted = lipgloss.NewStyle().
Foreground(ColorMuted)

s.Success = lipgloss.NewStyle().
Foreground(ColorGreen)

s.TTSOn = lipgloss.NewStyle().
Foreground(ColorGreen).
Bold(true)

s.TTSOff = lipgloss.NewStyle().
Foreground(ColorMuted)

s.Copyright = lipgloss.NewStyle().
Foreground(ColorMuted).
Italic(true)

s.ListTitle = lipgloss.NewStyle().
Foreground(ColorGold).
Background(ColorHeaderBg).
Bold(true).
Padding(0, 1)

s.SearchBox = lipgloss.NewStyle().
Border(lipgloss.RoundedBorder()).
BorderForeground(ColorPurple).
Padding(0, 1).
Foreground(ColorText)

return s
}
