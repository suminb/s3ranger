package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const DefaultConfigPath = "~/.s3ranger.config"

var ValidThemes = []string{"Github Dark", "Dracula", "Solarized", "Sepia"}

type S3Config struct {
	ProfileName       string `toml:"profile_name"`
	Theme             string `toml:"theme"`
	EnablePagination  *bool  `toml:"enable_pagination"`
	DownloadDirectory string `toml:"download_directory"`
}

func DefaultConfig() S3Config {
	enabled := true
	return S3Config{
		Theme:            "Github Dark",
		EnablePagination: &enabled,
	}
}

func ExpandPath(path string) string {
	path = stripBackslashEscapes(path)

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// stripBackslashEscapes removes shell-style backslash escaping.
// E.g. "file\ name" → "file name", "path\\" → "path\".
func stripBackslashEscapes(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			b.WriteByte(s[i])
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func LoadConfig(path string) (S3Config, error) {
	expanded := ExpandPath(path)
	cfg := DefaultConfig()

	data, err := os.ReadFile(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	if !IsValidTheme(cfg.Theme) {
		cfg.Theme = "Github Dark"
	}

	return cfg, nil
}

func SaveConfig(path string, cfg S3Config) error {
	expanded := ExpandPath(path)

	dir := filepath.Dir(expanded)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	f, err := os.Create(expanded)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

func IsValidTheme(theme string) bool {
	for _, t := range ValidThemes {
		if t == theme {
			return true
		}
	}
	return false
}

func ResolveDownloadDirectory(cliDir, configDir string) (string, string) {
	if cliDir != "" {
		return ExpandPath(cliDir), ""
	}

	if configDir != "" {
		expanded := ExpandPath(configDir)
		if info, err := os.Stat(expanded); err == nil && info.IsDir() {
			return expanded, ""
		}
		return expanded, fmt.Sprintf("Configured download directory '%s' does not exist, using it anyway", configDir)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		downloads := filepath.Join(home, "Downloads")
		if info, err := os.Stat(downloads); err == nil && info.IsDir() {
			return downloads, ""
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ".", "Using current directory as download directory"
	}
	return cwd, "~/Downloads not found, using current directory as download directory"
}

func PaginationEnabled(cfg S3Config) bool {
	if cfg.EnablePagination == nil {
		return true
	}
	return *cfg.EnablePagination
}
