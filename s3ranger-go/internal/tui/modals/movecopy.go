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
	"github.com/s3ranger/s3ranger-go/internal/tui/components"
	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
	"github.com/s3ranger/s3ranger-go/internal/util"
)

type MoveCopyDoneMsg struct {
	Err error
}

type MoveCopyItem struct {
	Key      string
	Name     string
	IsFolder bool
}

type MoveCopyModel struct {
	Theme     *theme.Theme
	Gateway   *s3gw.Gateway
	IsMove    bool
	SrcBucket string
	Items     []MoveCopyItem
	Width     int
	Height    int

	bucketList  components.BucketListModel
	objectList  components.ObjectListModel
	activePanel string // "buckets" or "objects"

	spinner    spinner.Model
	inProgress bool
	done       bool
}

func NewMoveCopy(t *theme.Theme, gw *s3gw.Gateway, isMove bool, srcBucket string, items []MoveCopyItem, enablePagination bool) MoveCopyModel {
	bl := components.NewBucketList(t, gw, enablePagination)
	bl.Focused = true

	ol := components.NewObjectList(t, gw, enablePagination)
	ol.FoldersOnly = true

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(t.Primary)

	return MoveCopyModel{
		Theme:       t,
		Gateway:     gw,
		IsMove:      isMove,
		SrcBucket:   srcBucket,
		Items:       items,
		bucketList:  bl,
		objectList:  ol,
		activePanel: "buckets",
		spinner:     s,
	}
}

func (m MoveCopyModel) Init() tea.Cmd {
	return tea.Batch(m.bucketList.Init(), m.spinner.Tick)
}

func (m MoveCopyModel) Update(msg tea.Msg) (MoveCopyModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case MoveCopyDoneMsg:
		m.done = true
		return m, func() tea.Msg { return msg }

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.updateSizes()
		return m, nil

	case tea.KeyMsg:
		if m.inProgress {
			return m, nil
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.done = true
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			if m.activePanel == "buckets" {
				m.activePanel = "objects"
				m.bucketList.Focused = false
				m.objectList.Focused = true
			} else {
				m.activePanel = "buckets"
				m.bucketList.Focused = true
				m.objectList.Focused = false
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+enter", "ctrl+y"))):
			if m.objectList.BucketName != "" {
				if err := m.validateDestination(); err == nil {
					m.inProgress = true
					return m, m.executeMoveCopy()
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if m.activePanel == "buckets" {
				bucket := m.bucketList.SelectedBucket()
				if bucket != "" {
					var cmd tea.Cmd
					m.objectList, cmd = m.objectList.SetBucket(bucket)
					m.activePanel = "objects"
					m.bucketList.Focused = false
					m.objectList.Focused = true
					return m, cmd
				}
			} else {
				item := m.objectList.CursorItemRaw()
				if item != nil && item.IsParent {
					var cmd tea.Cmd
					m.objectList, cmd = m.objectList.NavigateUp()
					return m, cmd
				} else if item != nil && item.IsFolder {
					var cmd tea.Cmd
					m.objectList, cmd = m.objectList.NavigateInto(item.Key)
					return m, cmd
				}
			}
			return m, nil
		}

		// Delegate to active panel
		if m.activePanel == "buckets" {
			var cmd tea.Cmd
			m.bucketList, cmd = m.bucketList.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.objectList, cmd = m.objectList.Update(msg)
		return m, cmd
	}

	// Pass through to sub-models
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.bucketList, cmd = m.bucketList.Update(msg)
	cmds = append(cmds, cmd)
	m.objectList, cmd = m.objectList.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *MoveCopyModel) updateSizes() {
	panelW := m.Width / 2
	panelH := m.Height - 6 // header + footer
	m.bucketList.Width = panelW
	m.bucketList.Height = panelH
	m.objectList.Width = m.Width - panelW
	m.objectList.Height = panelH
}

func (m MoveCopyModel) validateDestination() error {
	if m.objectList.BucketName == "" {
		return fmt.Errorf("no bucket selected")
	}
	// Check source != destination for all items
	for _, item := range m.Items {
		dstKey := m.objectList.Prefix + item.Name
		if item.IsFolder {
			dstKey += "/"
		}
		if m.SrcBucket == m.objectList.BucketName && item.Key == dstKey {
			return fmt.Errorf("source and destination are the same")
		}
	}
	return nil
}

func (m MoveCopyModel) executeMoveCopy() tea.Cmd {
	gw := m.Gateway
	isMove := m.IsMove
	srcBucket := m.SrcBucket
	dstBucket := m.objectList.BucketName
	dstPrefix := m.objectList.Prefix
	items := make([]MoveCopyItem, len(m.Items))
	copy(items, m.Items)

	return func() tea.Msg {
		ctx := context.Background()

		for _, item := range items {
			dstKey := dstPrefix + item.Name
			if item.IsFolder {
				dstKey += "/"
			}

			var err error
			if item.IsFolder {
				if isMove {
					err = gw.MoveDirectory(ctx, srcBucket, item.Key, dstBucket, dstKey)
				} else {
					err = gw.CopyDirectory(ctx, srcBucket, item.Key, dstBucket, dstKey)
				}
			} else {
				if isMove {
					err = gw.MoveFile(ctx, srcBucket, item.Key, dstBucket, dstKey)
				} else {
					err = gw.CopyFile(ctx, srcBucket, item.Key, dstBucket, dstKey)
				}
			}
			if err != nil {
				return MoveCopyDoneMsg{Err: err}
			}
		}

		return MoveCopyDoneMsg{}
	}
}

func (m MoveCopyModel) IsDone() bool {
	return m.done
}

func (m MoveCopyModel) View() string {
	opType := "Copy"
	if m.IsMove {
		opType = "Move"
	}

	// Header banner
	var itemDesc string
	if len(m.Items) == 1 {
		itemDesc = m.Items[0].Name
	} else {
		itemDesc = fmt.Sprintf("%d items", len(m.Items))
	}
	srcPath := util.BuildS3URI(m.SrcBucket, "")
	header := m.Theme.HeaderText.Render(fmt.Sprintf("%s: %s from %s", opType, itemDesc, srcPath))

	if m.IsMove {
		header += "\n" + m.Theme.WarningText.Render("⚠ Source files will be deleted after move")
	}

	if m.inProgress {
		header += "\n\n" + m.spinner.View() + fmt.Sprintf(" %sing...", strings.ToLower(opType))
		return header
	}

	// Destination info
	destInfo := ""
	if m.objectList.BucketName != "" {
		destInfo = m.Theme.DimText.Render("Destination: ") +
			util.BuildS3URI(m.objectList.BucketName, m.objectList.Prefix)
	}

	// Two-panel layout
	m.updateSizes()
	panels := lipgloss.JoinHorizontal(lipgloss.Top,
		m.bucketList.View(),
		m.objectList.View(),
	)

	footer := fmt.Sprintf("%s  %s  %s",
		m.Theme.FooterKey.Render("tab")+" "+m.Theme.FooterDesc.Render("switch panel"),
		m.Theme.FooterKey.Render("ctrl+y")+" "+m.Theme.FooterDesc.Render("confirm"),
		m.Theme.FooterKey.Render("esc")+" "+m.Theme.FooterDesc.Render("cancel"),
	)
	if destInfo != "" {
		footer = destInfo + "\n" + footer
	}

	return header + "\n\n" + panels + "\n" + footer
}
