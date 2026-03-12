package theme

import (
	"testing"
)

func TestGet_AllThemes(t *testing.T) {
	themes := []struct {
		name   string
		isDark bool
	}{
		{"Github Dark", true},
		{"Dracula", true},
		{"Solarized", true},
		{"Sepia", false},
	}

	for _, tt := range themes {
		t.Run(tt.name, func(t *testing.T) {
			th := Get(tt.name)
			if th == nil {
				t.Fatal("Get returned nil")
			}
			if th.Name != tt.name {
				t.Errorf("Name = %q, want %q", th.Name, tt.name)
			}
			if th.IsDark != tt.isDark {
				t.Errorf("IsDark = %v, want %v", th.IsDark, tt.isDark)
			}
			// Verify core colors are set (non-empty)
			if th.Primary == "" {
				t.Error("Primary color is empty")
			}
			if th.Foreground == "" {
				t.Error("Foreground color is empty")
			}
			if th.Background == "" {
				t.Error("Background color is empty")
			}
			if th.Error == "" {
				t.Error("Error color is empty")
			}
		})
	}
}

func TestGet_UnknownFallsBackToGithubDark(t *testing.T) {
	th := Get("NonExistent")
	if th == nil {
		t.Fatal("Get returned nil for unknown theme")
	}
	if th.Name != "Github Dark" {
		t.Errorf("Fallback theme Name = %q, want %q", th.Name, "Github Dark")
	}
}

func TestGet_EmptyStringFallback(t *testing.T) {
	th := Get("")
	if th.Name != "Github Dark" {
		t.Errorf("Empty string fallback Name = %q, want %q", th.Name, "Github Dark")
	}
}

func TestThemeStylesNotZero(t *testing.T) {
	// Verify that buildStyles populated the style fields
	th := GithubDark()

	// Spot-check a few styles by rendering with them (they shouldn't panic)
	_ = th.TitleBar.Render("test")
	_ = th.PanelBorder.Render("test")
	_ = th.ListItem.Render("test")
	_ = th.ListItemActive.Render("test")
	_ = th.ModalBox.Render("test")
	_ = th.ErrorText.Render("test")
	_ = th.SuccessText.Render("test")
	_ = th.WarningText.Render("test")
	_ = th.Footer.Render("test")
	_ = th.Breadcrumb.Render("test")
}

func TestAllThemeFactories(t *testing.T) {
	factories := []struct {
		name string
		fn   func() *Theme
	}{
		{"GithubDark", GithubDark},
		{"Dracula", Dracula},
		{"Solarized", Solarized},
		{"Sepia", Sepia},
	}

	for _, tt := range factories {
		t.Run(tt.name, func(t *testing.T) {
			th := tt.fn()
			if th == nil {
				t.Fatal("Factory returned nil")
			}
			// Ensure styles work (don't panic)
			_ = th.ModalBox.Render("modal content")
			_ = th.PanelActive.Render("panel")
		})
	}
}
