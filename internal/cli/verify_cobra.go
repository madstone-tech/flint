package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/madstone-tech/flint/internal/token"
)

// defaultLiveVerifier is the production live probe: it uses token.Verify with a
// default HTTP client.
func defaultLiveVerifier(ctx context.Context, bearerToken, region string) (token.LiveResult, error) {
	return token.Verify(ctx, nil, bearerToken, region)
}

func newVerifyCmd() *cobra.Command {
	var live bool

	cmd := &cobra.Command{
		Use:   "verify [token]",
		Short: "Validate a Bedrock bearer token, structurally and optionally against Bedrock",
		Long: `verify checks a Bedrock bearer token.

Without --live it performs structural validation only: it decodes the token and
confirms the prefix, base64 payload, version suffix, host, SigV4 parameters, the
region it is locked to, and whether its signature window has passed. This is
offline and makes no AWS call.

With --live it additionally makes one lightweight, read-only Bedrock call
(ListFoundationModels) using the token to confirm the service accepts it.

The token is read from (in order): the positional argument, standard input (when
piped), or the ` + envVar + ` environment variable.`,
		Example: `  # Structural check of a freshly minted token
  flint --raw my-profile | flint verify

  # Structural + live check
  flint --raw my-profile | flint verify --live

  # Verify whatever is currently exported
  flint verify --live`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := loggerFrom(cmd)
			tok, err := resolveToken(args, cmd.InOrStdin())
			if err != nil {
				log.Error("could not resolve a token to verify", "error", err)
				return withExit(exitUsage,
					fmt.Sprintf("pass a token as an argument, pipe it on stdin, or set %s", envVar),
					err)
			}
			log.Debug("token resolved for verification", "source", tokenSource(args, cmd.InOrStdin()))
			opts := verifyOptions{Token: tok, Live: live}
			return runVerify(cmd.Context(), cmd.OutOrStdout(), log, opts, nil)
		},
	}

	cmd.Flags().BoolVar(&live, "live", false, "additionally probe Bedrock to confirm the token is accepted")
	return cmd
}

// resolveToken finds the token from the positional arg, piped stdin, or the
// environment, in that order of precedence.
func resolveToken(args []string, stdin io.Reader) (string, error) {
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		return strings.TrimSpace(args[0]), nil
	}

	// Read stdin only when it is piped (not a terminal), so an interactive
	// `flint verify` doesn't hang waiting for input.
	if f, ok := stdin.(*os.File); ok {
		if info, err := f.Stat(); err == nil && (info.Mode()&os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(stdin)
			if err != nil {
				return "", fmt.Errorf("reading token from stdin: %w", err)
			}
			if t := strings.TrimSpace(string(data)); t != "" {
				return t, nil
			}
		}
	}

	if t := strings.TrimSpace(os.Getenv(envVar)); t != "" {
		return t, nil
	}

	return "", fmt.Errorf("no token provided (pass it as an argument, pipe it on stdin, or set %s)", envVar)
}

// tokenSource reports where resolveToken would read the token from, for
// diagnostic logging. It never returns the token value itself.
func tokenSource(args []string, stdin io.Reader) string {
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		return "argument"
	}
	if f, ok := stdin.(*os.File); ok {
		if info, err := f.Stat(); err == nil && (info.Mode()&os.ModeCharDevice) == 0 {
			return "stdin"
		}
	}
	if strings.TrimSpace(os.Getenv(envVar)) != "" {
		return "env:" + envVar
	}
	return "none"
}
