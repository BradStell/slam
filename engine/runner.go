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
// Termination is whichever fires first: Plan.Duration elapsed, Plan.Requests
// reached, or ctx canceled. When neither Duration nor Requests is set the
// run is indefinite — only ctx cancellation (typically SIGINT) will stop it.
func (r *Runner) Run(ctx context.Context) (*Summary, error) {
	if r.Plan.Concurrency < 1 {
		return nil, fmt.Errorf("engine: Plan.Concurrency must be >= 1")
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

	stopTicker := make(chan struct{})
	tickerDone := make(chan struct{})
	go func() {
		defer close(tickerDone)
		if r.Reporter == nil {
			return
		}
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.Reporter.OnTick(agg.snapshot())
			case <-stopTicker:
				return
			}
		}
	}()

	ctxErr := schedule(ctx, in, scheduleConfig{
		started:     started,
		rps:         r.Plan.RPS,
		rampUp:      r.Plan.RampUp,
		duration:    r.Plan.Duration,
		maxRequests: r.Plan.Requests,
	})
	close(in)

	<-poolDone
	sum := <-summaryCh
	close(stopTicker)
	<-tickerDone

	if r.Reporter != nil {
		r.Reporter.OnFinish(sum)
	}
	return sum, ctxErr
}
