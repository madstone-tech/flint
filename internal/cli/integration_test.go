package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/madstone-tech/flint/internal/awsconf"
)

// withFakeResolver swaps the package resolver for the duration of a test so the
// full command tree can run without touching AWS.
func withFakeResolver(t *testing.T, region string) {
	t.Helper()
	prev := newResolver
	newResolver = func() awsconf.Resolver {
		return awsconf.ResolverFunc(func(_ context.Context, _ awsconf.Options) (awsconf.Resolved, error) {
			return awsconf.Resolved{
				Credentials: aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET"}, //nolint:gosec // test placeholder
				Region:      region,
			}, nil
		})
	}
	t.Cleanup(func() { newResolver = prev })
}

// runRoot executes the whole Cobra tree with args and captured I/O.
func runRoot(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := newRootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetIn(strings.NewReader("")) // non-TTY stdin → no picker
	err = cmd.ExecuteContext(context.Background())
	return out.String(), errb.String(), err
}

func TestIntegrationTokenExportLine(t *testing.T) {
	withFakeResolver(t, "us-east-1")

	stdout, _, err := runRoot(t, "my-profile", "--duration", "1h")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout, "export "+envVar+"=bedrock-api-key-") {
		t.Errorf("stdout missing export line:\n%s", stdout)
	}
	if !strings.Contains(stdout, "region=us-east-1") {
		t.Errorf("stdout missing region comment:\n%s", stdout)
	}
}

func TestIntegrationTokenRawCleanStdout(t *testing.T) {
	withFakeResolver(t, "eu-west-1")

	stdout, stderr, err := runRoot(t, "my-profile", "--raw", "--verbose")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	tok := strings.TrimSpace(stdout)
	// --raw stdout must be exactly the token, even with --verbose logging on.
	if !strings.HasPrefix(tok, "bedrock-api-key-") || strings.Contains(tok, "\n") {
		t.Errorf("stdout should be a single bare token, got %q", tok)
	}
	// Verbose logs go to stderr and must not contain the token.
	if stderr == "" {
		t.Error("expected verbose logs on stderr")
	}
	if strings.Contains(stderr, tok) {
		t.Error("token leaked into stderr logs")
	}
}

func TestIntegrationExecExitCode(t *testing.T) {
	withFakeResolver(t, "us-east-1")
	// A child that exits 3 should make flint exit 3.
	_, _, err := runRoot(t, "my-profile", "--exec", "--", "sh", "-c", "exit 3")
	if err == nil {
		t.Fatal("expected non-nil error for non-zero child exit")
	}
	if got := codeFor(err); got != 3 {
		t.Errorf("exit code = %d, want 3 (propagated from child)", got)
	}
}

func TestIntegrationExecSuccess(t *testing.T) {
	withFakeResolver(t, "us-east-1")
	_, _, err := runRoot(t, "my-profile", "--exec", "--", "sh", "-c", "exit 0")
	if err != nil {
		t.Errorf("expected success for child exit 0, got %v", err)
	}
}

func TestIntegrationExecNotFound(t *testing.T) {
	withFakeResolver(t, "us-east-1")
	_, _, err := runRoot(t, "my-profile", "--exec", "--", "this-command-does-not-exist-xyz")
	if err == nil {
		t.Fatal("expected error when command is not found")
	}
}

func TestIntegrationVersion(t *testing.T) {
	stdout, _, err := runRoot(t, "version")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout, "flint ") {
		t.Errorf("version output unexpected:\n%s", stdout)
	}
}

func TestIntegrationShellenv(t *testing.T) {
	stdout, _, err := runRoot(t, "shellenv")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, want := range []string{"bedrock-auth()", "bedrock-run()", "flint --exec"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("shellenv output missing %q:\n%s", want, stdout)
		}
	}
}

func TestIntegrationProfilesEmpty(t *testing.T) {
	// Point profile discovery at empty temp files via env, so the command runs
	// deterministically regardless of the host's ~/.aws.
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/credentials")

	_, _, err := runRoot(t, "profiles")
	if err == nil {
		t.Fatal("expected an error when no profiles are found")
	}
	if codeFor(err) != exitGeneric {
		t.Errorf("exit code = %d, want %d", codeFor(err), exitGeneric)
	}
}

func TestRunProfilesLists(t *testing.T) {
	var buf bytes.Buffer
	list := func() ([]string, error) { return []string{"default", "dev", "prod"}, nil }

	if err := runProfiles(&buf, list); err != nil {
		t.Fatalf("runProfiles: %v", err)
	}
	got := strings.Fields(buf.String())
	want := []string{"default", "dev", "prod"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("profiles = %v, want %v", got, want)
	}
}

func TestWriteShellenv(t *testing.T) {
	var buf bytes.Buffer
	if err := writeShellenv(&buf); err != nil {
		t.Fatalf("writeShellenv: %v", err)
	}
	if !strings.Contains(buf.String(), `eval "$(flint shellenv)"`) {
		t.Errorf("shellenv script missing usage hint")
	}
}
