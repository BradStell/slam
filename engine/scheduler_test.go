package engine

import (
	"context"
	"testing"
	"time"
)

func TestSchedule_UnlimitedFiresFast(t *testing.T) {
	in := make(chan token, 100)
	cfg := scheduleConfig{
		started:     time.Now(),
		maxRequests: 50,
	}
	if err := schedule(context.Background(), in, cfg); err != nil {
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
	cfg := scheduleConfig{started: start, rps: rps, duration: duration}
	err := schedule(context.Background(), in, cfg)
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
	cfg := scheduleConfig{started: start, rps: rps, maxRequests: 5}
	if err := schedule(context.Background(), in, cfg); err != nil {
		t.Fatalf("schedule: %v", err)
	}
	close(in)
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

	cfg := scheduleConfig{started: time.Now(), rps: 100, duration: 5 * time.Second}
	err := schedule(ctx, in, cfg)
	if err == nil {
		t.Error("expected ctx error, got nil")
	}
	close(in)
}

func TestSchedule_RampUpReducesEarlyRate(t *testing.T) {
	const (
		rps  = 2000
		ramp = 500 * time.Millisecond
		win  = 250 * time.Millisecond
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

	cfg := scheduleConfig{
		started:  time.Now(),
		rps:      rps,
		rampUp:   ramp,
		duration: win,
	}
	if err := schedule(context.Background(), in, cfg); err != nil {
		t.Fatalf("schedule: %v", err)
	}
	close(in)
	n := <-drained

	// Without ramp: 250ms @ 2000 RPS = 500 tokens.
	// With 500ms linear ramp, the first 250ms is at half-rate ≈ 125 tokens.
	// Allow ±100 for timing slop.
	if n < 75 || n > 225 {
		t.Errorf("count = %d, want ~125 (ramp halves the early rate)", n)
	}
}

func TestScheduledAtFor_RampUpClosedForm(t *testing.T) {
	cfg := scheduleConfig{
		started: time.Unix(0, 0),
		rps:     1000,
		rampUp:  10 * time.Second,
	}
	// At end of ramp, N(T) = R*T/2 = 5000 tokens fired.
	// Token #5000 should be at t = ~10s.
	got := scheduledAtFor(cfg, 5000)
	want := 10 * time.Second
	diff := got.Sub(cfg.started) - want
	if diff < -5*time.Millisecond || diff > 5*time.Millisecond {
		t.Errorf("token 5000 at %v, want ~%v (diff %v)", got.Sub(cfg.started), want, diff)
	}

	// Token #1250 should be at sqrt(2*10*1250/1000) = sqrt(25) = 5s.
	got = scheduledAtFor(cfg, 1250)
	want = 5 * time.Second
	diff = got.Sub(cfg.started) - want
	if diff < -5*time.Millisecond || diff > 5*time.Millisecond {
		t.Errorf("token 1250 at %v, want ~%v (diff %v)", got.Sub(cfg.started), want, diff)
	}

	// Token #5001 should be at 10s + 1ms (steady-state at 1000 RPS).
	got = scheduledAtFor(cfg, 5001)
	want = 10*time.Second + time.Millisecond
	diff = got.Sub(cfg.started) - want
	if diff < -100*time.Microsecond || diff > 100*time.Microsecond {
		t.Errorf("token 5001 at %v, want ~%v (diff %v)", got.Sub(cfg.started), want, diff)
	}
}
