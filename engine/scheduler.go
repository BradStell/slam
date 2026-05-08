package engine

import (
	"context"
	"math"
	"time"
)

// scheduleConfig collects the producer-side knobs for a single run.
type scheduleConfig struct {
	started     time.Time
	rps         int           // 0 = unlimited (workers go flat-out)
	rampUp      time.Duration // 0 = no ramp; linear 0 → rps over this window
	duration    time.Duration // 0 = no time bound
	maxRequests int           // 0 = no count bound
}

// schedule emits tokens to in based on cfg until ctx is canceled, cfg.duration
// elapses, or cfg.maxRequests tokens have been sent. Returns nil on natural
// completion or ctx.Err() on cancellation.
//
// When cfg.rps > 0 the emission schedule is fixed: the i-th token is intended
// at scheduledAtFor(cfg, i). ScheduledAt records the *intended* time even if
// the worker pool is slow and the actual send is late — that's what preserves
// coordinated-omission correctness downstream.
//
// When cfg.rps == 0 tokens are emitted as fast as the channel accepts them
// and ScheduledAt equals the actual send time.
func schedule(ctx context.Context, in chan<- token, cfg scheduleConfig) error {
	var deadline <-chan time.Time
	if cfg.duration > 0 {
		timer := time.NewTimer(cfg.duration)
		defer timer.Stop()
		deadline = timer.C
	}

	rateLimited := cfg.rps > 0

	sent := 0
	for {
		if cfg.maxRequests > 0 && sent >= cfg.maxRequests {
			return nil
		}

		var scheduledAt time.Time
		if rateLimited {
			scheduledAt = scheduledAtFor(cfg, sent)
			if wait := time.Until(scheduledAt); wait > 0 {
				select {
				case <-time.After(wait):
				case <-deadline:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		} else {
			scheduledAt = time.Now()
		}

		select {
		case in <- token{ScheduledAt: scheduledAt}:
			sent++
		case <-deadline:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// scheduledAtFor returns the intended emission time of the i-th (0-indexed)
// token under cfg. Assumes cfg.rps > 0.
//
// With no ramp, t_i = i / rps (linear spacing).
// With a ramp of duration T, the rate is r(t) = rps*t/T for t in [0, T] and
// then constant rps. Cumulative count N(t) = rps*t²/(2T) during ramp, so
// solving N(t_i) = i gives t_i = sqrt(2*T*i/rps).
func scheduledAtFor(cfg scheduleConfig, i int) time.Time {
	if cfg.rampUp == 0 {
		interval := time.Second / time.Duration(cfg.rps)
		return cfg.started.Add(time.Duration(i) * interval)
	}

	rampSec := cfg.rampUp.Seconds()
	rate := float64(cfg.rps)
	tokensInRamp := rate * rampSec / 2

	if float64(i) <= tokensInRamp {
		offsetSec := math.Sqrt(2 * rampSec * float64(i) / rate)
		return cfg.started.Add(time.Duration(offsetSec * float64(time.Second)))
	}
	extra := float64(i) - tokensInRamp
	return cfg.started.Add(cfg.rampUp + time.Duration(extra*float64(time.Second)/rate))
}
