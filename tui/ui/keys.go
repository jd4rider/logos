package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
Up          key.Binding
Down        key.Binding
PageUp      key.Binding
PageDown    key.Binding
HalfUp      key.Binding
HalfDown    key.Binding
Top         key.Binding
Bottom      key.Binding
Back        key.Binding
Select      key.Binding
Search      key.Binding
Speak       key.Binding
StopSpeak   key.Binding
NextChapter key.Binding
PrevChapter key.Binding
VoiceSelect key.Binding
Quit        key.Binding
}

func DefaultKeyMap() KeyMap {
return KeyMap{
Up:          key.NewBinding(key.WithKeys("up", "k"),          key.WithHelp("↑/k", "up")),
Down:        key.NewBinding(key.WithKeys("down", "j"),         key.WithHelp("↓/j", "down")),
PageUp:      key.NewBinding(key.WithKeys("pgup", "ctrl+b"),    key.WithHelp("pgup", "page up")),
PageDown:    key.NewBinding(key.WithKeys("pgdown", "ctrl+f"),  key.WithHelp("pgdn", "page down")),
HalfUp:      key.NewBinding(key.WithKeys("ctrl+u"),            key.WithHelp("ctrl+u", "½ up")),
HalfDown:    key.NewBinding(key.WithKeys("ctrl+d"),            key.WithHelp("ctrl+d", "½ down")),
Top:         key.NewBinding(key.WithKeys("g"),                 key.WithHelp("g", "top")),
Bottom:      key.NewBinding(key.WithKeys("G"),                 key.WithHelp("G", "bottom")),
Back:        key.NewBinding(key.WithKeys("esc", "backspace"),  key.WithHelp("esc", "back")),
Select:      key.NewBinding(key.WithKeys("enter", "right", "l"), key.WithHelp("enter", "select")),
Search:      key.NewBinding(key.WithKeys("/"),                 key.WithHelp("/", "search")),
Speak:       key.NewBinding(key.WithKeys("s"),                 key.WithHelp("s", "speak")),
StopSpeak:   key.NewBinding(key.WithKeys("S"),                 key.WithHelp("S", "stop")),
NextChapter: key.NewBinding(key.WithKeys("n", "]"),            key.WithHelp("n/]", "next ch")),
PrevChapter: key.NewBinding(key.WithKeys("p", "["),            key.WithHelp("p/[", "prev ch")),
VoiceSelect: key.NewBinding(key.WithKeys("v"),                 key.WithHelp("v", "voice")),
Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"),       key.WithHelp("q", "quit")),
}
}
