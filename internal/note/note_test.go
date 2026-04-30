package note

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParse_BasicNote(t *testing.T) {
	content := `---
uuid: test-uuid-123
created: 2025-01-28T10:00:00-05:00
updated: 2025-01-28T11:00:00-05:00
---
# My Test Note
#global

This is the content with #inline tag.`

	note, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if note.UUID != "test-uuid-123" {
		t.Errorf("UUID = %q, want %q", note.UUID, "test-uuid-123")
	}

	if note.Title != "My Test Note" {
		t.Errorf("Title = %q, want %q", note.Title, "My Test Note")
	}

	if note.Created.Year() != 2025 {
		t.Errorf("Created year = %d, want 2025", note.Created.Year())
	}

	// Should have 1 global tag
	if len(note.Tags) != 1 {
		t.Errorf("Tags = %v, want 1 global tag", note.Tags)
	}

	// Should have 1 inline tag
	if len(note.InlineTags) != 1 {
		t.Errorf("InlineTags = %v, want 1 inline tag", note.InlineTags)
	}

	// AllTags should have both
	if len(note.AllTags()) != 2 {
		t.Errorf("AllTags() = %v, want 2 tags total", note.AllTags())
	}
}

func TestParse_NoFrontmatter(t *testing.T) {
	content := `# Simple Note

Just content here.`

	note, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if note.UUID != "" {
		t.Errorf("UUID = %q, want empty", note.UUID)
	}

	if note.Title != "Simple Note" {
		t.Errorf("Title = %q, want %q", note.Title, "Simple Note")
	}
}

func TestNote_Serialize(t *testing.T) {
	note := &Note{
		UUID:       "abc-123",
		Created:    time.Date(2025, 1, 28, 10, 0, 0, 0, time.UTC),
		Updated:    time.Date(2025, 1, 28, 11, 0, 0, 0, time.UTC),
		Tags:       []string{"#tag1", "#tag2"},
		InlineTags: []string{"#tag2"},
		Content:    "# My Note\n\nContent here.",
	}

	result, err := note.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	if !strings.HasPrefix(result, "---\n") {
		t.Error("Serialize() should start with frontmatter")
	}

	if !strings.Contains(result, "uuid: abc-123") {
		t.Error("Serialize() should contain UUID")
	}

	if !strings.Contains(result, "# My Note") {
		t.Error("Serialize() should contain content")
	}
}

func TestNote_RoundTrip(t *testing.T) {
	original := &Note{
		UUID:       "round-trip-test",
		Created:    time.Date(2025, 1, 28, 10, 0, 0, 0, time.FixedZone("EST", -5*3600)),
		Updated:    time.Date(2025, 1, 28, 11, 0, 0, 0, time.FixedZone("EST", -5*3600)),
		Tags:       []string{"#test"},
		InlineTags: []string{},
		Content:    "# Test Note\n#test\n\nContent here.",
		Extra:      map[string]any{"custom": "field"},
	}

	// Serialize
	serialized, err := original.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	// Parse back
	parsed, err := Parse(serialized)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if parsed.UUID != original.UUID {
		t.Errorf("UUID = %q, want %q", parsed.UUID, original.UUID)
	}

	if parsed.Title != "Test Note" {
		t.Errorf("Title = %q, want %q", parsed.Title, "Test Note")
	}
}

func TestNote_LoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	notePath := filepath.Join(tmpDir, "test-note.md")

	// Create a note
	note := &Note{
		UUID:     "file-test",
		Content:  "# File Test\n\nContent.",
		FilePath: notePath,
	}
	note.SetTimestamps()

	// Save
	if err := note.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(notePath); err != nil {
		t.Fatalf("File not created: %v", err)
	}

	// Load
	loaded, err := Load(notePath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.UUID != "file-test" {
		t.Errorf("UUID = %q, want %q", loaded.UUID, "file-test")
	}

	if loaded.Title != "File Test" {
		t.Errorf("Title = %q, want %q", loaded.Title, "File Test")
	}
}

func TestNote_EnsureUUID(t *testing.T) {
	note := &Note{}

	if note.UUID != "" {
		t.Error("UUID should be empty initially")
	}

	note.EnsureUUID()

	if note.UUID == "" {
		t.Error("UUID should be set after EnsureUUID()")
	}

	// Should be a valid UUID format (36 chars with hyphens)
	if len(note.UUID) != 36 {
		t.Errorf("UUID length = %d, want 36", len(note.UUID))
	}

	// Calling again should not change it
	originalUUID := note.UUID
	note.EnsureUUID()
	if note.UUID != originalUUID {
		t.Error("EnsureUUID() should not change existing UUID")
	}
}

func TestNote_SetTimestamps(t *testing.T) {
	note := &Note{}

	before := time.Now()
	note.SetTimestamps()
	after := time.Now()

	if note.Created.Before(before) || note.Created.After(after) {
		t.Error("Created should be set to current time")
	}

	if note.Updated.Before(before) || note.Updated.After(after) {
		t.Error("Updated should be set to current time")
	}

	// Second call should not change Created
	originalCreated := note.Created
	time.Sleep(10 * time.Millisecond)
	note.SetTimestamps()

	if !note.Created.Equal(originalCreated) {
		t.Error("SetTimestamps() should not change existing Created")
	}

	if note.Updated.Equal(originalCreated) {
		t.Error("SetTimestamps() should update Updated")
	}
}

func TestNote_GenerateFilename(t *testing.T) {
	tests := []struct {
		name  string
		note  *Note
		want  string
		check func(string) bool
	}{
		{
			name: "from title",
			note: &Note{Title: "My Great Note"},
			want: "My Great Note",
		},
		{
			name: "title with special chars",
			note: &Note{Title: "Note: A/B Test?"},
			want: "Note- A-B Test",
		},
		{
			name: "no title uses timestamp",
			note: &Note{Created: time.Date(2025, 1, 28, 10, 30, 45, 0, time.UTC)},
			want: "2025-01-28T10-30-45",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.note.GenerateFilename()
			if got != tt.want {
				t.Errorf("GenerateFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal name", "normal name"},
		{"with/slash", "with-slash"},
		{"with:colon", "with-colon"},
		{"with?question", "withquestion"},
		{"with\"quotes\"", "withquotes"},
		{"  spaces  ", "spaces"},
		{"...dots...", "dots"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNote_RefreshTags(t *testing.T) {
	note := &Note{
		Content: "# Note\n#old\n\nContent with #new tag.",
		Tags:    []string{"#stale"},
	}

	note.RefreshTags()

	// Should now have 1 global tag (stored form, no `#`)
	if len(note.Tags) != 1 || note.Tags[0] != "old" {
		t.Errorf("Tags = %v, want [old]", note.Tags)
	}

	// new should be inline (stored form, no `#`)
	if len(note.InlineTags) != 1 || note.InlineTags[0] != "new" {
		t.Errorf("InlineTags = %v, want [new]", note.InlineTags)
	}

	// AllTags should have both (own global + inline, NOT inherited)
	if len(note.AllTags()) != 2 {
		t.Errorf("AllTags() = %v, want 2 tags", note.AllTags())
	}
}

func TestNote_EffectiveGlobalTags(t *testing.T) {
	n := &Note{
		Tags:          []string{"own"},
		InheritedTags: []string{"inherited", "own"}, // own overlaps
	}

	effective := n.EffectiveGlobalTags()
	if len(effective) != 2 {
		t.Fatalf("EffectiveGlobalTags() = %v, want 2 tags", effective)
	}
	if effective[0] != "own" || effective[1] != "inherited" {
		t.Errorf("EffectiveGlobalTags() = %v, want [own, inherited]", effective)
	}
}

func TestNote_EffectiveGlobalTags_NoInherited(t *testing.T) {
	n := &Note{
		Tags: []string{"own"},
	}

	effective := n.EffectiveGlobalTags()
	if len(effective) != 1 || effective[0] != "own" {
		t.Errorf("EffectiveGlobalTags() = %v, want [own]", effective)
	}
}

func TestNote_AllTags_ExcludesInherited(t *testing.T) {
	n := &Note{
		Tags:          []string{"global"},
		InlineTags:    []string{"inline"},
		InheritedTags: []string{"inherited"},
	}

	all := n.AllTags()
	for _, tag := range all {
		if tag == "inherited" {
			t.Error("AllTags() should not include inherited tags")
		}
	}
	if len(all) != 2 {
		t.Errorf("AllTags() = %v, want 2 tags", all)
	}
}

func TestNote_ContentWithoutTitle(t *testing.T) {
	note := &Note{
		Title:   "My Title",
		Content: "# My Title\n\nContent here.",
	}

	got := note.ContentWithoutTitle()
	want := "Content here."

	if got != want {
		t.Errorf("ContentWithoutTitle() = %q, want %q", got, want)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"# Simple Title\n\nContent", "Simple Title"},
		{"# Title With Spaces  \n", "Title With Spaces"},
		{"No title here", ""},
		{"## H2 Title\n", "H2 Title"},
		{"### H3 Title\n", "H3 Title"},
		{"Text\n# Title After\n", "Title After"},
		{"Text\n## H2 After\n", "H2 After"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := extractTitle(tt.content)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
