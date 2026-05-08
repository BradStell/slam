package engine

import (
	"context"
	"time"
)

// schedule emits tokens to in until ctx is canceled, the duration elapses,
// or maxRequests tokens have been sent. It returns nil on natural completion
// or ctx.Err() on cancellation.
//
// When rps > 0 the schedule is fixed: token i is intended to fire at
// started + i/rps. The token's ScheduledAt records the *intended* time even
// if the worker pool is slow and the actual send is late — that's what
// preserves coordinated-omission correctness downstream.
//
// When rps == 0 tokens are emitted as fast as the channel accepts them,
// and ScheduledAt equals the actual send time.
func schedule(ctx context.Context, in chan<- token, started time.Time, rps int, duration time.Duration, maxRequests int) error {
	var deadline <-chan time.Time
	if duration > 0 {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		deadline = timer.C
	}

	rateLimited := rps > 0
	var (
		interval  time.Duration
		nextSched = started
	)
	if rateLimited {
		interval = time.Second / time.Duration(rps)
	}

	sent := 0
	for {
		if maxRequests > 0 && sent >= maxRequests {
			return nil
		}

		scheduledAt := time.Now()
		if rateLimited {
			scheduledAt = nextSched
			if wait := time.Until(scheduledAt); wait > 0 {
				select {
				case <-time.After(wait):
				case <-deadline:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		select {
		case in <- token{ScheduledAt: scheduledAt}:
			sent++
			if rateLimited {
				nextSched = nextSched.Add(interval)
			}
		case <-deadline:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
