package cli

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/bradstell/slam/engine"
)

// formatPreflight returns a one-line summary of the impending run that the
// CLI prints before invoking the engine.
func formatPreflight(target engine.Target, plan engine.Plan) string {
	method := target.Method
	if method == "" {
		method = http.MethodGet
	}

	parts := []string{fmt.Sprintf("%d workers", plan.Concurrency)}
	if plan.RPS > 0 {
		parts = append(parts, fmt.Sprintf("%d RPS", plan.RPS))
	} else {
		parts = append(parts, "no rate limit")
	}
	if plan.RampUp > 0 {
		parts = append(parts, fmt.Sprintf("ramp %s", plan.RampUp))
	}
	switch {
	case plan.Duration > 0:
		parts = append(parts, plan.Duration.String())
	case plan.Requests > 0:
		parts = append(parts, fmt.Sprintf("%d reqs", plan.Requests))
	}

	line := fmt.Sprintf("→ %s %s  (%s)", method, target.URL, strings.Join(parts, ", "))
	if plan.Duration == 0 && plan.Requests == 0 {
		line += " — ctrl-c to stop"
	}
	return line
}

// printTextSummary writes a human-readable summary of a completed run.
func printTextSummary(w io.Writer, sum *engine.Summary) {
	elapsed := sum.Ended.Sub(sum.Started)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s\n", sum.Target.URL)
	fmt.Fprintf(w, "  %d requests in %s — %.2f req/s\n",
		sum.TotalSent, roundDur(elapsed), sum.Throughput)
	fmt.Fprintf(w, "  errors: %d (%.2f%%)\n", sum.Errors, errorPercent(sum))
	if len(sum.StatusCodes) > 0 {
		fmt.Fprintf(w, "  status: %s\n", formatStatusCodes(sum.StatusCodes))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  latency (service)")
	fmt.Fprintf(w, "    min  %-9s  p50  %-9s  p90  %-9s\n",
		roundDur(sum.Service.Min), roundDur(sum.Service.P50), roundDur(sum.Service.P90))
	fmt.Fprintf(w, "    p95  %-9s  p99  %-9s  p999 %-9s\n",
		roundDur(sum.Service.P95), roundDur(sum.Service.P99), roundDur(sum.Service.P999))
	fmt.Fprintf(w, "    max  %-9s\n", roundDur(sum.Service.Max))
	fmt.Fprintln(w)
}

func errorPercent(sum *engine.Summary) float64 {
	if sum.TotalSent == 0 {
		return 0
	}
	return 100 * float64(sum.Errors) / float64(sum.TotalSent)
}

func formatStatusCodes(m map[int]int64) string {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%d=%d", k, m[k])
	}
	return strings.Join(parts, " ")
}

// roundDur renders a duration with sensible precision for human reading:
// values get truncated to the next-larger unit.
func roundDur(d time.Duration) string {
	switch {
	case d == 0:
		return "0s"
	case d < time.Microsecond:
		return d.String()
	case d < time.Millisecond:
		return (d / time.Microsecond * time.Microsecond).String()
	case d < time.Second:
		return (d / time.Millisecond * time.Millisecond).String()
	default:
		return d.Round(10 * time.Millisecond).String()
	}
}
