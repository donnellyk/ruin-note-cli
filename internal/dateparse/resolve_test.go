package dateparse

import (
	"testing"
	"time"
)

func TestResolveDateAt(t *testing.T) {
	// Wednesday, 2025-01-29
	ref := time.Date(2025, 1, 29, 14, 30, 0, 0, time.Local)

	tests := []struct {
		token string
		want  time.Time
		ok    bool
	}{
		// Fixed tokens
		{"today", time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local), true},
		{"tomorrow", time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local), true},
		{"yesterday", time.Date(2025, 1, 28, 0, 0, 0, 0, time.Local), true},

		// ISO dates
		{"2025-03-15", time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local), true},

		// Case insensitive
		{"TODAY", time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local), true},

		// Unrecognized
		{"invalid", time.Time{}, false},
		{"kevin", time.Time{}, false},
		{"deprecated", time.Time{}, false},
		{"", time.Time{}, false},

		// Removed terms should not resolve
		{"monday", time.Time{}, false},
		{"next-week", time.Date(2025, 2, 3, 0, 0, 0, 0, time.Local), true},
		{"2-days", time.Time{}, false},
		{"7d", time.Time{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			got, ok := ResolveDateAt(tt.token, ref)
			if ok != tt.ok {
				t.Fatalf("ResolveDateAt(%q) ok = %v, want %v", tt.token, ok, tt.ok)
			}
			if ok && !got.Equal(tt.want) {
				t.Errorf("ResolveDateAt(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}
