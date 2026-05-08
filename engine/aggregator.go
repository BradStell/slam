package engine

import (
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
type aggregator struct {
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

// run consumes results from in until it is closed, then returns the Summary.
func (a *aggregator) run(in <-chan Result, plan Plan, target Target) *Summary {
	for r := range in {
		a.record(r)
	}
	return a.summary(plan, target, time.Now())
}

func (a *aggregator) summary(plan Plan, target Target, ended time.Time) *Summary {
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
