package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/madstone-tech/flint/internal/awsconf"
	"github.com/madstone-tech/flint/internal/logging"
	"github.com/madstone-tech/flint/internal/token"
)

// envVar is the environment variable the Bedrock SDKs and AWS CLI read.
const envVar = "AWS_BEARER_TOKEN_BEDROCK"

// tokenOptions holds everything the token command needs, already decoupled from
// Cobra. The Cobra wrapper (token_cobra.go) populates this and the business
// logic below consumes it, so the logic can be tested without constructing a
// command.
type tokenOptions struct {
	// Profile is the shared-config profile. An empty value means the default
	// chain.
	Profile string
	Region  string

	Duration time.Duration

	// Output mode. At most one of these is true (enforced by the wrapper).
	JSON bool
	Raw  bool
	Exec bool

	// ExecArgv is the command (and args) to run in Exec mode.
	ExecArgv []string
}

// profilePicker returns a chosen profile name, or an error (for example
// ui.ErrAborted when the user cancels). flint invokes it only when the user
// supplies no profile and the session is interactive.
type profilePicker func() (string, error)

// deps groups runToken's injectable collaborators so tests can substitute
// fakes and the signature stays small. A zero-value field falls back to a
// sensible default inside runToken.
type deps struct {
	// resolve turns a profile into credentials. A nil value uses the default
	// SDK resolver.
	resolve awsconf.Resolver
	// pick chooses a profile interactively. A nil value disables the picker,
	// so flint uses the default credential chain when no profile is given.
	pick profilePicker
	// log receives diagnostics. A nil value uses a discard logger.
	log *slog.Logger
}

// newResolver constructs the default credential resolver. It is a package
// variable so an integration test can inject a fake through the full command
// tree without a live AWS environment. Production always uses the SDK resolver.
var newResolver = func() awsconf.Resolver { return awsconf.NewSDKResolver() }

// runToken is the Cobra-free entry point for the token command. It writes its
// normal output to w and returns any error. ctx carries cancellation. d holds
// injectable collaborators, so tests need no live AWS and no terminal.
func runToken(ctx context.Context, w io.Writer, opts tokenOptions, d deps) error {
	if d.resolve == nil {
		d.resolve = newResolver()
	}
	if d.log == nil {
		d.log = logging.Discard()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	log := d.log

	// If no profile was given and an interactive picker is available, use it.
	profile := opts.Profile
	switch {
	case profile != "":
		log.Debug("profile supplied on command line", "profile", profile)
	case d.pick != nil:
		log.Debug("no profile given; launching interactive picker")
		chosen, err := d.pick()
		if err != nil {
			log.Debug("profile selection failed", "error", err)
			return err
		}
		log.Info("profile selected", "profile", chosen)
		profile = chosen
	default:
		log.Debug("no profile given and no interactive picker; using default credential chain")
	}

	log.Debug("resolving credentials", "profile", profile, "region_override", opts.Region)
	resolved, err := d.resolve.Resolve(ctx, awsconf.Options{
		Profile: profile,
		Region:  opts.Region,
	})
	if err != nil {
		log.Error("credential resolution failed", "profile", profile, "error", err)
		return credentialError(err, profile)
	}
	log.Info("credentials resolved", "region", resolved.Region, "expires", credExpiryField(resolved.Credentials.Expires))

	effective := token.ClampDuration(opts.Duration)
	if opts.Duration != 0 && effective != opts.Duration {
		log.Debug("duration clamped to policy", "requested", opts.Duration, "effective", effective)
	}

	tok, err := token.Generate(ctx, resolved.Credentials, resolved.Region, opts.Duration)
	if err != nil {
		log.Error("token generation failed", "region", resolved.Region, "error", err)
		return withExit(exitGeneric, "", err)
	}

	expiry := effectiveExpiry(opts.Duration, resolved.Credentials.Expires)
	log.Info("token minted",
		"region", resolved.Region,
		"expires", expiry.Format(time.RFC3339),
		"mode", outputMode(opts),
	)

	switch {
	case opts.Exec:
		return runExec(ctx, opts.ExecArgv, tok)
	case opts.JSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(tokenJSON{
			Token:   tok,
			Region:  resolved.Region,
			Expires: expiry.Format(time.RFC3339),
		})
	case opts.Raw:
		_, err = fmt.Fprintln(w, tok)
		return err
	default:
		// Export line for `eval`. The comment carries the human-readable expiry
		// without polluting the exported value.
		if _, err = fmt.Fprintf(w, "export %s=%s\n", envVar, tok); err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "# flint: region=%s expires=%s\n", resolved.Region, expiry.Format(time.RFC3339))
		return err
	}
}

// tokenJSON is the shape emitted by --json.
type tokenJSON struct {
	Token   string `json:"token"`
	Region  string `json:"region"`
	Expires string `json:"expires"`
}

func runExec(ctx context.Context, argv []string, tok string) error {
	// #nosec G204 -- argv comes from the user's own shell command after "--".
	// flint is a local developer tool and the user chooses the command.
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	c.Env = append(os.Environ(), envVar+"="+tok)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr

	err := c.Run()
	if err == nil {
		return nil
	}

	// Propagate the child's exit code so flint acts as a transparent wrapper,
	// like env. If the command exits 2, flint exits 2.
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code := ee.ExitCode()
		if code < 0 {
			// A signal terminated the child. Use the shell convention
			// 128 plus the signal number.
			code = exitGeneric
			if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				code = 128 + int(ws.Signal())
			}
		}
		return withExit(code, "", fmt.Errorf("command %q exited with code %d", argv[0], code))
	}

	// The command did not start (not found, or not executable).
	return withExit(exitGeneric, "ensure the command exists and is on your PATH",
		fmt.Errorf("running %q: %w", argv[0], err))
}

// effectiveExpiry reports when the token stops working. It is the shorter of
// the requested duration (clamped to flint's 5m-12h policy) and the
// credentials' own expiry.
func effectiveExpiry(requested time.Duration, credsExpiry time.Time) time.Time {
	byRequest := time.Now().Add(token.ClampDuration(requested))
	if !credsExpiry.IsZero() && credsExpiry.Before(byRequest) {
		return credsExpiry
	}
	return byRequest
}

// outputMode names the active output mode for logging (never the token value).
func outputMode(opts tokenOptions) string {
	switch {
	case opts.Exec:
		return "exec"
	case opts.JSON:
		return "json"
	case opts.Raw:
		return "raw"
	default:
		return "export"
	}
}

// credExpiryField renders a credentials expiry for logging. It returns "none"
// when the credentials do not expire, such as static keys.
func credExpiryField(t time.Time) string {
	if t.IsZero() {
		return "none"
	}
	return t.Format(time.RFC3339)
}

// splitArgs separates positional args into the profile and, in --exec mode, the
// command that follows "--". dash is the number of args before "--" (cobra's
// ArgsLenAtDash). It is -1 when the user gave no "--". This is a plain
// function, independent of Cobra, so a test can call it directly.
func splitArgs(args []string, dash int, execMode bool) (profile string, execArgv []string, err error) {
	if execMode {
		if dash < 0 {
			return "", nil, fmt.Errorf("--exec requires a command after `--` (e.g. flint --exec my-profile -- aws ...)")
		}
		before, after := args[:dash], args[dash:]
		if len(after) == 0 {
			return "", nil, fmt.Errorf("--exec requires a command after `--`")
		}
		if len(before) > 1 {
			return "", nil, fmt.Errorf("expected at most one profile before `--`, got %d", len(before))
		}
		if len(before) == 1 {
			profile = before[0]
		}
		return profile, after, nil
	}

	positional := args
	if dash >= 0 {
		positional = args[:dash]
	}
	if len(positional) > 1 {
		return "", nil, fmt.Errorf("expected at most one profile argument, got %d", len(positional))
	}
	if len(positional) == 1 {
		profile = positional[0]
	}
	return profile, nil, nil
}
