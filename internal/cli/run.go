package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

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

func runE(cmd *cobra.Command, args []string) error {
	rawURL, err := parseURL(args[0])
	if err != nil {
		return err
	}

	target, err := buildTarget(cmd, rawURL)
	if err != nil {
		return err
	}
	plan, err := buildPlan(cmd)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), formatPreflight(target, plan))

	runner := &engine.Runner{Target: target, Plan: plan}
	sum, runErr := runner.Run(cmd.Context())
	if sum != nil {
		printTextSummary(cmd.OutOrStdout(), sum)
	}
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		return nil
	}
	return runErr
}

func buildTarget(cmd *cobra.Command, rawURL string) (engine.Target, error) {
	flags := cmd.Flags()
	target := engine.Target{URL: rawURL}

	if m, _ := flags.GetString("method"); m != "" {
		target.Method = strings.ToUpper(m)
	}

	headerArgs, _ := flags.GetStringArray("header")
	if len(headerArgs) > 0 {
		h := http.Header{}
		for _, raw := range headerArgs {
			k, v, err := parseHeader(raw)
			if err != nil {
				return target, err
			}
			h.Add(k, v)
		}
		target.Headers = h
	}

	body, _ := flags.GetString("body")
	bodyFile, _ := flags.GetString("body-file")
	if body != "" && bodyFile != "" {
		return target, fmt.Errorf("--body and --body-file are mutually exclusive")
	}
	switch {
	case body != "":
		target.Body = []byte(body)
	case bodyFile != "":
		data, err := os.ReadFile(bodyFile)
		if err != nil {
			return target, fmt.Errorf("read --body-file: %w", err)
		}
		target.Body = data
	}

	queryArgs, _ := flags.GetStringArray("query")
	if len(queryArgs) > 0 {
		q := url.Values{}
		for _, raw := range queryArgs {
			k, v, err := parseQueryArg(raw)
			if err != nil {
				return target, err
			}
			q.Add(k, v)
		}
		target.Query = q
	}

	return target, nil
}

func buildPlan(cmd *cobra.Command) (engine.Plan, error) {
	flags := cmd.Flags()
	plan := engine.Plan{}
	plan.Concurrency, _ = flags.GetInt("concurrency")
	plan.Requests, _ = flags.GetInt("requests")
	plan.RPS, _ = flags.GetInt("rate")
	plan.Duration, _ = flags.GetDuration("duration")
	plan.RampUp, _ = flags.GetDuration("ramp")
	plan.Timeout, _ = flags.GetDuration("timeout")
	plan.DisableKeepAlives, _ = flags.GetBool("no-keepalive")
	plan.HTTP2, _ = flags.GetBool("http2")

	if plan.Concurrency <= 0 {
		return plan, fmt.Errorf("--concurrency/-c must be > 0")
	}
	if plan.RampUp > 0 && plan.RPS <= 0 {
		return plan, fmt.Errorf("--ramp requires --rate to be set")
	}
	return plan, nil
}

// parseHeader splits "Key: Value" into key and value with whitespace trimmed.
func parseHeader(raw string) (string, string, error) {
	idx := strings.IndexByte(raw, ':')
	if idx < 0 {
		return "", "", fmt.Errorf("invalid header %q (expected 'Key: Value')", raw)
	}
	key := strings.TrimSpace(raw[:idx])
	value := strings.TrimSpace(raw[idx+1:])
	if key == "" {
		return "", "", fmt.Errorf("invalid header %q (empty key)", raw)
	}
	return key, value, nil
}

// parseQueryArg splits "key=value" into key and value (preserves whitespace
// in value; trims around key).
func parseQueryArg(raw string) (string, string, error) {
	idx := strings.IndexByte(raw, '=')
	if idx < 0 {
		return "", "", fmt.Errorf("invalid query %q (expected 'key=value')", raw)
	}
	key := strings.TrimSpace(raw[:idx])
	value := raw[idx+1:]
	if key == "" {
		return "", "", fmt.Errorf("invalid query %q (empty key)", raw)
	}
	return key, value, nil
}
