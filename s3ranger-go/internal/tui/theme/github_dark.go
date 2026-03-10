package theme

import "github.com/charmbracelet/lipgloss"

func GithubDark() *Theme {
	t := &Theme{
		Name:       "Github Dark",
		Primary:    lipgloss.Color("#58a6ff"),
		Secondary:  lipgloss.Color("#f0f6fc"),
		Accent:     lipgloss.Color("#58a6ff"),
		Foreground: lipgloss.Color("#c9d1d9"),
		Background: lipgloss.Color("#0d1117"),
		Success:    lipgloss.Color("#3fb950"),
		Warning:    lipgloss.Color("#f0d956"),
		Error:      lipgloss.Color("#da3633"),
		Surface:    lipgloss.Color("#161b22"),
		Panel:      lipgloss.Color("#161b22"),
		Border:     lipgloss.Color("#30363d"),
		Boost:      lipgloss.Color("#1f2937"),
		IsDark:     true,
	}
	buildStyles(t)
	return t
}
