package engine

import (
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

// aggregator consumes Results and produces a Summary. It maintains running
// counters and an HDR histogram for service latency (DoneAt - SentAt). The
// Response histogram (coordinated-omission corrected) is added in M2.4.
type aggregator struct {
	sent        int64
	errors      int64
	statusCodes map[int]int64
	service     *hdrhistogram.Histogram
	started     time.Time
}

// histogramRange is wide enough for any sane HTTP request; values above the
// upper bound are clamped to it rather than rejected.
const histogramMaxMicros = int64(5 * time.Minute / time.Microsecond)

func newAggregator(start time.Time) *aggregator {
	return &aggregator{
		statusCodes: map[int]int64{},
		service:     hdrhistogram.New(1, histogramMaxMicros, 3),
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
		us := r.DoneAt.Sub(r.SentAt).Microseconds()
		if us < 1 {
			us = 1
		}
		if us > histogramMaxMicros {
			us = histogramMaxMicros
		}
		_ = a.service.RecordValue(us)
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
