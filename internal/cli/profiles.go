package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/madstone-tech/flint/internal/awsprofiles"
)

// profileLister returns the discovered AWS profile names. Injectable for tests.
type profileLister func() ([]string, error)

func newProfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "profiles",
		Aliases: []string{"ls"},
		Short:   "List AWS profiles flint can see",
		Long: `profiles prints the profile names discovered in ~/.aws/config and
~/.aws/credentials (honoring AWS_CONFIG_FILE and AWS_SHARED_CREDENTIALS_FILE),
one per line. Useful in scripts and for confirming which profiles flint sees.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runProfiles(cmd.OutOrStdout(), awsprofiles.List)
		},
	}
	return cmd
}

func runProfiles(w io.Writer, list profileLister) error {
	profiles, err := list()
	if err != nil {
		return withExit(exitGeneric, "", fmt.Errorf("listing AWS profiles: %w", err))
	}
	if len(profiles) == 0 {
		return withExit(exitGeneric,
			"add a profile to ~/.aws/config or ~/.aws/credentials",
			fmt.Errorf("no AWS profiles found"))
	}
	for _, p := range profiles {
		if _, err := fmt.Fprintln(w, p); err != nil {
			return err
		}
	}
	return nil
}
