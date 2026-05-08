package cli

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slam [URL] [flags]",
		Short: "HTTP load testing tool",
		Long: `slam is an HTTP load testing tool. Hammer an API with tunable
concurrency and rate; get honest latency numbers backed by an HDR
histogram with coordinated-omission correction.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: route URL-shaped first positional to implicit run.
			return cmd.Help()
		},
	}
	cmd.SetVersionTemplate("slam {{.Version}}\n")
	return cmd
}
