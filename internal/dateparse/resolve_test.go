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

		// Day names (ref is Wednesday)
		{"wednesday", time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local), true}, // today
		{"thursday", time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local), true},  // tomorrow
		{"monday", time.Date(2025, 2, 3, 0, 0, 0, 0, time.Local), true},     // next Monday
		{"friday", time.Date(2025, 1, 31, 0, 0, 0, 0, time.Local), true},
		{"sunday", time.Date(2025, 2, 2, 0, 0, 0, 0, time.Local), true},
		{"tuesday", time.Date(2025, 2, 4, 0, 0, 0, 0, time.Local), true},

		// Named relative
		{"next-week", time.Date(2025, 2, 3, 0, 0, 0, 0, time.Local), true},  // next Monday
		{"next-month", time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local), true},  // Feb 1
		{"next-year", time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local), true},   // Jan 1 2026

		// Numeric offsets (forward)
		{"2-days", time.Date(2025, 1, 31, 0, 0, 0, 0, time.Local), true},
		{"7-days", time.Date(2025, 2, 5, 0, 0, 0, 0, time.Local), true},
		{"1-day", time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local), true},
		{"2-weeks", time.Date(2025, 2, 12, 0, 0, 0, 0, time.Local), true},
		{"1-week", time.Date(2025, 2, 5, 0, 0, 0, 0, time.Local), true},
		{"2-months", time.Date(2025, 3, 29, 0, 0, 0, 0, time.Local), true},
		{"1-month", time.Date(2025, 3, 1, 0, 0, 0, 0, time.Local), true}, // Jan 29 + 1 month = Mar 1 (Feb overflow)
		{"2-years", time.Date(2027, 1, 29, 0, 0, 0, 0, time.Local), true},
		{"1-year", time.Date(2026, 1, 29, 0, 0, 0, 0, time.Local), true},

		// Shorthand forms are NOT forward-resolved (they stay as lookback in Parse)
		{"7d", time.Date(2025, 1, 23, 0, 0, 0, 0, time.Local), true},  // lookback via Parse
		{"2w", time.Date(2025, 1, 16, 0, 0, 0, 0, time.Local), true},  // lookback via Parse

		// ISO dates
		{"2025-03-15", time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local), true},

		// Case insensitive
		{"TODAY", time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local), true},
		{"Monday", time.Date(2025, 2, 3, 0, 0, 0, 0, time.Local), true},
		{"Next-Week", time.Date(2025, 2, 3, 0, 0, 0, 0, time.Local), true},

		// Unrecognized
		{"invalid", time.Time{}, false},
		{"kevin", time.Time{}, false},
		{"deprecated", time.Time{}, false},
		{"", time.Time{}, false},
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

func TestResolveDateAt_DayNameOnThatDay(t *testing.T) {
	// Monday ref
	ref := time.Date(2025, 2, 3, 10, 0, 0, 0, time.Local)
	got, ok := ResolveDateAt("monday", ref)
	if !ok {
		t.Fatal("expected ok")
	}
	// Should return today (Monday) since ref is Monday
	want := time.Date(2025, 2, 3, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("ResolveDateAt(monday) on Monday = %v, want %v", got, want)
	}
}

func TestResolveDateAt_NextWeekFromMonday(t *testing.T) {
	// If today is Monday, next-week should be NEXT Monday (7 days)
	ref := time.Date(2025, 2, 3, 10, 0, 0, 0, time.Local) // Monday
	got, ok := ResolveDateAt("next-week", ref)
	if !ok {
		t.Fatal("expected ok")
	}
	want := time.Date(2025, 2, 10, 0, 0, 0, 0, time.Local) // next Monday
	if !got.Equal(want) {
		t.Errorf("ResolveDateAt(next-week) on Monday = %v, want %v", got, want)
	}
}
