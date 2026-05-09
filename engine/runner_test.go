package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunner_Run_RequestBounded(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := &Runner{
		Target: Target{URL: srv.URL},
		Plan:   Plan{Concurrency: 10, Requests: 100, Timeout: 5 * time.Second},
	}
	sum, err := r.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sum.TotalSent != 100 {
		t.Errorf("TotalSent = %d, want 100", sum.TotalSent)
	}
	if sum.Errors != 0 {
		t.Errorf("Errors = %d, want 0", sum.Errors)
	}
	if sum.StatusCodes[200] != 100 {
		t.Errorf("StatusCodes[200] = %d, want 100", sum.StatusCodes[200])
	}
	if got := atomic.LoadInt64(&hits); got != 100 {
		t.Errorf("server hits = %d, want 100", got)
	}
	if sum.Throughput <= 0 {
		t.Errorf("Throughput = %v, want > 0", sum.Throughput)
	}
}

func TestRunner_Run_RejectsZeroConcurrency(t *testing.T) {
	r := &Runner{Target: Target{URL: "http://example.com"}, Plan: Plan{Requests: 1}}
	if _, err := r.Run(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunner_Run_IndefiniteUntilCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	r := &Runner{
		Target: Target{URL: srv.URL},
		Plan:   Plan{Concurrency: 5, Timeout: 5 * time.Second}, // no Requests, no Duration
	}
	sum, err := r.Run(ctx)
	if err == nil {
		t.Error("expected ctx error after cancel")
	}
	if sum == nil {
		t.Fatal("Summary should be non-nil even on cancel")
	}
	if sum.TotalSent == 0 {
		t.Error("expected some requests to have been sent before cancel")
	}
}

func TestRunner_Run_EmitsOnTickPeriodically(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rep := &recordingReporter{}
	r := &Runner{
		Target:   Target{URL: srv.URL},
		Plan:     Plan{Concurrency: 5, Duration: 600 * time.Millisecond, Timeout: 5 * time.Second},
		Reporter: rep,
	}
	if _, err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// 600ms run with 250ms tick should give at least 1 tick.
	if len(rep.ticks) < 1 {
		t.Errorf("expected ≥1 tick over 600ms run, got %d", len(rep.ticks))
	}
	if len(rep.ticks) > 0 && rep.ticks[len(rep.ticks)-1].Sent == 0 {
		t.Error("last tick should reflect non-zero Sent")
	}
}

func TestRunner_Run_CallsReporter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rep := &recordingReporter{}
	r := &Runner{
		Target:   Target{URL: srv.URL},
		Plan:     Plan{Concurrency: 2, Requests: 5, Timeout: 5 * time.Second},
		Reporter: rep,
	}
	if _, err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.startCalled {
		t.Error("Reporter.OnStart not called")
	}
	if rep.finish == nil {
		t.Fatal("Reporter.OnFinish not called")
	}
	if rep.finish.TotalSent != 5 {
		t.Errorf("OnFinish summary TotalSent = %d, want 5", rep.finish.TotalSent)
	}
}

func TestRunner_Run_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(75 * time.Millisecond)
		cancel()
	}()

	r := &Runner{
		Target: Target{URL: srv.URL},
		Plan:   Plan{Concurrency: 5, Requests: 10000, Timeout: 5 * time.Second},
	}
	sum, err := r.Run(ctx)
	if err == nil {
		t.Error("expected ctx error, got nil")
	}
	if sum == nil {
		t.Fatal("Summary should be non-nil even on cancel")
	}
	if sum.TotalSent >= 10000 {
		t.Errorf("TotalSent = %d, expected < 10000 after cancel", sum.TotalSent)
	}
}

func TestRunner_Run_DurationBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := &Runner{
		Target: Target{URL: srv.URL},
		Plan:   Plan{Concurrency: 5, Duration: 400 * time.Millisecond, Timeout: 5 * time.Second},
	}
	start := time.Now()
	sum, err := r.Run(context.Background())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if elapsed < 350*time.Millisecond || elapsed > 750*time.Millisecond {
		t.Errorf("elapsed = %v, want ~400ms", elapsed)
	}
	if sum.TotalSent == 0 {
		t.Error("expected non-zero requests in 400ms run")
	}
}

func TestRunner_Run_RequestsBeforeDuration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := &Runner{
		Target: Target{URL: srv.URL},
		Plan:   Plan{Concurrency: 10, Requests: 50, Duration: 30 * time.Second, Timeout: 5 * time.Second},
	}
	start := time.Now()
	sum, err := r.Run(context.Background())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("elapsed = %v, expected to finish well before 30s Duration", elapsed)
	}
	if sum.TotalSent != 50 {
		t.Errorf("TotalSent = %d, want 50", sum.TotalSent)
	}
}

type recordingReporter struct {
	startCalled bool
	startPlan   Plan
	ticks       []Snapshot
	finish      *Summary
}

func (r *recordingReporter) OnStart(p Plan)        { r.startCalled = true; r.startPlan = p }
func (r *recordingReporter) OnTick(s Snapshot)     { r.ticks = append(r.ticks, s) }
func (r *recordingReporter) OnFinish(sum *Summary) { r.finish = sum }
