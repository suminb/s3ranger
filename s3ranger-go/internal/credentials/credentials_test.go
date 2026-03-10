package credentials

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve_CLICredentials(t *testing.T) {
	creds, err := Resolve(ResolveInput{
		CLIAccessKeyID:     "AKID123",
		CLISecretAccessKey: "SECRET456",
		CLISessionToken:    "TOKEN789",
		CLIProfileName:     "should-be-ignored",
		ConfigProfileName:  "also-ignored",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if creds.Source != "cli-credentials" {
		t.Errorf("Source = %q, want %q", creds.Source, "cli-credentials")
	}
	if creds.AWSAccessKeyID != "AKID123" {
		t.Errorf("AWSAccessKeyID = %q, want %q", creds.AWSAccessKeyID, "AKID123")
	}
	if creds.AWSSecretAccessKey != "SECRET456" {
		t.Errorf("AWSSecretAccessKey = %q, want %q", creds.AWSSecretAccessKey, "SECRET456")
	}
	if creds.AWSSessionToken != "TOKEN789" {
		t.Errorf("AWSSessionToken = %q, want %q", creds.AWSSessionToken, "TOKEN789")
	}
	// Profile should NOT be set when using explicit keys
	if creds.ProfileName != "" {
		t.Errorf("ProfileName = %q, want empty", creds.ProfileName)
	}
}

func TestResolve_CLICredentials_NoToken(t *testing.T) {
	creds, err := Resolve(ResolveInput{
		CLIAccessKeyID:     "AKID",
		CLISecretAccessKey: "SECRET",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if creds.Source != "cli-credentials" {
		t.Errorf("Source = %q, want %q", creds.Source, "cli-credentials")
	}
	if creds.AWSSessionToken != "" {
		t.Errorf("AWSSessionToken = %q, want empty", creds.AWSSessionToken)
	}
}

func TestResolve_CLIProfile(t *testing.T) {
	creds, err := Resolve(ResolveInput{
		CLIProfileName:    "my-cli-profile",
		ConfigProfileName: "config-profile",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if creds.Source != "cli-profile" {
		t.Errorf("Source = %q, want %q", creds.Source, "cli-profile")
	}
	if creds.ProfileName != "my-cli-profile" {
		t.Errorf("ProfileName = %q, want %q", creds.ProfileName, "my-cli-profile")
	}
}

func TestResolve_ConfigProfile(t *testing.T) {
	creds, err := Resolve(ResolveInput{
		ConfigProfileName: "my-config-profile",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if creds.Source != "config-profile" {
		t.Errorf("Source = %q, want %q", creds.Source, "config-profile")
	}
	if creds.ProfileName != "my-config-profile" {
		t.Errorf("ProfileName = %q, want %q", creds.ProfileName, "my-config-profile")
	}
}

func TestResolve_SDKDefault(t *testing.T) {
	creds, err := Resolve(ResolveInput{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if creds.Source != "sdk-default" {
		t.Errorf("Source = %q, want %q", creds.Source, "sdk-default")
	}
}

func TestResolve_Priority_CLIKeysOverProfile(t *testing.T) {
	// CLI keys should win over CLI profile
	creds, _ := Resolve(ResolveInput{
		CLIAccessKeyID:     "AKID",
		CLISecretAccessKey: "SECRET",
		CLIProfileName:     "profile",
		ConfigProfileName:  "config-profile",
	})
	if creds.Source != "cli-credentials" {
		t.Errorf("Source = %q, want cli-credentials (keys should win)", creds.Source)
	}
}

func TestResolve_Priority_CLIProfileOverConfig(t *testing.T) {
	creds, _ := Resolve(ResolveInput{
		CLIProfileName:    "cli-profile",
		ConfigProfileName: "config-profile",
	})
	if creds.Source != "cli-profile" {
		t.Errorf("Source = %q, want cli-profile", creds.Source)
	}
	if creds.ProfileName != "cli-profile" {
		t.Errorf("ProfileName = %q, want %q", creds.ProfileName, "cli-profile")
	}
}

func TestResolve_PartialCLICredentials_OnlyKeyID(t *testing.T) {
	// Only access key without secret key should NOT select cli-credentials
	creds, _ := Resolve(ResolveInput{
		CLIAccessKeyID:    "AKID",
		ConfigProfileName: "fallback",
	})
	if creds.Source == "cli-credentials" {
		t.Error("Partial credentials (key only) should not select cli-credentials")
	}
	if creds.Source != "config-profile" {
		t.Errorf("Source = %q, want config-profile (fallback)", creds.Source)
	}
}

func TestResolve_PartialCLICredentials_OnlySecret(t *testing.T) {
	creds, _ := Resolve(ResolveInput{
		CLISecretAccessKey: "SECRET",
		ConfigProfileName:  "fallback",
	})
	if creds.Source == "cli-credentials" {
		t.Error("Partial credentials (secret only) should not select cli-credentials")
	}
}

func TestValidate_ValidCredentials(t *testing.T) {
	tests := []struct {
		name  string
		creds ResolvedCredentials
	}{
		{"access key pair", ResolvedCredentials{AWSAccessKeyID: "AKID", AWSSecretAccessKey: "SECRET"}},
		{"profile only", ResolvedCredentials{ProfileName: "my-profile"}},
		{"access key pair with profile", ResolvedCredentials{AWSAccessKeyID: "AKID", AWSSecretAccessKey: "SECRET", ProfileName: "p"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.creds.Validate(); err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestValidate_InvalidCredentials(t *testing.T) {
	tests := []struct {
		name    string
		creds   ResolvedCredentials
		wantMsg string
	}{
		{
			"key without secret",
			ResolvedCredentials{AWSAccessKeyID: "AKID"},
			"aws_secret_access_key is required",
		},
		{
			"secret without key",
			ResolvedCredentials{AWSSecretAccessKey: "SECRET"},
			"aws_access_key_id is required",
		},
		{
			"empty credentials",
			ResolvedCredentials{},
			"Credentials required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.creds.Validate()
			if err == nil {
				t.Error("Validate() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Validate() error = %q, want containing %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestLookupEndpointURL(t *testing.T) {
	// Create a temp AWS config file
	tmpDir := t.TempDir()
	awsDir := filepath.Join(tmpDir, ".aws")
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `[default]
region = us-east-1
endpoint_url = http://localhost:9000

[profile staging]
region = eu-west-1
endpoint_url = https://s3.staging.example.com

[profile no-endpoint]
region = us-west-2
`
	cfgPath := filepath.Join(awsDir, "config")
	if err := os.WriteFile(cfgPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Override HOME for this test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	tests := []struct {
		profile string
		want    string
	}{
		{"default", "http://localhost:9000"},
		{"staging", "https://s3.staging.example.com"},
		{"no-endpoint", ""},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			got := LookupEndpointURL(tt.profile)
			if got != tt.want {
				t.Errorf("LookupEndpointURL(%q) = %q, want %q", tt.profile, got, tt.want)
			}
		})
	}
}

func TestLookupEndpointURL_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	got := LookupEndpointURL("default")
	if got != "" {
		t.Errorf("LookupEndpointURL() = %q, want empty when no config file", got)
	}
}

func TestProfileDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		creds *ResolvedCredentials
		want  string
	}{
		{
			"explicit credentials",
			&ResolvedCredentials{AWSAccessKeyID: "AKID", AWSSecretAccessKey: "SECRET"},
			"custom",
		},
		{
			"named profile",
			&ResolvedCredentials{ProfileName: "my-profile"},
			"my-profile",
		},
		{
			"sdk default",
			&ResolvedCredentials{},
			"default",
		},
		{
			"credentials take priority over profile for display",
			&ResolvedCredentials{AWSAccessKeyID: "AKID", ProfileName: "profile"},
			"custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProfileDisplayName(tt.creds)
			if got != tt.want {
				t.Errorf("ProfileDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
