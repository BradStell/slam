package engine

import "time"

// Plan describes how to load — the tunable knobs.
//
// Termination semantics: when both Duration and Requests are zero the run
// continues until the context is canceled (e.g. via SIGINT). Otherwise it
// stops when whichever bound fires first.
type Plan struct {
	Concurrency       int           // worker count
	RPS               int           // target rate; 0 = unlimited (workers go flat-out)
	Duration          time.Duration // bounded by elapsed time, OR
	Requests          int           // bounded by total request count
	RampUp            time.Duration // linear 0 → RPS over this window; 0 = no ramp
	Timeout           time.Duration // per-request timeout; 0 = no timeout
	DisableKeepAlives bool          // matches http.Transport.DisableKeepAlives
	HTTP2             bool          // prefer HTTP/2 (h2 over TLS, h2c over cleartext)
}
