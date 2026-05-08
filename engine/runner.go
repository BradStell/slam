package engine

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Runner executes a Plan against a Target.
type Runner struct {
	Target     Target
	Plan       Plan
	Reporter   Reporter     // optional; nil disables live updates
	HTTPClient *http.Client // optional; default with sensible pooling if nil
}

// Run executes the configured load test, returning the final Summary when
// the plan completes or the context is canceled.
//
// Currently supports request-bounded runs only (Plan.Requests > 0). Time-
// bounded, indefinite, and rate-limited runs are added in later milestones.
func (r *Runner) Run(ctx context.Context) (*Summary, error) {
	if r.Plan.Concurrency < 1 {
		return nil, fmt.Errorf("engine: Plan.Concurrency must be >= 1")
	}
	if r.Plan.Requests < 1 {
		return nil, fmt.Errorf("engine: Plan.Requests must be > 0")
	}

	client := r.HTTPClient
	if client == nil {
		client = defaultClient(r.Plan.Timeout, r.Plan.DisableKeepAlives, r.Plan.HTTP2)
	}

	started := time.Now()
	if r.Reporter != nil {
		r.Reporter.OnStart(r.Plan)
	}

	bufSize := r.Plan.Concurrency
	if bufSize > 1024 {
		bufSize = 1024
	}
	in := make(chan token, bufSize)
	out := make(chan Result, bufSize)

	agg := newAggregator(started)
	summaryCh := make(chan *Summary, 1)
	go func() {
		summaryCh <- agg.run(out, r.Plan, r.Target)
	}()

	poolDone := make(chan struct{})
	go func() {
		workerPool(ctx, r.Plan.Concurrency, client, r.Target, in, out)
		close(poolDone)
	}()

	ctxErr := func() error {
		for i := 0; i < r.Plan.Requests; i++ {
			select {
			case in <- token{ScheduledAt: time.Now()}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}()
	close(in)

	<-poolDone
	sum := <-summaryCh

	if r.Reporter != nil {
		r.Reporter.OnFinish(sum)
	}
	return sum, ctxErr
}
