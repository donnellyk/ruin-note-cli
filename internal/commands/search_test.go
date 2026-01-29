package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevin/ruin-note-cli/internal/note"
	"github.com/kevin/ruin-note-cli/internal/vault"
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
---
# Project Alpha
#project #work

Working on project alpha with the team.`,
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

	// Should find 1 note with "alpha"
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("found %d notes, want 1", len(lines))
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

	if len(results) != 1 {
		t.Errorf("found %d results, want 1", len(results))
	}

	if results[0].UUID != "uuid-3" {
		t.Errorf("UUID = %q, want %q", results[0].UUID, "uuid-3")
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

	if !strings.Contains(output, "Project Alpha") {
		t.Error("first output should contain note content")
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
			_, err := parseQuery(tt.query)
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
			matcher := tagMatcher(tt.tag)
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
		{"test", true},      // in title
		{"TEST", true},      // case insensitive
		{"content", true},   // in content
		{"testing", true},   // in content
		{"missing", false},  // not found
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
		{"created:7d", "created:7d", false},
		{"updated:date", "updated:2025-01-28", false},
		{"before:date", "before:2025-01-28", false},
		{"after:date", "after:2025-01-28", false},
		{"on:date", "on:2025-01-28", false},
		{"between:dates", "between:2025-01-01,2025-01-31", false},
		{"between:natural", "between:last-month,today", false},
		{"title:text", "title:meeting", false},
		{"path:text", "path:projects/", false},
		{"created:invalid", "created:invalid-date", true},
		{"between:missing-comma", "between:2025-01-01", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTermMatcher(tt.term)
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
		{"duration filter", "updated:7d", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseQuery(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseQuery(%q) error = %v, wantErr %v", tt.query, err, tt.wantErr)
			}
		})
	}
}
