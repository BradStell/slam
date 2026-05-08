package cli

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// parseURL normalizes a user-supplied URL. If no scheme is present it defaults
// to http://; if no path is present it defaults to /. Examples:
//
//	"https://example.com/path"   → "https://example.com/path"
//	"localhost:3000/foo"         → "http://localhost:3000/foo"
//	"example.com"                → "http://example.com/"
func parseURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("URL is empty")
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("URL %q has no host", raw)
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), nil
}

// urlHostPort matches a bare host[:port][/path] without scheme.
var urlHostPort = regexp.MustCompile(`^[A-Za-z0-9._-]+(:\d+)?(/.*)?$`)

// looksLikeURL returns true if s is plausibly a URL rather than a subcommand
// name. It triggers on "://" anywhere in s, or on a host[:port][/path] shape
// that contains at least one of '.', ':', or '/'. Flag-like strings (leading
// '-') are always rejected.
func looksLikeURL(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	if strings.Contains(s, "://") {
		return true
	}
	if !urlHostPort.MatchString(s) {
		return false
	}
	return strings.ContainsAny(s, ".:/")
}
