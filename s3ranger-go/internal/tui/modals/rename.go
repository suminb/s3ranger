package modals

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
	"github.com/s3ranger/s3ranger-go/internal/util"
)

type RenameResultMsg struct {
	Err error
}

type RenameModel struct {
	Theme    *theme.Theme
	Gateway  *s3gw.Gateway
	Bucket   string
	Key      string
	Prefix   string
	IsFolder bool
	Width    int
	Height   int

	existingNames map[string]bool
	input         textinput.Model
	spinner       spinner.Model
	inProgress    bool
	done          bool
	validationErr string
}

func NewRename(t *theme.Theme, gw *s3gw.Gateway, bucket, objKey, prefix string, isFolder bool, existingNames []string) RenameModel {
	ti := textinput.New()
	ti.Placeholder = "New name"
	ti.Focus()
	ti.CharLimit = 256
	name := util.ObjectName(objKey)
	ti.SetValue(name)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(t.Primary)

	names := make(map[string]bool)
	for _, n := range existingNames {
		names[n] = true
	}

	return RenameModel{
		Theme:         t,
		Gateway:       gw,
		Bucket:        bucket,
		Key:           objKey,
		Prefix:        prefix,
		IsFolder:      isFolder,
		existingNames: names,
		input:         ti,
		spinner:       s,
	}
}

func (m RenameModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m RenameModel) Update(msg tea.Msg) (RenameModel, tea.Cmd) {
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
				return m, m.executeRename()
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.validate()
			return m, cmd
		}

	case RenameResultMsg:
		m.done = true
		return m, func() tea.Msg { return msg }
	}

	return m, nil
}

func (m *RenameModel) validate() bool {
	newName := strings.TrimSpace(m.input.Value())
	if newName == "" {
		m.validationErr = "Name cannot be empty"
		return false
	}

	currentName := util.ObjectName(m.Key)
	if newName == currentName {
		m.validationErr = "Name is unchanged"
		return false
	}

	// Check for duplicates
	checkName := newName
	if m.IsFolder {
		checkName = newName + "/"
	}
	if m.existingNames[checkName] || m.existingNames[newName] {
		m.validationErr = "An item with this name already exists"
		return false
	}

	m.validationErr = ""
	return true
}

func (m RenameModel) executeRename() tea.Cmd {
	gw := m.Gateway
	bucket := m.Bucket
	oldKey := m.Key
	prefix := m.Prefix
	newName := strings.TrimSpace(m.input.Value())
	isFolder := m.IsFolder

	return func() tea.Msg {
		ctx := context.Background()
		var err error
		if isFolder {
			newPrefix := prefix + newName + "/"
			err = gw.RenameDirectory(ctx, bucket, oldKey, newPrefix)
		} else {
			newKey := prefix + newName
			err = gw.RenameFile(ctx, bucket, oldKey, newKey)
		}
		return RenameResultMsg{Err: err}
	}
}

func (m RenameModel) IsDone() bool {
	return m.done
}

func (m RenameModel) View() string {
	itemType := "File"
	if m.IsFolder {
		itemType = "Folder"
	}
	title := m.Theme.ModalTitle.Render(fmt.Sprintf("Rename %s", itemType))

	currentName := m.Theme.DimText.Render("Current: ") + util.ObjectName(m.Key)
	inputLine := "New name: " + m.input.View()

	var validLine string
	if m.validationErr != "" {
		validLine = m.Theme.ErrorText.Render(m.validationErr)
	}

	folderWarning := ""
	if m.IsFolder {
		folderWarning = m.Theme.WarningText.Render("⚠ Renaming a folder will move all files within it")
	}

	var content string
	if m.inProgress {
		content = fmt.Sprintf("%s\n\n%s\n\n%s Renaming...", title, currentName, m.spinner.View())
	} else {
		content = title + "\n\n" + currentName + "\n" + inputLine
		if validLine != "" {
			content += "\n" + validLine
		}
		if folderWarning != "" {
			content += "\n\n" + folderWarning
		}
		content += fmt.Sprintf("\n\n%s  %s",
			m.Theme.DimText.Render("[esc] cancel"),
			m.Theme.DimText.Render("[enter] confirm"),
		)
	}

	return m.Theme.ModalBox.
		Width(min(60, m.Width-4)).
		Render(content)
}
