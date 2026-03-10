package modals

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

type MultiDownloadDoneMsg struct {
	Succeeded int
	Failed    int
}

type multiDownloadTickMsg struct{}

type MultiDownloadItem struct {
	Key      string
	Name     string
	IsFolder bool
	SizeStr  string
}

// multiDownloadState holds shared state between the download goroutine and the UI.
type multiDownloadState struct {
	mu          sync.Mutex
	currentFile int
	totalFiles  int
	currentName string
	progress    *s3gw.DownloadProgress
}

type MultiDownloadModel struct {
	Theme   *theme.Theme
	Gateway *s3gw.Gateway
	Bucket  string
	Items   []MultiDownloadItem
	Width   int
	Height  int

	destInput   textinput.Model
	progressBar progress.Model
	inProgress  bool
	done        bool
	succeeded   int
	failed      int

	resolvedDest     string
	conflicts        []conflictInfo
	confirmOverwrite bool

	cancelCtx context.Context
	cancelFn  context.CancelFunc
	state     *multiDownloadState
}

type conflictInfo struct {
	name string
	size int64
}

func NewMultiDownload(t *theme.Theme, gw *s3gw.Gateway, bucket string, items []MultiDownloadItem, downloadDir string) MultiDownloadModel {
	ti := textinput.New()
	ti.Placeholder = "Destination folder"
	ti.Focus()
	ti.CharLimit = 512
	ti.SetValue(downloadDir)

	bar := progress.New(
		progress.WithSolidFill(string(t.Primary)),
		progress.WithoutPercentage(),
	)

	return MultiDownloadModel{
		Theme:       t,
		Gateway:     gw,
		Bucket:      bucket,
		Items:       items,
		destInput:   ti,
		progressBar: bar,
	}
}

func (m MultiDownloadModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m MultiDownloadModel) Update(msg tea.Msg) (MultiDownloadModel, tea.Cmd) {
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
				m.conflicts = nil
				m.resolvedDest = ""
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				m.confirmOverwrite = false
				m.inProgress = true
				m.state = &multiDownloadState{
					progress: &s3gw.DownloadProgress{},
				}
				m.cancelCtx, m.cancelFn = context.WithCancel(context.Background())
				return m, tea.Batch(m.executeDownload(), m.tickCmd())
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.done = true
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", "ctrl+enter"))):
			dest := config.ExpandPath(m.destInput.Value())
			m.resolvedDest = dest
			var conflicts []conflictInfo
			for _, item := range m.Items {
				if item.IsFolder {
					continue
				}
				localPath := filepath.Join(dest, filepath.Base(item.Key))
				if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
					conflicts = append(conflicts, conflictInfo{
						name: filepath.Base(item.Key),
						size: info.Size(),
					})
				}
			}
			if len(conflicts) > 0 {
				m.conflicts = conflicts
				m.confirmOverwrite = true
				return m, nil
			}
			m.inProgress = true
			m.state = &multiDownloadState{
				progress: &s3gw.DownloadProgress{},
			}
			m.cancelCtx, m.cancelFn = context.WithCancel(context.Background())
			return m, tea.Batch(m.executeDownload(), m.tickCmd())
		default:
			var cmd tea.Cmd
			m.destInput, cmd = m.destInput.Update(msg)
			return m, cmd
		}

	case multiDownloadTickMsg:
		if !m.inProgress {
			return m, nil
		}
		return m, m.tickCmd()

	case MultiDownloadDoneMsg:
		m.inProgress = false
		m.done = true
		m.succeeded = msg.Succeeded
		m.failed = msg.Failed
		return m, func() tea.Msg { return msg }
	}

	return m, nil
}

func (m MultiDownloadModel) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return multiDownloadTickMsg{}
	})
}

func (m MultiDownloadModel) executeDownload() tea.Cmd {
	gw := m.Gateway
	bucket := m.Bucket
	items := make([]MultiDownloadItem, len(m.Items))
	copy(items, m.Items)
	dest := config.ExpandPath(m.destInput.Value())
	ctx := m.cancelCtx
	state := m.state

	return func() tea.Msg {
		succeeded := 0
		failed := 0

		if err := os.MkdirAll(dest, 0755); err != nil {
			return MultiDownloadDoneMsg{Failed: len(items)}
		}

		for i, item := range items {
			fileProgress := &s3gw.DownloadProgress{}
			state.mu.Lock()
			state.currentFile = i + 1
			state.totalFiles = len(items)
			state.currentName = item.Name
			state.progress = fileProgress
			state.mu.Unlock()

			var err error
			if item.IsFolder {
				err = gw.DownloadDirectoryWithProgress(ctx, bucket, item.Key, dest,
					func(fileIdx, totalFiles int, currentFile string, progress *s3gw.DownloadProgress) {
						state.mu.Lock()
						state.currentName = currentFile
						state.progress = progress
						state.mu.Unlock()
					})
			} else {
				err = gw.DownloadFileWithProgress(ctx, bucket, item.Key, dest, fileProgress)
			}
			if err != nil {
				failed++
				if ctx.Err() != nil {
					failed += len(items) - i - 1
					break
				}
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

	modalWidth := min(65, m.Width-4)
	barWidth := modalWidth - 6
	if barWidth < 20 {
		barWidth = 20
	}
	m.progressBar.Width = barWidth

	var content string
	if m.inProgress && m.state != nil {
		m.state.mu.Lock()
		currentFile := m.state.currentFile
		totalFiles := m.state.totalFiles
		currentName := m.state.currentName
		prog := m.state.progress
		m.state.mu.Unlock()

		headerLine := fmt.Sprintf("Downloading %d/%d: %s", currentFile, totalFiles, currentName)

		var bar, statsLine string
		if prog != nil {
			pct := prog.Percent()
			downloaded := prog.BytesDownloaded.Load()
			total := prog.TotalBytes

			bar = m.progressBar.ViewAs(pct)

			if total > 0 {
				statsLine = fmt.Sprintf("%s / %s  •  %s  •  %s remaining",
					util.FormatFileSize(downloaded),
					util.FormatFileSize(total),
					prog.Bandwidth(),
					prog.ETA(),
				)
			} else {
				statsLine = fmt.Sprintf("%s  •  %s",
					util.FormatFileSize(downloaded),
					prog.Bandwidth(),
				)
			}
		}

		content = fmt.Sprintf("%s\n\n%s\n\n%s\n%s\n\n%s",
			title, headerLine,
			bar,
			m.Theme.DimText.Render(statsLine),
			m.Theme.DimText.Render("[esc] cancel"),
		)
	} else if m.confirmOverwrite {
		warningLine := m.Theme.WarningText.Render(
			fmt.Sprintf("⚠ %d files already exist and will be overwritten:", len(m.conflicts)))

		var conflictLines []string
		maxShow := min(10, len(m.conflicts))
		for i := 0; i < maxShow; i++ {
			c := m.conflicts[i]
			conflictLines = append(conflictLines,
				fmt.Sprintf("  %s (%s)", c.name, util.FormatFileSize(c.size)))
		}
		if len(m.conflicts) > maxShow {
			conflictLines = append(conflictLines,
				fmt.Sprintf("  ... and %d more", len(m.conflicts)-maxShow))
		}

		conflictList := strings.Join(conflictLines, "\n")
		content = fmt.Sprintf("%s\n\n%s\n%s\n\n%s  %s",
			title,
			warningLine, conflictList,
			m.Theme.DimText.Render("[esc] back"),
			m.Theme.DimText.Render("[enter] overwrite all"),
		)
	} else {
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

		content = fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s  %s",
			title, countLine, itemList, destLine,
			m.Theme.DimText.Render("[esc] cancel"),
			m.Theme.DimText.Render("[enter] confirm"),
		)
	}

	return m.Theme.ModalBox.
		Width(modalWidth).
		Render(content)
}
