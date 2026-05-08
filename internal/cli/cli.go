// Package cli implements the slam command-line interface.
package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// Version is the slam release version. Overridden at build time via -ldflags.
var Version = "0.0.1-dev"

// Run executes the slam CLI with the given args (excluding argv[0]) and
// returns the process exit code. SIGINT and SIGTERM cancel the run; on
// graceful cancel the partial summary is still printed and the exit code
// is 0.
func Run(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runWithContext(ctx, args)
}

// runWithContext is the testable Run: it accepts an externally provided
// context so tests can drive ctx cancellation directly.
func runWithContext(ctx context.Context, args []string) int {
	cmd := newRootCmd()
	cmd.SetArgs(args)
	cmd.SetContext(ctx)
	if err := cmd.Execute(); err != nil {
		return 1
	}
	return 0
}
