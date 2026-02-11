package commands

import (
	"os"
	"path/filepath"
	"testing"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
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
	matches := pickLinesFromNote(n, queryTags, false, doneExclude)

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
	matches := pickLinesFromNote(n, queryTags, false, doneExclude)

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
	matches := pickLinesFromNote(n, queryTags, true, doneExclude)

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
	matches := pickLinesFromNote(n, queryTags, false, doneExclude)

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
	matches := pickLinesFromNote(n, queryTags, false, doneExclude)

	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0 (global tags excluded)", len(matches))
	}

	// #done is a trailing global tag, should not be picked
	queryTags = []string{"#done"}
	matches = pickLinesFromNote(n, queryTags, false, doneExclude)

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
	matches := pickLinesFromNote(n, queryTags, false, doneExclude)

	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0 (tag-only lines in content should be excluded)", len(matches))
	}

	// #followup on a content line should still match
	queryTags = []string{"#followup"}
	matches = pickLinesFromNote(n, queryTags, false, doneExclude)

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
	matches := pickLinesFromNote(n, queryTags, false, doneExclude)

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
		{"Content with #tag", false},
		{"", false},
		{"Just text", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := isTagOnlyLine(tt.line); got != tt.want {
				t.Errorf("isTagOnlyLine(%q) = %v, want %v", tt.line, got, tt.want)
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
	matches := pickLinesFromNote(n, queryTags, false, doneExclude)

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
	matches := pickLinesFromNote(n, queryTags, false, doneInclude)

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
	matches := pickLinesFromNote(n, queryTags, false, doneOnly)

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
	matches := pickLinesFromNote(n, queryTags, false, doneInclude)

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
