package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFlag_OutputJSON_ProducesParseableSummary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{srv.URL, "-n", "5", "-c", "2", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Output must be valid JSON with no preflight or status-line noise.
	if strings.Contains(buf.String(), "→ GET") {
		t.Errorf("JSON mode should suppress preflight; got:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "elapsed ·") {
		t.Errorf("JSON mode should suppress live ticks; got:\n%s", buf.String())
	}

	var got jsonSummary
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput:\n%s", err, buf.String())
	}
	if got.TotalSent != 5 {
		t.Errorf("total_sent = %d, want 5", got.TotalSent)
	}
	if got.StatusCodes["200"] != 5 {
		t.Errorf("status_codes[200] = %d, want 5", got.StatusCodes["200"])
	}
	if got.Plan.Concurrency != 2 {
		t.Errorf("plan.concurrency = %d, want 2", got.Plan.Concurrency)
	}
	// parseURL adds a default trailing "/" path when none is present.
	if want := srv.URL + "/"; got.Target.URL != want {
		t.Errorf("target.url = %q, want %q", got.Target.URL, want)
	}
	if got.Service.MaxMS <= 0 {
		t.Errorf("service_latency.max_ms = %v, want > 0", got.Service.MaxMS)
	}
}

func TestFlag_OutputInvalidValueErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{srv.URL, "-n", "1", "-o", "yaml"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unsupported output value")
	}
}
