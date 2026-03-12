package modals

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
)

type MultiDeleteProgressMsg struct {
	Current int
	Total   int
	Name    string
}

type MultiDeleteDoneMsg struct {
	Succeeded int
	Failed    int
}

type MultiDeleteItem struct {
	Key      string
	Name     string
	IsFolder bool
	SizeStr  string
}

type MultiDeleteModel struct {
	Theme   *theme.Theme
	Gateway *s3gw.Gateway
	Bucket  string
	Items   []MultiDeleteItem
	Width   int
	Height  int

	spinner    spinner.Model
	inProgress bool
	done       bool
	current    int
	total      int
	currentName string
	succeeded  int
	failed     int
}

func NewMultiDelete(t *theme.Theme, gw *s3gw.Gateway, bucket string, items []MultiDeleteItem) MultiDeleteModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(t.Primary)

	return MultiDeleteModel{
		Theme:   t,
		Gateway: gw,
		Bucket:  bucket,
		Items:   items,
		spinner: s,
		total:   len(items),
	}
}

func (m MultiDeleteModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m MultiDeleteModel) Update(msg tea.Msg) (MultiDeleteModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if m.inProgress {
			return m, nil
		}
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.done = true
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", "ctrl+enter"))):
			m.inProgress = true
			return m, m.executeDelete()
		}

	case MultiDeleteProgressMsg:
		m.current = msg.Current
		m.currentName = msg.Name
		return m, nil

	case MultiDeleteDoneMsg:
		m.done = true
		m.succeeded = msg.Succeeded
		m.failed = msg.Failed
		return m, func() tea.Msg { return msg }
	}

	return m, nil
}

func (m MultiDeleteModel) executeDelete() tea.Cmd {
	gw := m.Gateway
	bucket := m.Bucket
	items := make([]MultiDeleteItem, len(m.Items))
	copy(items, m.Items)

	return func() tea.Msg {
		ctx := context.Background()
		succeeded := 0
		failed := 0

		for i, item := range items {
			// We can't send progress messages from within a Cmd easily,
			// so we just do all deletes and report at the end.
			var err error
			if item.IsFolder {
				err = gw.DeleteDirectory(ctx, bucket, item.Key)
			} else {
				err = gw.DeleteFile(ctx, bucket, item.Key)
			}
			if err != nil {
				failed++
			} else {
				succeeded++
			}
			_ = i // progress tracking
		}

		return MultiDeleteDoneMsg{
			Succeeded: succeeded,
			Failed:    failed,
		}
	}
}

func (m MultiDeleteModel) IsDone() bool {
	return m.done
}

func (m MultiDeleteModel) View() string {
	title := m.Theme.ModalTitle.Render("Delete Multiple Items")
	countLine := fmt.Sprintf("%d items selected", len(m.Items))

	var itemLines []string
	maxShow := min(10, len(m.Items))
	for i := 0; i < maxShow; i++ {
		item := m.Items[i]
		icon := "📄"
		if item.IsFolder {
			icon = "📁"
		}
		line := fmt.Sprintf("  %s %s", icon, item.Name)
		if item.SizeStr != "" {
			line += fmt.Sprintf(" (%s)", item.SizeStr)
		}
		itemLines = append(itemLines, line)
	}
	if len(m.Items) > maxShow {
		itemLines = append(itemLines, fmt.Sprintf("  ... and %d more", len(m.Items)-maxShow))
	}

	itemList := strings.Join(itemLines, "\n")
	warning := m.Theme.WarningText.Render("⚠ This action cannot be undone")

	var content string
	if m.inProgress {
		progress := fmt.Sprintf("Deleting %d/%d: %s", m.current+1, m.total, m.currentName)
		content = fmt.Sprintf("%s\n\n%s\n\n%s %s", title, countLine, m.spinner.View(), progress)
	} else {
		content = fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s  %s",
			title, countLine, itemList, warning,
			m.Theme.DimText.Render("[esc] cancel"),
			m.Theme.DimText.Render("[enter] confirm"),
		)
	}

	return m.Theme.ModalBox.
		Width(min(65, m.Width-4)).
		Render(content)
}
