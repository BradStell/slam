package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
