package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit       key.Binding
	Tab        key.Binding
	Refresh    key.Binding
	Help       key.Binding
	Enter      key.Binding
	Space      key.Binding
	Escape     key.Binding
	Up         key.Binding
	Down       key.Binding
	Delete     key.Binding
	Download   key.Binding
	Upload     key.Binding
	Rename     key.Binding
	Move       key.Binding
	Copy       key.Binding
	Sort       key.Binding
	SelectAll  key.Binding
	Filter     key.Binding
	Confirm    key.Binding
	OpenFile   key.Binding
	OpenFolder key.Binding
}

var Keys = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+q"),
		key.WithHelp("ctrl+q", "quit"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch panel"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "refresh"),
	),
	Help: key.NewBinding(
		key.WithKeys("ctrl+h"),
		key.WithHelp("ctrl+h", "help"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Space: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "toggle select"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel/clear"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Delete: key.NewBinding(
		key.WithKeys("delete", "backspace"),
		key.WithHelp("del", "delete"),
	),
	Download: key.NewBinding(
		key.WithKeys("d", "D"),
		key.WithHelp("d", "download"),
	),
	Upload: key.NewBinding(
		key.WithKeys("u", "U"),
		key.WithHelp("u", "upload"),
	),
	Rename: key.NewBinding(
		key.WithKeys("ctrl+k"),
		key.WithHelp("ctrl+k", "rename"),
	),
	Move: key.NewBinding(
		key.WithKeys("m", "M"),
		key.WithHelp("m", "move"),
	),
	Copy: key.NewBinding(
		key.WithKeys("c", "C"),
		key.WithHelp("c", "copy"),
	),
	Sort: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "sort"),
	),
	SelectAll: key.NewBinding(
		key.WithKeys("ctrl+a"),
		key.WithHelp("ctrl+a", "select all"),
	),
	Filter: key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("ctrl+f", "filter"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("ctrl+enter"),
		key.WithHelp("ctrl+enter", "confirm"),
	),
	OpenFile: key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("ctrl+o", "browse file"),
	),
	OpenFolder: key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp("ctrl+l", "browse folder"),
	),
}
