package engine

import (
	"context"
	"testing"
	"time"
)

func TestSchedule_UnlimitedFiresFast(t *testing.T) {
	in := make(chan token, 100)
	if err := schedule(context.Background(), in, time.Now(), 0, 0, 50); err != nil {
		t.Fatalf("schedule: %v", err)
	}
	close(in)

	count := 0
	for range in {
		count++
	}
	if count != 50 {
		t.Errorf("count = %d, want 50", count)
	}
}

func TestSchedule_RateLimitedHitsTarget(t *testing.T) {
	const (
		rps      = 100
		duration = 1 * time.Second
	)

	in := make(chan token, rps*2)
	drained := make(chan int, 1)
	go func() {
		n := 0
		for range in {
			n++
		}
		drained <- n
	}()

	start := time.Now()
	err := schedule(context.Background(), in, start, rps, duration, 0)
	elapsed := time.Since(start)
	close(in)
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	n := <-drained

	if n < rps-15 || n > rps+15 {
		t.Errorf("count = %d, want ~%d (±15)", n, rps)
	}
	if elapsed < 900*time.Millisecond || elapsed > 1300*time.Millisecond {
		t.Errorf("elapsed = %v, want ~1s", elapsed)
	}
}

func TestSchedule_TokensCarryScheduledAt(t *testing.T) {
	const rps = 100
	in := make(chan token, 10)
	drained := make(chan []token, 1)
	go func() {
		var got []token
		for tok := range in {
			got = append(got, tok)
		}
		drained <- got
	}()

	start := time.Now()
	err := schedule(context.Background(), in, start, rps, 0, 5)
	close(in)
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	tokens := <-drained

	if len(tokens) != 5 {
		t.Fatalf("len = %d, want 5", len(tokens))
	}
	want := time.Second / time.Duration(rps)
	for i := 1; i < len(tokens); i++ {
		gap := tokens[i].ScheduledAt.Sub(tokens[i-1].ScheduledAt)
		if gap < want-time.Millisecond || gap > want+time.Millisecond {
			t.Errorf("gap[%d→%d] = %v, want %v ±1ms", i-1, i, gap, want)
		}
	}
}

func TestSchedule_CtxCancel(t *testing.T) {
	in := make(chan token, 1)
	go func() {
		for range in {
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := schedule(ctx, in, time.Now(), 100, 5*time.Second, 0)
	if err == nil {
		t.Error("expected ctx error, got nil")
	}
	close(in)
}
