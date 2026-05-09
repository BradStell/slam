package engine

import (
	"sync"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

// aggregator consumes Results and produces a Summary.
//
// Two histograms are maintained side by side:
//
//   - service: DoneAt - SentAt — what the server actually took to handle
//     the request once it was sent.
//   - response: DoneAt - ScheduledAt — what a client on a fixed schedule
//     would have observed, including any time the request waited because
//     the worker pool was busy. Higher than service under server stalls;
//     this is the coordinated-omission-corrected number.
//
// All public methods are safe to call concurrently; the snapshot() method
// in particular is intended to be polled from a separate ticker goroutine
// while record() is being called from the result-consumer goroutine.
type aggregator struct {
	mu          sync.Mutex
	sent        int64
	errors      int64
	statusCodes map[int]int64
	service     *hdrhistogram.Histogram
	response    *hdrhistogram.Histogram
	started     time.Time
}

const histogramMaxMicros = int64(5 * time.Minute / time.Microsecond)

func newAggregator(start time.Time) *aggregator {
	return &aggregator{
		statusCodes: map[int]int64{},
		service:     hdrhistogram.New(1, histogramMaxMicros, 3),
		response:    hdrhistogram.New(1, histogramMaxMicros, 3),
		started:     start,
	}
}

func (a *aggregator) record(r Result) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sent++
	if r.Err != nil {
		a.errors++
	}
	if r.Status != 0 {
		a.statusCodes[r.Status]++
	}
	if !r.SentAt.IsZero() && !r.DoneAt.IsZero() && r.DoneAt.After(r.SentAt) {
		recordLatency(a.service, r.DoneAt.Sub(r.SentAt))
	}
	if !r.ScheduledAt.IsZero() && !r.DoneAt.IsZero() && r.DoneAt.After(r.ScheduledAt) {
		recordLatency(a.response, r.DoneAt.Sub(r.ScheduledAt))
	}
}

func recordLatency(h *hdrhistogram.Histogram, d time.Duration) {
	us := d.Microseconds()
	if us < 1 {
		us = 1
	}
	if us > histogramMaxMicros {
		us = histogramMaxMicros
	}
	_ = h.RecordValue(us)
}

// snapshot returns a point-in-time view of the run for live reporting.
// Safe to call concurrently with record(). CurrentRPS and RollingP99 are
// computed over the entire run so far (v1 simplification — true rolling
// window is a future refinement).
func (a *aggregator) snapshot() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	elapsed := time.Since(a.started)
	var rps float64
	if elapsed > 0 {
		rps = float64(a.sent) / elapsed.Seconds()
	}
	var p99 time.Duration
	if a.service.TotalCount() > 0 {
		p99 = time.Duration(a.service.ValueAtQuantile(99)) * time.Microsecond
	}
	statusCopy := make(map[int]int64, len(a.statusCodes))
	for k, v := range a.statusCodes {
		statusCopy[k] = v
	}
	return Snapshot{
		Elapsed:     elapsed,
		Sent:        a.sent,
		Errors:      a.errors,
		CurrentRPS:  rps,
		RollingP99:  p99,
		StatusCodes: statusCopy,
	}
}

// run consumes results from in until it is closed, then returns the Summary.
func (a *aggregator) run(in <-chan Result, plan Plan, target Target) *Summary {
	for r := range in {
		a.record(r)
	}
	return a.summary(plan, target, time.Now())
}

func (a *aggregator) summary(plan Plan, target Target, ended time.Time) *Summary {
	a.mu.Lock()
	defer a.mu.Unlock()

	elapsed := ended.Sub(a.started)
	var throughput float64
	if elapsed > 0 {
		throughput = float64(a.sent) / elapsed.Seconds()
	}
	return &Summary{
		Plan:        plan,
		Target:      target,
		Started:     a.started,
		Ended:       ended,
		TotalSent:   a.sent,
		Errors:      a.errors,
		StatusCodes: a.statusCodes,
		Throughput:  throughput,
		Service:     statsFromHistogram(a.service),
		Response:    statsFromHistogram(a.response),
	}
}

func statsFromHistogram(h *hdrhistogram.Histogram) LatencyStats {
	if h == nil || h.TotalCount() == 0 {
		return LatencyStats{Histogram: h}
	}
	p := func(q float64) time.Duration {
		return time.Duration(h.ValueAtQuantile(q)) * time.Microsecond
	}
	return LatencyStats{
		Min:       time.Duration(h.Min()) * time.Microsecond,
		P50:       p(50),
		P90:       p(90),
		P95:       p(95),
		P99:       p(99),
		P999:      p(99.9),
		Max:       time.Duration(h.Max()) * time.Microsecond,
		Histogram: h,
	}
}
