package note

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatterFast_AllFields(t *testing.T) {
	data := []byte(`---
uuid: abc-123
created: 2025-01-15T10:00:00-05:00
updated: 2025-01-15T11:00:00-05:00
tags:
  - "#daily"
  - "#work"
inline-tags:
  - "#todo"
  - "#followup"
parent: parent-uuid
order: 5
linked-cards:
  - "card-1"
  - "card-2"
---
# My Note

Body content here.
`)

	fm, bodyOffset, err := ParseFrontmatterFast(data)
	if err != nil {
		t.Fatalf("ParseFrontmatterFast() error = %v", err)
	}

	if fm.UUID != "abc-123" {
		t.Errorf("UUID = %q, want %q", fm.UUID, "abc-123")
	}
	if fm.Created != "2025-01-15T10:00:00-05:00" {
		t.Errorf("Created = %q, want %q", fm.Created, "2025-01-15T10:00:00-05:00")
	}
	if fm.Updated != "2025-01-15T11:00:00-05:00" {
		t.Errorf("Updated = %q, want %q", fm.Updated, "2025-01-15T11:00:00-05:00")
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "#daily" || fm.Tags[1] != "#work" {
		t.Errorf("Tags = %v, want [#daily, #work]", fm.Tags)
	}
	if len(fm.InlineTags) != 2 || fm.InlineTags[0] != "#todo" || fm.InlineTags[1] != "#followup" {
		t.Errorf("InlineTags = %v, want [#todo, #followup]", fm.InlineTags)
	}
	if fm.Parent != "parent-uuid" {
		t.Errorf("Parent = %q, want %q", fm.Parent, "parent-uuid")
	}
	if fm.Order == nil || *fm.Order != 5 {
		t.Errorf("Order = %v, want 5", fm.Order)
	}
	if len(fm.LinkedCards) != 2 || fm.LinkedCards[0] != "card-1" || fm.LinkedCards[1] != "card-2" {
		t.Errorf("LinkedCards = %v, want [card-1, card-2]", fm.LinkedCards)
	}

	body := string(data[bodyOffset:])
	if body[:10] != "# My Note\n" {
		t.Errorf("body starts with %q, want '# My Note\\n'", body[:10])
	}
}

func TestParseFrontmatterFast_FlowStyleLists(t *testing.T) {
	data := []byte(`---
uuid: flow-test
tags: ["#daily", "#work"]
inline-tags: [#todo, #followup]
linked-cards: ["card-1"]
---
# Title
`)

	fm, _, err := ParseFrontmatterFast(data)
	if err != nil {
		t.Fatalf("ParseFrontmatterFast() error = %v", err)
	}

	if len(fm.Tags) != 2 || fm.Tags[0] != "#daily" || fm.Tags[1] != "#work" {
		t.Errorf("Tags = %v, want [#daily, #work]", fm.Tags)
	}
	if len(fm.InlineTags) != 2 || fm.InlineTags[0] != "#todo" || fm.InlineTags[1] != "#followup" {
		t.Errorf("InlineTags = %v, want [#todo, #followup]", fm.InlineTags)
	}
	if len(fm.LinkedCards) != 1 || fm.LinkedCards[0] != "card-1" {
		t.Errorf("LinkedCards = %v, want [card-1]", fm.LinkedCards)
	}
}

func TestParseFrontmatterFast_QuotedAndUnquoted(t *testing.T) {
	data := []byte(`---
uuid: "quoted-uuid"
parent: 'single-quoted'
created: 2025-01-15T10:00:00-05:00
tags:
  - "#daily"
  - unquoted-tag
---
Body
`)

	fm, _, err := ParseFrontmatterFast(data)
	if err != nil {
		t.Fatalf("ParseFrontmatterFast() error = %v", err)
	}

	if fm.UUID != "quoted-uuid" {
		t.Errorf("UUID = %q, want %q", fm.UUID, "quoted-uuid")
	}
	if fm.Parent != "single-quoted" {
		t.Errorf("Parent = %q, want %q", fm.Parent, "single-quoted")
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "#daily" || fm.Tags[1] != "unquoted-tag" {
		t.Errorf("Tags = %v, want [#daily, unquoted-tag]", fm.Tags)
	}
}

func TestParseFrontmatterFast_MissingFields(t *testing.T) {
	data := []byte(`---
uuid: minimal
---
Body
`)

	fm, _, err := ParseFrontmatterFast(data)
	if err != nil {
		t.Fatalf("ParseFrontmatterFast() error = %v", err)
	}

	if fm.UUID != "minimal" {
		t.Errorf("UUID = %q, want %q", fm.UUID, "minimal")
	}
	if fm.Created != "" {
		t.Errorf("Created = %q, want empty", fm.Created)
	}
	if len(fm.Tags) != 0 {
		t.Errorf("Tags = %v, want empty", fm.Tags)
	}
	if fm.Parent != "" {
		t.Errorf("Parent = %q, want empty", fm.Parent)
	}
	if fm.Order != nil {
		t.Errorf("Order = %v, want nil", fm.Order)
	}
}

func TestParseFrontmatterFast_NoFrontmatter(t *testing.T) {
	data := []byte("# Just a title\n\nSome content\n")

	fm, bodyOffset, err := ParseFrontmatterFast(data)
	if err != nil {
		t.Fatalf("ParseFrontmatterFast() error = %v", err)
	}

	if fm.UUID != "" {
		t.Errorf("UUID = %q, want empty", fm.UUID)
	}
	if bodyOffset != 0 {
		t.Errorf("bodyOffset = %d, want 0", bodyOffset)
	}
}

func TestParseFrontmatterFast_Truncated(t *testing.T) {
	// Opening --- but no closing ---
	data := []byte("---\nuuid: test\ntags:\n  - \"#daily\"\n")

	_, _, err := ParseFrontmatterFast(data)
	if err != ErrFrontmatterTruncated {
		t.Errorf("err = %v, want ErrFrontmatterTruncated", err)
	}
}

func TestParseFrontmatterFast_UnknownKeysSkipped(t *testing.T) {
	data := []byte(`---
uuid: test-uuid
custom-field: some value
another: 42
tags:
  - "#daily"
---
Body
`)

	fm, _, err := ParseFrontmatterFast(data)
	if err != nil {
		t.Fatalf("ParseFrontmatterFast() error = %v", err)
	}

	if fm.UUID != "test-uuid" {
		t.Errorf("UUID = %q, want %q", fm.UUID, "test-uuid")
	}
	if len(fm.Tags) != 1 || fm.Tags[0] != "#daily" {
		t.Errorf("Tags = %v, want [#daily]", fm.Tags)
	}
	// Extra should be nil (fast path doesn't populate Extra)
	if fm.Extra != nil {
		t.Errorf("Extra = %v, want nil", fm.Extra)
	}
}

func TestParseFrontmatterFast_OrderAsInt(t *testing.T) {
	data := []byte(`---
uuid: order-test
order: 42
---
Body
`)

	fm, _, err := ParseFrontmatterFast(data)
	if err != nil {
		t.Fatalf("ParseFrontmatterFast() error = %v", err)
	}

	if fm.Order == nil || *fm.Order != 42 {
		t.Errorf("Order = %v, want 42", fm.Order)
	}
}

func TestParseFrontmatterFast_LeadingNewlines(t *testing.T) {
	data := []byte("\n\n---\nuuid: test\n---\nBody\n")

	fm, _, err := ParseFrontmatterFast(data)
	if err != nil {
		t.Fatalf("ParseFrontmatterFast() error = %v", err)
	}

	if fm.UUID != "test" {
		t.Errorf("UUID = %q, want %q", fm.UUID, "test")
	}
}

func TestParseFrontmatterFast_ConsistencyWithYAML(t *testing.T) {
	// Verify that fast parser produces same known fields as parseFrontmatterYAML
	testCases := []string{
		`
uuid: abc-123
created: "2025-01-15T10:00:00-05:00"
updated: "2025-01-15T11:00:00-05:00"
tags:
  - "#daily"
  - "#work"
inline-tags:
  - "#todo"
parent: parent-uuid
order: 3
linked-cards:
  - "card-1"
`,
		`
uuid: minimal-note
created: "2025-06-01T08:00:00-05:00"
updated: "2025-06-01T08:00:00-05:00"
`,
		`
uuid: tagged-only
tags:
  - "#project"
  - "#2025/jan"
`,
		`
uuid: with-parent
parent: some-parent-uuid
tags:
  - "#child"
inline-tags:
  - "#inline1"
  - "#inline2"
`,
	}

	for i, yamlContent := range testCases {
		// Parse with YAML
		yamlFM, err := parseFrontmatterYAML(yamlContent)
		if err != nil {
			t.Fatalf("case %d: parseFrontmatterYAML() error = %v", i, err)
		}

		// Parse with fast parser (wrap in --- delimiters)
		fastData := []byte("---" + yamlContent + "---\nBody\n")
		fastFM, _, err := ParseFrontmatterFast(fastData)
		if err != nil {
			t.Fatalf("case %d: ParseFrontmatterFast() error = %v", i, err)
		}

		// Compare known fields
		if fastFM.UUID != yamlFM.UUID {
			t.Errorf("case %d: UUID fast=%q yaml=%q", i, fastFM.UUID, yamlFM.UUID)
		}
		if fastFM.Created != yamlFM.Created {
			t.Errorf("case %d: Created fast=%q yaml=%q", i, fastFM.Created, yamlFM.Created)
		}
		if fastFM.Updated != yamlFM.Updated {
			t.Errorf("case %d: Updated fast=%q yaml=%q", i, fastFM.Updated, yamlFM.Updated)
		}
		if fastFM.Parent != yamlFM.Parent {
			t.Errorf("case %d: Parent fast=%q yaml=%q", i, fastFM.Parent, yamlFM.Parent)
		}
		if !slicesEqual(fastFM.Tags, yamlFM.Tags) {
			t.Errorf("case %d: Tags fast=%v yaml=%v", i, fastFM.Tags, yamlFM.Tags)
		}
		if !slicesEqual(fastFM.InlineTags, yamlFM.InlineTags) {
			t.Errorf("case %d: InlineTags fast=%v yaml=%v", i, fastFM.InlineTags, yamlFM.InlineTags)
		}
		if !slicesEqual(fastFM.LinkedCards, yamlFM.LinkedCards) {
			t.Errorf("case %d: LinkedCards fast=%v yaml=%v", i, fastFM.LinkedCards, yamlFM.LinkedCards)
		}
		if (fastFM.Order == nil) != (yamlFM.Order == nil) {
			t.Errorf("case %d: Order fast=%v yaml=%v", i, fastFM.Order, yamlFM.Order)
		}
		if fastFM.Order != nil && yamlFM.Order != nil && *fastFM.Order != *yamlFM.Order {
			t.Errorf("case %d: Order fast=%d yaml=%d", i, *fastFM.Order, *yamlFM.Order)
		}
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestLoadFrontmatterOnly(t *testing.T) {
	tmpDir := t.TempDir()

	content := `---
uuid: load-test
created: "2025-01-15T10:00:00-05:00"
updated: "2025-01-15T11:00:00-05:00"
tags:
  - "#daily"
  - "#work"
inline-tags:
  - "#todo"
parent: parent-uuid
---
# My Test Note

This is the body content.
It has multiple lines.
`
	path := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	n, err := LoadFrontmatterOnly(path)
	if err != nil {
		t.Fatalf("LoadFrontmatterOnly() error = %v", err)
	}

	if n.UUID != "load-test" {
		t.Errorf("UUID = %q, want %q", n.UUID, "load-test")
	}
	if n.Title != "My Test Note" {
		t.Errorf("Title = %q, want %q", n.Title, "My Test Note")
	}
	if n.FilePath != path {
		t.Errorf("FilePath = %q, want %q", n.FilePath, path)
	}
	if len(n.Tags) != 2 {
		t.Errorf("Tags = %v, want 2 tags", n.Tags)
	}
	if len(n.InlineTags) != 1 {
		t.Errorf("InlineTags = %v, want 1 tag", n.InlineTags)
	}
	if n.Parent != "parent-uuid" {
		t.Errorf("Parent = %q, want %q", n.Parent, "parent-uuid")
	}
	if n.Content != "" {
		t.Errorf("Content = %q, want empty (fast path)", n.Content)
	}
	if n.Created.IsZero() {
		t.Error("Created should be parsed")
	}
	if n.Updated.IsZero() {
		t.Error("Updated should be parsed")
	}
}

func TestLoadFrontmatterOnly_FallbackOnLargeFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with frontmatter > 4KB to trigger fallback
	var content string
	content = "---\nuuid: large-fm\ntags:\n"
	for i := 0; i < 300; i++ {
		content += "  - \"#tag-with-a-longer-name-" + string(rune('a'+i%26)) + "\"\n"
	}
	content += "---\n# Title\n\nBody.\n"

	path := filepath.Join(tmpDir, "large.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	n, err := LoadFrontmatterOnly(path)
	if err != nil {
		t.Fatalf("LoadFrontmatterOnly() error = %v", err)
	}

	// Should have fallen back to full Load and still work
	if n.UUID != "large-fm" {
		t.Errorf("UUID = %q, want %q", n.UUID, "large-fm")
	}
}

func TestLoadFrontmatterOnly_Consistency(t *testing.T) {
	// Verify that LoadFrontmatterOnly produces same metadata as Load
	tmpDir := t.TempDir()

	content := `---
uuid: consistency-test
created: "2025-03-10T08:30:00-05:00"
updated: "2025-03-10T09:00:00-05:00"
tags:
  - "#daily"
  - "#work"
inline-tags:
  - "#todo"
parent: some-parent
order: 7
linked-cards:
  - "linked-1"
  - "linked-2"
---
# Consistency Check
#daily #work

Some body content with #todo inline tag.
More text here.
`
	path := filepath.Join(tmpDir, "consistency.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fast, err := LoadFrontmatterOnly(path)
	if err != nil {
		t.Fatalf("LoadFrontmatterOnly() error = %v", err)
	}

	full, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Compare metadata fields
	if fast.UUID != full.UUID {
		t.Errorf("UUID: fast=%q full=%q", fast.UUID, full.UUID)
	}
	if fast.Title != full.Title {
		t.Errorf("Title: fast=%q full=%q", fast.Title, full.Title)
	}
	if fast.Parent != full.Parent {
		t.Errorf("Parent: fast=%q full=%q", fast.Parent, full.Parent)
	}
	if !fast.Created.Equal(full.Created) {
		t.Errorf("Created: fast=%v full=%v", fast.Created, full.Created)
	}
	if !fast.Updated.Equal(full.Updated) {
		t.Errorf("Updated: fast=%v full=%v", fast.Updated, full.Updated)
	}
	// Tags come from frontmatter in fast path, from body in full path
	// They should match since frontmatter was written from body tags
	if !slicesEqual(fast.Tags, full.Tags) {
		t.Errorf("Tags: fast=%v full=%v", fast.Tags, full.Tags)
	}
	if !slicesEqual(fast.InlineTags, full.InlineTags) {
		t.Errorf("InlineTags: fast=%v full=%v", fast.InlineTags, full.InlineTags)
	}
	if (fast.Order == nil) != (full.Order == nil) {
		t.Errorf("Order nil: fast=%v full=%v", fast.Order == nil, full.Order == nil)
	}
	if fast.Order != nil && full.Order != nil && *fast.Order != *full.Order {
		t.Errorf("Order: fast=%d full=%d", *fast.Order, *full.Order)
	}
	if !slicesEqual(fast.LinkedCards, full.LinkedCards) {
		t.Errorf("LinkedCards: fast=%v full=%v", fast.LinkedCards, full.LinkedCards)
	}
	if fast.FilePath != full.FilePath {
		t.Errorf("FilePath: fast=%q full=%q", fast.FilePath, full.FilePath)
	}
}

// --- Benchmarks ---

var benchFrontmatter = []byte(`---
uuid: bench-uuid-1234
created: "2025-01-15T10:00:00-05:00"
updated: "2025-01-15T11:00:00-05:00"
tags:
  - "#daily"
  - "#work"
  - "#project"
inline-tags:
  - "#todo"
  - "#followup"
parent: parent-uuid-5678
order: 3
linked-cards:
  - "card-aaa"
  - "card-bbb"
---
# Benchmark Note

This is body content for benchmarking.
`)

func BenchmarkParseFrontmatterFast(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := ParseFrontmatterFast(benchFrontmatter)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFrontmatterYAML(b *testing.B) {
	// Extract YAML portion to match what parseFrontmatterYAML receives
	yamlContent := `
uuid: bench-uuid-1234
created: "2025-01-15T10:00:00-05:00"
updated: "2025-01-15T11:00:00-05:00"
tags:
  - "#daily"
  - "#work"
  - "#project"
inline-tags:
  - "#todo"
  - "#followup"
parent: parent-uuid-5678
order: 3
linked-cards:
  - "card-aaa"
  - "card-bbb"
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parseFrontmatterYAML(yamlContent)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadFrontmatterOnly(b *testing.B) {
	tmpDir := b.TempDir()

	content := `---
uuid: bench-load-uuid
created: "2025-01-15T10:00:00-05:00"
updated: "2025-01-15T11:00:00-05:00"
tags:
  - "#daily"
  - "#work"
inline-tags:
  - "#todo"
parent: parent-uuid
---
# Benchmark Note

This is body content for benchmarking load performance.
Some more text to simulate a real note.
`
	path := filepath.Join(tmpDir, "bench.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadFrontmatterOnly(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadFull(b *testing.B) {
	tmpDir := b.TempDir()

	content := `---
uuid: bench-load-uuid
created: "2025-01-15T10:00:00-05:00"
updated: "2025-01-15T11:00:00-05:00"
tags:
  - "#daily"
  - "#work"
inline-tags:
  - "#todo"
parent: parent-uuid
---
# Benchmark Note

This is body content for benchmarking load performance.
Some more text to simulate a real note.
`
	path := filepath.Join(tmpDir, "bench.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Load(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadFrontmatterOnly_LargeFile(b *testing.B) {
	tmpDir := b.TempDir()

	// 5KB body
	body := ""
	for i := 0; i < 80; i++ {
		body += "Lorem ipsum dolor sit amet, consectetur adipiscing elit. "
	}

	content := `---
uuid: bench-large-uuid
created: "2025-01-15T10:00:00-05:00"
updated: "2025-01-15T11:00:00-05:00"
tags:
  - "#daily"
  - "#work"
inline-tags:
  - "#todo"
parent: parent-uuid
---
# Large Note

` + body + "\n"

	path := filepath.Join(tmpDir, "large.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadFrontmatterOnly(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadFull_LargeFile(b *testing.B) {
	tmpDir := b.TempDir()

	body := ""
	for i := 0; i < 80; i++ {
		body += "Lorem ipsum dolor sit amet, consectetur adipiscing elit. "
	}

	content := `---
uuid: bench-large-uuid
created: "2025-01-15T10:00:00-05:00"
updated: "2025-01-15T11:00:00-05:00"
tags:
  - "#daily"
  - "#work"
inline-tags:
  - "#todo"
parent: parent-uuid
---
# Large Note

` + body + "\n"

	path := filepath.Join(tmpDir, "large.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Load(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}
