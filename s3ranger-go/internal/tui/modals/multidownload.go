package modals

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/s3ranger/s3ranger-go/internal/config"
	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
)

type MultiDownloadDoneMsg struct {
	Succeeded int
	Failed    int
}

type MultiDownloadItem struct {
	Key      string
	Name     string
	IsFolder bool
	SizeStr  string
}

type MultiDownloadModel struct {
	Theme       *theme.Theme
	Gateway     *s3gw.Gateway
	Bucket      string
	Items       []MultiDownloadItem
	Width       int
	Height      int

	destInput  textinput.Model
	spinner    spinner.Model
	inProgress bool
	done       bool
	succeeded  int
	failed     int
}

func NewMultiDownload(t *theme.Theme, gw *s3gw.Gateway, bucket string, items []MultiDownloadItem, downloadDir string) MultiDownloadModel {
	ti := textinput.New()
	ti.Placeholder = "Destination folder"
	ti.Focus()
	ti.CharLimit = 512
	ti.SetValue(downloadDir)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(t.Primary)

	return MultiDownloadModel{
		Theme:     t,
		Gateway:   gw,
		Bucket:    bucket,
		Items:     items,
		destInput: ti,
		spinner:   s,
	}
}

func (m MultiDownloadModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m MultiDownloadModel) Update(msg tea.Msg) (MultiDownloadModel, tea.Cmd) {
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

	case MultiDownloadDoneMsg:
		m.done = true
		m.succeeded = msg.Succeeded
		m.failed = msg.Failed
		return m, func() tea.Msg { return msg }
	}

	return m, nil
}

func (m MultiDownloadModel) executeDownload() tea.Cmd {
	gw := m.Gateway
	bucket := m.Bucket
	items := make([]MultiDownloadItem, len(m.Items))
	copy(items, m.Items)
	dest := config.ExpandPath(m.destInput.Value())

	return func() tea.Msg {
		ctx := context.Background()
		succeeded := 0
		failed := 0

		if err := os.MkdirAll(dest, 0755); err != nil {
			return MultiDownloadDoneMsg{Failed: len(items)}
		}

		for _, item := range items {
			var err error
			if item.IsFolder {
				err = gw.DownloadDirectory(ctx, bucket, item.Key, dest)
			} else {
				err = gw.DownloadFile(ctx, bucket, item.Key, dest)
			}
			if err != nil {
				failed++
			} else {
				succeeded++
			}
		}

		return MultiDownloadDoneMsg{
			Succeeded: succeeded,
			Failed:    failed,
		}
	}
}

func (m MultiDownloadModel) IsDone() bool {
	return m.done
}

func (m MultiDownloadModel) View() string {
	title := m.Theme.ModalTitle.Render("Download Multiple Files")
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
	destLine := "Destination: " + m.destInput.View()

	var content string
	if m.inProgress {
		content = fmt.Sprintf("%s\n\n%s\n\n%s Downloading...", title, countLine, m.spinner.View())
	} else {
		content = fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s  %s",
			title, countLine, itemList, destLine,
			m.Theme.DimText.Render("[esc] cancel"),
			m.Theme.DimText.Render("[enter] confirm"),
		)
	}

	return m.Theme.ModalBox.
		Width(min(65, m.Width-4)).
		Render(content)
}
