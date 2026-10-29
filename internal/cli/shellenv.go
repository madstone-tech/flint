package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// shellEnvScript is the POSIX-shell integration that `flint shellenv` prints.
// The user adds `eval "$(flint shellenv)"` to their ~/.bashrc or ~/.zshrc. It
// defines lazy helpers. It does not export a token at shell startup, which
// would freeze a token for up to 12 hours and run flint on every shell launch.
const shellEnvScript = `# flint shell integration. Add this to ~/.bashrc or ~/.zshrc:
#   eval "$(flint shellenv)"
#
# flint mints short-term Amazon Bedrock bearer tokens on demand. These helpers
# do not export a token at shell startup. An eager export would be stale within
# 12 hours and would run flint on every new shell.

# bedrock-auth: mint a fresh token into the current shell, on demand.
#   usage: bedrock-auth [profile] [region]
bedrock-auth() {
  local _out
  if ! _out=$(command flint "${1:-$AWS_PROFILE}" ${2:+--region "$2"}); then
    echo "bedrock-auth: flint failed" >&2
    return 1
  fi
  eval "$_out"
}

# bedrock-run: run a command with a fresh token in its environment only. The
# token never touches your shell or history.
#   usage: bedrock-run [profile] -- <command> [args...]
#   example: bedrock-run my-profile -- aws bedrock-runtime converse --model-id ...
bedrock-run() {
  command flint --exec "$@"
}
`

func newShellenvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shellenv",
		Short: "Print shell integration to eval in your ~/.bashrc or ~/.zshrc",
		Long: `shellenv prints POSIX-shell functions you can load with:

    eval "$(flint shellenv)"

It defines two helpers:

  bedrock-auth [profile] [region]       mint a token into the current shell
  bedrock-run  [profile] -- cmd args    run a command with the token in its env

The helpers are lazy. They do not export a token at shell startup, which would
freeze a token for up to 12 hours and run flint on every new shell.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeShellenv(cmd.OutOrStdout())
		},
	}
}

func writeShellenv(w io.Writer) error {
	_, err := fmt.Fprint(w, shellEnvScript)
	return err
}
