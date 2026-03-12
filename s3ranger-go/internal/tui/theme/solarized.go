package theme

import "github.com/charmbracelet/lipgloss"

func Solarized() *Theme {
	t := &Theme{
		Name:       "Solarized",
		Primary:    lipgloss.Color("#268bd2"),
		Secondary:  lipgloss.Color("#fdf6e3"),
		Accent:     lipgloss.Color("#b58900"),
		Foreground: lipgloss.Color("#839496"),
		Background: lipgloss.Color("#002b36"),
		Success:    lipgloss.Color("#859900"),
		Warning:    lipgloss.Color("#cb4b16"),
		Error:      lipgloss.Color("#dc322f"),
		Surface:    lipgloss.Color("#073642"),
		Panel:      lipgloss.Color("#073642"),
		Border:     lipgloss.Color("#073642"),
		Boost:      lipgloss.Color("#586e75"),
		IsDark:     true,
	}
	buildStyles(t)
	return t
}
