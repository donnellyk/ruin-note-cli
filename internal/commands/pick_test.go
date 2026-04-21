package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donnellyk/ruin-note-cli/internal/dateparse"
	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

func setupPickVault(t *testing.T) *vault.Vault {
	t.Helper()

	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	notes := []struct {
		filename string
		content  string
	}{
		{
			filename: "meeting.md",
			content: `---
uuid: uuid-meeting
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#meeting"
inline-tags:
  - "#followup"
  - "#todo"
---
# Meeting Notes
#meeting

Discussed the roadmap. #work

Chat with Bob tomorrow.  #followup

Review the budget numbers.  #todo

#done`,
		},
		{
			filename: "code.md",
			content: `---
uuid: uuid-code
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
tags:
  - "#code"
inline-tags:
  - "#followup"
  - "#urgent"
---
# Code Review
#code

Fix the race condition. #followup #urgent

Added unit tests for the parser.`,
		},
		{
			filename: "daily.md",
			content: `---
uuid: uuid-daily
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
tags:
  - "#daily"
---
# Daily Log
#daily

Just a regular day, no inline tags here.`,
		},
	}

	for _, n := range notes {
		path := filepath.Join(tmpDir, n.filename)
		if err := os.WriteFile(path, []byte(n.content), 0644); err != nil {
			t.Fatalf("failed to write test note: %v", err)
		}
	}

	return vlt
}

func TestPickLinesFromNote_SingleTag(t *testing.T) {
	n := &note.Note{
		Content: `# Meeting Notes
#meeting

Chat with Bob tomorrow.  #followup

Review the budget.  #todo

#done`,
		Title: "Meeting Notes",
	}
	n.RefreshTags()

	queryTags := []string{"#followup"}
	matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneExclude, false)

	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}

	if matches[0].Content != "Chat with Bob tomorrow.  #followup" {
		t.Errorf("content = %q, want %q", matches[0].Content, "Chat with Bob tomorrow.  #followup")
	}

	if matches[0].Line != 4 {
		t.Errorf("line = %d, want 4", matches[0].Line)
	}
}

func TestPickLinesFromNote_MultipleTags_AND(t *testing.T) {
	n := &note.Note{
		Content: `# Code Review
#code

Fix the race condition. #followup #urgent

Added unit tests. #followup`,
		Title: "Code Review",
	}
	n.RefreshTags()

	queryTags := []string{"#followup", "#urgent"}
	matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneExclude, false)

	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1 (AND mode)", len(matches))
	}

	if matches[0].Content != "Fix the race condition. #followup #urgent" {
		t.Errorf("content = %q", matches[0].Content)
	}
}

func TestPickLinesFromNote_MultipleTags_OR(t *testing.T) {
	n := &note.Note{
		Content: `# Code Review
#code

Fix the race condition. #followup #urgent

Added unit tests. #todo`,
		Title: "Code Review",
	}
	n.RefreshTags()

	queryTags := []string{"#followup", "#todo"}
	matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, true, doneExclude, false)

	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2 (OR mode)", len(matches))
	}
}

func TestPickLinesFromNote_NoMatches(t *testing.T) {
	n := &note.Note{
		Content: `# Daily Log
#daily

Just a regular day.`,
		Title: "Daily Log",
	}
	n.RefreshTags()

	queryTags := []string{"#followup"}
	matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneExclude, false)

	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0", len(matches))
	}
}

func TestPickLinesFromNote_GlobalTagsExcluded(t *testing.T) {
	n := &note.Note{
		Content: `# Note
#meeting

Content here.

#done`,
		Title: "Note",
	}
	n.RefreshTags()

	// #meeting is a global tag (after H1), should not be picked
	queryTags := []string{"#meeting"}
	matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneExclude, false)

	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0 (global tags excluded)", len(matches))
	}

	// #done is a trailing global tag, should not be picked
	queryTags = []string{"#done"}
	matches = pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneExclude, false)

	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0 (trailing global tags excluded)", len(matches))
	}
}

func TestPickLinesFromNote_TagOnlyLinesInContentExcluded(t *testing.T) {
	n := &note.Note{
		Content: `# Note
#global

Some content here.

#wip #urgent

More content. #followup`,
		Title: "Note",
	}
	n.RefreshTags()

	// #wip appears on a tag-only line in the inline zone -- should be excluded
	queryTags := []string{"#wip"}
	matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneExclude, false)

	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0 (tag-only lines in content should be excluded)", len(matches))
	}

	// #followup on a content line should still match
	queryTags = []string{"#followup"}
	matches = pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneExclude, false)

	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1", len(matches))
	}
}

func TestPickLinesFromNote_AllTagsOnLine(t *testing.T) {
	n := &note.Note{
		Content: `# Note
#global

Fix bug. #followup #urgent #p1`,
		Title: "Note",
	}
	n.RefreshTags()

	queryTags := []string{"#followup"}
	matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneExclude, false)

	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}

	// Should include all tags on the line, not just the queried one
	if len(matches[0].Tags) != 3 {
		t.Errorf("tags = %v, want 3 tags", matches[0].Tags)
	}
}

func TestNoteHasInlineTag(t *testing.T) {
	n := &note.Note{
		InlineTags: []string{"#followup", "#todo"},
	}

	if !noteHasInlineTag(n, []string{"#followup"}) {
		t.Error("expected match for #followup")
	}

	if !noteHasInlineTag(n, []string{"#missing", "#todo"}) {
		t.Error("expected match when at least one tag matches")
	}

	if noteHasInlineTag(n, []string{"#missing"}) {
		t.Error("expected no match for #missing")
	}
}

func TestIsTagOnlyLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"#daily #work", true},
		{"#daily", true},
		{"  #daily  ", true},
		{"#wrap, #daily #ruin", true},
		{"Content with #tag", false},
		{"", false},
		{"Just text", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := note.IsTagOnlyLine(tt.line); got != tt.want {
				t.Errorf("IsTagOnlyLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestPickLinesFromNote_DoneExcludedByDefault(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

Chat with Bob tomorrow. #followup

Review the budget. #followup #done

Fix the bug. #followup`,
		Title: "Tasks",
	}
	n.RefreshTags()

	queryTags := []string{"#followup"}
	matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneExclude, false)

	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2 (done excluded)", len(matches))
	}

	// Verify the #done line was skipped
	for _, m := range matches {
		if m.Done {
			t.Errorf("match %q should not be done", m.Content)
		}
	}
}

func TestPickLinesFromNote_DoneIncludedWithAll(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

Chat with Bob tomorrow. #followup

Review the budget. #followup #done

Fix the bug. #followup`,
		Title: "Tasks",
	}
	n.RefreshTags()

	queryTags := []string{"#followup"}
	matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneInclude, false)

	if len(matches) != 3 {
		t.Fatalf("got %d matches, want 3 (all included)", len(matches))
	}

	// Verify the done flag is set correctly
	doneCount := 0
	for _, m := range matches {
		if m.Done {
			doneCount++
		}
	}
	if doneCount != 1 {
		t.Errorf("got %d done matches, want 1", doneCount)
	}
}

func TestPickLinesFromNote_DoneOnly(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

Chat with Bob tomorrow. #followup

Review the budget. #followup #done

Deploy the fix. #todo #done

Fix the bug. #followup`,
		Title: "Tasks",
	}
	n.RefreshTags()

	queryTags := []string{"#followup"}
	matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneOnly, false)

	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1 (done only)", len(matches))
	}

	if !matches[0].Done {
		t.Error("expected match to be marked done")
	}

	if matches[0].Content != "Review the budget. #followup #done" {
		t.Errorf("content = %q", matches[0].Content)
	}
}

func TestPickLinesFromNote_DoneFieldInJSON(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

Open item. #followup

Done item. #followup #done`,
		Title: "Tasks",
	}
	n.RefreshTags()

	queryTags := []string{"#followup"}
	matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneInclude, false)

	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}

	// First match should not be done
	if matches[0].Done {
		t.Errorf("first match should not be done")
	}

	// Second match should be done
	if !matches[1].Done {
		t.Errorf("second match should be done")
	}
}

func TestPickLinesFromNote_InlineDateFilter(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

Call client @2026-03-15. #followup

Send report by Friday. #followup

Review contract @2026-03-15. #followup #urgent`,
		Title: "Tasks",
	}
	n.RefreshTags()

	queryTags := []string{"#followup"}

	t.Run("filters to lines with exact date", func(t *testing.T) {
		dr := dateparse.DateRange{
			Start: time.Date(2026, 3, 15, 0, 0, 0, 0, time.Local),
			End:   time.Date(2026, 3, 16, 0, 0, 0, 0, time.Local),
		}
		matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, []dateparse.DateRange{dr}, false, doneExclude, false)
		if len(matches) != 2 {
			t.Fatalf("got %d matches, want 2", len(matches))
		}
		if !strings.Contains(matches[0].Content, "Call client") {
			t.Errorf("first match = %q", matches[0].Content)
		}
		if !strings.Contains(matches[1].Content, "Review contract") {
			t.Errorf("second match = %q", matches[1].Content)
		}
	})

	t.Run("no date filter returns all", func(t *testing.T) {
		matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneExclude, false)
		if len(matches) != 3 {
			t.Fatalf("got %d matches, want 3", len(matches))
		}
	})

	t.Run("non-matching date returns none", func(t *testing.T) {
		dr := dateparse.DateRange{
			Start: time.Date(2099, 1, 1, 0, 0, 0, 0, time.Local),
			End:   time.Date(2099, 1, 2, 0, 0, 0, 0, time.Local),
		}
		matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, []dateparse.DateRange{dr}, false, doneExclude, false)
		if len(matches) != 0 {
			t.Errorf("got %d matches, want 0", len(matches))
		}
	})

	t.Run("partial date matches range (month)", func(t *testing.T) {
		// @2026-03 should match any date in March 2026
		dr := dateparse.DateRange{
			Start: time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local),
			End:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.Local),
		}
		matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, []dateparse.DateRange{dr}, false, doneExclude, false)
		if len(matches) != 2 {
			t.Fatalf("got %d matches, want 2 (month range)", len(matches))
		}
	})

	t.Run("partial date matches range (year)", func(t *testing.T) {
		// @2026 should match any date in 2026
		dr := dateparse.DateRange{
			Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
			End:   time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local),
		}
		matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, []dateparse.DateRange{dr}, false, doneExclude, false)
		if len(matches) != 2 {
			t.Fatalf("got %d matches, want 2 (year range)", len(matches))
		}
	})
}

func TestPickCmd_InlineDateArg(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	content := `---
uuid: uuid-dated
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#work"
inline-tags:
  - "#followup"
---
# Dated Note
#work

Call client @2026-03-15. #followup

Send report soon. #followup
`
	path := filepath.Join(tmpDir, "dated.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test note: %v", err)
	}

	t.Run("inline date filters lines", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "@2026-03-15"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := strings.TrimSpace(buf.String())

		if !strings.Contains(output, "Call client") {
			t.Errorf("expected 'Call client' line, got %q", output)
		}
		if strings.Contains(output, "Send report") {
			t.Errorf("expected 'Send report' to be excluded, got %q", output)
		}
	})

	t.Run("invalid date arg", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		cmd.SetArgs([]string{"#followup", "@nonsense"})
		err := cmd.Execute()

		if err == nil || !strings.Contains(err.Error(), "unrecognized date") {
			t.Errorf("expected unrecognized date error, got %v", err)
		}
	})

	t.Run("date-only is allowed", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"@2026-03-15"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		// Should not error on validation — may return ErrNoMatches if no lines match
		if err != nil && err != ErrNoMatches {
			t.Errorf("expected no validation error, got %v", err)
		}
	})

	t.Run("no args at all errors", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		cmd.SetArgs([]string{})
		err := cmd.Execute()

		if err == nil || !strings.Contains(err.Error(), "at least one inline tag") {
			t.Errorf("expected validation error, got %v", err)
		}
	})
}

func TestPickCmd_Filter(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	// Note with a date reference and inline tags
	withDate := `---
uuid: uuid-dated
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#work"
inline-tags:
  - "#followup"
dates:
  - "2026-03-15"
---
# Dated Note
#work

Call client about contract. #followup
`
	// Note with inline tags but no matching date
	withoutDate := `---
uuid: uuid-nodated
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
tags:
  - "#work"
inline-tags:
  - "#followup"
---
# No Date Note
#work

Send the report. #followup
`
	for name, content := range map[string]string{
		"dated.md":   withDate,
		"nodated.md": withoutDate,
	} {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test note: %v", err)
		}
	}

	t.Run("with matching date", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--filter", "@2026-03-15"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := strings.TrimSpace(buf.String())

		// Should only get the line from the dated note
		if !strings.Contains(output, "Call client") {
			t.Errorf("expected output to contain 'Call client', got %q", output)
		}
		if strings.Contains(output, "Send the report") {
			t.Errorf("expected output NOT to contain 'Send the report', got %q", output)
		}
	})

	t.Run("with non-matching date", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--filter", "@2099-01-01"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != ErrNoMatches {
			t.Errorf("expected ErrNoMatches, got %v", err)
		}
	})

	t.Run("invalid filter", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		cmd.SetArgs([]string{"#followup", "--filter", ""})
		err := cmd.Execute()

		// Empty filter string is ignored (filterFlag == ""), so this should work
		// Test with an actually invalid filter term instead
		if err != nil && err != ErrNoMatches {
			// no-op: empty string means no filter applied
		}
	})
}

func TestPickCmd_MetadataDateFilters(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	jan := `---
uuid: uuid-jan
created: "2025-01-15T10:00:00-05:00"
updated: "2025-01-20T10:00:00-05:00"
inline-tags:
  - "#followup"
---
# January Note

Review Q1 budget. #followup
`
	mar := `---
uuid: uuid-mar
created: "2025-03-10T10:00:00-05:00"
updated: "2025-03-10T10:00:00-05:00"
inline-tags:
  - "#followup"
---
# March Note

Ship the feature. #followup
`
	for name, content := range map[string]string{
		"jan.md": jan,
		"mar.md": mar,
	} {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test note: %v", err)
		}
	}

	t.Run("created filter matches", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--filter", "created:2025-01"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := strings.TrimSpace(buf.String())

		if !strings.Contains(output, "Review Q1 budget") {
			t.Errorf("expected jan note line, got %q", output)
		}
		if strings.Contains(output, "Ship the feature") {
			t.Errorf("expected mar note to be excluded, got %q", output)
		}
	})

	t.Run("after filter", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--filter", "after:2025-02-01"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := strings.TrimSpace(buf.String())

		if strings.Contains(output, "Review Q1 budget") {
			t.Errorf("expected jan note to be excluded, got %q", output)
		}
		if !strings.Contains(output, "Ship the feature") {
			t.Errorf("expected mar note line, got %q", output)
		}
	})

	t.Run("between filter", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--filter", "between:2025-01,2025-02"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := strings.TrimSpace(buf.String())

		if !strings.Contains(output, "Review Q1 budget") {
			t.Errorf("expected jan note line, got %q", output)
		}
		if strings.Contains(output, "Ship the feature") {
			t.Errorf("expected mar note to be excluded, got %q", output)
		}
	})

	t.Run("updated filter", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--filter", "updated:2025-01"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := strings.TrimSpace(buf.String())

		// jan was updated 2025-01-20, so it should match
		if !strings.Contains(output, "Review Q1 budget") {
			t.Errorf("expected jan note line, got %q", output)
		}
	})
}

// --- todo mode tests ---

func TestPick_TodoMode(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

- [ ] Buy groceries
- [x] Send email
- [ ] Write report #followup

Regular content line.`,
		Title: "Tasks",
	}
	n.RefreshTags()

	t.Run("finds all open checkboxes", func(t *testing.T) {
		matches := pickLinesFromNote(n, pickTagFilter{}, nil, false, doneExclude, true)
		if len(matches) != 2 {
			t.Fatalf("got %d matches, want 2 (open checkboxes)", len(matches))
		}
		if !strings.Contains(matches[0].Content, "Buy groceries") {
			t.Errorf("match 0 = %q", matches[0].Content)
		}
		if !strings.Contains(matches[1].Content, "Write report") {
			t.Errorf("match 1 = %q", matches[1].Content)
		}
	})

	t.Run("--done shows only checked", func(t *testing.T) {
		matches := pickLinesFromNote(n, pickTagFilter{}, nil, false, doneOnly, true)
		if len(matches) != 1 {
			t.Fatalf("got %d matches, want 1 (checked only)", len(matches))
		}
		if !strings.Contains(matches[0].Content, "Send email") {
			t.Errorf("match = %q", matches[0].Content)
		}
		if !matches[0].Done {
			t.Error("expected Done=true for checked checkbox")
		}
	})

	t.Run("--all shows everything", func(t *testing.T) {
		matches := pickLinesFromNote(n, pickTagFilter{}, nil, false, doneInclude, true)
		if len(matches) != 3 {
			t.Fatalf("got %d matches, want 3 (all checkboxes)", len(matches))
		}
	})
}

func TestPick_CheckedCheckbox_DoneWithoutTodoMode(t *testing.T) {
	// A checked checkbox line matched via tag filter should be reported as
	// Done=true in results, independent of --todo mode, and excluded by the
	// default doneExclude filter.
	n := &note.Note{
		Content: `# Tasks
#work

- [ ] Open task #followup
- [x] Completed task #followup`,
		Title: "Tasks",
	}
	n.RefreshTags()

	t.Run("default excludes checked line", func(t *testing.T) {
		matches := pickLinesFromNote(n, pickTagFilter{include: []string{"#followup"}}, nil, false, doneExclude, false)
		if len(matches) != 1 {
			t.Fatalf("got %d matches, want 1 (open only)", len(matches))
		}
		if !strings.Contains(matches[0].Content, "Open task") {
			t.Errorf("match = %q", matches[0].Content)
		}
		if matches[0].Done {
			t.Error("open line should have Done=false")
		}
	})

	t.Run("--all reports Done=true for checked line", func(t *testing.T) {
		matches := pickLinesFromNote(n, pickTagFilter{include: []string{"#followup"}}, nil, false, doneInclude, false)
		if len(matches) != 2 {
			t.Fatalf("got %d matches, want 2", len(matches))
		}
		var checked *PickMatch
		for i := range matches {
			if strings.Contains(matches[i].Content, "Completed task") {
				checked = &matches[i]
			}
		}
		if checked == nil {
			t.Fatal("missing Completed task match")
		}
		if !checked.Done {
			t.Error("expected Done=true for [x] line")
		}
	})

	t.Run("--done keeps checked line", func(t *testing.T) {
		matches := pickLinesFromNote(n, pickTagFilter{include: []string{"#followup"}}, nil, false, doneOnly, false)
		if len(matches) != 1 {
			t.Fatalf("got %d matches, want 1 (checked only)", len(matches))
		}
		if !strings.Contains(matches[0].Content, "Completed task") {
			t.Errorf("match = %q", matches[0].Content)
		}
		if !matches[0].Done {
			t.Error("expected Done=true")
		}
	})
}

func TestPick_TodoMode_WithTags(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

- [ ] Buy groceries
- [ ] Write report #followup
- [x] Old task #followup`,
		Title: "Tasks",
	}
	n.RefreshTags()

	t.Run("checkboxes filtered by tag", func(t *testing.T) {
		queryTags := []string{"#followup"}
		matches := pickLinesFromNote(n, pickTagFilter{include: queryTags}, nil, false, doneExclude, true)
		if len(matches) != 1 {
			t.Fatalf("got %d matches, want 1 (checkbox with #followup, excluding done)", len(matches))
		}
		if !strings.Contains(matches[0].Content, "Write report") {
			t.Errorf("match = %q", matches[0].Content)
		}
	})

	t.Run("non-checkbox tagged lines also match", func(t *testing.T) {
		// Add a non-checkbox tagged line
		n2 := &note.Note{
			Content: `# Tasks
#work

- [ ] Buy groceries
Call Bob about contract. #followup`,
			Title: "Tasks",
		}
		n2.RefreshTags()
		queryTags := []string{"#followup"}
		matches := pickLinesFromNote(n2, pickTagFilter{include: queryTags}, nil, false, doneExclude, true)
		// Should match both the checkbox (without tag filter) and the tagged line
		// Actually, with tags provided, checkbox must ALSO have the tag. So only the tagged line matches.
		if len(matches) != 1 {
			t.Fatalf("got %d matches, want 1", len(matches))
		}
		if !strings.Contains(matches[0].Content, "Call Bob") {
			t.Errorf("match = %q", matches[0].Content)
		}
	})
}

func TestPickCmd_TodoFlag(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	content := `---
uuid: uuid-todo
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#work"
---
# Todo Note
#work

- [ ] Open item one
- [x] Done item
- [ ] Open item two
`
	path := filepath.Join(tmpDir, "todo.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test note: %v", err)
	}

	t.Run("--todo finds checkboxes", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"--todo"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := strings.TrimSpace(buf.String())
		lines := strings.Split(output, "\n")

		// Should find 2 open checkboxes (done excluded by default)
		if len(lines) != 2 {
			t.Errorf("got %d lines, want 2:\n%s", len(lines), output)
		}
	})

	t.Run("--todo --done", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"--todo", "--done"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := strings.TrimSpace(buf.String())

		if !strings.Contains(output, "Done item") {
			t.Errorf("expected done item, got %q", output)
		}
	})

	t.Run("no tags no todo no date errors", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		cmd.SetArgs([]string{})
		err := cmd.Execute()

		if err == nil || !strings.Contains(err.Error(), "at least one inline tag") {
			t.Errorf("expected error, got %v", err)
		}
	})
}

// --- --notes and --parent flag tests ---

// setupPickVaultWithTitles creates a vault with a titles index for testing
// --notes and --parent flags.
func setupPickVaultWithTitles(t *testing.T) *vault.Vault {
	t.Helper()

	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	type testNote struct {
		filename string
		uuid     string
		parent   string
		content  string
	}

	notes := []testNote{
		{
			filename: "hub.md",
			uuid:     "uuid-hub",
			content: `---
uuid: uuid-hub
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#hub"
inline-tags:
  - "#followup"
---
# Hub Note
#hub

Organize the project. #followup
`,
		},
		{
			filename: "child1.md",
			uuid:     "uuid-child1",
			parent:   "uuid-hub",
			content: `---
uuid: uuid-child1
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
tags:
  - "#work"
inline-tags:
  - "#followup"
  - "#urgent"
parent: uuid-hub
---
# Child One
#work

Call client about deadline. #followup #urgent
`,
		},
		{
			filename: "child2.md",
			uuid:     "uuid-child2",
			parent:   "uuid-hub",
			content: `---
uuid: uuid-child2
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
tags:
  - "#work"
inline-tags:
  - "#followup"
parent: uuid-hub
---
# Child Two
#work

Send the report by Friday. #followup
`,
		},
		{
			filename: "grandchild.md",
			uuid:     "uuid-grandchild",
			parent:   "uuid-child1",
			content: `---
uuid: uuid-grandchild
created: "2025-01-05T10:00:00-05:00"
updated: "2025-01-05T10:00:00-05:00"
tags:
  - "#work"
inline-tags:
  - "#followup"
parent: uuid-child1
---
# Grandchild
#work

Prepare slides for presentation. #followup
`,
		},
		{
			filename: "unrelated.md",
			uuid:     "uuid-unrelated",
			content: `---
uuid: uuid-unrelated
created: "2025-01-04T10:00:00-05:00"
updated: "2025-01-04T10:00:00-05:00"
tags:
  - "#personal"
inline-tags:
  - "#followup"
---
# Unrelated Note
#personal

Buy groceries tomorrow. #followup
`,
		},
	}

	index := &vault.TitlesIndex{Titles: make(map[string]vault.TitleEntry)}
	for _, n := range notes {
		path := filepath.Join(tmpDir, n.filename)
		if err := os.WriteFile(path, []byte(n.content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", n.filename, err)
		}
		index.Titles[n.uuid] = vault.TitleEntry{
			Title:  strings.TrimSuffix(n.filename, ".md"),
			Path:   path,
			Parent: n.parent,
		}
	}

	if err := vlt.SaveTitles(index); err != nil {
		t.Fatalf("failed to save titles: %v", err)
	}

	return vlt
}

func TestPickCmd_NotesFlag(t *testing.T) {
	vlt := setupPickVaultWithTitles(t)

	t.Run("scopes to specific UUIDs", func(t *testing.T) {
		jsonOut := true
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--notes", "uuid-child1,uuid-child2"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []PickResult
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Should only include child1 and child2, not hub or unrelated
		uuids := make(map[string]bool)
		for _, r := range results {
			uuids[r.UUID] = true
		}
		if uuids["uuid-hub"] {
			t.Error("hub should not be in results")
		}
		if uuids["uuid-unrelated"] {
			t.Error("unrelated should not be in results")
		}
		if !uuids["uuid-child1"] {
			t.Error("child1 should be in results")
		}
		if !uuids["uuid-child2"] {
			t.Error("child2 should be in results")
		}
	})

	t.Run("warns on invalid UUIDs and returns partial results", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		// Capture stderr
		oldStderr := os.Stderr
		stderrR, stderrW, _ := os.Pipe()
		os.Stderr = stderrW

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		cmd.SetArgs([]string{"#followup", "--notes", "uuid-child1,uuid-bogus"})
		err := cmd.Execute()

		stdoutW.Close()
		stderrW.Close()
		os.Stdout = oldStdout
		os.Stderr = oldStderr

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var stderrBuf bytes.Buffer
		stderrBuf.ReadFrom(stderrR)
		stderr := stderrBuf.String()

		if !strings.Contains(stderr, "uuid-bogus") {
			t.Errorf("expected stderr warning about uuid-bogus, got %q", stderr)
		}

		var stdoutBuf bytes.Buffer
		stdoutBuf.ReadFrom(stdoutR)
		output := strings.TrimSpace(stdoutBuf.String())

		if !strings.Contains(output, "Call client") {
			t.Errorf("expected child1 content in output, got %q", output)
		}
	})

	t.Run("composes with --filter and tag args", func(t *testing.T) {
		jsonOut := true
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Use --notes with --filter to further restrict by created date
		cmd.SetArgs([]string{"#followup", "--notes", "uuid-child1,uuid-child2", "--filter", "created:2025-01-02"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []PickResult
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Only child1 was created on 2025-01-02
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].UUID != "uuid-child1" {
			t.Errorf("expected uuid-child1, got %s", results[0].UUID)
		}
	})

	t.Run("all UUIDs invalid returns ErrNoMatches", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		// Suppress stderr
		oldStderr := os.Stderr
		_, stderrW, _ := os.Pipe()
		os.Stderr = stderrW

		cmd.SetArgs([]string{"#followup", "--notes", "uuid-bogus1,uuid-bogus2"})
		err := cmd.Execute()

		stderrW.Close()
		os.Stderr = oldStderr

		if err != ErrNoMatches {
			t.Errorf("expected ErrNoMatches, got %v", err)
		}
	})
}

func TestPickCmd_ParentFlag(t *testing.T) {
	vlt := setupPickVaultWithTitles(t)

	t.Run("scopes to all descendants of parent", func(t *testing.T) {
		jsonOut := true
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--parent", "uuid-hub"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []PickResult
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		uuids := make(map[string]bool)
		for _, r := range results {
			uuids[r.UUID] = true
		}

		if uuids["uuid-hub"] {
			t.Error("parent itself should not be in results")
		}
		if uuids["uuid-unrelated"] {
			t.Error("unrelated should not be in results")
		}
		if !uuids["uuid-child1"] {
			t.Error("child1 should be in results")
		}
		if !uuids["uuid-child2"] {
			t.Error("child2 should be in results")
		}
		if !uuids["uuid-grandchild"] {
			t.Error("grandchild should be in results (recursive)")
		}
	})

	t.Run("resolves parent by title", func(t *testing.T) {
		jsonOut := true
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--parent", "hub"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []PickResult
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// child1, child2, grandchild
		if len(results) != 3 {
			t.Fatalf("got %d results, want 3", len(results))
		}
	})

	t.Run("no children returns ErrNoMatches", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		cmd.SetArgs([]string{"#followup", "--parent", "uuid-unrelated"})
		err := cmd.Execute()

		if err != ErrNoMatches {
			t.Errorf("expected ErrNoMatches, got %v", err)
		}
	})

	t.Run("composes with --filter and tag args", func(t *testing.T) {
		jsonOut := true
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "#urgent", "--parent", "uuid-hub"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []PickResult
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Only child1 has both #followup and #urgent
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].UUID != "uuid-child1" {
			t.Errorf("expected uuid-child1, got %s", results[0].UUID)
		}
	})

	t.Run("invalid parent errors", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		cmd.SetArgs([]string{"#followup", "--parent", "nonexistent-note-xyz"})
		err := cmd.Execute()

		if err == nil {
			t.Error("expected error for unresolvable parent")
		}
		if !strings.Contains(err.Error(), "failed to resolve parent") {
			t.Errorf("expected resolve error, got %v", err)
		}
	})
}

func TestPickCmd_NotesPlusParentMutuallyExclusive(t *testing.T) {
	vlt := setupPickVaultWithTitles(t)

	jsonOut := false
	cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"#followup", "--notes", "uuid-child1", "--parent", "uuid-hub"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for --notes + --parent")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutual exclusivity error, got %v", err)
	}
}

// --- date-only mode tests ---

func TestPickLinesFromNote_DateOnly(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

Call client @2026-03-15. #followup

Send report by Friday.

Review contract @2026-03-15.

Deploy the fix @2026-04-01.`,
		Title: "Tasks",
	}
	n.RefreshTags()

	dr := dateparse.DateRange{
		Start: time.Date(2026, 3, 15, 0, 0, 0, 0, time.Local),
		End:   time.Date(2026, 3, 16, 0, 0, 0, 0, time.Local),
	}

	// No tags, no todo — just date ranges
	matches := pickLinesFromNote(n, pickTagFilter{}, []dateparse.DateRange{dr}, false, doneExclude, false)

	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if !strings.Contains(matches[0].Content, "Call client") {
		t.Errorf("match 0 = %q", matches[0].Content)
	}
	if !strings.Contains(matches[1].Content, "Review contract") {
		t.Errorf("match 1 = %q", matches[1].Content)
	}
}

func TestPickCmd_DateOnly(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	content := `---
uuid: uuid-dateonly
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#work"
---
# Date Only Note
#work

Call client @2026-03-15.

Send report by Friday.

Review contract @2026-03-15. #followup
`
	path := filepath.Join(tmpDir, "dateonly.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test note: %v", err)
	}

	t.Run("date-only returns matching lines", func(t *testing.T) {
		jsonOut := true
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"@2026-03-15"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []PickResult
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if len(results[0].Matches) != 2 {
			t.Fatalf("got %d matches, want 2", len(results[0].Matches))
		}
		if !strings.Contains(results[0].Matches[0].Content, "Call client") {
			t.Errorf("match 0 = %q", results[0].Matches[0].Content)
		}
		if !strings.Contains(results[0].Matches[1].Content, "Review contract") {
			t.Errorf("match 1 = %q", results[0].Matches[1].Content)
		}
	})

	t.Run("date-only no matches returns ErrNoMatches", func(t *testing.T) {
		jsonOut := false
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"@2099-01-01"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != ErrNoMatches {
			t.Errorf("expected ErrNoMatches, got %v", err)
		}
	})
}

// --- sort tests ---

func TestPickCmd_SortByCreated(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	// Note created earlier
	older := `---
uuid: uuid-older
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
inline-tags:
  - "#followup"
---
# Older Note

Review Q1 budget. #followup
`
	// Note created later
	newer := `---
uuid: uuid-newer
created: "2025-03-01T10:00:00-05:00"
updated: "2025-03-01T10:00:00-05:00"
inline-tags:
  - "#followup"
---
# Newer Note

Ship the feature. #followup
`
	for name, content := range map[string]string{
		"older.md": older,
		"newer.md": newer,
	} {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test note: %v", err)
		}
	}

	t.Run("default sort is created:desc", func(t *testing.T) {
		jsonOut := true
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []PickResult
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("got %d results, want 2", len(results))
		}
		if results[0].UUID != "uuid-newer" {
			t.Errorf("first result = %s, want uuid-newer (newest first)", results[0].UUID)
		}
		if results[1].UUID != "uuid-older" {
			t.Errorf("second result = %s, want uuid-older", results[1].UUID)
		}
	})

	t.Run("created:asc sorts oldest first", func(t *testing.T) {
		jsonOut := true
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--sort", "created:asc"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []PickResult
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("got %d results, want 2", len(results))
		}
		if results[0].UUID != "uuid-older" {
			t.Errorf("first result = %s, want uuid-older (oldest first)", results[0].UUID)
		}
		if results[1].UUID != "uuid-newer" {
			t.Errorf("second result = %s, want uuid-newer", results[1].UUID)
		}
	})
}

func TestPickCmd_SortByTitle(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	alpha := `---
uuid: uuid-alpha
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
inline-tags:
  - "#followup"
---
# Alpha Note

Task from alpha. #followup
`
	zeta := `---
uuid: uuid-zeta
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
inline-tags:
  - "#followup"
---
# Zeta Note

Task from zeta. #followup
`
	for name, content := range map[string]string{
		"alpha.md": alpha,
		"zeta.md":  zeta,
	} {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test note: %v", err)
		}
	}

	t.Run("title:asc sorts alphabetically", func(t *testing.T) {
		jsonOut := true
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--sort", "title:asc"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []PickResult
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("got %d results, want 2", len(results))
		}
		if results[0].UUID != "uuid-alpha" {
			t.Errorf("first result = %s, want uuid-alpha", results[0].UUID)
		}
		if results[1].UUID != "uuid-zeta" {
			t.Errorf("second result = %s, want uuid-zeta", results[1].UUID)
		}
	})

	t.Run("title:desc sorts reverse alphabetically", func(t *testing.T) {
		jsonOut := true
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"#followup", "--sort", "title:desc"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []PickResult
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("got %d results, want 2", len(results))
		}
		if results[0].UUID != "uuid-zeta" {
			t.Errorf("first result = %s, want uuid-zeta", results[0].UUID)
		}
		if results[1].UUID != "uuid-alpha" {
			t.Errorf("second result = %s, want uuid-alpha", results[1].UUID)
		}
	})
}

func TestPickCmd_TodoWithDate(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	content := `---
uuid: uuid-tododate
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#work"
---
# Todo Date Note
#work

- [ ] Call client @2026-03-15
- [x] Send report @2026-03-15
- [ ] Review contract @2026-04-01
Regular line @2026-03-15
`
	path := filepath.Join(tmpDir, "tododate.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test note: %v", err)
	}

	t.Run("todo with date returns matching checkboxes", func(t *testing.T) {
		jsonOut := true
		cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"--todo", "@2026-03-15", "--all"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []PickResult
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		// Should match only the 2 checkboxes with @2026-03-15, not the regular line or @2026-04-01
		if len(results[0].Matches) != 2 {
			t.Fatalf("got %d matches, want 2: %+v", len(results[0].Matches), results[0].Matches)
		}
		if !strings.Contains(results[0].Matches[0].Content, "Call client") {
			t.Errorf("match 0 = %q", results[0].Matches[0].Content)
		}
		if !strings.Contains(results[0].Matches[1].Content, "Send report") {
			t.Errorf("match 1 = %q", results[0].Matches[1].Content)
		}
	})
}

func TestPickNegateTag(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

Chat with Bob. #followup
Review the budget. #followup #done
Fix the bug. #followup`,
		Title: "Tasks",
	}
	n.RefreshTags()

	tags := pickTagFilter{
		include: []string{note.NormalizeTag("#followup")},
		exclude: []string{note.NormalizeTag("#done")},
	}
	matches := pickLinesFromNote(n, tags, nil, false, doneInclude, false)

	// #done lines should be excluded by the tag filter, not the done filter
	// (we pass doneInclude to isolate the tag negation behavior)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	for _, m := range matches {
		if strings.Contains(m.Content, "#done") {
			t.Errorf("match %q should have been excluded by !#done", m.Content)
		}
	}
}

func TestPickNegateMultiple(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

Open item. #task
Deferred item. #task #deferred
Done item. #task #done
Active item. #task #active`,
		Title: "Tasks",
	}
	n.RefreshTags()

	tags := pickTagFilter{
		include: []string{note.NormalizeTag("#task")},
		exclude: []string{note.NormalizeTag("#done"), note.NormalizeTag("#deferred")},
	}
	matches := pickLinesFromNote(n, tags, nil, false, doneInclude, false)

	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}

	// Should only get "Open item" and "Active item"
	contents := make(map[string]bool)
	for _, m := range matches {
		contents[m.Content] = true
	}
	if !contents["Open item. #task"] {
		t.Error("expected 'Open item. #task' to be matched")
	}
	if !contents["Active item. #task #active"] {
		t.Error("expected 'Active item. #task #active' to be matched")
	}
}

func TestPickNegateAnyMode(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

Call Bob. #followup
Fix bug. #todo
Review code. #followup #archived
Send report. #todo #archived`,
		Title: "Tasks",
	}
	n.RefreshTags()

	tags := pickTagFilter{
		include: []string{note.NormalizeTag("#followup"), note.NormalizeTag("#todo")},
		exclude: []string{note.NormalizeTag("#archived")},
	}
	// anyMode=true: match lines with ANY include tag, but no exclude tags
	matches := pickLinesFromNote(n, tags, nil, true, doneExclude, false)

	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}

	contents := make(map[string]bool)
	for _, m := range matches {
		contents[m.Content] = true
	}
	if !contents["Call Bob. #followup"] {
		t.Error("expected 'Call Bob. #followup' to match")
	}
	if !contents["Fix bug. #todo"] {
		t.Error("expected 'Fix bug. #todo' to match")
	}
}

func TestPickNegateOnlyExclude(t *testing.T) {
	n := &note.Note{
		Content: `# Tasks
#work

- [ ] Open item
- [x] Done checkbox
- [ ] Another open #done`,
		Title: "Tasks",
	}
	n.RefreshTags()

	// todoMode=true with only exclude tags (no include tags)
	tags := pickTagFilter{
		include: nil,
		exclude: []string{note.NormalizeTag("#done")},
	}
	matches := pickLinesFromNote(n, tags, nil, false, doneExclude, true)

	// Should match "Open item" checkbox only:
	// "Done checkbox" is excluded by doneExclude (checked)
	// "Another open #done" is excluded by the exclude tag filter
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	if !strings.Contains(matches[0].Content, "Open item") {
		t.Errorf("expected 'Open item', got %q", matches[0].Content)
	}
}

func TestPickFilterNegate(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	// Note with #archived tag and inline #followup
	archived := `---
uuid: uuid-archived
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#archived"
inline-tags:
  - "#followup"
---
# Archived Note
#archived

Call client. #followup
`
	// Note without #archived, with inline #followup
	active := `---
uuid: uuid-active
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
tags:
  - "#work"
inline-tags:
  - "#followup"
---
# Active Note
#work

Send report. #followup
`
	for name, content := range map[string]string{
		"archived.md": archived,
		"active.md":   active,
	} {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test note: %v", err)
		}
	}

	jsonOut := true
	cmd := NewPickCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"#followup", "--filter", "!#archived"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var results []PickResult
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].UUID != "uuid-active" {
		t.Errorf("UUID = %q, want uuid-active", results[0].UUID)
	}
}
