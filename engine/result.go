package engine

import (
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

// Result is the outcome of a single request execution.
//
// ScheduledAt and SentAt may differ when the scheduler runs at a fixed rate
// and a worker is late picking up its token (coordinated-omission territory).
// Recording both lets the aggregator compute service latency (DoneAt-SentAt)
// and response latency (DoneAt-ScheduledAt) as separate distributions.
type Result struct {
	ScheduledAt time.Time
	SentAt      time.Time
	DoneAt      time.Time
	Status      int   // HTTP status code; 0 if no response
	Err         error // transport-level error, if any
	BytesIn     int64 // response body bytes
}

// Snapshot is a point-in-time view of an in-progress run, emitted to the
// Reporter on a tick.
type Snapshot struct {
	Elapsed     time.Duration
	Sent        int64
	Errors      int64
	CurrentRPS  float64       // measured over the most recent window
	RollingP99  time.Duration // p99 over the most recent window
	StatusCodes map[int]int64
}

// Summary is the final report at the end of a run.
type Summary struct {
	Plan        Plan
	Target      Target
	Started     time.Time
	Ended       time.Time
	TotalSent   int64
	Errors      int64
	StatusCodes map[int]int64
	Throughput  float64      // RPS achieved over the whole run
	Service     LatencyStats // SentAt → DoneAt — server-side perspective
	Response    LatencyStats // ScheduledAt → DoneAt — client-perceived, CO-corrected
}

// LatencyStats summarizes a latency distribution. Histogram is exported so
// callers can serialize it for storage or merge across runs.
type LatencyStats struct {
	Min, P50, P90, P95, P99, P999, Max time.Duration
	Histogram                          *hdrhistogram.Histogram
}
