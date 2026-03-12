package components

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
	"github.com/s3ranger/s3ranger-go/internal/util"
)

const ObjectPageSize = 25

// Messages
type objectsLoadedMsg struct {
	Files             []s3gw.ObjectInfo
	Folders           []s3gw.ObjectInfo
	ContinuationToken string
	HasMore           bool
	Append            bool
	Err               error
}

type ObjectItem struct {
	Key          string
	Name         string
	Type         string
	Size         int64
	SizeStr      string
	LastModified time.Time
	ModifiedStr  string
	IsFolder     bool
	IsParent     bool
	IsSelected   bool
}

type ObjectListModel struct {
	Theme    *theme.Theme
	Gateway  *s3gw.Gateway
	Focused  bool
	Width    int
	Height   int

	BucketName        string
	Prefix            string
	FoldersOnly       bool

	items             []ObjectItem
	cursor            int
	continuationToken string
	hasMore           bool
	isLoading         bool
	isLoadingMore     bool
	enablePagination  bool

	selectedKeys map[string]bool

	sortColumn    int // 0=name, 1=type, 2=modified, 3=size
	sortAscending bool
}

func NewObjectList(t *theme.Theme, gw *s3gw.Gateway, enablePagination bool) ObjectListModel {
	return ObjectListModel{
		Theme:            t,
		Gateway:          gw,
		enablePagination: enablePagination,
		selectedKeys:     make(map[string]bool),
		sortColumn:       0,
		sortAscending:    true,
	}
}

func (m ObjectListModel) loadObjects(append bool) tea.Cmd {
	gw := m.Gateway
	bucket := m.BucketName
	prefix := m.Prefix
	token := ""
	if append {
		token = m.continuationToken
	}
	pageSize := int32(0)
	if m.enablePagination {
		pageSize = ObjectPageSize
	}

	return func() tea.Msg {
		page, err := gw.ListObjectsForPrefix(context.Background(), bucket, prefix, pageSize, token)
		if err != nil {
			return objectsLoadedMsg{Err: err}
		}
		return objectsLoadedMsg{
			Files:             page.Files,
			Folders:           page.Folders,
			ContinuationToken: page.ContinuationToken,
			HasMore:           page.HasMore,
			Append:            append,
		}
	}
}

func (m ObjectListModel) SetBucket(bucket string) (ObjectListModel, tea.Cmd) {
	m.BucketName = bucket
	m.Prefix = ""
	m.items = nil
	m.cursor = 0
	m.selectedKeys = make(map[string]bool)
	m.continuationToken = ""
	m.hasMore = false
	m.isLoading = true
	return m, m.loadObjects(false)
}

func (m ObjectListModel) NavigateInto(folder string) (ObjectListModel, tea.Cmd) {
	m.Prefix = folder
	m.items = nil
	m.cursor = 0
	m.selectedKeys = make(map[string]bool)
	m.continuationToken = ""
	m.hasMore = false
	m.isLoading = true
	return m, m.loadObjects(false)
}

func (m ObjectListModel) NavigateUp() (ObjectListModel, tea.Cmd) {
	m.Prefix = util.ParentPrefix(m.Prefix)
	m.items = nil
	m.cursor = 0
	m.selectedKeys = make(map[string]bool)
	m.continuationToken = ""
	m.hasMore = false
	m.isLoading = true
	return m, m.loadObjects(false)
}

func (m ObjectListModel) Refresh() (ObjectListModel, tea.Cmd) {
	m.items = nil
	m.cursor = 0
	m.selectedKeys = make(map[string]bool)
	m.continuationToken = ""
	m.hasMore = false
	m.isLoading = true
	return m, m.loadObjects(false)
}

func (m ObjectListModel) SelectedCount() int {
	return len(m.selectedKeys)
}

func (m ObjectListModel) AllItems() []ObjectItem {
	return m.items
}

func (m ObjectListModel) SelectedItems() []ObjectItem {
	var selected []ObjectItem
	for _, item := range m.items {
		if m.selectedKeys[item.Key] {
			selected = append(selected, item)
		}
	}
	return selected
}

// CursorItem returns the item under the cursor, excluding parent entries.
// Use CursorItemRaw to include parent entries.
func (m ObjectListModel) CursorItem() *ObjectItem {
	if m.cursor >= 0 && m.cursor < len(m.items) {
		item := m.items[m.cursor]
		if !item.IsParent {
			return &item
		}
	}
	return nil
}

// CursorItemRaw returns the item under the cursor, including parent entries.
func (m ObjectListModel) CursorItemRaw() *ObjectItem {
	if m.cursor >= 0 && m.cursor < len(m.items) {
		item := m.items[m.cursor]
		return &item
	}
	return nil
}

func (m ObjectListModel) Update(msg tea.Msg) (ObjectListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case objectsLoadedMsg:
		m.isLoading = false
		m.isLoadingMore = false
		if msg.Err != nil {
			return m, nil
		}

		var newItems []ObjectItem

		if !msg.Append {
			// Add parent directory entry if in subfolder
			if m.Prefix != "" {
				newItems = append(newItems, ObjectItem{
					Key:      "..",
					Name:     "..",
					IsFolder: true,
					IsParent: true,
				})
			}
		} else {
			newItems = m.items
		}

		// Add folders
		for _, f := range msg.Folders {
			newItems = append(newItems, ObjectItem{
				Key:      f.Key,
				Name:     util.ObjectName(f.Key),
				Type:     "dir",
				IsFolder: true,
			})
		}

		// Add files (skip in foldersOnly mode)
		if !m.FoldersOnly {
			for _, f := range msg.Files {
				newItems = append(newItems, ObjectItem{
					Key:          f.Key,
					Name:         util.ObjectName(f.Key),
					Type:         util.FileExtension(f.Key),
					Size:         f.Size,
					SizeStr:      util.FormatFileSize(f.Size),
					LastModified: f.LastModified,
					ModifiedStr:  f.LastModified.Format("2006-01-02 15:04"),
					IsFolder:     false,
				})
			}
		}

		m.continuationToken = msg.ContinuationToken
		m.hasMore = msg.HasMore

		m.items = newItems
		m.sortItems()

		return m, nil

	case tea.KeyMsg:
		if m.BucketName == "" {
			return m, nil
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if m.cursor < len(m.items)-1 {
				m.cursor++
				// Pagination trigger
				if m.enablePagination && m.hasMore && !m.isLoadingMore &&
					m.cursor >= len(m.items)-ScrollThreshold {
					m.isLoadingMore = true
					return m, m.loadObjects(true)
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys(" "))):
			if m.cursor >= 0 && m.cursor < len(m.items) {
				item := &m.items[m.cursor]
				if !item.IsParent {
					item.IsSelected = !item.IsSelected
					if item.IsSelected {
						m.selectedKeys[item.Key] = true
					} else {
						delete(m.selectedKeys, item.Key)
					}
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+a"))):
			for i := range m.items {
				if !m.items[i].IsParent {
					m.items[i].IsSelected = true
					m.selectedKeys[m.items[i].Key] = true
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if len(m.selectedKeys) > 0 {
				for i := range m.items {
					m.items[i].IsSelected = false
				}
				m.selectedKeys = make(map[string]bool)
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *ObjectListModel) sortItems() {
	if len(m.items) <= 1 {
		return
	}

	sort.SliceStable(m.items, func(i, j int) bool {
		a, b := m.items[i], m.items[j]

		// Parent always first
		if a.IsParent {
			return true
		}
		if b.IsParent {
			return false
		}

		// Folders before files
		if a.IsFolder != b.IsFolder {
			return a.IsFolder
		}

		var less bool
		switch m.sortColumn {
		case 0: // Name
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case 1: // Type
			less = strings.ToLower(a.Type) < strings.ToLower(b.Type)
		case 2: // Modified
			less = a.LastModified.Before(b.LastModified)
		case 3: // Size
			less = a.Size < b.Size
		default:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}

		if !m.sortAscending {
			return !less
		}
		return less
	})
}

func (m *ObjectListModel) SetSort(column int) {
	if m.sortColumn == column {
		m.sortAscending = !m.sortAscending
	} else {
		m.sortColumn = column
		m.sortAscending = false
	}
	m.sortItems()
}

func (m ObjectListModel) breadcrumbView() string {
	if m.BucketName == "" {
		return m.Theme.DimText.Render("No bucket selected")
	}

	if m.Prefix == "" {
		return m.Theme.Breadcrumb.Render(m.BucketName)
	}

	parts := []string{m.Theme.BreadcrumbDim.Render(m.BucketName)}
	segments := strings.Split(strings.TrimSuffix(m.Prefix, "/"), "/")
	for i, seg := range segments {
		parts = append(parts, m.Theme.BreadcrumbDim.Render("/"))
		if i == len(segments)-1 {
			parts = append(parts, m.Theme.Breadcrumb.Render(seg))
		} else {
			parts = append(parts, m.Theme.BreadcrumbDim.Render(seg))
		}
	}
	return strings.Join(parts, "")
}

func (m ObjectListModel) View() string {
	// Breadcrumb
	bc := m.breadcrumbView()

	// Column header
	colHeader := m.renderColumnHeader()

	// Items
	listHeight := m.Height - 5 // breadcrumb + header + borders
	if m.isLoadingMore {
		listHeight--
	}
	if listHeight < 1 {
		listHeight = 1
	}

	var lines []string
	if m.isLoading {
		lines = append(lines, m.Theme.DimText.Render("  Loading..."))
	} else if len(m.items) == 0 {
		lines = append(lines, m.Theme.DimText.Render("  Empty"))
	} else {
		// Visible window
		start := 0
		if m.cursor >= listHeight {
			start = m.cursor - listHeight + 1
		}
		end := start + listHeight
		if end > len(m.items) {
			end = len(m.items)
		}

		for i := start; i < end; i++ {
			lines = append(lines, m.renderItem(i))
		}
	}

	if m.isLoadingMore {
		lines = append(lines, m.Theme.DimText.Render("  Loading more..."))
	}

	content := bc + "\n" + colHeader + "\n" + strings.Join(lines, "\n")

	borderStyle := m.Theme.PanelBorder
	if m.Focused {
		borderStyle = m.Theme.PanelActive
	}

	return borderStyle.
		Width(m.Width - 2).
		Height(m.Height - 2).
		Render(content)
}

func (m ObjectListModel) renderColumnHeader() string {
	if m.FoldersOnly {
		return m.Theme.DimText.Render("  Name")
	}

	nameW := max(m.Width/3, 10)
	typeW := 8
	modW := 18
	sizeW := 10

	// Sort indicator
	indicator := func(col int) string {
		if m.sortColumn == col {
			if m.sortAscending {
				return " ↑"
			}
			return " ↓"
		}
		return ""
	}

	header := fmt.Sprintf("    %-*s %-*s %-*s %*s",
		nameW, "Name"+indicator(0),
		typeW, "Type"+indicator(1),
		modW, "Modified"+indicator(2),
		sizeW, "Size"+indicator(3),
	)

	return m.Theme.DimText.Render(header)
}

func (m ObjectListModel) renderItem(idx int) string {
	item := m.items[idx]
	isActive := idx == m.cursor && m.Focused

	// Checkbox
	checkbox := "  "
	if !item.IsParent && !m.FoldersOnly {
		if item.IsSelected {
			checkbox = m.Theme.CheckboxActive.Render("✓ ")
		} else {
			checkbox = m.Theme.Checkbox.Render("· ")
		}
	}

	// Icon
	icon := "📄 "
	if item.IsFolder {
		icon = "📁 "
	}
	if item.IsParent {
		icon = "⬆  "
	}

	nameW := max(m.Width/3, 10)

	var line string
	if m.FoldersOnly || item.IsParent {
		line = checkbox + icon + item.Name
	} else {
		typeW := 8
		modW := 18
		sizeW := 10
		name := item.Name
		if len(name) > nameW {
			name = name[:nameW-3] + "..."
		}
		modStr := item.ModifiedStr
		if item.IsFolder {
			modStr = ""
		}
		sizeStr := item.SizeStr
		if item.IsFolder {
			sizeStr = ""
		}
		line = fmt.Sprintf("%s%s%-*s %-*s %-*s %*s",
			checkbox, icon,
			nameW, name,
			typeW, item.Type,
			modW, modStr,
			sizeW, sizeStr,
		)
	}

	style := m.Theme.ListItem
	if isActive {
		style = m.Theme.ListItemActive
	}
	if item.IsSelected {
		style = m.Theme.SelectedItem
	}

	return style.Width(m.Width - 4).Render(line)
}
