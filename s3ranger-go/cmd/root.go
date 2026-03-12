package cmd

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/s3ranger/s3ranger-go/internal/config"
	"github.com/s3ranger/s3ranger-go/internal/credentials"
	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
	"github.com/s3ranger/s3ranger-go/internal/tui"
)

const Version = "1.3.0"

var (
	flagEndpointURL      string
	flagRegionName       string
	flagProfileName      string
	flagAccessKeyID      string
	flagSecretAccessKey   string
	flagSessionToken     string
	flagTheme            string
	flagConfigPath       string
	flagEnablePagination  *bool
	flagDownloadDir      string
	flagVersion          bool
)

var rootCmd = &cobra.Command{
	Use:   "s3ranger",
	Short: "S3 Ranger — TUI file manager for S3",
	Long:  "S3 Ranger is a terminal-based file manager for Amazon S3 and S3-compatible services.",
	RunE:  runRoot,
}

func init() {
	rootCmd.Flags().StringVar(&flagEndpointURL, "endpoint-url", "", "Custom S3 endpoint URL")
	rootCmd.Flags().StringVar(&flagRegionName, "region-name", "", "AWS region name")
	rootCmd.Flags().StringVar(&flagProfileName, "profile-name", "", "AWS profile name")
	rootCmd.Flags().StringVar(&flagAccessKeyID, "aws-access-key-id", "", "AWS access key ID")
	rootCmd.Flags().StringVar(&flagSecretAccessKey, "aws-secret-access-key", "", "AWS secret access key")
	rootCmd.Flags().StringVar(&flagSessionToken, "aws-session-token", "", "AWS session token")
	rootCmd.Flags().StringVar(&flagTheme, "theme", "", "UI theme (Github Dark, Dracula, Solarized, Sepia)")
	rootCmd.Flags().StringVar(&flagConfigPath, "config", config.DefaultConfigPath, "Path to config file")
	rootCmd.Flags().StringVar(&flagDownloadDir, "download-directory", "", "Default download directory")
	rootCmd.Flags().BoolVar(&flagVersion, "version", false, "Show version")

	rootCmd.Flags().Bool("enable-pagination", false, "Enable pagination")
	rootCmd.Flags().Bool("disable-pagination", false, "Disable pagination")
}

func Execute() error {
	return rootCmd.Execute()
}

func runRoot(cmd *cobra.Command, args []string) error {
	if flagVersion {
		fmt.Printf("S3 Ranger v%s\n", Version)
		return nil
	}

	cfg, err := config.LoadConfig(flagConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// Apply CLI overrides
	if flagTheme != "" {
		if config.IsValidTheme(flagTheme) {
			cfg.Theme = flagTheme
		} else {
			return fmt.Errorf("invalid theme '%s'. Valid themes: %v", flagTheme, config.ValidThemes)
		}
	}

	enablePagChanged := cmd.Flags().Changed("enable-pagination")
	disablePagChanged := cmd.Flags().Changed("disable-pagination")
	if enablePagChanged {
		t := true
		cfg.EnablePagination = &t
	}
	if disablePagChanged {
		f := false
		cfg.EnablePagination = &f
	}

	downloadDir, downloadWarning := config.ResolveDownloadDirectory(flagDownloadDir, cfg.DownloadDirectory)

	creds, err := credentials.Resolve(credentials.ResolveInput{
		CLIAccessKeyID:     flagAccessKeyID,
		CLISecretAccessKey: flagSecretAccessKey,
		CLISessionToken:    flagSessionToken,
		CLIProfileName:     flagProfileName,
		ConfigProfileName:  cfg.ProfileName,
	})
	if err != nil {
		return fmt.Errorf("resolving credentials: %w", err)
	}

	if err := creds.Validate(); err != nil {
		return err
	}

	// Resolve endpoint URL: CLI flag > ~/.aws/config for profile
	endpointURL := flagEndpointURL
	if endpointURL == "" {
		endpointURL = credentials.LookupEndpointURL(credentials.ProfileDisplayName(creds))
	}

	ctx := context.Background()
	awsCfg, err := credentials.BuildAWSConfig(ctx, creds, flagRegionName, endpointURL)
	if err != nil {
		return fmt.Errorf("building AWS config: %w", err)
	}

	gw := s3gw.NewGateway(awsCfg, endpointURL)

	appModel := tui.New(tui.AppConfig{
		Gateway:           gw,
		Theme:             cfg.Theme,
		ProfileDisplay:    credentials.ProfileDisplayName(creds),
		EndpointURL:       endpointURL,
		EnablePagination:  config.PaginationEnabled(cfg),
		DownloadDirectory: downloadDir,
		DownloadWarning:   downloadWarning,
	})

	p := tea.NewProgram(appModel, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
