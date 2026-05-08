package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestImplicitRunFromRoot_PrintsSummary(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{srv.URL, "-n", "20", "-c", "5"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, srv.URL) {
		t.Errorf("missing URL in output: %s", out)
	}
	if !strings.Contains(out, "20 requests") {
		t.Errorf("missing request count: %s", out)
	}
	if !strings.Contains(out, "200=20") {
		t.Errorf("missing status code summary: %s", out)
	}
	if !strings.Contains(out, "latency (service)") {
		t.Errorf("missing latency block: %s", out)
	}
	if got := atomic.LoadInt64(&hits); got != 20 {
		t.Errorf("server hits = %d, want 20", got)
	}
}

func TestExplicitRunSubcommand_PrintsSummary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"run", srv.URL, "-n", "10"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "10 requests") {
		t.Errorf("missing request count: %s", buf.String())
	}
}

func TestRun_RequiresRequestCount(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"http://localhost:1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when -n is missing")
	}
	if !strings.Contains(err.Error(), "requests") {
		t.Errorf("error %q should mention --requests", err)
	}
}

func TestRoot_NoArgsShowsHelp(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Errorf("expected help, got %q", buf.String())
	}
}
