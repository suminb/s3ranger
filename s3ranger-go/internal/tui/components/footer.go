package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
)

type FooterModel struct {
	Theme         *theme.Theme
	ActivePanel   string // "buckets" or "objects"
	SelectedCount int
	HasItems      bool
	InModal       bool
	Width         int
}

func NewFooter(t *theme.Theme) FooterModel {
	return FooterModel{
		Theme:       t,
		ActivePanel: "buckets",
	}
}

type footerBinding struct {
	Key  string
	Desc string
}

func (m FooterModel) View() string {
	var bindings []footerBinding

	if m.InModal {
		bindings = []footerBinding{
			{"esc", "cancel"},
			{"ctrl+enter", "confirm"},
		}
	} else {
		bindings = []footerBinding{
			{"tab", "switch panel"},
			{"ctrl+r", "refresh"},
			{"ctrl+h", "help"},
			{"ctrl+q", "quit"},
		}

		if m.ActivePanel == "buckets" {
			bindings = append([]footerBinding{
				{"ctrl+f", "filter"},
				{"enter", "select bucket"},
			}, bindings...)
		} else if m.ActivePanel == "objects" {
			if m.SelectedCount > 1 {
				bindings = append([]footerBinding{
					{"d", "download"},
					{"del", "delete"},
					{"m", "move"},
					{"c", "copy"},
					{"esc", "clear selection"},
				}, bindings...)
			} else if m.SelectedCount == 1 || m.HasItems {
				bindings = append([]footerBinding{
					{"d", "download"},
					{"u", "upload"},
					{"del", "delete"},
					{"ctrl+k", "rename"},
					{"m", "move"},
					{"c", "copy"},
					{"ctrl+s", "sort"},
					{"space", "select"},
				}, bindings...)
			} else {
				bindings = append([]footerBinding{
					{"u", "upload"},
				}, bindings...)
			}
		}
	}

	var parts []string
	for _, b := range bindings {
		keyStyle := m.Theme.FooterKey.Render(b.Key)
		descStyle := m.Theme.FooterDesc.Render(b.Desc)
		parts = append(parts, fmt.Sprintf("%s %s", keyStyle, descStyle))
	}

	content := strings.Join(parts, "  ")

	bar := lipgloss.NewStyle().
		Background(m.Theme.Surface).
		Foreground(m.Theme.Foreground).
		Width(m.Width).
		Padding(0, 1)

	return bar.Render(content)
}
