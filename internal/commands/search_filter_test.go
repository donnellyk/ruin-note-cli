package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
)

func TestSearchCmd_DateSearch(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	// Create a note with a resolved date
	content := "---\nuuid: uuid-date-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\ntags:\n  - \"#followup\"\ndates:\n  - \"2026-03-15\"\n---\n# Follow Up\n#followup\n\nNeed to follow up @2026-03-15\n"
	path := filepath.Join(tmpDir, "date-note.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test note: %v", err)
	}

	// Search by exact date
	matcher, info, err := parseQuery("@2026-03-15", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	results, err := searchNotes(vlt, matcher, info)
	if err != nil {
		t.Fatalf("searchNotes error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].UUID != "uuid-date-1" {
		t.Errorf("expected uuid-date-1, got %s", results[0].UUID)
	}

	// Search by date + tag
	matcher, info, err = parseQuery("#followup @2026-03-15", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	results, err = searchNotes(vlt, matcher, info)
	if err != nil {
		t.Fatalf("searchNotes error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result for tag+date, got %d", len(results))
	}

	// Search by non-matching date
	matcher, info, err = parseQuery("@2026-12-25", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	results, err = searchNotes(vlt, matcher, info)
	if err != nil {
		t.Fatalf("searchNotes error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for non-matching date, got %d", len(results))
	}
}

func TestIsDateTerm(t *testing.T) {
	tests := []struct {
		term string
		want bool
	}{
		{"@2026-02-13", true},
		{"@2025-12-31", true},
		{"@today", false},
		{"@tomorrow", false},
		{"#tag", false},
		{"text", false},
		{"@2026-1-1", false},    // wrong format
		{"@20260213", false},    // no hyphens
		{"@2026-02-133", false}, // too long
	}

	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			if got := isDateTerm(tt.term); got != tt.want {
				t.Errorf("isDateTerm(%q) = %v, want %v", tt.term, got, tt.want)
			}
		})
	}
}

func TestSearch_TodoFilter(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	// Note with open and done checkboxes
	withTodos := `---
uuid: uuid-todos
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#work"
---
# Todo Note
#work

- [ ] Open task
- [x] Done task
`
	// Note with no checkboxes
	withoutTodos := `---
uuid: uuid-notodos
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
tags:
  - "#work"
---
# Plain Note
#work

Regular content only.
`
	// Note with only done checkboxes
	allDone := `---
uuid: uuid-alldone
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
tags:
  - "#work"
---
# All Done
#work

- [x] First done
- [x] Second done
`

	for name, content := range map[string]string{
		"todos.md":   withTodos,
		"notodos.md": withoutTodos,
		"alldone.md": allDone,
	} {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test note: %v", err)
		}
	}

	t.Run("todo:open", func(t *testing.T) {
		jsonOut := true
		cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"todo:open"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []json.RawMessage
		json.Unmarshal(buf.Bytes(), &results)

		// Only uuid-todos has unchecked checkboxes
		if len(results) != 1 {
			t.Errorf("got %d results, want 1 (only note with open todos)\noutput: %s", len(results), buf.String())
		}
	})

	t.Run("todo:done", func(t *testing.T) {
		jsonOut := true
		cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"todo:done"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []json.RawMessage
		json.Unmarshal(buf.Bytes(), &results)

		// Both uuid-todos and uuid-alldone have checked checkboxes
		if len(results) != 2 {
			t.Errorf("got %d results, want 2\noutput: %s", len(results), buf.String())
		}
	})

	t.Run("todo:any", func(t *testing.T) {
		jsonOut := true
		cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"todo:any"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []json.RawMessage
		json.Unmarshal(buf.Bytes(), &results)

		// uuid-todos and uuid-alldone have checkboxes; uuid-notodos does not
		if len(results) != 2 {
			t.Errorf("got %d results, want 2\noutput: %s", len(results), buf.String())
		}
	})

	t.Run("todo:invalid", func(t *testing.T) {
		jsonOut := false
		cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

		cmd.SetArgs([]string{"todo:garbage"})
		err := cmd.Execute()

		if err == nil || !strings.Contains(err.Error(), "unknown todo filter") {
			t.Errorf("expected error for invalid todo filter, got %v", err)
		}
	})
}

func TestLinkFilter(t *testing.T) {
	// Test that link: filter is parsed correctly
	matcher, info, err := parseQuery("link:example.com", TagScopeAll)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.NeedsBody {
		t.Error("link filter should not need body")
	}

	// Test matching
	n := &note.Note{URL: "https://example.com/page"}
	if !matcher(n) {
		t.Error("expected match for note with matching URL")
	}

	n2 := &note.Note{URL: "https://other.com/page"}
	if matcher(n2) {
		t.Error("expected no match for note with different URL")
	}

	n3 := &note.Note{}
	if matcher(n3) {
		t.Error("expected no match for note with no URL")
	}
}

func TestLinkFilter_CaseInsensitive(t *testing.T) {
	matcher, _, err := parseQuery("link:EXAMPLE.COM", TagScopeAll)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n := &note.Note{URL: "https://example.com/page"}
	if !matcher(n) {
		t.Error("expected case-insensitive match")
	}
}
