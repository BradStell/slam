package engine

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// token represents one scheduled request emission. ScheduledAt is the time
// the request was meant to be sent. For unrate-limited runs the scheduler
// sets ScheduledAt = now at emission, making it equivalent to SentAt.
type token struct {
	ScheduledAt time.Time
}

// workerPool runs n workers, each consuming tokens from in, executing the
// request described by target via client, and writing each Result to out.
// It blocks until in is closed and all in-flight requests produce results,
// then closes out.
//
// If ctx is canceled, in-flight requests fail fast (execute is ctx-aware)
// and remaining tokens are drained as fast-failing executions — workers
// never block on a closed-loop send back to the scheduler.
func workerPool(ctx context.Context, n int, client *http.Client, target Target, in <-chan token, out chan<- Result) {
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			for tok := range in {
				res := execute(ctx, client, target)
				res.ScheduledAt = tok.ScheduledAt
				select {
				case out <- res:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	wg.Wait()
	close(out)
}
