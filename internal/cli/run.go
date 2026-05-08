package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run URL",
		Short: "Run a load test against a single URL",
		Args:  cobra.ExactArgs(1),
		RunE:  runE,
	}
}

// runE is shared between the explicit `run` subcommand and the implicit
// URL-shaped first positional dispatch from the root command.
func runE(cmd *cobra.Command, args []string) error {
	target, err := parseURL(args[0])
	if err != nil {
		return err
	}
	// TODO: build engine.Plan/Target from flags and execute.
	fmt.Fprintf(cmd.OutOrStdout(), "would load test: %s\n", target)
	return nil
}
