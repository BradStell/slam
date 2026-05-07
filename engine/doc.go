// Package engine is the load-testing core for slam.
//
// It is consumed by the slam CLI and (later) the slam GUI. The public entry
// point is Runner, which executes a Plan against a Target and returns a
// Summary including HDR-histogram-based latency statistics with
// coordinated-omission correction.
package engine
