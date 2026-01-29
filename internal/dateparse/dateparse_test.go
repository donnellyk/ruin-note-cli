package dateparse

import (
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
			wantStart: time.Date(2025, 1, 20, 0, 0, 0, 0, time.Local), // Previous Monday
			wantEnd:   time.Date(2025, 1, 27, 0, 0, 0, 0, time.Local),
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
			input:     "this-year",
			wantStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "last-year",
			wantStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
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

func TestParseRelativeDuration(t *testing.T) {
	ref := time.Date(2025, 1, 29, 14, 30, 0, 0, time.Local)

	tests := []struct {
		input     string
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			input:     "7d",
			wantStart: time.Date(2025, 1, 23, 0, 0, 0, 0, time.Local), // 6 days back + today
			wantEnd:   time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "7-days",
			wantStart: time.Date(2025, 1, 23, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "1d",
			wantStart: time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local), // Just today
			wantEnd:   time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "2w",
			wantStart: time.Date(2025, 1, 16, 0, 0, 0, 0, time.Local), // 13 days back + today
			wantEnd:   time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "2-weeks",
			wantStart: time.Date(2025, 1, 16, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "1m",
			wantStart: time.Date(2024, 12, 30, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
		},
		{
			input:     "3-months",
			wantStart: time.Date(2024, 10, 30, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2025, 1, 30, 0, 0, 0, 0, time.Local),
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseWithReference(tt.input, ref)
			if err != nil {
				t.Errorf("ParseWithReference(%q) error = %v", tt.input, err)
				return
			}
			if !got.Start.Equal(tt.wantStart) {
				t.Errorf("ParseWithReference(%q).Start = %v, want %v", tt.input, got.Start, tt.wantStart)
			}
			if !got.End.Equal(tt.wantEnd) {
				t.Errorf("ParseWithReference(%q).End = %v, want %v", tt.input, got.End, tt.wantEnd)
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

func TestCaseInsensitive(t *testing.T) {
	ref := time.Date(2025, 1, 29, 14, 30, 0, 0, time.Local)

	tests := []string{"TODAY", "Today", "YESTERDAY", "Yesterday", "THIS-WEEK", "This-Week"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := ParseWithReference(input, ref)
			if err != nil {
				t.Errorf("ParseWithReference(%q) should not error for case variations, got %v", input, err)
			}
		})
	}
}
