package cli

import (
	"time"

	"github.com/spf13/cobra"
)

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
		Args:          cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && looksLikeURL(args[0]) {
				return runE(cmd, args)
			}
			return cmd.Help()
		},
	}
	cmd.SetVersionTemplate("slam {{.Version}}\n")
	addRunFlags(cmd)
	cmd.AddCommand(newRunCmd())
	return cmd
}

// addRunFlags registers the load-test flags as persistent flags on cmd so
// they are inherited by the run subcommand and usable on the root command
// (for the implicit-URL form).
func addRunFlags(cmd *cobra.Command) {
	pf := cmd.PersistentFlags()

	// Load shape
	pf.IntP("concurrency", "c", 50, "number of concurrent workers")
	pf.IntP("requests", "n", 0, "stop after this many requests (0 = unlimited)")
	pf.IntP("rate", "r", 0, "target requests per second (0 = unlimited)")
	pf.DurationP("duration", "t", 0, "stop after this duration (0 = unlimited)")
	pf.Duration("ramp", 0, "linear ramp from 0 to --rate over this window")

	// Request shape
	pf.String("method", "", "HTTP method (default GET)")
	pf.StringArrayP("header", "H", nil, "request header (repeatable: -H 'Key: Value')")
	pf.StringP("body", "d", "", "request body string")
	pf.String("body-file", "", "request body read from file")
	pf.StringArray("query", nil, "query parameter (repeatable: --query 'key=value')")

	// Transport
	pf.Duration("timeout", 30*time.Second, "per-request timeout")
	pf.Bool("no-keepalive", false, "disable HTTP keep-alive")
	pf.Bool("http2", false, "prefer HTTP/2 (h2 over TLS, h2c over cleartext)")

	// Output
	pf.StringP("output", "o", "text", "output format: text|json")
}
