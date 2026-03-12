package components

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
)

const (
	BucketPageSize          = 250
	ScrollThreshold         = 5
	BucketFilterDebounceMs  = 200
)

// Messages specific to BucketList
type bucketsLoadedMsg struct {
	Buckets           []s3gw.BucketInfo
	ContinuationToken string
	HasMore           bool
	Append            bool
	Err               error
}

type bucketFilterTickMsg struct {
	FilterValue string
}

type allBucketsLoadedMsg struct {
	Buckets []s3gw.BucketInfo
	Err     error
}

type BucketListModel struct {
	Theme    *theme.Theme
	Gateway  *s3gw.Gateway
	Focused  bool
	Width    int
	Height   int

	buckets           []s3gw.BucketInfo
	filteredBuckets   []s3gw.BucketInfo
	cursor            int
	continuationToken string
	hasMore           bool
	isLoading         bool
	isLoadingMore     bool
	enablePagination  bool

	filterInput   textinput.Model
	filterFocused bool
	filterValue   string

	// For debounced all-bucket loading when filter is active
	allBucketsLoaded bool
	allBuckets       []s3gw.BucketInfo
}

func NewBucketList(t *theme.Theme, gw *s3gw.Gateway, enablePagination bool) BucketListModel {
	ti := textinput.New()
	ti.Placeholder = "Filter buckets..."
	ti.CharLimit = 256

	return BucketListModel{
		Theme:            t,
		Gateway:          gw,
		enablePagination: enablePagination,
		filterInput:      ti,
	}
}

func (m BucketListModel) Init() tea.Cmd {
	return m.loadBuckets(false)
}

func (m BucketListModel) loadBuckets(append bool) tea.Cmd {
	gw := m.Gateway
	token := ""
	if append {
		token = m.continuationToken
	}
	pageSize := int32(0)
	if m.enablePagination {
		pageSize = BucketPageSize
	}

	return func() tea.Msg {
		page, err := gw.ListBuckets(context.Background(), "", pageSize, token)
		if err != nil {
			return bucketsLoadedMsg{Err: err}
		}
		return bucketsLoadedMsg{
			Buckets:           page.Buckets,
			ContinuationToken: page.ContinuationToken,
			HasMore:           page.HasMore,
			Append:            append,
		}
	}
}

func (m BucketListModel) loadAllBuckets() tea.Cmd {
	gw := m.Gateway
	existing := make([]s3gw.BucketInfo, len(m.buckets))
	copy(existing, m.buckets)
	token := m.continuationToken

	return func() tea.Msg {
		all := existing
		for token != "" {
			page, err := gw.ListBuckets(context.Background(), "", BucketPageSize, token)
			if err != nil {
				return allBucketsLoadedMsg{Buckets: all, Err: err}
			}
			all = append(all, page.Buckets...)
			token = page.ContinuationToken
			if !page.HasMore {
				break
			}
		}
		return allBucketsLoadedMsg{Buckets: all}
	}
}

func (m BucketListModel) Update(msg tea.Msg) (BucketListModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case bucketsLoadedMsg:
		m.isLoading = false
		m.isLoadingMore = false
		if msg.Err != nil {
			return m, nil
		}
		if msg.Append {
			m.buckets = append(m.buckets, msg.Buckets...)
		} else {
			m.buckets = msg.Buckets
			m.cursor = 0
		}
		m.continuationToken = msg.ContinuationToken
		m.hasMore = msg.HasMore
		m.applyFilter()
		return m, nil

	case allBucketsLoadedMsg:
		m.isLoading = false
		if msg.Err != nil {
			return m, nil
		}
		m.allBuckets = msg.Buckets
		m.allBucketsLoaded = true
		m.buckets = msg.Buckets
		m.hasMore = false
		m.continuationToken = ""
		m.applyFilter()
		return m, nil

	case bucketFilterTickMsg:
		if msg.FilterValue != m.filterValue {
			return m, nil
		}
		// If we have more buckets to load for filtering, load them all
		if m.hasMore && !m.allBucketsLoaded {
			m.isLoading = true
			return m, m.loadAllBuckets()
		}
		return m, nil

	case tea.KeyMsg:
		if m.filterFocused {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.filterFocused = false
				m.filterInput.Blur()
				m.filterValue = ""
				m.filterInput.SetValue("")
				m.applyFilter()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				m.filterFocused = false
				m.filterInput.Blur()
				if len(m.filteredBuckets) > 0 {
					m.cursor = 0
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				cmds = append(cmds, cmd)

				newVal := m.filterInput.Value()
				if newVal != m.filterValue {
					m.filterValue = newVal
					m.applyFilter()
					// Debounced server-side fetch
					fv := m.filterValue
					cmds = append(cmds, tea.Tick(BucketFilterDebounceMs*time.Millisecond, func(time.Time) tea.Msg {
						return bucketFilterTickMsg{FilterValue: fv}
					}))
				}
				return m, tea.Batch(cmds...)
			}
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+f"))):
			m.filterFocused = true
			m.filterInput.Focus()
			return m, textinput.Blink

		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if m.cursor < len(m.filteredBuckets)-1 {
				m.cursor++
				// Pagination trigger
				if m.enablePagination && m.hasMore && !m.isLoadingMore &&
					m.cursor >= len(m.filteredBuckets)-ScrollThreshold {
					m.isLoadingMore = true
					return m, m.loadBuckets(true)
				}
			}
			return m, nil
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *BucketListModel) applyFilter() {
	if m.filterValue == "" {
		m.filteredBuckets = m.buckets
		return
	}
	lower := strings.ToLower(m.filterValue)
	m.filteredBuckets = nil
	for _, b := range m.buckets {
		if strings.Contains(strings.ToLower(b.Name), lower) {
			m.filteredBuckets = append(m.filteredBuckets, b)
		}
	}
	if m.cursor >= len(m.filteredBuckets) {
		m.cursor = max(0, len(m.filteredBuckets)-1)
	}
}

func (m BucketListModel) SelectedBucket() string {
	if m.cursor >= 0 && m.cursor < len(m.filteredBuckets) {
		return m.filteredBuckets[m.cursor].Name
	}
	return ""
}

func (m BucketListModel) Refresh() tea.Cmd {
	return m.loadBuckets(false)
}

func (m BucketListModel) View() string {
	// Title
	title := "Buckets"
	total := len(m.buckets)
	if m.filterValue != "" {
		title = fmt.Sprintf("Buckets (%d/%d)", len(m.filteredBuckets), total)
	} else if m.hasMore {
		title = fmt.Sprintf("Buckets (%d+)", total)
	} else {
		title = fmt.Sprintf("Buckets (%d)", total)
	}
	titleStyle := m.Theme.HeaderText
	titleLine := titleStyle.Render(title)

	// Filter input
	filterLine := ""
	if m.filterFocused || m.filterValue != "" {
		filterLine = m.filterInput.View()
	}

	// Bucket list
	listHeight := m.Height - 3 // title + border padding
	if filterLine != "" {
		listHeight--
	}
	if m.isLoadingMore {
		listHeight--
	}
	if listHeight < 1 {
		listHeight = 1
	}

	var lines []string
	if m.isLoading && len(m.filteredBuckets) == 0 {
		lines = append(lines, m.Theme.DimText.Render("  Loading..."))
	} else {
		// Determine visible window
		start := 0
		if m.cursor >= listHeight {
			start = m.cursor - listHeight + 1
		}
		end := start + listHeight
		if end > len(m.filteredBuckets) {
			end = len(m.filteredBuckets)
		}

		for i := start; i < end; i++ {
			b := m.filteredBuckets[i]
			if i == m.cursor && m.Focused {
				lines = append(lines, m.Theme.ListItemActive.Render(" "+b.Name))
			} else {
				lines = append(lines, m.Theme.ListItem.Render(" "+b.Name))
			}
		}
	}

	if m.isLoadingMore {
		lines = append(lines, m.Theme.DimText.Render("  Loading more..."))
	}

	content := titleLine + "\n"
	if filterLine != "" {
		content += filterLine + "\n"
	}
	content += strings.Join(lines, "\n")

	// Panel border
	borderStyle := m.Theme.PanelBorder
	if m.Focused {
		borderStyle = m.Theme.PanelActive
	}

	return borderStyle.
		Width(m.Width - 2).
		Height(m.Height - 2).
		Render(content)
}

