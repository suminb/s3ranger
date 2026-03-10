package modals

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

type UploadResultMsg struct {
	Err error
}

type UploadModel struct {
	Theme   *theme.Theme
	Gateway *s3gw.Gateway
	Bucket  string
	Prefix  string
	Width   int
	Height  int

	sourceInput   textinput.Model
	spinner       spinner.Model
	inProgress    bool
	done          bool
	validationErr string
}

func NewUpload(t *theme.Theme, gw *s3gw.Gateway, bucket, prefix string) UploadModel {
	ti := textinput.New()
	ti.Placeholder = "Local file or folder path"
	ti.Focus()
	ti.CharLimit = 512

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(t.Primary)

	return UploadModel{
		Theme:       t,
		Gateway:     gw,
		Bucket:      bucket,
		Prefix:      prefix,
		sourceInput: ti,
		spinner:     s,
	}
}

func (m UploadModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m UploadModel) Update(msg tea.Msg) (UploadModel, tea.Cmd) {
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
			if m.validate() {
				m.inProgress = true
				return m, m.executeUpload()
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.sourceInput, cmd = m.sourceInput.Update(msg)
			m.validationErr = ""
			return m, cmd
		}

	case UploadResultMsg:
		m.done = true
		return m, func() tea.Msg { return msg }
	}

	return m, nil
}

func (m *UploadModel) validate() bool {
	src := config.ExpandPath(m.sourceInput.Value())
	if src == "" {
		m.validationErr = "Source path cannot be empty"
		return false
	}

	_, err := os.Stat(src)
	if err != nil {
		m.validationErr = "Source path does not exist"
		return false
	}

	m.validationErr = ""
	return true
}

func (m UploadModel) executeUpload() tea.Cmd {
	gw := m.Gateway
	bucket := m.Bucket
	prefix := m.Prefix
	src := config.ExpandPath(m.sourceInput.Value())

	return func() tea.Msg {
		ctx := context.Background()

		info, err := os.Stat(src)
		if err != nil {
			return UploadResultMsg{Err: err}
		}

		if info.IsDir() {
			dirName := filepath.Base(src)
			dstPrefix := prefix + dirName + "/"
			err = gw.UploadDirectory(ctx, src, bucket, dstPrefix)
		} else {
			key := prefix + filepath.Base(src)
			err = gw.UploadFile(ctx, src, bucket, key)
		}
		return UploadResultMsg{Err: err}
	}
}

func (m UploadModel) IsDone() bool {
	return m.done
}

func (m UploadModel) View() string {
	title := m.Theme.ModalTitle.Render("Upload Files")

	dest := util.BuildS3URI(m.Bucket, m.Prefix)
	destLine := m.Theme.DimText.Render("Destination: ") + dest
	sourceLine := "Source: " + m.sourceInput.View()

	var validLine string
	if m.validationErr != "" {
		validLine = m.Theme.ErrorText.Render(m.validationErr)
	}

	var content string
	if m.inProgress {
		content = fmt.Sprintf("%s\n\n%s\n\n%s Uploading...", title, destLine, m.spinner.View())
	} else {
		content = fmt.Sprintf("%s\n\n%s\n%s", title, sourceLine, destLine)
		if validLine != "" {
			content += "\n" + validLine
		}
		content += fmt.Sprintf("\n\n%s  %s",
			m.Theme.DimText.Render("[esc] cancel"),
			m.Theme.DimText.Render("[enter] confirm"),
		)
	}

	return m.Theme.ModalBox.
		Width(min(65, m.Width-4)).
		Render(content)
}
