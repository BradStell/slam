package engine

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestExecute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "hello")
	}))
	defer srv.Close()

	client := defaultClient(5*time.Second, false, false)
	res := execute(context.Background(), client, Target{URL: srv.URL})

	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
	if res.Status != http.StatusOK {
		t.Errorf("Status = %d, want 200", res.Status)
	}
	if res.BytesIn != int64(len("hello")) {
		t.Errorf("BytesIn = %d, want %d", res.BytesIn, len("hello"))
	}
	if res.SentAt.IsZero() || res.DoneAt.IsZero() || !res.DoneAt.After(res.SentAt) {
		t.Errorf("expected DoneAt after SentAt; got SentAt=%v DoneAt=%v", res.SentAt, res.DoneAt)
	}
}

func TestExecute_PostWithBodyAndHeaders(t *testing.T) {
	const wantBody = "payload"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		got, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(got) != wantBody {
			t.Errorf("body = %q, want %q", got, wantBody)
		}
		if r.Header.Get("X-Test") != "yes" {
			t.Errorf("X-Test header missing or wrong: %q", r.Header.Get("X-Test"))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := defaultClient(5*time.Second, false, false)
	target := Target{
		URL:     srv.URL,
		Method:  http.MethodPost,
		Body:    []byte(wantBody),
		Headers: http.Header{"X-Test": []string{"yes"}},
	}
	res := execute(context.Background(), client, target)
	if res.Err != nil {
		t.Fatalf("Err = %v", res.Err)
	}
	if res.Status != http.StatusCreated {
		t.Errorf("Status = %d, want 201", res.Status)
	}
}

func TestExecute_QueryParamsMerge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("a") != "1" {
			t.Errorf("a = %q, want 1 (from URL)", q.Get("a"))
		}
		if q.Get("b") != "2" {
			t.Errorf("b = %q, want 2 (from Target.Query)", q.Get("b"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := defaultClient(5*time.Second, false, false)
	target := Target{
		URL:   srv.URL + "/path?a=1",
		Query: map[string][]string{"b": {"2"}},
	}
	res := execute(context.Background(), client, target)
	if res.Err != nil {
		t.Fatalf("Err = %v", res.Err)
	}
	if res.Status != http.StatusOK {
		t.Errorf("Status = %d, want 200", res.Status)
	}
}

func TestExecute_ConnRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // make further requests fail with conn refused

	client := defaultClient(2*time.Second, false, false)
	res := execute(context.Background(), client, Target{URL: url})
	if res.Err == nil {
		t.Fatalf("Err = nil, want connection error; status=%d", res.Status)
	}
	if res.Status != 0 {
		t.Errorf("Status = %d, want 0 on transport error", res.Status)
	}
	if res.SentAt.IsZero() || res.DoneAt.IsZero() {
		t.Error("SentAt/DoneAt should still be set on transport error")
	}
}

func TestExecute_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()

	client := defaultClient(150*time.Millisecond, false, false)
	res := execute(context.Background(), client, Target{URL: srv.URL})
	if res.Err == nil {
		t.Fatal("Err = nil, want timeout error")
	}
}

func TestExecute_AllMethods(t *testing.T) {
	for _, method := range []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodHead,
	} {
		t.Run(method, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != method {
					t.Errorf("server saw method %q, want %q", r.Method, method)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			client := defaultClient(5*time.Second, false, false)
			res := execute(context.Background(), client, Target{URL: srv.URL, Method: method})
			if res.Err != nil {
				t.Fatalf("Err = %v", res.Err)
			}
			if res.Status != http.StatusOK {
				t.Errorf("Status = %d, want 200", res.Status)
			}
		})
	}
}

func TestExecute_MultipleHeadersAndMultiValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-One") != "1" {
			t.Errorf("X-One = %q", r.Header.Get("X-One"))
		}
		if r.Header.Get("X-Two") != "2" {
			t.Errorf("X-Two = %q", r.Header.Get("X-Two"))
		}
		if vals := r.Header.Values("X-Multi"); len(vals) != 2 || vals[0] != "a" || vals[1] != "b" {
			t.Errorf("X-Multi = %v, want [a b]", vals)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := defaultClient(5*time.Second, false, false)
	target := Target{
		URL: srv.URL,
		Headers: http.Header{
			"X-One":   []string{"1"},
			"X-Two":   []string{"2"},
			"X-Multi": []string{"a", "b"},
		},
	}
	res := execute(context.Background(), client, target)
	if res.Err != nil {
		t.Fatalf("Err = %v", res.Err)
	}
}

func TestExecute_KeepAliveKnobControlsConnReuse(t *testing.T) {
	var newConns int64
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			atomic.AddInt64(&newConns, 1)
		}
	}
	srv.Start()
	defer srv.Close()

	const reqs = 5

	// Keep-alive on (default): sequential reqs reuse the same connection.
	atomic.StoreInt64(&newConns, 0)
	client := defaultClient(5*time.Second, false, false)
	for i := 0; i < reqs; i++ {
		if r := execute(context.Background(), client, Target{URL: srv.URL}); r.Err != nil {
			t.Fatalf("keep-alive on, req %d: %v", i, r.Err)
		}
	}
	if got := atomic.LoadInt64(&newConns); got != 1 {
		t.Errorf("keep-alive on: newConns = %d, want 1", got)
	}

	// Keep-alive disabled: each request opens a new connection.
	atomic.StoreInt64(&newConns, 0)
	client = defaultClient(5*time.Second, true, false)
	for i := 0; i < reqs; i++ {
		if r := execute(context.Background(), client, Target{URL: srv.URL}); r.Err != nil {
			t.Fatalf("keep-alive off, req %d: %v", i, r.Err)
		}
	}
	if got := atomic.LoadInt64(&newConns); got != reqs {
		t.Errorf("keep-alive off: newConns = %d, want %d", got, reqs)
	}
}

func TestDefaultClient_AppliesPlanKnobs(t *testing.T) {
	c := defaultClient(7*time.Second, true, true)
	if c.Timeout != 7*time.Second {
		t.Errorf("Timeout = %v, want 7s", c.Timeout)
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", c.Transport)
	}
	if !tr.DisableKeepAlives {
		t.Error("DisableKeepAlives knob not applied")
	}
	if !tr.ForceAttemptHTTP2 {
		t.Error("HTTP2 knob (ForceAttemptHTTP2) not applied")
	}

	c2 := defaultClient(0, false, false)
	tr2 := c2.Transport.(*http.Transport)
	if tr2.DisableKeepAlives {
		t.Error("DisableKeepAlives should be false by default")
	}
	if tr2.ForceAttemptHTTP2 {
		t.Error("ForceAttemptHTTP2 should be false by default")
	}
}

func TestExecute_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	client := defaultClient(5*time.Second, false, false)
	res := execute(ctx, client, Target{URL: srv.URL})
	if res.Err == nil {
		t.Fatal("Err = nil, want context canceled")
	}
}
