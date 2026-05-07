package engine

import (
	"errors"
	"testing"
	"time"
)

func TestAggregator_TotalsAndPercentiles(t *testing.T) {
	a := newAggregator(time.Now())
	base := time.Now()

	// 95 fast successes at 10ms, 5 slow errors at 50ms.
	for i := 0; i < 95; i++ {
		a.record(Result{
			SentAt: base,
			DoneAt: base.Add(10 * time.Millisecond),
			Status: 200,
		})
	}
	for i := 0; i < 5; i++ {
		a.record(Result{
			SentAt: base,
			DoneAt: base.Add(50 * time.Millisecond),
			Err:    errors.New("boom"),
		})
	}

	sum := a.summary(Plan{}, Target{}, time.Now())

	if sum.TotalSent != 100 {
		t.Errorf("TotalSent = %d, want 100", sum.TotalSent)
	}
	if sum.Errors != 5 {
		t.Errorf("Errors = %d, want 5", sum.Errors)
	}
	if sum.StatusCodes[200] != 95 {
		t.Errorf("StatusCodes[200] = %d, want 95", sum.StatusCodes[200])
	}

	// p50 should land in the 95-count cluster at ~10ms.
	if sum.Service.P50 < 8*time.Millisecond || sum.Service.P50 > 12*time.Millisecond {
		t.Errorf("Service.P50 = %v, want ~10ms", sum.Service.P50)
	}
	// Max should reflect the 50ms slow path.
	if sum.Service.Max < 40*time.Millisecond || sum.Service.Max > 60*time.Millisecond {
		t.Errorf("Service.Max = %v, want ~50ms", sum.Service.Max)
	}
}

func TestAggregator_RunDrainsChannel(t *testing.T) {
	a := newAggregator(time.Now())
	in := make(chan Result, 10)
	base := time.Now()
	for i := 0; i < 10; i++ {
		in <- Result{
			SentAt: base,
			DoneAt: base.Add(time.Millisecond),
			Status: 200,
		}
	}
	close(in)

	sum := a.run(in, Plan{}, Target{})
	if sum.TotalSent != 10 {
		t.Errorf("TotalSent = %d, want 10", sum.TotalSent)
	}
	if sum.StatusCodes[200] != 10 {
		t.Errorf("StatusCodes[200] = %d, want 10", sum.StatusCodes[200])
	}
}

func TestAggregator_ThroughputIsPerSecond(t *testing.T) {
	start := time.Now()
	a := newAggregator(start)
	for i := 0; i < 100; i++ {
		a.record(Result{
			SentAt: start,
			DoneAt: start.Add(time.Millisecond),
			Status: 200,
		})
	}
	// Pretend 2 seconds elapsed; 100/2 = 50 RPS.
	sum := a.summary(Plan{}, Target{}, start.Add(2*time.Second))
	if sum.Throughput < 49 || sum.Throughput > 51 {
		t.Errorf("Throughput = %v, want ~50", sum.Throughput)
	}
}

func TestAggregator_EmptyHistogramYieldsZeroStats(t *testing.T) {
	a := newAggregator(time.Now())
	sum := a.summary(Plan{}, Target{}, time.Now())
	if sum.TotalSent != 0 || sum.Errors != 0 {
		t.Errorf("expected zero counts, got sent=%d errors=%d", sum.TotalSent, sum.Errors)
	}
	if sum.Service.P50 != 0 || sum.Service.Max != 0 {
		t.Errorf("expected zero latency stats; got P50=%v Max=%v", sum.Service.P50, sum.Service.Max)
	}
}
