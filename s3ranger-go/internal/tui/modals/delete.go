package modals

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
	"github.com/s3ranger/s3ranger-go/internal/util"
)

type DeleteResultMsg struct {
	Err error
}

type DeleteModel struct {
	Theme    *theme.Theme
	Gateway  *s3gw.Gateway
	Bucket   string
	Key      string
	IsFolder bool
	Width    int
	Height   int

	spinner    spinner.Model
	inProgress bool
	done       bool
}

func NewDelete(t *theme.Theme, gw *s3gw.Gateway, bucket, key string, isFolder bool) DeleteModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(t.Primary)

	return DeleteModel{
		Theme:    t,
		Gateway:  gw,
		Bucket:   bucket,
		Key:      key,
		IsFolder: isFolder,
		spinner:  s,
	}
}

func (m DeleteModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m DeleteModel) Update(msg tea.Msg) (DeleteModel, tea.Cmd) {
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

	case DeleteResultMsg:
		m.done = true
		return m, func() tea.Msg { return msg }
	}

	return m, nil
}

func (m DeleteModel) executeDelete() tea.Cmd {
	gw := m.Gateway
	bucket := m.Bucket
	objKey := m.Key
	isFolder := m.IsFolder

	return func() tea.Msg {
		ctx := context.Background()
		var err error
		if isFolder {
			err = gw.DeleteDirectory(ctx, bucket, objKey)
		} else {
			err = gw.DeleteFile(ctx, bucket, objKey)
		}
		return DeleteResultMsg{Err: err}
	}
}

func (m DeleteModel) IsDone() bool {
	return m.done
}

func (m DeleteModel) View() string {
	itemType := "File"
	if m.IsFolder {
		itemType = "Folder"
	}
	title := m.Theme.ModalTitle.Render(fmt.Sprintf("Delete %s", itemType))

	s3Path := util.BuildS3URI(m.Bucket, m.Key)
	pathLine := m.Theme.DimText.Render("Path: ") + s3Path

	warning := m.Theme.WarningText.Render("⚠ This action cannot be undone")

	var content string
	if m.inProgress {
		content = fmt.Sprintf("%s\n\n%s\n\n%s Deleting...", title, pathLine, m.spinner.View())
	} else {
		content = fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s  %s",
			title, pathLine, warning,
			m.Theme.DimText.Render("[esc] cancel"),
			m.Theme.DimText.Render("[enter] confirm"),
		)
	}

	return m.Theme.ModalBox.
		Width(min(60, m.Width-4)).
		Render(content)
}

