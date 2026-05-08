package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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
	if got := atomic.LoadInt64(&hits); got != 20 {
		t.Errorf("server hits = %d, want 20", got)
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

// runFlagsTest spins up an httptest server with a verifier handler, runs
// `slam URL <flags>`, and returns nothing if all server-side asserts pass.
func runWithFlags(t *testing.T, args []string, handler http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(append([]string{srv.URL}, args...))
	cmd.SetContext(context.Background())
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return buf.String()
}

func TestFlag_Method(t *testing.T) {
	runWithFlags(t,
		[]string{"-n", "1", "--method", "PUT"},
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %q, want PUT", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		})
}

func TestFlag_Headers(t *testing.T) {
	runWithFlags(t,
		[]string{"-n", "1", "-H", "X-One: 1", "-H", "X-Two: 2"},
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-One") != "1" {
				t.Errorf("X-One = %q", r.Header.Get("X-One"))
			}
			if r.Header.Get("X-Two") != "2" {
				t.Errorf("X-Two = %q", r.Header.Get("X-Two"))
			}
			w.WriteHeader(http.StatusOK)
		})
}

func TestFlag_BodyString(t *testing.T) {
	const want = `{"hello":"world"}`
	runWithFlags(t,
		[]string{"-n", "1", "--method", "POST", "-d", want},
		func(w http.ResponseWriter, r *http.Request) {
			got, _ := io.ReadAll(r.Body)
			if string(got) != want {
				t.Errorf("body = %q, want %q", got, want)
			}
			w.WriteHeader(http.StatusOK)
		})
}

func TestFlag_BodyFile(t *testing.T) {
	const want = "file-contents-here"
	dir := t.TempDir()
	path := filepath.Join(dir, "body.txt")
	if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	runWithFlags(t,
		[]string{"-n", "1", "--method", "POST", "--body-file", path},
		func(w http.ResponseWriter, r *http.Request) {
			got, _ := io.ReadAll(r.Body)
			if string(got) != want {
				t.Errorf("body = %q, want %q", got, want)
			}
			w.WriteHeader(http.StatusOK)
		})
}

func TestFlag_BodyAndBodyFileMutuallyExclusive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{srv.URL, "-n", "1", "-d", "x", "--body-file", "/dev/null"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for --body + --body-file")
	}
}

func TestFlag_QueryParams(t *testing.T) {
	runWithFlags(t,
		[]string{"-n", "1", "--query", "a=1", "--query", "b=two,with comma"},
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("a") != "1" {
				t.Errorf("a = %q", r.URL.Query().Get("a"))
			}
			if r.URL.Query().Get("b") != "two,with comma" {
				t.Errorf("b = %q", r.URL.Query().Get("b"))
			}
			w.WriteHeader(http.StatusOK)
		})
}

func TestFlag_Duration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{srv.URL, "-t", "200ms", "-c", "5"})

	start := time.Now()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 150*time.Millisecond || elapsed > 600*time.Millisecond {
		t.Errorf("elapsed = %v, want ~200ms", elapsed)
	}
}

func TestFlag_RateRequiresRamp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{srv.URL, "-n", "1", "--ramp", "1s"}) // no --rate
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error: --ramp without --rate")
	}
}

func TestFlag_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{srv.URL, "-r", "100", "-t", "300ms", "-c", "10"})

	start := time.Now()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	elapsed := time.Since(start)
	// 300ms @ 100 RPS ≈ 30 requests; should take ~300ms ± 200ms
	if elapsed < 250*time.Millisecond || elapsed > 700*time.Millisecond {
		t.Errorf("elapsed = %v", elapsed)
	}
	if !strings.Contains(buf.String(), "requests") {
		t.Errorf("expected summary; got %s", buf.String())
	}
}

func TestParseHeader(t *testing.T) {
	cases := []struct {
		in, k, v string
		err      bool
	}{
		{"X-Foo: bar", "X-Foo", "bar", false},
		{"X-Foo:bar", "X-Foo", "bar", false},
		{"Authorization: Bearer abc:def", "Authorization", "Bearer abc:def", false},
		{"  X-Foo  :  bar  ", "X-Foo", "bar", false},
		{"no-colon", "", "", true},
		{": empty-key", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			k, v, err := parseHeader(tc.in)
			if tc.err {
				if err == nil {
					t.Errorf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseHeader(%q): %v", tc.in, err)
			}
			if k != tc.k || v != tc.v {
				t.Errorf("parseHeader(%q) = (%q, %q), want (%q, %q)", tc.in, k, v, tc.k, tc.v)
			}
		})
	}
}

func TestParseQueryArg(t *testing.T) {
	cases := []struct {
		in, k, v string
		err      bool
	}{
		{"a=1", "a", "1", false},
		{"a=hello world", "a", "hello world", false},
		{"a=", "a", "", false},
		{"a=val=ue", "a", "val=ue", false},
		{"no-equals", "", "", true},
		{"=empty-key", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			k, v, err := parseQueryArg(tc.in)
			if tc.err {
				if err == nil {
					t.Errorf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseQueryArg(%q): %v", tc.in, err)
			}
			if k != tc.k || v != tc.v {
				t.Errorf("parseQueryArg(%q) = (%q, %q), want (%q, %q)", tc.in, k, v, tc.k, tc.v)
			}
		})
	}
}
