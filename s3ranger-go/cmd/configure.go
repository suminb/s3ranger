package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/s3ranger/s3ranger-go/internal/config"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Interactive configuration wizard",
	RunE:  runConfigure,
}

func init() {
	configureCmd.Flags().StringVar(&flagConfigPath, "config", config.DefaultConfigPath, "Path to config file")
	rootCmd.AddCommand(configureCmd)
}

func runConfigure(cmd *cobra.Command, args []string) error {
	cfg, _ := config.LoadConfig(flagConfigPath)
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("S3 Ranger Configuration")
	fmt.Println("========================")
	fmt.Println()

	// Profile name
	fmt.Printf("AWS Profile Name [%s]: ", defaultStr(cfg.ProfileName, "none"))
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			cfg.ProfileName = v
		}
	}

	// Theme
	fmt.Printf("Theme (%s) [%s]: ", strings.Join(config.ValidThemes, ", "), cfg.Theme)
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			if config.IsValidTheme(v) {
				cfg.Theme = v
			} else {
				fmt.Printf("  Invalid theme '%s', keeping '%s'\n", v, cfg.Theme)
			}
		}
	}

	// Pagination
	pagStr := "true"
	if cfg.EnablePagination != nil && !*cfg.EnablePagination {
		pagStr = "false"
	}
	fmt.Printf("Enable Pagination (true/false) [%s]: ", pagStr)
	if scanner.Scan() {
		if v := strings.TrimSpace(strings.ToLower(scanner.Text())); v != "" {
			if v == "true" || v == "yes" {
				t := true
				cfg.EnablePagination = &t
			} else if v == "false" || v == "no" {
				f := false
				cfg.EnablePagination = &f
			}
		}
	}

	// Download directory
	fmt.Printf("Download Directory [%s]: ", defaultStr(cfg.DownloadDirectory, "~/Downloads"))
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			cfg.DownloadDirectory = v
		}
	}

	if err := config.SaveConfig(flagConfigPath, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	expanded := config.ExpandPath(flagConfigPath)
	fmt.Printf("\nConfiguration saved to %s\n", expanded)
	return nil
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
