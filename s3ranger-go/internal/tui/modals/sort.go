package modals

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
)

type SortSelectedMsg struct {
	Column int // 0-3
}

type SortModel struct {
	Theme    *theme.Theme
	Width    int
	Height   int
	done     bool
	selected int // -1 = no selection, 0-3 = column
}

func NewSort(t *theme.Theme) SortModel {
	return SortModel{Theme: t, selected: -1}
}

func (m SortModel) Init() tea.Cmd {
	return nil
}

func (m SortModel) Update(msg tea.Msg) (SortModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.done = true
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("1"))):
			m.done = true
			m.selected = 0
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("2"))):
			m.done = true
			m.selected = 1
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("3"))):
			m.done = true
			m.selected = 2
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("4"))):
			m.done = true
			m.selected = 3
			return m, nil
		}
	}
	return m, nil
}

func (m SortModel) IsDone() bool {
	return m.done
}

func (m SortModel) SelectedColumn() int {
	return m.selected
}

func (m SortModel) View() string {
	title := m.Theme.ModalTitle.Render("Sort By")

	options := fmt.Sprintf(
		"%s Name\n%s Type\n%s Modified\n%s Size",
		m.Theme.FooterKey.Render("[1]"),
		m.Theme.FooterKey.Render("[2]"),
		m.Theme.FooterKey.Render("[3]"),
		m.Theme.FooterKey.Render("[4]"),
	)

	footer := m.Theme.DimText.Render("[esc] cancel")

	content := fmt.Sprintf("%s\n\n%s\n\n%s", title, options, footer)

	return m.Theme.ModalBox.
		Width(min(40, m.Width-4)).
		Render(content)
}
