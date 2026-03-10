package modals

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/s3ranger/s3ranger-go/internal/config"
	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
	"github.com/s3ranger/s3ranger-go/internal/util"
)

type DownloadResultMsg struct {
	Err error
}

type DownloadModel struct {
	Theme       *theme.Theme
	Gateway     *s3gw.Gateway
	Bucket      string
	Key         string
	IsFolder    bool
	Width       int
	Height      int

	destInput  textinput.Model
	spinner    spinner.Model
	inProgress bool
	done       bool
}

func NewDownload(t *theme.Theme, gw *s3gw.Gateway, bucket, objKey string, isFolder bool, downloadDir string) DownloadModel {
	ti := textinput.New()
	ti.Placeholder = "Destination path"
	ti.Focus()
	ti.CharLimit = 512
	ti.SetValue(downloadDir)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(t.Primary)

	return DownloadModel{
		Theme:     t,
		Gateway:   gw,
		Bucket:    bucket,
		Key:       objKey,
		IsFolder:  isFolder,
		destInput: ti,
		spinner:   s,
	}
}

func (m DownloadModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m DownloadModel) Update(msg tea.Msg) (DownloadModel, tea.Cmd) {
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
			return m, m.executeDownload()
		default:
			var cmd tea.Cmd
			m.destInput, cmd = m.destInput.Update(msg)
			return m, cmd
		}

	case DownloadResultMsg:
		m.done = true
		return m, func() tea.Msg { return msg }
	}

	return m, nil
}

func (m DownloadModel) executeDownload() tea.Cmd {
	gw := m.Gateway
	bucket := m.Bucket
	objKey := m.Key
	isFolder := m.IsFolder
	dest := config.ExpandPath(m.destInput.Value())

	return func() tea.Msg {
		ctx := context.Background()
		var err error
		if isFolder {
			err = gw.DownloadDirectory(ctx, bucket, objKey, dest)
		} else {
			err = gw.DownloadFile(ctx, bucket, objKey, dest)
		}
		return DownloadResultMsg{Err: err}
	}
}

func (m DownloadModel) IsDone() bool {
	return m.done
}

func (m DownloadModel) View() string {
	title := m.Theme.ModalTitle.Render("Download Files")

	s3Path := util.BuildS3URI(m.Bucket, m.Key)
	sourceLine := m.Theme.DimText.Render("Source: ") + s3Path
	destLine := "Destination: " + m.destInput.View()

	var content string
	if m.inProgress {
		content = fmt.Sprintf("%s\n\n%s\n\n%s Downloading...", title, sourceLine, m.spinner.View())
	} else {
		content = fmt.Sprintf("%s\n\n%s\n%s\n\n%s  %s",
			title, sourceLine, destLine,
			m.Theme.DimText.Render("[esc] cancel"),
			m.Theme.DimText.Render("[enter] confirm"),
		)
	}

	return m.Theme.ModalBox.
		Width(min(65, m.Width-4)).
		Render(content)
}
