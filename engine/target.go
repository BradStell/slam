package engine

import (
	"net/http"
	"net/url"
)

// Target describes what to load — for v1, a single endpoint.
type Target struct {
	URL     string
	Method  string
	Headers http.Header
	Body    []byte
	Query   url.Values
}
