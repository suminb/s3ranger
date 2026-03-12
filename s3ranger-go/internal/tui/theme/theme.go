package theme

import "github.com/charmbracelet/lipgloss"

// Theme holds all colors and derived styles for the TUI.
type Theme struct {
	Name string

	// Core colors
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Accent     lipgloss.Color
	Foreground lipgloss.Color
	Background lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Surface    lipgloss.Color
	Panel      lipgloss.Color
	Border     lipgloss.Color
	Boost      lipgloss.Color
	IsDark     bool

	// Pre-built styles
	TitleBar       lipgloss.Style
	StatusDotOK    lipgloss.Style
	StatusDotErr   lipgloss.Style
	PanelBorder    lipgloss.Style
	PanelActive    lipgloss.Style
	ListItem       lipgloss.Style
	ListItemActive lipgloss.Style
	Breadcrumb     lipgloss.Style
	BreadcrumbDim  lipgloss.Style
	Footer         lipgloss.Style
	FooterKey      lipgloss.Style
	FooterDesc     lipgloss.Style
	ModalBox       lipgloss.Style
	ModalTitle     lipgloss.Style
	InputField     lipgloss.Style
	ButtonPrimary  lipgloss.Style
	ButtonDanger   lipgloss.Style
	WarningText    lipgloss.Style
	ErrorText      lipgloss.Style
	SuccessText    lipgloss.Style
	FolderIcon     lipgloss.Style
	FileIcon       lipgloss.Style
	DimText        lipgloss.Style
	HeaderText     lipgloss.Style
	SelectedItem   lipgloss.Style
	Checkbox       lipgloss.Style
	CheckboxActive lipgloss.Style
}

func buildStyles(t *Theme) {
	t.TitleBar = lipgloss.NewStyle().
		Background(t.Surface).
		Foreground(t.Foreground).
		Bold(true).
		Padding(0, 1)

	t.StatusDotOK = lipgloss.NewStyle().Foreground(t.Success)
	t.StatusDotErr = lipgloss.NewStyle().Foreground(t.Error)

	t.PanelBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Foreground(t.Foreground)

	t.PanelActive = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Foreground(t.Foreground)

	t.ListItem = lipgloss.NewStyle().
		Foreground(t.Foreground).
		Padding(0, 1)

	t.ListItemActive = lipgloss.NewStyle().
		Background(t.Boost).
		Foreground(t.Foreground).
		Bold(true).
		Padding(0, 1)

	t.Breadcrumb = lipgloss.NewStyle().
		Foreground(t.Foreground).
		Bold(true)

	t.BreadcrumbDim = lipgloss.NewStyle().
		Foreground(t.Border)

	t.Footer = lipgloss.NewStyle().
		Background(t.Surface).
		Foreground(t.Foreground).
		Padding(0, 1)

	t.FooterKey = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	t.FooterDesc = lipgloss.NewStyle().
		Foreground(t.Foreground)

	t.ModalBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Background(t.Surface).
		Foreground(t.Foreground).
		Padding(1, 2)

	t.ModalTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	t.InputField = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		Foreground(t.Foreground).
		Padding(0, 1)

	t.ButtonPrimary = lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 2)

	t.ButtonDanger = lipgloss.NewStyle().
		Background(t.Error).
		Foreground(lipgloss.Color("#ffffff")).
		Bold(true).
		Padding(0, 2)

	t.WarningText = lipgloss.NewStyle().Foreground(t.Warning)
	t.ErrorText = lipgloss.NewStyle().Foreground(t.Error)
	t.SuccessText = lipgloss.NewStyle().Foreground(t.Success)

	t.FolderIcon = lipgloss.NewStyle().Foreground(t.Warning)
	t.FileIcon = lipgloss.NewStyle().Foreground(t.Foreground)

	t.DimText = lipgloss.NewStyle().Foreground(t.Border)

	t.HeaderText = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	t.SelectedItem = lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 1)

	t.Checkbox = lipgloss.NewStyle().Foreground(t.Border)
	t.CheckboxActive = lipgloss.NewStyle().Foreground(t.Success).Bold(true)
}

// Get returns a theme by name. Falls back to Github Dark.
func Get(name string) *Theme {
	switch name {
	case "Dracula":
		return Dracula()
	case "Solarized":
		return Solarized()
	case "Sepia":
		return Sepia()
	default:
		return GithubDark()
	}
}
