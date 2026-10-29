package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/madstone-tech/flint/internal/awsprofiles"
	"github.com/madstone-tech/flint/internal/ui"
)

// tokenFlags holds the raw flag values bound by Cobra, before they are turned
// into the Cobra-free tokenOptions.
type tokenFlags struct {
	region   string
	duration time.Duration
	json     bool
	raw      bool
	exec     bool
}

// bindTokenCommand wires flint's token flags onto cmd and sets its RunE to call
// the Cobra-free runToken. This is the only place that touches *cobra.Command
// for the token command, keeping token.go testable in isolation.
func bindTokenCommand(cmd *cobra.Command) {
	f := &tokenFlags{}

	fl := cmd.Flags()
	fl.StringVar(&f.region, "region", "", "AWS region to sign the token for (defaults to the profile/env region)")
	fl.DurationVar(&f.duration, "duration", 0, "requested token lifetime; clamped to 5m–12h (default 12h)")
	fl.BoolVar(&f.json, "json", false, "emit the token and expiry as JSON")
	fl.BoolVar(&f.raw, "raw", false, "print only the raw token (no export prefix)")
	fl.BoolVar(&f.exec, "exec", false, "run a command with the token in its environment: flint --exec profile -- cmd args")

	cmd.MarkFlagsMutuallyExclusive("json", "raw", "exec")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// ArgsLenAtDash is the Cobra-specific bit: resolve it here and pass a
		// plain int into the business logic.
		profile, execArgv, err := splitArgs(args, cmd.ArgsLenAtDash(), f.exec)
		if err != nil {
			return err
		}

		opts := tokenOptions{
			Profile:  profile,
			Region:   f.region,
			Duration: f.duration,
			JSON:     f.json,
			Raw:      f.raw,
			Exec:     f.exec,
			ExecArgv: execArgv,
		}

		d := deps{
			// flint wires up the interactive picker only when the user gave no
			// profile and the session is a terminal. Otherwise pick stays nil
			// and flint uses the default credential chain.
			pick: interactivePicker(cmd, profile),
			log:  loggerFrom(cmd),
		}
		return runToken(cmd.Context(), cmd.OutOrStdout(), opts, d)
	}
}

// interactivePicker returns a profilePicker when it makes sense to prompt. It
// prompts only when the user gave no profile and stdin and stderr are a
// terminal. Otherwise it returns nil. The picker renders on stderr, so stdout
// stays clean for `eval`.
func interactivePicker(cmd *cobra.Command, profile string) profilePicker {
	if profile != "" {
		return nil
	}
	if !isInteractive() {
		return nil
	}
	return func() (string, error) {
		profiles, err := awsprofiles.List()
		if err != nil {
			return "", fmt.Errorf("listing AWS profiles: %w", err)
		}
		if len(profiles) == 0 {
			return "", fmt.Errorf("no AWS profiles found in ~/.aws/config or ~/.aws/credentials")
		}
		return ui.PickProfile(cmd.InOrStdin(), cmd.ErrOrStderr(), profiles)
	}
}

// isInteractive reports whether we can run a TUI: stdin must be readable from a
// terminal and stderr (where the TUI draws) must be a terminal too.
func isInteractive() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stderr.Fd())
}
