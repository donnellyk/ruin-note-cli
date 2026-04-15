package note

import (
	"strings"
	"testing"
)

func TestResolveDateTokens(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		contain string // substring that must be in output
		absent  string // substring that must NOT be in output (optional)
	}{
		{
			name:    "resolve @today",
			input:   "follow up @today",
			contain: "follow up @20", // starts with @20xx-
			absent:  "@today",
		},
		{
			name:    "resolve @tomorrow",
			input:   "call them @tomorrow",
			contain: "call them @20",
			absent:  "@tomorrow",
		},
		{
			name:    "leave literal date alone",
			input:   "due @2026-02-15",
			contain: "@2026-02-15",
		},
		{
			name:    "leave email alone",
			input:   "email user@today.com",
			contain: "user@today.com",
		},
		{
			name:    "leave unrecognized token alone",
			input:   "see @kevin for details",
			contain: "@kevin",
		},
		{
			name:   "multiple tokens",
			input:  "@today and @tomorrow",
			absent: "@today",
		},
		{
			name:   "token at start of line",
			input:  "@tomorrow do the thing",
			absent: "@tomorrow",
		},
		{
			name:    "no tokens",
			input:   "plain text without any tokens",
			contain: "plain text without any tokens",
		},
		{
			name:    "leave @monday alone (removed)",
			input:   "meeting @monday",
			contain: "@monday",
		},
		{
			name:    "resolve @next-week",
			input:   "review @next-week",
			contain: "@20", // resolves to @YYYY-MM-DD
			absent:  "@next-week",
		},
		{
			name:    "leave @2-days alone (removed)",
			input:   "deadline @2-days",
			contain: "@2-days",
		},
		{
			name:    "underscore prefix leaves alone",
			input:   "var_@today",
			contain: "var_@today",
		},
		{
			name:    "skip @today inside static embed",
			input:   "![[pick: #followup @today]]",
			contain: "@today",
			absent:  "@20",
		},
		{
			name:    "skip @tomorrow inside dynamic embed",
			input:   "![[search: #daily @tomorrow | limit=5]]",
			contain: "@tomorrow",
			absent:  "@20",
		},
		{
			name:   "resolve @today outside embed but skip inside",
			input:  "meeting @today\n![[pick: #followup @today]]",
			absent: "meeting @today",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveDateTokens(tt.input)
			if !strings.Contains(got, tt.contain) {
				t.Errorf("ResolveDateTokens(%q) = %q, expected to contain %q", tt.input, got, tt.contain)
			}
			if tt.absent != "" && strings.Contains(got, tt.absent) {
				t.Errorf("ResolveDateTokens(%q) = %q, expected NOT to contain %q", tt.input, got, tt.absent)
			}
		})
	}
}

func TestExtractDates(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single date",
			input: "due @2026-02-15",
			want:  []string{"2026-02-15"},
		},
		{
			name:  "multiple dates sorted",
			input: "@2026-03-01 and @2026-02-15",
			want:  []string{"2026-02-15", "2026-03-01"},
		},
		{
			name:  "duplicates removed",
			input: "@2026-02-15 again @2026-02-15",
			want:  []string{"2026-02-15"},
		},
		{
			name:  "no dates",
			input: "no dates here @tomorrow @kevin",
			want:  nil,
		},
		{
			name:  "empty content",
			input: "",
			want:  nil,
		},
		{
			name:  "mixed content",
			input: "# Title\n\n#followup @2026-02-20\nsome text @2026-01-10\n",
			want:  []string{"2026-01-10", "2026-02-20"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractDates(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractDates(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractDates(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
