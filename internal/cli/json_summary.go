package cli

import (
	"encoding/json"
	"io"
	"strconv"
	"time"

	"github.com/bradstell/slam/engine"
)

type jsonSummary struct {
	Target        jsonTarget       `json:"target"`
	Plan          jsonPlan         `json:"plan"`
	Started       time.Time        `json:"started"`
	Ended         time.Time        `json:"ended"`
	ElapsedMS     float64          `json:"elapsed_ms"`
	TotalSent     int64            `json:"total_sent"`
	Errors        int64            `json:"errors"`
	ThroughputRPS float64          `json:"throughput_rps"`
	StatusCodes   map[string]int64 `json:"status_codes,omitempty"`
	Service       jsonLatency      `json:"service_latency"`
	Response      jsonLatency      `json:"response_latency"`
}

type jsonTarget struct {
	URL     string              `json:"url"`
	Method  string              `json:"method,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
	Query   map[string][]string `json:"query,omitempty"`
}

type jsonPlan struct {
	Concurrency       int     `json:"concurrency"`
	RPS               int     `json:"rps,omitempty"`
	DurationSeconds   float64 `json:"duration_seconds,omitempty"`
	Requests          int     `json:"requests,omitempty"`
	RampUpSeconds     float64 `json:"ramp_up_seconds,omitempty"`
	TimeoutSeconds    float64 `json:"timeout_seconds,omitempty"`
	DisableKeepAlives bool    `json:"disable_keepalives,omitempty"`
	HTTP2             bool    `json:"http2,omitempty"`
}

type jsonLatency struct {
	MinMS  float64 `json:"min_ms,omitempty"`
	P50MS  float64 `json:"p50_ms,omitempty"`
	P90MS  float64 `json:"p90_ms,omitempty"`
	P95MS  float64 `json:"p95_ms,omitempty"`
	P99MS  float64 `json:"p99_ms,omitempty"`
	P999MS float64 `json:"p999_ms,omitempty"`
	MaxMS  float64 `json:"max_ms,omitempty"`
}

func printJSONSummary(w io.Writer, sum *engine.Summary) error {
	js := jsonSummary{
		Target: jsonTarget{
			URL:     sum.Target.URL,
			Method:  sum.Target.Method,
			Headers: sum.Target.Headers,
			Query:   sum.Target.Query,
		},
		Plan: jsonPlan{
			Concurrency:       sum.Plan.Concurrency,
			RPS:               sum.Plan.RPS,
			DurationSeconds:   sum.Plan.Duration.Seconds(),
			Requests:          sum.Plan.Requests,
			RampUpSeconds:     sum.Plan.RampUp.Seconds(),
			TimeoutSeconds:    sum.Plan.Timeout.Seconds(),
			DisableKeepAlives: sum.Plan.DisableKeepAlives,
			HTTP2:             sum.Plan.HTTP2,
		},
		Started:       sum.Started,
		Ended:         sum.Ended,
		ElapsedMS:     msFloat(sum.Ended.Sub(sum.Started)),
		TotalSent:     sum.TotalSent,
		Errors:        sum.Errors,
		ThroughputRPS: sum.Throughput,
		StatusCodes:   stringifyStatusKeys(sum.StatusCodes),
		Service:       toJSONLatency(sum.Service),
		Response:      toJSONLatency(sum.Response),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(js)
}

func toJSONLatency(l engine.LatencyStats) jsonLatency {
	return jsonLatency{
		MinMS:  msFloat(l.Min),
		P50MS:  msFloat(l.P50),
		P90MS:  msFloat(l.P90),
		P95MS:  msFloat(l.P95),
		P99MS:  msFloat(l.P99),
		P999MS: msFloat(l.P999),
		MaxMS:  msFloat(l.Max),
	}
}

func msFloat(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func stringifyStatusKeys(m map[int]int64) map[string]int64 {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[strconv.Itoa(k)] = v
	}
	return out
}
