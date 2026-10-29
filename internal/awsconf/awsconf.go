// Package awsconf isolates AWS SDK credential resolution so the rest of flint
// depends on a small, testable surface rather than the SDK directly.
package awsconf

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// Resolved holds the output of resolving a profile: concrete credentials and
// the region they should be signed for.
type Resolved struct {
	Credentials aws.Credentials
	Region      string
}

// Options controls how credentials are resolved.
type Options struct {
	// Profile is the shared-config profile name. An empty value means the
	// default credential chain (AWS_PROFILE and environment variables).
	Profile string
	// Region overrides the region resolved from the profile or environment.
	Region string
}

// Resolver turns Options into concrete credentials and a region. It is the
// seam that callers depend on. Tests substitute a fake. Production uses
// SDKResolver.
type Resolver interface {
	Resolve(ctx context.Context, opts Options) (Resolved, error)
}

// ResolverFunc adapts a plain function to the Resolver interface.
type ResolverFunc func(ctx context.Context, opts Options) (Resolved, error)

// Resolve implements Resolver.
func (f ResolverFunc) Resolve(ctx context.Context, opts Options) (Resolved, error) {
	return f(ctx, opts)
}

// Resolve is a package-level convenience that uses the default SDK-backed
// resolver. It keeps the common call site a one-liner.
func Resolve(ctx context.Context, opts Options) (Resolved, error) {
	return NewSDKResolver().Resolve(ctx, opts)
}
