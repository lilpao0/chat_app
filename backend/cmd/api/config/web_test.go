package config

import (
	"strings"
	"testing"
)

func TestWebOrigins(t *testing.T) {
	for _, tc := range []struct {
		raw, want string
		invalid   bool
	}{
		{"", "", false},
		{" http://localhost:5173,https://EXAMPLE.com:443,http://localhost:5173 ", "http://localhost:5173,https://example.com", false},
		{"http://[::1]:80", "http://[::1]", false},
		{"http://localhost:5173,http://127.0.0.1:5173", "http://localhost:5173,http://127.0.0.1:5173", false},
		{"*", "", true}, {"null", "", true}, {"ws://localhost:5173", "", true},
		{"http://localhost:5173/", "", true}, {"https://user@example.com", "", true},
		{"http://localhost:0", "", true}, {"http://localhost:65536", "", true},
		{"http://localhost:", "", true}, {"http://localhost:abc", "", true},
		{"https://example.com?", "", true}, {"https://example.com#", "", true},
		{"https://*.example.com", "", true}, {"https://", "", true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			t.Setenv("WEB_ALLOWED_ORIGINS", tc.raw)
			got, err := LoadWebOrigins()
			if (err != nil) != tc.invalid || (err == nil && strings.Join(got, ",") != tc.want) {
				t.Fatalf("origins=%v err=%v", got, err)
			}
		})
	}
}
