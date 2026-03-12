package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidTheme(t *testing.T) {
	valid := []string{"Github Dark", "Dracula", "Solarized", "Sepia"}
	for _, theme := range valid {
		if !IsValidTheme(theme) {
			t.Errorf("IsValidTheme(%q) = false, want true", theme)
		}
	}

	invalid := []string{"", "github dark", "DRACULA", "Nord", "Monokai", "github_dark"}
	for _, theme := range invalid {
		if IsValidTheme(theme) {
			t.Errorf("IsValidTheme(%q) = true, want false", theme)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Theme != "Github Dark" {
		t.Errorf("DefaultConfig().Theme = %q, want %q", cfg.Theme, "Github Dark")
	}
	if cfg.EnablePagination == nil || !*cfg.EnablePagination {
		t.Error("DefaultConfig().EnablePagination should be true")
	}
	if cfg.ProfileName != "" {
		t.Errorf("DefaultConfig().ProfileName = %q, want empty", cfg.ProfileName)
	}
	if cfg.DownloadDirectory != "" {
		t.Errorf("DefaultConfig().DownloadDirectory = %q, want empty", cfg.DownloadDirectory)
	}
}

func TestPaginationEnabled(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name string
		cfg  S3Config
		want bool
	}{
		{"nil defaults to true", S3Config{}, true},
		{"explicit true", S3Config{EnablePagination: &trueVal}, true},
		{"explicit false", S3Config{EnablePagination: &falseVal}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PaginationEnabled(tt.cfg)
			if got != tt.want {
				t.Errorf("PaginationEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/file.txt", filepath.Join(home, "file.txt")},
		{"~/path/to/file", filepath.Join(home, "path/to/file")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
		{"~", "~"}, // only "~/" prefix triggers expansion
	}

	for _, tt := range tests {
		got := ExpandPath(tt.input)
		if got != tt.want {
			t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "test.config")

	trueVal := true
	original := S3Config{
		ProfileName:       "my-profile",
		Theme:             "Dracula",
		EnablePagination:  &trueVal,
		DownloadDirectory: "/tmp/downloads",
	}

	// Save
	if err := SaveConfig(cfgPath, original); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("Config file not created: %v", err)
	}

	// Load
	loaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.ProfileName != original.ProfileName {
		t.Errorf("ProfileName = %q, want %q", loaded.ProfileName, original.ProfileName)
	}
	if loaded.Theme != original.Theme {
		t.Errorf("Theme = %q, want %q", loaded.Theme, original.Theme)
	}
	if loaded.EnablePagination == nil || *loaded.EnablePagination != *original.EnablePagination {
		t.Error("EnablePagination mismatch")
	}
	if loaded.DownloadDirectory != original.DownloadDirectory {
		t.Errorf("DownloadDirectory = %q, want %q", loaded.DownloadDirectory, original.DownloadDirectory)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("LoadConfig() should not error for missing file, got: %v", err)
	}

	// Should return defaults
	if cfg.Theme != "Github Dark" {
		t.Errorf("Theme = %q, want %q", cfg.Theme, "Github Dark")
	}
}

func TestLoadConfigInvalidTheme(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "test.config")

	content := `theme = "InvalidTheme"
profile_name = "test"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Invalid theme should be replaced with default
	if cfg.Theme != "Github Dark" {
		t.Errorf("Theme = %q, want %q (should fallback for invalid theme)", cfg.Theme, "Github Dark")
	}
	// Other fields should still be loaded
	if cfg.ProfileName != "test" {
		t.Errorf("ProfileName = %q, want %q", cfg.ProfileName, "test")
	}
}

func TestLoadConfigBadTOML(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "test.config")

	if err := os.WriteFile(cfgPath, []byte("not valid {{{{ toml"), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Error("LoadConfig() should error on invalid TOML")
	}
}

func TestSaveConfigCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "subdir", "nested", "test.config")

	cfg := DefaultConfig()
	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("Config file not created at nested path: %v", err)
	}
}

func TestResolveDownloadDirectory_CLIOverride(t *testing.T) {
	tmpDir := t.TempDir()
	dir, warning := ResolveDownloadDirectory(tmpDir, "/other/path")
	if dir != tmpDir {
		t.Errorf("dir = %q, want %q", dir, tmpDir)
	}
	if warning != "" {
		t.Errorf("warning = %q, want empty", warning)
	}
}

func TestResolveDownloadDirectory_ConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	dir, warning := ResolveDownloadDirectory("", tmpDir)
	if dir != tmpDir {
		t.Errorf("dir = %q, want %q", dir, tmpDir)
	}
	if warning != "" {
		t.Errorf("warning = %q, want empty (dir exists)", warning)
	}
}

func TestResolveDownloadDirectory_ConfigDirNotExist(t *testing.T) {
	dir, warning := ResolveDownloadDirectory("", "/nonexistent/download/path")
	if dir != "/nonexistent/download/path" {
		t.Errorf("dir = %q, want %q", dir, "/nonexistent/download/path")
	}
	if warning == "" {
		t.Error("warning should be non-empty when config dir doesn't exist")
	}
}

func TestResolveDownloadDirectory_DefaultFallback(t *testing.T) {
	dir, _ := ResolveDownloadDirectory("", "")
	// Should resolve to ~/Downloads or cwd — just verify it returns something
	if dir == "" {
		t.Error("dir should not be empty")
	}
}

func TestLoadConfigAllThemes(t *testing.T) {
	tmpDir := t.TempDir()

	for _, theme := range ValidThemes {
		cfgPath := filepath.Join(tmpDir, theme+".config")
		content := `theme = "` + theme + `"`
		if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
			t.Fatalf("writing config for theme %q: %v", theme, err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("LoadConfig() for theme %q error = %v", theme, err)
		}
		if cfg.Theme != theme {
			t.Errorf("Theme = %q, want %q", cfg.Theme, theme)
		}
	}
}

func TestSaveConfigPagination(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "test.config")

	falseVal := false
	cfg := S3Config{
		Theme:            "Solarized",
		EnablePagination: &falseVal,
	}

	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	loaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.EnablePagination == nil {
		t.Fatal("EnablePagination should not be nil")
	}
	if *loaded.EnablePagination {
		t.Error("EnablePagination should be false after save/load round-trip")
	}
}
