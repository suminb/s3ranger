package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
)

type TitleBarModel struct {
	Theme           *theme.Theme
	ProfileDisplay  string
	EndpointURL     string
	ConnectionError bool
	Width           int
}

func NewTitleBar(t *theme.Theme, profile, endpoint string) TitleBarModel {
	return TitleBarModel{
		Theme:          t,
		ProfileDisplay: profile,
		EndpointURL:    endpoint,
	}
}

func (m TitleBarModel) View() string {
	// Connection dot
	dot := m.Theme.StatusDotOK.Render("●")
	if m.ConnectionError {
		dot = m.Theme.StatusDotErr.Render("●")
	}

	title := m.Theme.TitleBar.Render("S3 Ranger")

	profileInfo := fmt.Sprintf("aws-profile: %s", m.ProfileDisplay)
	if m.EndpointURL != "" {
		profileInfo = fmt.Sprintf("%s (%s)", profileInfo, m.EndpointURL)
	}

	left := fmt.Sprintf("%s %s %s", title, dot, profileInfo)

	bar := lipgloss.NewStyle().
		Background(m.Theme.Surface).
		Foreground(m.Theme.Foreground).
		Width(m.Width).
		Padding(0, 1)

	return bar.Render(left)
}
