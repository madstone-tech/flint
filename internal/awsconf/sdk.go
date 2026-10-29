package awsconf

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// loadedConfig is the part of aws.Config that SDKResolver needs. It is an
// interface, so a test can fake the SDK's config loading without a live AWS
// environment or network.
type loadedConfig interface {
	region() string
	retrieve(ctx context.Context) (aws.Credentials, error)
}

// loader loads config for the given options. The production loader wraps
// config.LoadDefaultConfig. A test injects its own.
type loader func(ctx context.Context, opts Options) (loadedConfig, error)

// SDKResolver resolves credentials through the standard AWS config chain. The
// chain supports static keys, SSO, assume-role, and environment variables.
type SDKResolver struct {
	load loader
}

// NewSDKResolver returns a resolver backed by the real AWS SDK.
func NewSDKResolver() *SDKResolver {
	return &SDKResolver{load: defaultLoad}
}

// Resolve implements Resolver. The option assembly, region validation and error
// wrapping here are independent of the SDK (the SDK is reachable only through
// the injected loader), so this method is unit-testable with a fake loader.
func (r *SDKResolver) Resolve(ctx context.Context, opts Options) (Resolved, error) {
	cfg, err := r.load(ctx, opts)
	if err != nil {
		return Resolved{}, fmt.Errorf("loading AWS config: %w", err)
	}
	if cfg.region() == "" {
		return Resolved{}, fmt.Errorf("no region resolved; pass --region or set one in the profile/environment")
	}

	creds, err := cfg.retrieve(ctx)
	if err != nil {
		return Resolved{}, fmt.Errorf("retrieving credentials (is the profile signed in? try `aws sso login`): %w", err)
	}

	return Resolved{Credentials: creds, Region: cfg.region()}, nil
}

// defaultLoad is the production loader: it builds the SDK load options from
// Options and calls config.LoadDefaultConfig.
func defaultLoad(ctx context.Context, opts Options) (loadedConfig, error) {
	loadOpts := []func(*config.LoadOptions) error{}
	if opts.Profile != "" {
		loadOpts = append(loadOpts, config.WithSharedConfigProfile(opts.Profile))
	}
	if opts.Region != "" {
		loadOpts = append(loadOpts, config.WithRegion(opts.Region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, err
	}
	return awsConfig{cfg}, nil
}

// awsConfig adapts aws.Config to the loadedConfig interface.
type awsConfig struct {
	cfg aws.Config
}

func (c awsConfig) region() string { return c.cfg.Region }

func (c awsConfig) retrieve(ctx context.Context) (aws.Credentials, error) {
	return c.cfg.Credentials.Retrieve(ctx)
}
