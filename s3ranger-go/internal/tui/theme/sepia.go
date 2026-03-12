package theme

import "github.com/charmbracelet/lipgloss"

func Sepia() *Theme {
	t := &Theme{
		Name:       "Sepia",
		Primary:    lipgloss.Color("#8b4513"),
		Secondary:  lipgloss.Color("#2f1b14"),
		Accent:     lipgloss.Color("#cd853f"),
		Foreground: lipgloss.Color("#2f1b14"),
		Background: lipgloss.Color("#f5deb3"),
		Success:    lipgloss.Color("#6b8e23"),
		Warning:    lipgloss.Color("#ff8c00"),
		Error:      lipgloss.Color("#8b0000"),
		Surface:    lipgloss.Color("#deb887"),
		Panel:      lipgloss.Color("#d2b48c"),
		Border:     lipgloss.Color("#bc9a6a"),
		Boost:      lipgloss.Color("#bc9a6a"),
		IsDark:     false,
	}
	buildStyles(t)
	return t
}
