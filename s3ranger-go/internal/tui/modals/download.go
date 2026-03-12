package modals

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/s3ranger/s3ranger-go/internal/config"
	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
	"github.com/s3ranger/s3ranger-go/internal/util"
)

type DownloadResultMsg struct {
	Err error
}

type downloadTickMsg struct{}

type DownloadModel struct {
	Theme    *theme.Theme
	Gateway  *s3gw.Gateway
	Bucket   string
	Key      string
	IsFolder bool
	Width    int
	Height   int

	destInput   textinput.Model
	progressBar progress.Model
	inProgress  bool
	done        bool

	resolvedPath     string
	confirmOverwrite bool
	existingSize     int64

	cancelCtx context.Context
	cancelFn  context.CancelFunc
	progress  *s3gw.DownloadProgress
}

func NewDownload(t *theme.Theme, gw *s3gw.Gateway, bucket, objKey string, isFolder bool, downloadDir string) DownloadModel {
	ti := textinput.New()
	ti.Placeholder = "Destination path"
	ti.Focus()
	ti.CharLimit = 512
	ti.SetValue(downloadDir)

	bar := progress.New(
		progress.WithSolidFill(string(t.Primary)),
		progress.WithoutPercentage(),
	)

	return DownloadModel{
		Theme:       t,
		Gateway:     gw,
		Bucket:      bucket,
		Key:         objKey,
		IsFolder:    isFolder,
		destInput:   ti,
		progressBar: bar,
	}
}

func (m DownloadModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m DownloadModel) Update(msg tea.Msg) (DownloadModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inProgress {
			if key.Matches(msg, key.NewBinding(key.WithKeys("esc"))) {
				m.cancelFn()
			}
			return m, nil
		}
		if m.confirmOverwrite {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.confirmOverwrite = false
				m.resolvedPath = ""
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				m.confirmOverwrite = false
				m.inProgress = true
				m.progress = &s3gw.DownloadProgress{}
				m.cancelCtx, m.cancelFn = context.WithCancel(context.Background())
				return m, tea.Batch(m.executeDownloadPath(m.resolvedPath), m.tickCmd())
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.done = true
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", "ctrl+enter"))):
			resolved := resolveDownloadPath(m.destInput.Value(), m.Key)
			if info, err := os.Stat(resolved); err == nil && !info.IsDir() {
				m.resolvedPath = resolved
				m.existingSize = info.Size()
				m.confirmOverwrite = true
				return m, nil
			}
			m.inProgress = true
			m.progress = &s3gw.DownloadProgress{}
			m.cancelCtx, m.cancelFn = context.WithCancel(context.Background())
			return m, tea.Batch(m.executeDownload(), m.tickCmd())
		default:
			var cmd tea.Cmd
			m.destInput, cmd = m.destInput.Update(msg)
			return m, cmd
		}

	case downloadTickMsg:
		if !m.inProgress {
			return m, nil
		}
		return m, m.tickCmd()

	case DownloadResultMsg:
		m.inProgress = false
		m.done = true
		return m, func() tea.Msg { return msg }
	}

	return m, nil
}

func (m DownloadModel) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return downloadTickMsg{}
	})
}

func resolveDownloadPath(dest, key string) string {
	dest = config.ExpandPath(dest)
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		return filepath.Join(dest, filepath.Base(key))
	}
	if strings.HasSuffix(dest, "/") || strings.HasSuffix(dest, string(os.PathSeparator)) {
		return filepath.Join(dest, filepath.Base(key))
	}
	return dest
}

func (m DownloadModel) executeDownload() tea.Cmd {
	gw := m.Gateway
	bucket := m.Bucket
	objKey := m.Key
	isFolder := m.IsFolder
	dest := config.ExpandPath(m.destInput.Value())
	ctx := m.cancelCtx
	prog := m.progress

	return func() tea.Msg {
		var err error
		if isFolder {
			err = gw.DownloadDirectoryWithProgress(ctx, bucket, objKey, dest, nil)
		} else {
			err = gw.DownloadFileWithProgress(ctx, bucket, objKey, dest, prog)
		}
		return DownloadResultMsg{Err: err}
	}
}

func (m DownloadModel) executeDownloadPath(dest string) tea.Cmd {
	gw := m.Gateway
	bucket := m.Bucket
	objKey := m.Key
	isFolder := m.IsFolder
	ctx := m.cancelCtx
	prog := m.progress

	return func() tea.Msg {
		var err error
		if isFolder {
			err = gw.DownloadDirectoryWithProgress(ctx, bucket, objKey, dest, nil)
		} else {
			err = gw.DownloadFileWithProgress(ctx, bucket, objKey, dest, prog)
		}
		return DownloadResultMsg{Err: err}
	}
}

func (m DownloadModel) IsDone() bool {
	return m.done
}

func (m DownloadModel) View() string {
	title := m.Theme.ModalTitle.Render("Download File")

	s3Path := util.BuildS3URI(m.Bucket, m.Key)
	sourceLine := m.Theme.DimText.Render("Source: ") + s3Path

	modalWidth := min(65, m.Width-4)
	barWidth := modalWidth - 6 // account for modal padding and borders
	if barWidth < 20 {
		barWidth = 20
	}
	m.progressBar.Width = barWidth

	var content string
	if m.inProgress && m.progress != nil {
		pct := m.progress.Percent()
		downloaded := m.progress.BytesDownloaded.Load()
		total := m.progress.TotalBytes

		bar := m.progressBar.ViewAs(pct)

		var statsLine string
		if total > 0 {
			statsLine = fmt.Sprintf("%s / %s  •  %s  •  %s remaining",
				util.FormatFileSize(downloaded),
				util.FormatFileSize(total),
				m.progress.Bandwidth(),
				m.progress.ETA(),
			)
		} else {
			statsLine = fmt.Sprintf("%s  •  %s",
				util.FormatFileSize(downloaded),
				m.progress.Bandwidth(),
			)
		}

		content = fmt.Sprintf("%s\n\n%s\n\n%s\n%s\n\n%s",
			title, sourceLine,
			bar,
			m.Theme.DimText.Render(statsLine),
			m.Theme.DimText.Render("[esc] cancel"),
		)
	} else if m.confirmOverwrite {
		warningLine := m.Theme.WarningText.Render("⚠ File already exists:")
		fileLine := fmt.Sprintf("  %s (%s)", m.resolvedPath, util.FormatFileSize(m.existingSize))

		content = fmt.Sprintf("%s\n\n%s\n\n%s\n%s\n\n%s  %s",
			title, sourceLine,
			warningLine, fileLine,
			m.Theme.DimText.Render("[esc] back"),
			m.Theme.DimText.Render("[enter] overwrite"),
		)
	} else {
		destLine := "Destination: " + m.destInput.View()
		content = fmt.Sprintf("%s\n\n%s\n%s\n\n%s  %s",
			title, sourceLine, destLine,
			m.Theme.DimText.Render("[esc] cancel"),
			m.Theme.DimText.Render("[enter] confirm"),
		)
	}

	return m.Theme.ModalBox.
		Width(modalWidth).
		Render(content)
}
