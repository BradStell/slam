package engine

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
)

// execute performs a single HTTP request described by target, returning a
// Result with SentAt/DoneAt timestamps, status, error, and response size.
// The caller is responsible for setting ScheduledAt — the transport doesn't
// know about scheduling.
func execute(ctx context.Context, client *http.Client, target Target) Result {
	method := target.Method
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if len(target.Body) > 0 {
		body = bytes.NewReader(target.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, target.URL, body)
	if err != nil {
		now := time.Now()
		return Result{SentAt: now, DoneAt: now, Err: err}
	}

	if len(target.Headers) > 0 {
		req.Header = target.Headers.Clone()
	}
	if len(target.Query) > 0 {
		q := req.URL.Query()
		for k, vs := range target.Query {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		req.URL.RawQuery = q.Encode()
	}

	res := Result{SentAt: time.Now()}

	resp, err := client.Do(req)
	if err != nil {
		res.DoneAt = time.Now()
		res.Err = err
		return res
	}
	defer resp.Body.Close()

	n, copyErr := io.Copy(io.Discard, resp.Body)
	res.DoneAt = time.Now()
	res.Status = resp.StatusCode
	res.BytesIn = n
	if copyErr != nil {
		res.Err = copyErr
	}
	return res
}

// defaultClient builds an *http.Client tuned for high-volume load tests.
// Connection pooling is generous; Timeout applies per-request.
func defaultClient(timeout time.Duration, disableKeepAlives, preferHTTP2 bool) *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DisableKeepAlives:     disableKeepAlives,
		MaxIdleConns:          0,
		MaxIdleConnsPerHost:   1024,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     preferHTTP2,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}
