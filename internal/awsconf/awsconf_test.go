package awsconf

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// fakeConfig is a loadedConfig stub for testing SDKResolver without the SDK.
type fakeConfig struct {
	reg       string
	creds     aws.Credentials
	retrieveE error
}

func (f fakeConfig) region() string { return f.reg }

func (f fakeConfig) retrieve(_ context.Context) (aws.Credentials, error) {
	return f.creds, f.retrieveE
}

func resolverWith(load loader) *SDKResolver {
	return &SDKResolver{load: load}
}

func TestSDKResolverSuccess(t *testing.T) {
	want := aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET"} //nolint:gosec // test placeholder
	r := resolverWith(func(_ context.Context, _ Options) (loadedConfig, error) {
		return fakeConfig{reg: "us-east-1", creds: want}, nil
	})

	got, err := r.Resolve(context.Background(), Options{Profile: "p"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Region != "us-east-1" {
		t.Errorf("region = %q, want us-east-1", got.Region)
	}
	if got.Credentials.AccessKeyID != want.AccessKeyID {
		t.Errorf("creds = %+v, want %+v", got.Credentials, want)
	}
}

func TestSDKResolverLoadError(t *testing.T) {
	boom := errors.New("bad profile")
	r := resolverWith(func(_ context.Context, _ Options) (loadedConfig, error) {
		return nil, boom
	})

	_, err := r.Resolve(context.Background(), Options{})
	if !errors.Is(err, boom) {
		t.Errorf("expected load error to wrap %v, got %v", boom, err)
	}
}

func TestSDKResolverNoRegion(t *testing.T) {
	r := resolverWith(func(_ context.Context, _ Options) (loadedConfig, error) {
		return fakeConfig{reg: ""}, nil
	})

	_, err := r.Resolve(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "no region resolved") {
		t.Errorf("expected 'no region resolved' error, got %v", err)
	}
}

func TestSDKResolverRetrieveError(t *testing.T) {
	boom := errors.New("sso expired")
	r := resolverWith(func(_ context.Context, _ Options) (loadedConfig, error) {
		return fakeConfig{reg: "eu-west-1", retrieveE: boom}, nil
	})

	_, err := r.Resolve(context.Background(), Options{})
	if !errors.Is(err, boom) {
		t.Errorf("expected retrieve error to wrap %v, got %v", boom, err)
	}
}

func TestResolverFuncAdapter(t *testing.T) {
	called := false
	var rf Resolver = ResolverFunc(func(_ context.Context, _ Options) (Resolved, error) {
		called = true
		return Resolved{Region: "ap-south-1"}, nil
	})

	got, err := rf.Resolve(context.Background(), Options{})
	if err != nil || !called || got.Region != "ap-south-1" {
		t.Errorf("ResolverFunc adapter did not delegate correctly: called=%v got=%+v err=%v", called, got, err)
	}
}
