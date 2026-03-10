package modals

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
)

type HelpModel struct {
	Theme    *theme.Theme
	Width    int
	Height   int
	viewport viewport.Model
	done     bool
	ready    bool
}

func NewHelp(t *theme.Theme, width, height int) HelpModel {
	content := helpContent(t)
	vp := viewport.New(min(70, width-6), min(30, height-6))
	vp.SetContent(content)

	return HelpModel{
		Theme:    t,
		Width:    width,
		Height:   height,
		viewport: vp,
		ready:    true,
	}
}

func (m HelpModel) Init() tea.Cmd {
	return nil
}

func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "enter", "q"))):
			m.done = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m HelpModel) IsDone() bool {
	return m.done
}

func (m HelpModel) View() string {
	title := m.Theme.ModalTitle.Render("S3 Ranger — Keybinding Reference")
	footer := m.Theme.DimText.Render("[esc/enter] close  [↑↓] scroll")

	content := fmt.Sprintf("%s\n\n%s\n\n%s", title, m.viewport.View(), footer)

	return m.Theme.ModalBox.
		Width(min(74, m.Width-2)).
		Render(content)
}

func helpContent(t *theme.Theme) string {
	k := t.FooterKey
	d := t.DimText

	section := func(title string, bindings [][2]string) string {
		s := t.HeaderText.Render(title) + "\n"
		for _, b := range bindings {
			s += fmt.Sprintf("  %s  %s\n", k.Render(fmt.Sprintf("%-14s", b[0])), d.Render(b[1]))
		}
		return s
	}

	return section("Navigation", [][2]string{
		{"Tab", "Switch between bucket list and object list"},
		{"↑/k", "Move cursor up"},
		{"↓/j", "Move cursor down"},
		{"Enter", "Select bucket / open folder"},
		{"Ctrl+R", "Refresh current view"},
	}) + "\n" +
		section("File Operations", [][2]string{
			{"D", "Download selected file/folder"},
			{"U", "Upload file to current location"},
			{"Del", "Delete selected file/folder"},
			{"Ctrl+K", "Rename selected file/folder"},
			{"M", "Move selected items"},
			{"C", "Copy selected items"},
		}) + "\n" +
		section("Selection", [][2]string{
			{"Space", "Toggle selection of current item"},
			{"Ctrl+A", "Select all items"},
			{"Escape", "Clear all selections"},
		}) + "\n" +
		section("General", [][2]string{
			{"Ctrl+F", "Focus bucket filter"},
			{"Ctrl+S", "Sort objects"},
			{"Ctrl+H", "Show this help"},
			{"Ctrl+Q", "Quit application"},
		}) + "\n" +
		section("Modal Controls", [][2]string{
			{"Escape", "Cancel / close modal"},
			{"Ctrl+Enter", "Confirm action"},
			{"Ctrl+O", "Browse for file (in upload/download)"},
			{"Ctrl+L", "Browse for folder (in upload)"},
		}) + "\n" +
		section("Tips", [][2]string{
			{"", "Multi-select with Space, then use D/Del/M/C for batch ops"},
			{"", "Rename is disabled when multiple items selected"},
			{"", "Sort is disabled when multiple items selected"},
		})
}
