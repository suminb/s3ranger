package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
	"github.com/s3ranger/s3ranger-go/internal/tui/components"
	"github.com/s3ranger/s3ranger-go/internal/tui/modals"
	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
)

type AppConfig struct {
	Gateway           *s3gw.Gateway
	Theme             string
	ProfileDisplay    string
	EndpointURL       string
	EnablePagination  bool
	DownloadDirectory string
	DownloadWarning   string
}

type activeModal int

const (
	noModal activeModal = iota
	deleteModal
	renameModal
	sortModal
	helpModal
	downloadModal
	uploadModal
	multiDeleteModal
	multiDownloadModal
	moveCopyModal
)

type Model struct {
	config  AppConfig
	theme   *theme.Theme
	gateway *s3gw.Gateway

	width  int
	height int

	titleBar   components.TitleBarModel
	footer     components.FooterModel
	bucketList components.BucketListModel
	objectList components.ObjectListModel

	activePanel string // "buckets" or "objects"

	// Modal state
	modal         activeModal
	deleteModel   modals.DeleteModel
	renameModel   modals.RenameModel
	sortModel     modals.SortModel
	helpModel     modals.HelpModel
	downloadModel modals.DownloadModel
	uploadModel   modals.UploadModel
	multiDelModel modals.MultiDeleteModel
	multiDlModel  modals.MultiDownloadModel
	moveCopyModel modals.MoveCopyModel

	notification    string
	notificationErr bool
}

func New(cfg AppConfig) Model {
	t := theme.Get(cfg.Theme)

	m := Model{
		config:      cfg,
		theme:       t,
		gateway:     cfg.Gateway,
		activePanel: "buckets",
		modal:       noModal,
	}

	m.titleBar = components.NewTitleBar(t, cfg.ProfileDisplay, cfg.EndpointURL)
	m.footer = components.NewFooter(t)
	m.bucketList = components.NewBucketList(t, cfg.Gateway, cfg.EnablePagination)
	m.bucketList.Focused = true
	m.objectList = components.NewObjectList(t, cfg.Gateway, cfg.EnablePagination)

	if cfg.DownloadWarning != "" {
		m.notification = cfg.DownloadWarning
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return m.bucketList.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()

		// If move/copy modal is active, pass size
		if m.modal == moveCopyModal {
			m.moveCopyModel.Width = m.width
			m.moveCopyModel.Height = m.height
			var cmd tea.Cmd
			m.moveCopyModel, cmd = m.moveCopyModel.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		// Global quit — always works
		if key.Matches(msg, Keys.Quit) {
			return m, tea.Quit
		}

		// If modal is active, delegate to modal
		if m.modal != noModal {
			return m.updateModal(msg)
		}

		// Global keys
		switch {
		case key.Matches(msg, Keys.Tab):
			m.switchPanel()
			return m, nil

		case key.Matches(msg, Keys.Help):
			m.modal = helpModal
			m.helpModel = modals.NewHelp(m.theme, m.width, m.height)
			m.footer.InModal = true
			return m, m.helpModel.Init()

		case key.Matches(msg, Keys.Refresh):
			if m.activePanel == "buckets" {
				cmd := m.bucketList.Refresh()
				return m, cmd
			}
			var cmd tea.Cmd
			m.objectList, cmd = m.objectList.Refresh()
			return m, cmd
		}

		// Panel-specific keys
		if m.activePanel == "buckets" {
			return m.updateBucketPanel(msg)
		}
		return m.updateObjectPanel(msg)
	}

	// Non-key messages: pass to modal if active
	if m.modal != noModal {
		return m.updateModal(msg)
	}

	// Pass to sub-models
	var cmd tea.Cmd
	m.bucketList, cmd = m.bucketList.Update(msg)
	cmds = append(cmds, cmd)
	m.objectList, cmd = m.objectList.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) switchPanel() {
	if m.activePanel == "buckets" {
		m.activePanel = "objects"
		m.bucketList.Focused = false
		m.objectList.Focused = true
	} else {
		m.activePanel = "buckets"
		m.bucketList.Focused = true
		m.objectList.Focused = false
	}
	m.footer.ActivePanel = m.activePanel
	m.footer.SelectedCount = m.objectList.SelectedCount()
	m.footer.HasItems = len(m.objectList.SelectedItems()) > 0 || m.objectList.CursorItem() != nil
}

func (m Model) updateBucketPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Enter selects bucket
	if key.Matches(msg, Keys.Enter) {
		bucket := m.bucketList.SelectedBucket()
		if bucket != "" {
			var cmd tea.Cmd
			m.objectList, cmd = m.objectList.SetBucket(bucket)
			m.activePanel = "objects"
			m.bucketList.Focused = false
			m.objectList.Focused = true
			m.footer.ActivePanel = "objects"
			return m, cmd
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.bucketList, cmd = m.bucketList.Update(msg)
	return m, cmd
}

func (m Model) updateObjectPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	selectedCount := m.objectList.SelectedCount()

	switch {
	case key.Matches(msg, Keys.Enter):
		item := m.objectList.CursorItemRaw()
		if item != nil {
			if item.IsParent {
				var cmd tea.Cmd
				m.objectList, cmd = m.objectList.NavigateUp()
				return m, cmd
			}
			if item.IsFolder {
				var cmd tea.Cmd
				m.objectList, cmd = m.objectList.NavigateInto(item.Key)
				return m, cmd
			}
		}
		return m, nil

	case key.Matches(msg, Keys.Download):
		if selectedCount > 1 {
			return m.openMultiDownload()
		}
		item := m.objectList.CursorItem()
		if item == nil {
			return m, nil
		}
		if selectedCount == 1 {
			items := m.objectList.SelectedItems()
			if len(items) > 0 {
				item = &items[0]
			}
		}
		return m.openDownload(item)

	case key.Matches(msg, Keys.Upload):
		if selectedCount > 0 {
			return m, nil // Upload disabled with selection
		}
		return m.openUpload()

	case key.Matches(msg, Keys.Delete):
		if selectedCount > 1 {
			return m.openMultiDelete()
		}
		item := m.objectList.CursorItem()
		if item == nil {
			return m, nil
		}
		if selectedCount == 1 {
			items := m.objectList.SelectedItems()
			if len(items) > 0 {
				item = &items[0]
			}
		}
		return m.openDelete(item)

	case key.Matches(msg, Keys.Rename):
		if selectedCount != 1 && (selectedCount > 0 || m.objectList.CursorItem() == nil) {
			return m, nil
		}
		item := m.objectList.CursorItem()
		if selectedCount == 1 {
			items := m.objectList.SelectedItems()
			if len(items) > 0 {
				item = &items[0]
			}
		}
		if item == nil {
			return m, nil
		}
		return m.openRename(item)

	case key.Matches(msg, Keys.Move):
		return m.openMoveCopy(true)

	case key.Matches(msg, Keys.Copy):
		return m.openMoveCopy(false)

	case key.Matches(msg, Keys.Sort):
		if selectedCount > 1 {
			return m, nil
		}
		m.modal = sortModal
		m.sortModel = modals.NewSort(m.theme)
		m.sortModel.Width = m.width
		m.sortModel.Height = m.height
		m.footer.InModal = true
		return m, m.sortModel.Init()
	}

	// Pass to object list for navigation/selection
	var cmd tea.Cmd
	m.objectList, cmd = m.objectList.Update(msg)
	m.footer.SelectedCount = m.objectList.SelectedCount()
	m.footer.HasItems = m.objectList.CursorItem() != nil
	return m, cmd
}

func (m Model) openDelete(item *components.ObjectItem) (tea.Model, tea.Cmd) {
	m.modal = deleteModal
	m.deleteModel = modals.NewDelete(m.theme, m.gateway, m.objectList.BucketName, item.Key, item.IsFolder)
	m.deleteModel.Width = m.width
	m.deleteModel.Height = m.height
	m.footer.InModal = true
	return m, m.deleteModel.Init()
}

func (m Model) openRename(item *components.ObjectItem) (tea.Model, tea.Cmd) {
	// Collect all visible item names for duplicate detection
	var existingNames []string
	for _, it := range m.objectList.AllItems() {
		if !it.IsParent && it.Key != item.Key {
			existingNames = append(existingNames, it.Name)
		}
	}

	m.modal = renameModal
	m.renameModel = modals.NewRename(m.theme, m.gateway, m.objectList.BucketName,
		item.Key, m.objectList.Prefix, item.IsFolder, existingNames)
	m.renameModel.Width = m.width
	m.renameModel.Height = m.height
	m.footer.InModal = true
	return m, m.renameModel.Init()
}

func (m Model) openDownload(item *components.ObjectItem) (tea.Model, tea.Cmd) {
	m.modal = downloadModal
	m.downloadModel = modals.NewDownload(m.theme, m.gateway, m.objectList.BucketName,
		item.Key, item.IsFolder, m.config.DownloadDirectory)
	m.downloadModel.Width = m.width
	m.downloadModel.Height = m.height
	m.footer.InModal = true
	return m, m.downloadModel.Init()
}

func (m Model) openUpload() (tea.Model, tea.Cmd) {
	m.modal = uploadModal
	m.uploadModel = modals.NewUpload(m.theme, m.gateway, m.objectList.BucketName, m.objectList.Prefix)
	m.uploadModel.Width = m.width
	m.uploadModel.Height = m.height
	m.footer.InModal = true
	return m, m.uploadModel.Init()
}

func (m Model) openMultiDelete() (tea.Model, tea.Cmd) {
	selected := m.objectList.SelectedItems()
	var items []modals.MultiDeleteItem
	for _, s := range selected {
		items = append(items, modals.MultiDeleteItem{
			Key:      s.Key,
			Name:     s.Name,
			IsFolder: s.IsFolder,
			SizeStr:  s.SizeStr,
		})
	}

	m.modal = multiDeleteModal
	m.multiDelModel = modals.NewMultiDelete(m.theme, m.gateway, m.objectList.BucketName, items)
	m.multiDelModel.Width = m.width
	m.multiDelModel.Height = m.height
	m.footer.InModal = true
	return m, m.multiDelModel.Init()
}

func (m Model) openMultiDownload() (tea.Model, tea.Cmd) {
	selected := m.objectList.SelectedItems()
	var items []modals.MultiDownloadItem
	for _, s := range selected {
		items = append(items, modals.MultiDownloadItem{
			Key:      s.Key,
			Name:     s.Name,
			IsFolder: s.IsFolder,
			SizeStr:  s.SizeStr,
		})
	}

	m.modal = multiDownloadModal
	m.multiDlModel = modals.NewMultiDownload(m.theme, m.gateway, m.objectList.BucketName,
		items, m.config.DownloadDirectory)
	m.multiDlModel.Width = m.width
	m.multiDlModel.Height = m.height
	m.footer.InModal = true
	return m, m.multiDlModel.Init()
}

func (m Model) openMoveCopy(isMove bool) (tea.Model, tea.Cmd) {
	selected := m.objectList.SelectedItems()
	// If nothing selected, use cursor item
	if len(selected) == 0 {
		item := m.objectList.CursorItem()
		if item == nil {
			return m, nil
		}
		selected = []components.ObjectItem{*item}
	}

	var items []modals.MoveCopyItem
	for _, s := range selected {
		items = append(items, modals.MoveCopyItem{
			Key:      s.Key,
			Name:     s.Name,
			IsFolder: s.IsFolder,
		})
	}

	m.modal = moveCopyModal
	m.moveCopyModel = modals.NewMoveCopy(m.theme, m.gateway, isMove,
		m.objectList.BucketName, items, m.config.EnablePagination)
	m.moveCopyModel.Width = m.width
	m.moveCopyModel.Height = m.height
	m.footer.InModal = true
	return m, m.moveCopyModel.Init()
}

func (m Model) updateModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.modal {
	case deleteModal:
		var cmd tea.Cmd
		m.deleteModel, cmd = m.deleteModel.Update(msg)
		if m.deleteModel.IsDone() {
			m.modal = noModal
			m.footer.InModal = false
			// Check if it was a successful delete
			if delMsg, ok := msg.(modals.DeleteResultMsg); ok && delMsg.Err == nil {
				var refreshCmd tea.Cmd
				m.objectList, refreshCmd = m.objectList.Refresh()
				return m, tea.Batch(cmd, refreshCmd)
			}
		}
		return m, cmd

	case renameModal:
		var cmd tea.Cmd
		m.renameModel, cmd = m.renameModel.Update(msg)
		if m.renameModel.IsDone() {
			m.modal = noModal
			m.footer.InModal = false
			if renMsg, ok := msg.(modals.RenameResultMsg); ok && renMsg.Err == nil {
				var refreshCmd tea.Cmd
				m.objectList, refreshCmd = m.objectList.Refresh()
				return m, tea.Batch(cmd, refreshCmd)
			}
		}
		return m, cmd

	case sortModal:
		var cmd tea.Cmd
		m.sortModel, cmd = m.sortModel.Update(msg)
		if m.sortModel.IsDone() {
			m.modal = noModal
			m.footer.InModal = false
			if col := m.sortModel.SelectedColumn(); col >= 0 {
				m.objectList.SetSort(col)
			}
		}
		return m, cmd

	case helpModal:
		var cmd tea.Cmd
		m.helpModel, cmd = m.helpModel.Update(msg)
		if m.helpModel.IsDone() {
			m.modal = noModal
			m.footer.InModal = false
		}
		return m, cmd

	case downloadModal:
		var cmd tea.Cmd
		m.downloadModel, cmd = m.downloadModel.Update(msg)
		if m.downloadModel.IsDone() {
			m.modal = noModal
			m.footer.InModal = false
		}
		return m, cmd

	case uploadModal:
		var cmd tea.Cmd
		m.uploadModel, cmd = m.uploadModel.Update(msg)
		if m.uploadModel.IsDone() {
			m.modal = noModal
			m.footer.InModal = false
			if uplMsg, ok := msg.(modals.UploadResultMsg); ok && uplMsg.Err == nil {
				var refreshCmd tea.Cmd
				m.objectList, refreshCmd = m.objectList.Refresh()
				return m, tea.Batch(cmd, refreshCmd)
			}
		}
		return m, cmd

	case multiDeleteModal:
		var cmd tea.Cmd
		m.multiDelModel, cmd = m.multiDelModel.Update(msg)
		if m.multiDelModel.IsDone() {
			m.modal = noModal
			m.footer.InModal = false
			if _, ok := msg.(modals.MultiDeleteDoneMsg); ok {
				var refreshCmd tea.Cmd
				m.objectList, refreshCmd = m.objectList.Refresh()
				return m, tea.Batch(cmd, refreshCmd)
			}
		}
		return m, cmd

	case multiDownloadModal:
		var cmd tea.Cmd
		m.multiDlModel, cmd = m.multiDlModel.Update(msg)
		if m.multiDlModel.IsDone() {
			m.modal = noModal
			m.footer.InModal = false
		}
		return m, cmd

	case moveCopyModal:
		var cmd tea.Cmd
		m.moveCopyModel, cmd = m.moveCopyModel.Update(msg)
		if m.moveCopyModel.IsDone() {
			m.modal = noModal
			m.footer.InModal = false
			if mcMsg, ok := msg.(modals.MoveCopyDoneMsg); ok && mcMsg.Err == nil {
				var refreshCmd tea.Cmd
				m.objectList, refreshCmd = m.objectList.Refresh()
				return m, tea.Batch(cmd, refreshCmd)
			}
		}
		return m, cmd
	}

	return m, nil
}

func (m *Model) updateSizes() {
	panelH := m.height - 3 // titlebar(1) + footer(1) + padding
	bucketW := m.width / 3
	objectW := m.width - bucketW

	m.titleBar.Width = m.width
	m.footer.Width = m.width
	m.bucketList.Width = bucketW
	m.bucketList.Height = panelH
	m.objectList.Width = objectW
	m.objectList.Height = panelH
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Move/copy is full-screen
	if m.modal == moveCopyModal {
		return m.moveCopyModel.View()
	}

	// Build base layout
	titleBar := m.titleBar.View()

	m.footer.SelectedCount = m.objectList.SelectedCount()
	m.footer.HasItems = m.objectList.CursorItem() != nil
	footer := m.footer.View()

	panels := lipgloss.JoinHorizontal(lipgloss.Top,
		m.bucketList.View(),
		m.objectList.View(),
	)

	// Notification
	notifLine := ""
	if m.notification != "" {
		style := m.theme.WarningText
		if m.notificationErr {
			style = m.theme.ErrorText
		}
		notifLine = style.Render(m.notification)
	}

	base := titleBar + "\n" + panels
	if notifLine != "" {
		base += "\n" + notifLine
	}
	base += "\n" + footer

	// Modal overlay
	if m.modal != noModal {
		var modalView string
		switch m.modal {
		case deleteModal:
			modalView = m.deleteModel.View()
		case renameModal:
			modalView = m.renameModel.View()
		case sortModal:
			modalView = m.sortModel.View()
		case helpModal:
			modalView = m.helpModel.View()
		case downloadModal:
			modalView = m.downloadModel.View()
		case uploadModal:
			modalView = m.uploadModel.View()
		case multiDeleteModal:
			modalView = m.multiDelModel.View()
		case multiDownloadModal:
			modalView = m.multiDlModel.View()
		}

		if modalView != "" {
			overlay := lipgloss.Place(m.width, m.height,
				lipgloss.Center, lipgloss.Center,
				modalView)
			return overlay
		}
	}

	return base
}

