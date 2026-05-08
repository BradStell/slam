package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCLI_VersionFlag(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), Version) {
		t.Errorf("--version output %q does not contain %q", buf.String(), Version)
	}
}

func TestCLI_HelpFlag(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "load testing tool") {
		t.Errorf("--help missing description: %q", out)
	}
	if !strings.Contains(out, "Usage:") {
		t.Errorf("--help missing usage: %q", out)
	}
}

func TestRun_VersionExitsZero(t *testing.T) {
	if code := Run([]string{"--version"}); code != 0 {
		t.Errorf("Run([--version]) = %d, want 0", code)
	}
}

func TestRunWithContext_GracefulCancelExitsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	code := runWithContext(ctx, []string{srv.URL, "-c", "5"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0 for ctx-canceled run", code)
	}
}
