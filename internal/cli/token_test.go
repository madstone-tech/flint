package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/madstone-tech/flint/internal/awsconf"
)

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		dash        int // cobra's ArgsLenAtDash; -1 means no `--`
		execMode    bool
		wantProfile string
		wantExec    []string
		wantErr     bool
	}{
		{name: "no profile", args: nil, dash: -1},
		{name: "single profile", args: []string{"prod"}, dash: -1, wantProfile: "prod"},
		{name: "two profiles is an error", args: []string{"a", "b"}, dash: -1, wantErr: true},
		{
			name:        "exec with profile and command",
			args:        []string{"prod", "aws", "bedrock-runtime", "converse"},
			dash:        1,
			execMode:    true,
			wantProfile: "prod",
			wantExec:    []string{"aws", "bedrock-runtime", "converse"},
		},
		{
			name:     "exec without profile (default chain)",
			args:     []string{"python", "app.py"},
			dash:     0,
			execMode: true,
			wantExec: []string{"python", "app.py"},
		},
		{name: "exec without dash is an error", args: []string{"prod"}, dash: -1, execMode: true, wantErr: true},
		{name: "exec with empty command is an error", args: []string{"prod"}, dash: 1, execMode: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, execArgv, err := splitArgs(tt.args, tt.dash, tt.execMode)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got profile=%q exec=%v", profile, execArgv)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if profile != tt.wantProfile {
				t.Errorf("profile = %q, want %q", profile, tt.wantProfile)
			}
			if len(execArgv) != len(tt.wantExec) {
				t.Fatalf("exec = %v, want %v", execArgv, tt.wantExec)
			}
			for i := range execArgv {
				if execArgv[i] != tt.wantExec[i] {
					t.Errorf("exec[%d] = %q, want %q", i, execArgv[i], tt.wantExec[i])
				}
			}
		})
	}
}

// fakeDeps returns deps whose resolver yields fixed credentials without
// touching AWS, so runToken can be exercised end-to-end in a unit test.
func fakeDeps(region string) deps {
	return deps{
		resolve: awsconf.ResolverFunc(func(_ context.Context, _ awsconf.Options) (awsconf.Resolved, error) {
			return awsconf.Resolved{
				Credentials: aws.Credentials{
					AccessKeyID:     "AKID", //nolint:gosec // test-only placeholder
					SecretAccessKey: "SECRET",
				},
				Region: region,
			}, nil
		}),
	}
}

func TestRunTokenExportMode(t *testing.T) {
	var buf bytes.Buffer
	err := runToken(context.Background(), &buf, tokenOptions{
		Duration: time.Hour,
	}, fakeDeps("us-east-1"))
	if err != nil {
		t.Fatalf("runToken: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "export "+envVar+"=bedrock-api-key-") {
		t.Errorf("missing export line with token prefix, got:\n%s", out)
	}
	if !strings.Contains(out, "region=us-east-1") {
		t.Errorf("missing region comment, got:\n%s", out)
	}
}

func TestRunTokenRawMode(t *testing.T) {
	var buf bytes.Buffer
	err := runToken(context.Background(), &buf, tokenOptions{
		Raw: true,
	}, fakeDeps("eu-west-1"))
	if err != nil {
		t.Fatalf("runToken: %v", err)
	}

	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "bedrock-api-key-") {
		t.Errorf("raw output should be just the token, got %q", out)
	}
	if strings.Contains(out, "export") || strings.Contains(out, "\n") {
		t.Errorf("raw output should be a single bare token, got %q", out)
	}
}

func TestRunTokenJSONMode(t *testing.T) {
	var buf bytes.Buffer
	err := runToken(context.Background(), &buf, tokenOptions{
		JSON: true,
	}, fakeDeps("ap-southeast-2"))
	if err != nil {
		t.Fatalf("runToken: %v", err)
	}

	var got tokenJSON
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if !strings.HasPrefix(got.Token, "bedrock-api-key-") {
		t.Errorf("token = %q, want bedrock-api-key- prefix", got.Token)
	}
	if got.Region != "ap-southeast-2" {
		t.Errorf("region = %q, want ap-southeast-2", got.Region)
	}
	if _, err := time.Parse(time.RFC3339, got.Expires); err != nil {
		t.Errorf("expires %q is not RFC3339: %v", got.Expires, err)
	}
}

func TestRunTokenResolverError(t *testing.T) {
	wantErr := errors.New("sso session expired")
	d := deps{
		resolve: awsconf.ResolverFunc(func(_ context.Context, _ awsconf.Options) (awsconf.Resolved, error) {
			return awsconf.Resolved{}, wantErr
		}),
	}
	err := runToken(context.Background(), &bytes.Buffer{}, tokenOptions{}, d)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected resolver error to propagate, got %v", err)
	}
}

func TestRunTokenUsesPickerWhenNoProfile(t *testing.T) {
	var gotProfile string
	d := fakeDeps("us-east-1")
	// Capture the profile the resolver is asked for.
	d.resolve = awsconf.ResolverFunc(func(_ context.Context, opts awsconf.Options) (awsconf.Resolved, error) {
		gotProfile = opts.Profile
		return awsconf.Resolved{Region: "us-east-1", Credentials: aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "S"}}, nil //nolint:gosec // test placeholder
	})
	d.pick = func() (string, error) { return "chosen-profile", nil }

	err := runToken(context.Background(), &bytes.Buffer{}, tokenOptions{Raw: true}, d)
	if err != nil {
		t.Fatalf("runToken: %v", err)
	}
	if gotProfile != "chosen-profile" {
		t.Errorf("resolver got profile %q, want the picker's choice", gotProfile)
	}
}

func TestRunTokenSkipsPickerWhenProfileGiven(t *testing.T) {
	pickerCalled := false
	d := fakeDeps("us-east-1")
	d.pick = func() (string, error) {
		pickerCalled = true
		return "should-not-be-used", nil
	}

	err := runToken(context.Background(), &bytes.Buffer{}, tokenOptions{Profile: "explicit", Raw: true}, d)
	if err != nil {
		t.Fatalf("runToken: %v", err)
	}
	if pickerCalled {
		t.Error("picker should not run when a profile is supplied")
	}
}

func TestRunTokenPickerError(t *testing.T) {
	wantErr := errors.New("aborted")
	d := fakeDeps("us-east-1")
	d.pick = func() (string, error) { return "", wantErr }

	err := runToken(context.Background(), &bytes.Buffer{}, tokenOptions{Raw: true}, d)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected picker error to propagate, got %v", err)
	}
}

func TestEffectiveExpiry(t *testing.T) {
	now := time.Now()

	// Requested shorter than creds expiry → requested wins.
	got := effectiveExpiry(time.Hour, now.Add(10*time.Hour))
	if got.After(now.Add(time.Hour + time.Minute)) {
		t.Errorf("expected ~1h expiry, got %v", got)
	}

	// Creds expire before the 12h cap → creds expiry wins.
	credsExp := now.Add(30 * time.Minute)
	got = effectiveExpiry(0, credsExp)
	if !got.Equal(credsExp) {
		t.Errorf("expected creds expiry %v, got %v", credsExp, got)
	}

	// Over-cap request is clamped to 12h.
	got = effectiveExpiry(48*time.Hour, time.Time{})
	if got.After(now.Add(12*time.Hour + time.Minute)) {
		t.Errorf("expected 12h cap, got %v", got)
	}

	// Sub-floor request is raised to the 5m floor (not truncated toward zero).
	got = effectiveExpiry(30*time.Second, time.Time{})
	if got.Before(now.Add(5*time.Minute-time.Second)) || got.After(now.Add(5*time.Minute+time.Second)) {
		t.Errorf("expected ~5m floor, got %v", got)
	}
}
