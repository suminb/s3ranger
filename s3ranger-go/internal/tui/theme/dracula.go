package theme

import "github.com/charmbracelet/lipgloss"

func Dracula() *Theme {
	t := &Theme{
		Name:       "Dracula",
		Primary:    lipgloss.Color("#bd93f9"),
		Secondary:  lipgloss.Color("#f8f8f2"),
		Accent:     lipgloss.Color("#ff79c6"),
		Foreground: lipgloss.Color("#f8f8f2"),
		Background: lipgloss.Color("#282a36"),
		Success:    lipgloss.Color("#50fa7b"),
		Warning:    lipgloss.Color("#f1fa8c"),
		Error:      lipgloss.Color("#ff5555"),
		Surface:    lipgloss.Color("#44475a"),
		Panel:      lipgloss.Color("#44475a"),
		Border:     lipgloss.Color("#44475a"),
		Boost:      lipgloss.Color("#6272a4"),
		IsDark:     true,
	}
	buildStyles(t)
	return t
}
