package engine

import (
	"context"
	"net/http"
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
func (r *Runner) Run(ctx context.Context) (*Summary, error) {
	_ = ctx
	// TODO: wire scheduler + workers + aggregator.
	return &Summary{Target: r.Target, Plan: r.Plan}, nil
}
