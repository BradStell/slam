// Package cli implements the slam command-line interface.
package cli

// Version is the slam release version. Overridden at build time via -ldflags.
var Version = "0.0.1-dev"

// Run executes the slam CLI with the given args (excluding argv[0]) and
// returns the process exit code.
func Run(args []string) int {
	cmd := newRootCmd()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return 1
	}
	return 0
}
