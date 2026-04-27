package dateparse

import (
	"strings"
	"testing"
	"time"
)

func TestParseNaturalLanguage(t *testing.T) {
	// Use a fixed reference time for consistent testing
	ref := time.Date(2025, 1, 29, 14, 30, 0, 0, time.Local) // Wednesday

	tests := []struct {
		input     string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			input:     "today",
			wantStart: time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "yesterday",
			wantStart: time.Date(2025, 1, 28, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "tomorrow",
			wantStart: time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 31, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "this-week",
			wantStart: time.Date(2025, 1, 27, 0, 0, 0, 0, time.Local), // Monday
			wantEnd:   time.Date(2025, 2, 3, 0, 0, 0, 0, time.Local),  // Next Monday
		},
		{
			input:     "last-week",
			wantStart: time.Date(2025, 1, 20, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 27, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "next-week",
			wantStart: time.Date(2025, 2, 3, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 2, 10, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "this-month",
			wantStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "last-month",
			wantStart: time.Date(2024, 12, 1, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "next-month",
			wantStart: time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 3, 1, 0, 0, 0, 0, time.Local),
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseWithReference(tt.input, ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseWithReference(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !got.Start.Equal(tt.wantStart) {
					t.Errorf("ParseWithReference(%q).Start = %v, want %v", tt.input, got.Start, tt.wantStart)
				}
				if !got.End.Equal(tt.wantEnd) {
					t.Errorf("ParseWithReference(%q).End = %v, want %v", tt.input, got.End, tt.wantEnd)
				}
			}
		})
	}
}

func TestParseRemovedTermsError(t *testing.T) {
	ref := time.Date(2025, 1, 29, 14, 30, 0, 0, time.Local)

	removed := []string{
		"this-year", "last-year", "next-year",
		"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday",
		"7d", "7-days", "2w", "2-weeks", "3m", "3-months",
	}

	for _, input := range removed {
		t.Run(input, func(t *testing.T) {
			_, err := ParseWithReference(input, ref)
			if err == nil {
				t.Errorf("ParseWithReference(%q) should error, but got nil", input)
			}
		})
	}
}

func TestParseISODate(t *testing.T) {
	tests := []struct {
		input     string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			input:     "2025-01-29",
			wantStart: time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "2025-01",
			wantStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "2025",
			wantStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !got.Start.Equal(tt.wantStart) {
					t.Errorf("Parse(%q).Start = %v, want %v", tt.input, got.Start, tt.wantStart)
				}
				if !got.End.Equal(tt.wantEnd) {
					t.Errorf("Parse(%q).End = %v, want %v", tt.input, got.End, tt.wantEnd)
				}
			}
		})
	}
}

func TestDateRangeContains(t *testing.T) {
	// Range: 2025-01-29 00:00:00 to 2025-01-30 00:00:00
	r := DateRange{
		Start: time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local),
		End:   time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
	}

	tests := []struct {
		name string
		t    time.Time
		want bool
	}{
		{
			name: "before range",
			t:    time.Date(2025, 1, 28, 23, 59, 59, 0, time.Local),
			want: false,
		},
		{
			name: "at start (inclusive)",
			t:    time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local),
			want: true,
		},
		{
			name: "middle of range",
			t:    time.Date(2025, 1, 29, 12, 0, 0, 0, time.Local),
			want: true,
		},
		{
			name: "just before end",
			t:    time.Date(2025, 1, 29, 23, 59, 59, 0, time.Local),
			want: true,
		},
		{
			name: "at end (exclusive)",
			t:    time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
			want: false,
		},
		{
			name: "after range",
			t:    time.Date(2025, 1, 30, 0, 0, 1, 0, time.Local),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.Contains(tt.t); got != tt.want {
				t.Errorf("DateRange.Contains(%v) = %v, want %v", tt.t, got, tt.want)
			}
		})
	}
}

func TestParseRelativeArithmetic(t *testing.T) {
	ref := time.Date(2025, 1, 29, 14, 30, 0, 0, time.Local)

	tests := []struct {
		input     string
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			input:     "today+0",
			wantStart: time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "today+1",
			wantStart: time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 31, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "today+6",
			wantStart: time.Date(2025, 2, 4, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 2, 5, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "today-1",
			wantStart: time.Date(2025, 1, 28, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "today-7",
			wantStart: time.Date(2025, 1, 22, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 23, 0, 0, 0, 0, time.Local),
		},
		{
			// Crosses end-of-month boundary.
			input:     "today-30",
			wantStart: time.Date(2024, 12, 30, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2024, 12, 31, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "TODAY+3",
			wantStart: time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 2, 2, 0, 0, 0, 0, time.Local),
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseWithReference(tt.input, ref)
			if err != nil {
				t.Fatalf("ParseWithReference(%q) unexpected error: %v", tt.input, err)
			}
			if !got.Start.Equal(tt.wantStart) {
				t.Errorf("Start = %v, want %v", got.Start, tt.wantStart)
			}
			if !got.End.Equal(tt.wantEnd) {
				t.Errorf("End = %v, want %v", got.End, tt.wantEnd)
			}
		})
	}
}

func TestParseRelativeArithmeticLeapYear(t *testing.T) {
	// 2024-02-28 + 1 day = 2024-02-29 (leap year)
	ref := time.Date(2024, 2, 28, 12, 0, 0, 0, time.Local)
	got, err := ParseWithReference("today+1", ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2024, 2, 29, 0, 0, 0, 0, time.Local)
	if !got.Start.Equal(want) {
		t.Errorf("Start = %v, want %v", got.Start, want)
	}
}

func TestParseRelativeArithmeticErrors(t *testing.T) {
	ref := time.Date(2025, 1, 29, 14, 30, 0, 0, time.Local)

	cases := []struct {
		input     string
		wantInErr string
	}{
		{"today--7", "non-negative"},
		{"today-+7", "non-negative"},
		{"today + 6", "whitespace"},
		{"today +6", "whitespace"},
		{"today- 6", "whitespace"},
		{"today+", "expected today+N"},
		{"today-", "expected today+N"},
		{"today+abc", "non-negative integer"},
		{"today+1.5", "non-negative integer"},
	}

	for _, tt := range cases {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ParseWithReference(tt.input, ref)
			if err == nil {
				t.Fatalf("ParseWithReference(%q) expected error, got nil", tt.input)
			}
			if !strings.Contains(err.Error(), tt.wantInErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantInErr)
			}
		})
	}
}

func TestParseRejectsYesterdayTomorrowArithmetic(t *testing.T) {
	ref := time.Date(2025, 1, 29, 14, 30, 0, 0, time.Local)

	rejected := []string{"yesterday-1", "yesterday+1", "tomorrow+1", "tomorrow-1"}
	for _, input := range rejected {
		t.Run(input, func(t *testing.T) {
			_, err := ParseWithReference(input, ref)
			if err == nil {
				t.Errorf("ParseWithReference(%q) should error (only today supports arithmetic)", input)
			}
		})
	}
}

func TestCaseInsensitive(t *testing.T) {
	ref := time.Date(2025, 1, 29, 14, 30, 0, 0, time.Local)

	tests := []string{"TODAY", "Today", "YESTERDAY", "Yesterday"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := ParseWithReference(input, ref)
			if err != nil {
				t.Errorf("ParseWithReference(%q) should not error for case variations, got %v", input, err)
			}
		})
	}
}
