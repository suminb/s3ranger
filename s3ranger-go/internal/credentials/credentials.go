package credentials

import (
	"context"
	"fmt"

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

func ProfileDisplayName(creds *ResolvedCredentials) string {
	if creds.AWSAccessKeyID != "" {
		return "custom"
	}
	if creds.ProfileName != "" {
		return creds.ProfileName
	}
	return "default"
}
