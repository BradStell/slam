package cli

import "testing"

func TestParseURL_Normalization(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://example.com/path", "https://example.com/path"},
		{"http://localhost:3000", "http://localhost:3000/"},
		{"localhost:3000/foo", "http://localhost:3000/foo"},
		{"localhost:3000", "http://localhost:3000/"},
		{"example.com", "http://example.com/"},
		{"127.0.0.1:8080/api", "http://127.0.0.1:8080/api"},
		{"  https://example.com  ", "https://example.com/"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseURL(tc.in)
			if err != nil {
				t.Fatalf("parseURL(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("parseURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseURL_Invalid(t *testing.T) {
	for _, in := range []string{"", "   "} {
		if _, err := parseURL(in); err == nil {
			t.Errorf("parseURL(%q) should error", in)
		}
	}
}

func TestLooksLikeURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"https://example.com", true},
		{"http://localhost:3000", true},
		{"localhost:3000", true},
		{"localhost:3000/foo", true},
		{"example.com", true},
		{"127.0.0.1:8080", true},
		{"127.0.0.1", true},
		{"run", false},
		{"serve", false},
		{"compare", false},
		{"-h", false},
		{"--version", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := looksLikeURL(tc.in); got != tc.want {
				t.Errorf("looksLikeURL(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
