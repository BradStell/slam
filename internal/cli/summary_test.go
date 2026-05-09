package cli

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bradstell/slam/engine"
)

func TestFormatPreflight_DurationBounded(t *testing.T) {
	line := formatPreflight(
		engine.Target{URL: "http://api.foo/get"},
		engine.Plan{Concurrency: 50, RPS: 1000, Duration: 30 * time.Second},
	)

	for _, want := range []string{"→ GET", "http://api.foo/get", "50 workers", "1000 RPS", "30s"} {
		if !strings.Contains(line, want) {
			t.Errorf("preflight %q missing %q", line, want)
		}
	}
	if strings.Contains(line, "ctrl-c") {
		t.Errorf("preflight should not show ctrl-c hint when bounded: %q", line)
	}
}

func TestFormatPreflight_IndefiniteShowsCtrlCHint(t *testing.T) {
	line := formatPreflight(
		engine.Target{URL: "http://api.foo/"},
		engine.Plan{Concurrency: 50},
	)
	for _, want := range []string{"50 workers", "no rate limit", "ctrl-c to stop"} {
		if !strings.Contains(line, want) {
			t.Errorf("preflight %q missing %q", line, want)
		}
	}
}

func TestFormatPreflight_RequestBoundedAndRamped(t *testing.T) {
	line := formatPreflight(
		engine.Target{URL: "http://x", Method: "POST"},
		engine.Plan{Concurrency: 10, RPS: 500, RampUp: 5 * time.Second, Requests: 1000},
	)
	for _, want := range []string{"→ POST", "10 workers", "500 RPS", "ramp 5s", "1000 reqs"} {
		if !strings.Contains(line, want) {
			t.Errorf("preflight %q missing %q", line, want)
		}
	}
}

func TestRunPrintsPreflightThenSummary(t *testing.T) {
	out := runWithFlags(t,
		[]string{"-n", "5", "-c", "2"},
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	)
	pre := strings.Index(out, "→ GET")
	post := strings.Index(out, "5 requests")
	if pre < 0 || post < 0 {
		t.Fatalf("missing preflight or summary in:\n%s", out)
	}
	if pre >= post {
		t.Errorf("preflight should appear before summary; pre=%d post=%d", pre, post)
	}
}
