package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPool_FiresAllRequests(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	const (
		nWorkers = 100
		nTokens  = 1000
	)

	in := make(chan token, nTokens)
	out := make(chan Result, nTokens)
	for i := 0; i < nTokens; i++ {
		in <- token{ScheduledAt: time.Now()}
	}
	close(in)

	client := defaultClient(5*time.Second, false, false)
	target := Target{URL: srv.URL}

	done := make(chan struct{})
	go func() {
		workerPool(context.Background(), nWorkers, client, target, in, out)
		close(done)
	}()

	var got int
	for res := range out {
		if res.Err != nil {
			t.Errorf("result error: %v", res.Err)
		}
		if res.Status != http.StatusOK {
			t.Errorf("status = %d, want 200", res.Status)
		}
		if res.ScheduledAt.IsZero() {
			t.Error("ScheduledAt not propagated to Result")
		}
		got++
	}
	<-done

	if got != nTokens {
		t.Errorf("results = %d, want %d", got, nTokens)
	}
	if h := atomic.LoadInt64(&hits); h != int64(nTokens) {
		t.Errorf("server hits = %d, want %d", h, nTokens)
	}
}

func TestWorkerPool_ClosedInputClosesOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	in := make(chan token)
	close(in)
	out := make(chan Result)

	done := make(chan struct{})
	go func() {
		workerPool(context.Background(), 4, defaultClient(time.Second, false, false), Target{URL: srv.URL}, in, out)
		close(done)
	}()

	for range out {
		t.Error("got unexpected result from empty input")
	}
	<-done
}
