// Package cli wires flint's command-line interface using Cobra.
package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/madstone-tech/flint/internal/logging"
	"github.com/madstone-tech/flint/internal/ui"
	"github.com/madstone-tech/flint/internal/version"
)

// globalFlags holds options shared by every command.
type globalFlags struct {
	verbose   bool
	logFormat string
}

// loggerKey stores the per-invocation logger on the command context.
type loggerKey struct{}

// loggerFrom returns the logger stored on the command's context. It returns a
// discard logger when none is set, such as a test that calls RunE directly.
func loggerFrom(cmd *cobra.Command) *slog.Logger {
	if cmd != nil {
		if l, ok := cmd.Context().Value(loggerKey{}).(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return logging.Discard()
}

// Execute runs the root command and exits with the mapped code. A SIGINT or
// SIGTERM cancels the command's context. The cancellation aborts in-flight
// credential calls and, in --exec mode, the child process. All diagnostics go
// to stderr. stdout carries only the token.
func Execute() {
	os.Exit(run())
}

// run executes the command tree and returns the process exit code. It is
// separate from Execute, so the deferred signal cleanup runs before exit.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := newRootCmd().ExecuteContext(ctx)
	if err == nil {
		return exitOK
	}

	// The picker abort is a clean, quiet cancellation.
	if errors.Is(err, ui.ErrAborted) {
		fmt.Fprintln(os.Stderr, "flint: cancelled")
		return exitAborted
	}

	// Cobra already printed the error message (SilenceErrors=false). Add a
	// remediation hint when we have one, then return the mapped code.
	if hint := hintFor(err); hint != "" {
		fmt.Fprintf(os.Stderr, "hint: %s\n", hint)
	}
	return codeFor(err)
}

func newRootCmd() *cobra.Command {
	g := &globalFlags{}

	cmd := &cobra.Command{
		Use:   "flint [flags] [profile]",
		Short: "Issue a temporary Amazon Bedrock bearer token from an AWS profile",
		Long: `flint issues a temporary Amazon Bedrock bearer token from an AWS profile,
in the spirit of granted.dev's ` + "`assume`" + `.

A Bedrock short-term API key is not a server-issued resource: it is a SigV4
query-presigned request against bedrock.amazonaws.com, base64-encoded and
prefixed. flint derives it locally from whatever credentials your profile
resolves to (static, SSO, assume-role, env).

The token inherits the resolved principal's permissions, is region-locked, and
is valid for min(--duration, 12h, remaining credential lifetime), where
--duration is clamped to a 5m floor.

By default flint prints a shell export line so it can be used like assume:

    eval "$(flint my-profile)"

which sets AWS_BEARER_TOKEN_BEDROCK for the Bedrock SDKs and AWS CLI.`,
		Example: `  # Export into the current shell
  eval "$(flint my-profile)"

  # Just the token
  flint --raw my-profile

  # JSON for scripts
  flint --json my-profile

  # Run a command with the token injected (nothing hits your shell history)
  flint --exec my-profile -- aws bedrock-runtime converse --model-id ... --region us-east-1`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: false,
		Version:       version.Short(),
		// Build the logger once, before any command runs, and stash it on the
		// context so every command (and the Cobra-free logic) can reach it.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			format, ok := logging.ParseFormat(g.logFormat)
			if !ok {
				return withExit(exitUsage, "use --log-format text or json",
					fmt.Errorf("invalid --log-format %q", g.logFormat))
			}
			logger := logging.New(cmd.ErrOrStderr(), g.verbose, format)
			ctx := context.WithValue(cmd.Context(), loggerKey{}, logger)
			cmd.SetContext(ctx)
			logger.Debug("flint starting",
				"version", version.Short(),
				"command", cmd.Name(),
			)
			return nil
		},
	}

	cmd.SetVersionTemplate(version.String() + "\n")

	// Persistent (global) flags, available on every subcommand. --verbose is
	// long-only to avoid clashing with Cobra's -v (--version) shorthand.
	pf := cmd.PersistentFlags()
	pf.BoolVar(&g.verbose, "verbose", false, "enable diagnostic logging to stderr")
	pf.StringVar(&g.logFormat, "log-format", "text", "log output format: text or json")

	// Flag wiring and RunE live in token_cobra.go so the business logic stays
	// Cobra-free and testable.
	bindTokenCommand(cmd)

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newVerifyCmd())
	cmd.AddCommand(newProfilesCmd())
	cmd.AddCommand(newShellenvCmd())
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print detailed version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version.String())
			return err
		},
	}
}
