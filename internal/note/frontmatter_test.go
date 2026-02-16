package note

import (
	"strings"
	"testing"
)

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "# My Note\n\nSome content here."

	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseFrontmatter() error = %v", err)
	}

	if fm.UUID != "" {
		t.Errorf("UUID = %q, want empty", fm.UUID)
	}

	if body != content {
		t.Errorf("body = %q, want %q", body, content)
	}
}

func TestParseFrontmatter_WithFrontmatter(t *testing.T) {
	content := `---
uuid: abc-123
created: 2025-01-28T10:00:00-05:00
updated: 2025-01-28T11:00:00-05:00
tags:
  - "#ruin"
  - "#log"
inline-tags:
  - "#followup"
---
# My Note

Some content here.`

	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseFrontmatter() error = %v", err)
	}

	if fm.UUID != "abc-123" {
		t.Errorf("UUID = %q, want %q", fm.UUID, "abc-123")
	}

	// YAML parses timestamps to time.Time, which we convert back to string
	if fm.Created == "" {
		t.Error("Created should not be empty")
	}

	if len(fm.Tags) != 2 {
		t.Errorf("Tags = %v, want 2 items", fm.Tags)
	}

	if len(fm.InlineTags) != 1 {
		t.Errorf("InlineTags = %v, want 1 item", fm.InlineTags)
	}

	expectedBody := "# My Note\n\nSome content here."
	if body != expectedBody {
		t.Errorf("body = %q, want %q", body, expectedBody)
	}
}

func TestParseFrontmatter_PreservesExtraFields(t *testing.T) {
	content := `---
uuid: abc-123
custom_field: custom_value
another: 42
---
Content`

	fm, _, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseFrontmatter() error = %v", err)
	}

	if fm.UUID != "abc-123" {
		t.Errorf("UUID = %q, want %q", fm.UUID, "abc-123")
	}

	if fm.Extra["custom_field"] != "custom_value" {
		t.Errorf("Extra[custom_field] = %v, want %q", fm.Extra["custom_field"], "custom_value")
	}

	if fm.Extra["another"] != 42 {
		t.Errorf("Extra[another] = %v, want 42", fm.Extra["another"])
	}
}

func TestParseFrontmatter_UnclosedDelimiter(t *testing.T) {
	content := `---
uuid: abc-123
# No closing delimiter
Content here`

	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseFrontmatter() error = %v", err)
	}

	// Should treat as no frontmatter
	if fm.UUID != "" {
		t.Errorf("UUID = %q, want empty (unclosed delimiter)", fm.UUID)
	}

	if body != content {
		t.Errorf("body should be original content when no valid frontmatter")
	}
}

func TestFrontmatter_Serialize(t *testing.T) {
	fm := &Frontmatter{
		UUID:       "abc-123",
		Created:    "2025-01-28T10:00:00-05:00",
		Tags:       []string{"#ruin", "#log"},
		InlineTags: []string{"#followup"},
		Extra:      map[string]interface{}{"custom": "value"},
	}

	result, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	if !strings.HasPrefix(result, "---\n") {
		t.Error("Serialize() should start with ---")
	}

	if !strings.HasSuffix(result, "---\n") {
		t.Error("Serialize() should end with ---")
	}

	if !strings.Contains(result, "uuid: abc-123") {
		t.Error("Serialize() should contain uuid")
	}

	if !strings.Contains(result, "custom: value") {
		t.Error("Serialize() should contain custom field")
	}
}

func TestFrontmatter_SerializeEmpty(t *testing.T) {
	fm := &Frontmatter{}

	result, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	if result != "" {
		t.Errorf("Serialize() = %q, want empty for empty frontmatter", result)
	}
}

func TestFrontmatter_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		fm   *Frontmatter
		want bool
	}{
		{
			name: "empty",
			fm:   &Frontmatter{},
			want: true,
		},
		{
			name: "has uuid",
			fm:   &Frontmatter{UUID: "abc"},
			want: false,
		},
		{
			name: "has tags",
			fm:   &Frontmatter{Tags: []string{"#tag"}},
			want: false,
		},
		{
			name: "has extra",
			fm:   &Frontmatter{Extra: map[string]interface{}{"key": "val"}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fm.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseFrontmatter_WithParent(t *testing.T) {
	content := `---
uuid: child-1
parent: parent-uuid-123
tags:
  - "#work"
---
# Child Note

Content here.`

	fm, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseFrontmatter() error = %v", err)
	}

	if fm.Parent != "parent-uuid-123" {
		t.Errorf("Parent = %q, want %q", fm.Parent, "parent-uuid-123")
	}

	if fm.UUID != "child-1" {
		t.Errorf("UUID = %q, want %q", fm.UUID, "child-1")
	}

	// Parent should NOT leak into Extra
	if _, ok := fm.Extra["parent"]; ok {
		t.Error("parent should not appear in Extra")
	}

	if !strings.Contains(body, "Child Note") {
		t.Errorf("body = %q, should contain 'Child Note'", body)
	}
}

func TestFrontmatter_SerializeParent(t *testing.T) {
	fm := &Frontmatter{
		UUID:   "child-1",
		Parent: "parent-uuid-123",
	}

	result, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	if !strings.Contains(result, "parent: parent-uuid-123") {
		t.Errorf("Serialize() = %q, should contain parent field", result)
	}

	if !strings.Contains(result, "uuid: child-1") {
		t.Errorf("Serialize() = %q, should contain uuid field", result)
	}
}

func TestFrontmatter_IsEmpty_WithParent(t *testing.T) {
	fm := &Frontmatter{Parent: "some-uuid"}
	if fm.IsEmpty() {
		t.Error("IsEmpty() should return false when Parent is set")
	}
}

func TestFrontmatter_Merge(t *testing.T) {
	fm1 := &Frontmatter{
		UUID:    "original",
		Created: "2025-01-01",
		Extra:   map[string]interface{}{"key1": "val1"},
	}

	fm2 := &Frontmatter{
		UUID:    "new",
		Updated: "2025-01-02",
		Extra:   map[string]interface{}{"key2": "val2"},
	}

	fm1.Merge(fm2)

	if fm1.UUID != "new" {
		t.Errorf("UUID = %q, want %q", fm1.UUID, "new")
	}

	if fm1.Created != "2025-01-01" {
		t.Errorf("Created = %q, want %q (should be preserved)", fm1.Created, "2025-01-01")
	}

	if fm1.Updated != "2025-01-02" {
		t.Errorf("Updated = %q, want %q", fm1.Updated, "2025-01-02")
	}

	if fm1.Extra["key1"] != "val1" {
		t.Error("Extra[key1] should be preserved")
	}

	if fm1.Extra["key2"] != "val2" {
		t.Error("Extra[key2] should be merged")
	}
}

func TestFrontmatter_MergeParent(t *testing.T) {
	fm1 := &Frontmatter{
		UUID:  "child",
		Extra: map[string]interface{}{},
	}

	fm2 := &Frontmatter{
		Parent: "parent-uuid",
	}

	fm1.Merge(fm2)

	if fm1.Parent != "parent-uuid" {
		t.Errorf("Parent = %q, want %q", fm1.Parent, "parent-uuid")
	}

	// Empty parent should NOT overwrite
	fm3 := &Frontmatter{}
	fm1.Merge(fm3)

	if fm1.Parent != "parent-uuid" {
		t.Errorf("Parent = %q, want %q (should be preserved when merging empty)", fm1.Parent, "parent-uuid")
	}
}

func TestParseFrontmatter_ParentRoundTrip(t *testing.T) {
	fm := &Frontmatter{
		UUID:    "note-1",
		Created: "2025-01-28T10:00:00-05:00",
		Parent:  "parent-uuid",
		Tags:    []string{"#test"},
	}

	serialized, err := fm.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	content := serialized + "# Test Note\n\nContent."
	parsed, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseFrontmatter() error = %v", err)
	}

	if parsed.Parent != "parent-uuid" {
		t.Errorf("round-trip Parent = %q, want %q", parsed.Parent, "parent-uuid")
	}

	if parsed.UUID != "note-1" {
		t.Errorf("round-trip UUID = %q, want %q", parsed.UUID, "note-1")
	}

	if !strings.Contains(body, "Test Note") {
		t.Errorf("body = %q, should contain 'Test Note'", body)
	}
}
