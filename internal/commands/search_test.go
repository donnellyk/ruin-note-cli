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

// setupTestVault creates a test vault with sample notes.
func setupTestVault(t *testing.T) *vault.Vault {
	t.Helper()

	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	// Create test notes
	notes := []struct {
		filename string
		content  string
	}{
		{
			filename: "note1.md",
			content: `---
uuid: uuid-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#daily"
  - "#work"
---
# Daily Note 1
#daily #work

This is a daily work note.`,
		},
		{
			filename: "note2.md",
			content: `---
uuid: uuid-2
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T11:00:00-05:00"
tags:
  - "#daily"
  - "#personal"
---
# Daily Note 2
#daily #personal

This is a personal daily note.`,
		},
		{
			filename: "note3.md",
			content: `---
uuid: uuid-3
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
tags:
  - "#project"
  - "#work"
parent: uuid-1
---
# Project Alpha
#project #work

Working on project alpha with the team.`,
		},
		{
			filename: "note4.md",
			content: `---
uuid: uuid-4
created: "2025-01-04T10:00:00-05:00"
updated: "2025-01-04T10:00:00-05:00"
tags:
  - "#project"
parent: uuid-3
---
# Alpha Sub-task
#project

A sub-task of project alpha.`,
		},
		{
			filename: "note5.md",
			content: `---
uuid: uuid-5
created: "2025-01-05T10:00:00-05:00"
updated: "2025-01-05T10:00:00-05:00"
tags:
  - "#idea"
parent: uuid-nonexistent
---
# Orphan Idea
#idea

This note references a non-existent parent.`,
		},
	}

	for _, n := range notes {
		path := filepath.Join(tmpDir, n.filename)
		if err := os.WriteFile(path, []byte(n.content), 0644); err != nil {
			t.Fatalf("failed to create test note: %v", err)
		}
	}

	return vlt
}

func TestSearchCmd_TagSearch(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Prevent os.Exit from actually exiting
	cmd.SetArgs([]string{"#daily"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should find 2 notes with #daily
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("found %d notes, want 2", len(lines))
	}
}

func TestSearchCmd_TextSearch(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"alpha"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should find 2 notes with "alpha" (note3 "Project Alpha" + note4 "Alpha Sub-task")
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("found %d notes, want 2", len(lines))
	}
}

func TestSearchCmd_CombinedSearch(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"#daily", "#work"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should find 1 note with both #daily AND #work
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("found %d notes, want 1", len(lines))
	}
}

func TestSearchCmd_JSONOutput(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := true
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"#project"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var results []struct {
		Path  string   `json:"path"`
		UUID  string   `json:"uuid"`
		Title string   `json:"title"`
		Tags  []string `json:"tags"`
	}

	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Should find 2 notes with #project (uuid-3 and uuid-4)
	if len(results) != 2 {
		t.Errorf("found %d results, want 2", len(results))
	}

	// Verify both project notes are present
	uuids := make(map[string]bool)
	for _, r := range results {
		uuids[r.UUID] = true
	}
	if !uuids["uuid-3"] || !uuids["uuid-4"] {
		t.Errorf("expected uuid-3 and uuid-4, got %v", uuids)
	}
}

func TestSearchCmd_BulkOutput(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--bulk", "#project"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "%%%% uuid-3 %%%%") {
		t.Error("bulk output should contain %%%% uuid-3 %%%% separator")
	}

	if !strings.Contains(output, "Project Alpha") {
		t.Error("bulk output should contain note content")
	}

	// Should NOT contain frontmatter
	if strings.Contains(output, "created:") {
		t.Error("bulk output should not contain frontmatter")
	}
}

func TestSearchCmd_FirstOutput(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--first", "#project"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Default sort is created:desc, so first result is note4 (newest with #project)
	if !strings.Contains(output, "#project") {
		t.Error("first output should contain note content with #project tag")
	}

	// Should NOT contain uuid separator
	if strings.Contains(output, "%%%%") {
		t.Error("first output should not contain separator")
	}
}

func TestSearchCmd_SortByCreated(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := true
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--sort", "created:desc", "#daily"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var results []struct {
		UUID string `json:"uuid"`
	}

	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Should be sorted by created desc (newest first)
	if results[0].UUID != "uuid-2" {
		t.Errorf("first result UUID = %q, want uuid-2 (newest)", results[0].UUID)
	}
}

func TestSearchCmd_Limit(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--limit", "1", "#daily"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("found %d results, want 1 (limited)", len(lines))
	}
}

func TestSearchCmd_Everything(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--everything"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should find all 5 test notes
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 5 {
		t.Errorf("found %d notes, want 5", len(lines))
	}
}

func TestSearchCmd_ParentFilter(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := true
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"parent:uuid-1"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var results []struct {
		UUID   string `json:"uuid"`
		Parent string `json:"parent"`
	}

	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// uuid-3 has parent uuid-1
	if len(results) != 1 {
		t.Errorf("found %d results, want 1", len(results))
	}

	if len(results) > 0 && results[0].UUID != "uuid-3" {
		t.Errorf("UUID = %q, want uuid-3", results[0].UUID)
	}
}

func TestSearchCmd_ParentNone(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := true
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"parent:none"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var results []struct {
		UUID   string `json:"uuid"`
		Parent string `json:"parent"`
	}

	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// uuid-1 and uuid-2 have no parent
	if len(results) != 2 {
		t.Errorf("found %d results, want 2 (notes without parent)", len(results))
	}
}

func TestSearchCmd_MutualExclusivity(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := false
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"--bulk", "--first", "#daily"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for mutually exclusive flags")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want to mention 'mutually exclusive'", err.Error())
	}
}

func TestSearchCmd_EditJsonIncompatible(t *testing.T) {
	vlt := setupTestVault(t)

	jsonOut := true
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"--edit", "#daily"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for --edit with --json")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want to mention 'mutually exclusive'", err.Error())
	}
}

func TestSearchCmd_EditFirstAllowed(t *testing.T) {
	vlt := setupTestVault(t)

	// --edit --first should be allowed (not mutually exclusive)
	// This test verifies the flags are accepted; actual editing requires EDITOR
	jsonOut := false
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// Capture stderr to suppress "No changes made" message
	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	cmd.SetArgs([]string{"--edit", "--first", "#daily"})
	err := cmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	// Should not error on flag combination (EDITOR=true makes it a no-op)
	if err != nil {
		t.Errorf("--edit --first should be allowed, got error: %v", err)
	}
}

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"simple tag", "#daily", false},
		{"simple text", "hello", false},
		{"tag and text", "#daily work", false},
		{"explicit AND", "#daily && #work", false},
		{"spaced tag", "#daily note#", false},
		{"empty", "", true},
		{"only spaces", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseQuery(tt.query, TagScopeAll)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseSort(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []SortField
		wantErr bool
	}{
		{
			name:  "single field asc",
			input: "created:asc",
			want:  []SortField{{Field: "created", Ascending: true}},
		},
		{
			name:  "single field desc",
			input: "updated:desc",
			want:  []SortField{{Field: "updated", Ascending: false}},
		},
		{
			name:  "field without direction",
			input: "title",
			want:  []SortField{{Field: "title", Ascending: true}},
		},
		{
			name:  "multiple fields",
			input: "created:desc,title:asc",
			want: []SortField{
				{Field: "created", Ascending: false},
				{Field: "title", Ascending: true},
			},
		},
		{
			name:    "invalid field",
			input:   "invalid:asc",
			wantErr: true,
		},
		{
			name:    "invalid direction",
			input:   "created:wrong",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSort(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("parseSort() returned %d fields, want %d", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("parseSort()[%d] = %+v, want %+v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestSplitTerms(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"#tag1 #tag2", []string{"#tag1", "#tag2"}},
		{"#tag word", []string{"#tag", "word"}},
		{"#spaced tag# other", []string{"#spaced tag#", "other"}},
		{"word1 word2", []string{"word1", "word2"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitTerms(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitTerms() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitTerms()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseBulk(t *testing.T) {
	content := `%%%% uuid-1 %%%%
# Note 1
Content 1

%%%% uuid-2 %%%%
# Note 2
Content 2
`

	result := note.ParseBulk(content)

	if len(result) != 2 {
		t.Errorf("note.ParseBulk() returned %d entries, want 2", len(result))
	}

	if !strings.Contains(result["uuid-1"], "Note 1") {
		t.Error("uuid-1 content should contain 'Note 1'")
	}

	if !strings.Contains(result["uuid-2"], "Note 2") {
		t.Error("uuid-2 content should contain 'Note 2'")
	}
}

func TestTagMatcher(t *testing.T) {
	n := &note.Note{
		Tags: []string{"#daily", "#work"},
	}

	tests := []struct {
		tag  string
		want bool
	}{
		{"#daily", true},
		{"#Daily", true}, // case insensitive
		{"#WORK", true},
		{"#missing", false},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			matcher := tagMatcher(tt.tag, TagScopeAll)
			if got := matcher(n); got != tt.want {
				t.Errorf("tagMatcher(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestTextMatcher(t *testing.T) {
	n := &note.Note{
		Title:   "My Test Note",
		Content: "This is some content about testing.",
	}

	tests := []struct {
		text string
		want bool
	}{
		{"test", true},     // in title
		{"TEST", true},     // case insensitive
		{"content", true},  // in content
		{"testing", true},  // in content
		{"missing", false}, // not found
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			matcher := textMatcher(tt.text)
			if got := matcher(n); got != tt.want {
				t.Errorf("textMatcher(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestTitleMatcher(t *testing.T) {
	n := &note.Note{
		Title:   "Meeting Notes",
		Content: "Some content",
	}

	tests := []struct {
		text string
		want bool
	}{
		{"meeting", true},
		{"NOTES", true},
		{"content", false}, // in content, not title
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			matcher := titleMatcher(tt.text)
			if got := matcher(n); got != tt.want {
				t.Errorf("titleMatcher(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestPathMatcher(t *testing.T) {
	n := &note.Note{
		FilePath: "/vault/projects/alpha/notes.md",
	}

	tests := []struct {
		text string
		want bool
	}{
		{"projects", true},
		{"alpha", true},
		{"NOTES", true},
		{"beta", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			matcher := pathMatcher(tt.text)
			if got := matcher(n); got != tt.want {
				t.Errorf("pathMatcher(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestDateFilters(t *testing.T) {
	// Test parsing of date filter terms
	tests := []struct {
		name    string
		term    string
		wantErr bool
	}{
		{"created:date", "created:2025-01-28", false},
		{"created:month", "created:2025-01", false},
		{"created:year", "created:2025", false},
		{"created:today", "created:today", false},
		{"created:yesterday", "created:yesterday", false},
		{"updated:date", "updated:2025-01-28", false},
		{"before:date", "before:2025-01-28", false},
		{"after:date", "after:2025-01-28", false},
		{"on:date", "on:2025-01-28", false},
		{"between:dates", "between:2025-01-01,2025-01-31", false},
		{"title:text", "title:meeting", false},
		{"path:text", "path:projects/", false},
		{"created:invalid", "created:invalid-date", true},
		{"between:missing-comma", "between:2025-01-01", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseTermMatcher(tt.term, TagScopeAll)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTermMatcher(%q) error = %v, wantErr %v", tt.term, err, tt.wantErr)
			}
		})
	}
}

func TestParseQuery_WithFilters(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"tag with date filter", "#daily && created:today", false},
		{"date filter only", "created:2025-01", false},
		{"multiple filters", "title:meeting && path:projects/", false},
		{"tag and title filter", "#work title:report", false},
		{"between filter", "between:2025-01-01,2025-01-31", false},
		{"date filter", "updated:yesterday", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseQuery(tt.query, TagScopeAll)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseQuery(%q) error = %v, wantErr %v", tt.query, err, tt.wantErr)
			}
		})
	}
}

func TestSearchOptions_EarlyTermination(t *testing.T) {
	vlt := setupTestVault(t)

	// Test that early termination works with limit and no sorting
	jsonOut := false
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Search with limit, should return exactly 1 result
	cmd.SetArgs([]string{"--limit", "1", "#daily"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("with limit=1, found %d results, want 1", len(lines))
	}
}

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
