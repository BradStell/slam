package cli

import (
	"fmt"
	"time"

	"github.com/bradstell/slam/engine"
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
	rawURL, err := parseURL(args[0])
	if err != nil {
		return err
	}

	flags := cmd.Flags()
	concurrency, _ := flags.GetInt("concurrency")
	requests, _ := flags.GetInt("requests")

	if requests <= 0 {
		return fmt.Errorf("--requests/-n must be > 0")
	}
	if concurrency <= 0 {
		return fmt.Errorf("--concurrency/-c must be > 0")
	}

	runner := &engine.Runner{
		Target: engine.Target{URL: rawURL},
		Plan: engine.Plan{
			Concurrency: concurrency,
			Requests:    requests,
			Timeout:     30 * time.Second,
		},
	}

	sum, runErr := runner.Run(cmd.Context())
	if sum != nil {
		printTextSummary(cmd.OutOrStdout(), sum)
	}
	return runErr
}
