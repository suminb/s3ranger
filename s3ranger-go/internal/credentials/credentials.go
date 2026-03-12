package credentials

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
)

type ResolvedCredentials struct {
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSSessionToken    string
	ProfileName        string
	Source             string
}

// Validate checks that the resolved credentials are complete and consistent.
// Mirrors the Python ResolvedCredentials.validate() behavior.
func (c *ResolvedCredentials) Validate() error {
	if c.AWSAccessKeyID != "" && c.AWSSecretAccessKey == "" {
		return fmt.Errorf("aws_secret_access_key is required when aws_access_key_id is provided")
	}
	if c.AWSSecretAccessKey != "" && c.AWSAccessKeyID == "" {
		return fmt.Errorf("aws_access_key_id is required when aws_secret_access_key is provided")
	}
	if c.AWSAccessKeyID == "" && c.AWSSecretAccessKey == "" && c.ProfileName == "" {
		return fmt.Errorf("Credentials required: provide either a profile_name, or both aws_access_key_id and aws_secret_access_key. These can be specified via CLI arguments or by setting a profile name in the config file.")
	}
	return nil
}

type ResolveInput struct {
	CLIAccessKeyID     string
	CLISecretAccessKey string
	CLISessionToken    string
	CLIProfileName     string
	ConfigProfileName  string
}

func Resolve(input ResolveInput) (*ResolvedCredentials, error) {
	if input.CLIAccessKeyID != "" && input.CLISecretAccessKey != "" {
		return &ResolvedCredentials{
			AWSAccessKeyID:     input.CLIAccessKeyID,
			AWSSecretAccessKey: input.CLISecretAccessKey,
			AWSSessionToken:    input.CLISessionToken,
			Source:             "cli-credentials",
		}, nil
	}

	if input.CLIProfileName != "" {
		return &ResolvedCredentials{
			ProfileName: input.CLIProfileName,
			Source:      "cli-profile",
		}, nil
	}

	if input.ConfigProfileName != "" {
		return &ResolvedCredentials{
			ProfileName: input.ConfigProfileName,
			Source:      "config-profile",
		}, nil
	}

	return &ResolvedCredentials{
		Source: "sdk-default",
	}, nil
}

func BuildAWSConfig(ctx context.Context, creds *ResolvedCredentials, region, endpointURL string) (aws.Config, error) {
	var opts []func(*awsconfig.LoadOptions) error

	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}

	if creds.AWSAccessKeyID != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			awscreds.NewStaticCredentialsProvider(
				creds.AWSAccessKeyID,
				creds.AWSSecretAccessKey,
				creds.AWSSessionToken,
			),
		))
	} else if creds.ProfileName != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(creds.ProfileName))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading AWS config: %w", err)
	}

	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	return cfg, nil
}

// LookupEndpointURL reads the endpoint_url from ~/.aws/config for the given profile.
// Returns empty string if not found or on any error.
func LookupEndpointURL(profileName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	cfgPath := filepath.Join(home, ".aws", "config")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return ""
	}

	// Parse INI-style config
	sectionName := "default"
	if profileName != "" && profileName != "default" {
		sectionName = "profile " + profileName
	}

	inSection := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := line[1 : len(line)-1]
			inSection = (name == sectionName)
			continue
		}

		if inSection && strings.HasPrefix(line, "endpoint_url") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

func ProfileDisplayName(creds *ResolvedCredentials) string {
	if creds.AWSAccessKeyID != "" {
		return "custom"
	}
	if creds.ProfileName != "" {
		return creds.ProfileName
	}
	return "default"
}
