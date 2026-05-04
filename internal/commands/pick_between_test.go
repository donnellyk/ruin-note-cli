package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donnellyk/ruin-note-cli/internal/dateparse"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

func TestParsePickBetween(t *testing.T) {
	ref := time.Date(2025, 1, 29, 14, 30, 0, 0, time.Local)

	t.Run("absolute dates", func(t *testing.T) {
		dr, err := parsePickBetween("@between:2025-01-01,2025-01-31", ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := dateparse.DateRange{
			Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
			End:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local),
		}
		if !dr.Start.Equal(want.Start) || !dr.End.Equal(want.End) {
			t.Errorf("got %v..%v, want %v..%v", dr.Start, dr.End, want.Start, want.End)
		}
	})

	t.Run("relative arithmetic endpoints", func(t *testing.T) {
		dr, err := parsePickBetween("@between:today,today+6", ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := dateparse.DateRange{
			Start: time.Date(2025, 1, 29, 0, 0, 0, 0, time.Local),
			End:   time.Date(2025, 2, 5, 0, 0, 0, 0, time.Local),
		}
		if !dr.Start.Equal(want.Start) || !dr.End.Equal(want.End) {
			t.Errorf("got %v..%v, want %v..%v", dr.Start, dr.End, want.Start, want.End)
		}
	})

	t.Run("mixed relative and absolute", func(t *testing.T) {
		dr, err := parsePickBetween("@between:today-7,2025-02-15", ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantStart := time.Date(2025, 1, 22, 0, 0, 0, 0, time.Local)
		wantEnd := time.Date(2025, 2, 16, 0, 0, 0, 0, time.Local)
		if !dr.Start.Equal(wantStart) || !dr.End.Equal(wantEnd) {
			t.Errorf("got %v..%v, want %v..%v", dr.Start, dr.End, wantStart, wantEnd)
		}
	})

	t.Run("D1 > D2 yields empty contains", func(t *testing.T) {
		dr, err := parsePickBetween("@between:today+6,today", ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Anything we test should fall outside the inverted range.
		probes := []time.Time{
			time.Date(2025, 1, 28, 12, 0, 0, 0, time.Local),
			time.Date(2025, 1, 29, 12, 0, 0, 0, time.Local),
			time.Date(2025, 2, 4, 12, 0, 0, 0, time.Local),
			time.Date(2025, 2, 5, 12, 0, 0, 0, time.Local),
		}
		for _, p := range probes {
			if dr.Contains(p) {
				t.Errorf("inverted range should not contain %v", p)
			}
		}
	})

	t.Run("missing comma", func(t *testing.T) {
		_, err := parsePickBetween("@between:today", ref)
		if err == nil || !strings.Contains(err.Error(), "two dates") {
			t.Errorf("expected error about two dates, got %v", err)
		}
	})

	t.Run("invalid start", func(t *testing.T) {
		_, err := parsePickBetween("@between:gibberish,today", ref)
		if err == nil || !strings.Contains(err.Error(), "start date") {
			t.Errorf("expected error about start date, got %v", err)
		}
	})

	t.Run("invalid end", func(t *testing.T) {
		_, err := parsePickBetween("@between:today,gibberish", ref)
		if err == nil || !strings.Contains(err.Error(), "end date") {
			t.Errorf("expected error about end date, got %v", err)
		}
	})
}

func TestPickCmd_BetweenArg(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	content := `---
uuid: uuid-week
created: "2026-04-20T10:00:00-05:00"
updated: "2026-04-20T10:00:00-05:00"
tags:
  - "#work"
inline-tags:
  - "#followup"
---
# Week View

In range A @2026-04-27. #followup

In range B @2026-04-29. #followup

Out of range early @2026-04-20. #followup

Out of range late @2026-05-10. #followup
`
	path := filepath.Join(tmpDir, "week.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test note: %v", err)
	}
	// Seed titles index with stored-form tag mirror so the pick command can
	// resolve `#followup` from titles.json (the v0.4.0 contract).
	if err := vlt.UpdateTitleEntryFull("uuid-week", "Week View", path, "", []string{"work"}, []string{"followup"}, nil, nil); err != nil {
		t.Fatalf("failed to seed titles: %v", err)
	}

	t.Run("absolute window matches inclusive both endpoints", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "@between:2026-04-27,2026-04-29"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "In range A") {
			t.Errorf("expected line A in output, got %q", output)
		}
		if !strings.Contains(output, "In range B") {
			t.Errorf("expected line B in output, got %q", output)
		}
		if strings.Contains(output, "Out of range early") {
			t.Errorf("expected early line excluded, got %q", output)
		}
		if strings.Contains(output, "Out of range late") {
			t.Errorf("expected late line excluded, got %q", output)
		}
	})

	t.Run("inverted range returns no matches", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)
		cmd.SetArgs([]string{"#followup", "@between:2026-04-29,2026-04-27"})
		err := cmd.Execute()
		if err != ErrNoMatches {
			t.Errorf("expected ErrNoMatches for inverted range, got %v", err)
		}
	})

	t.Run("mixed with single @date AND'd together", func(t *testing.T) {
		// Range covers A and B; @2026-04-27 narrows to just A.
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "@between:2026-04-27,2026-04-29", "@2026-04-27"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()
		if !strings.Contains(output, "In range A") {
			t.Errorf("expected line A in output, got %q", output)
		}
		if strings.Contains(output, "In range B") {
			t.Errorf("expected line B excluded by @2026-04-27 narrowing, got %q", output)
		}
	})

	t.Run("malformed @between errors", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)
		cmd.SetArgs([]string{"#followup", "@between:gibberish"})
		err := cmd.Execute()
		if err == nil {
			t.Errorf("expected error for malformed @between, got nil")
		}
	})
}

func TestNormalizePickQueryCommas(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"@between:today,today+6", "@between:today,today+6"},
		{"@between:today, today+6", "@between:today,today+6"},
		{"@between:today,  today+6", "@between:today,today+6"},
		{"@between:today,\ttoday+6", "@between:today,today+6"},
		{"#followup @between:a, b", "#followup @between:a,b"},
		{"plain text", "plain text"},
		{"", ""},
	}
	for _, tt := range cases {
		t.Run(tt.in, func(t *testing.T) {
			got := normalizePickQueryCommas(tt.in)
			if got != tt.want {
				t.Errorf("normalizePickQueryCommas(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
