package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

func TestIsListOnlyContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"dash list", "- item 1\n- item 2\n- item 3", true},
		{"asterisk list", "* item 1\n* item 2", true},
		{"plus list", "+ item 1\n+ item 2", true},
		{"numbered list", "1. first\n2. second\n3. third", true},
		{"checkboxes", "- [ ] todo\n- [x] done", true},
		{"nested list with indentation", "- top\n  - nested\n    - deep", true},
		{"blank lines between items", "- item 1\n\n- item 2\n\n- item 3", true},
		{"mixed markers", "- dash\n* star\n+ plus\n1. number", true},
		{"header plus list", "# Title\n- item", false},
		{"prose", "This is a paragraph of text.", false},
		{"mixed prose and list", "Some text\n- item", false},
		{"empty string", "", false},
		{"whitespace only", "   \n  ", false},
		{"list then prose", "- item\nsome text", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isListOnlyContent(tt.content)
			if got != tt.want {
				t.Errorf("isListOnlyContent(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestAdjustHeadings(t *testing.T) {
	tests := []struct {
		name    string
		content string
		depth   int
		want    string
	}{
		{
			name:    "depth 1 shifts all headings by 1",
			content: "# Title\nsome text\n## Sub",
			depth:   1,
			want:    "## Title\nsome text\n### Sub",
		},
		{
			name:    "caps at H6",
			content: "##### Deep\ntext\n###### Deeper",
			depth:   3,
			want:    "###### Deep\ntext\n###### Deeper",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adjustHeadings(tt.content, tt.depth)
			if got != tt.want {
				t.Errorf("adjustHeadings() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeHeadings(t *testing.T) {
	tests := []struct {
		name    string
		content string
		depth   int
		want    string
	}{
		{
			name:    "H1 at depth 1 becomes H2",
			content: "# Child Title\nsome text",
			depth:   1,
			want:    "## Child Title\nsome text",
		},
		{
			name:    "H3 at depth 2 stays H3",
			content: "### Grandchild\ntext",
			depth:   2,
			want:    "### Grandchild\ntext",
		},
		{
			name:    "H1 at depth 2 becomes H3",
			content: "# Grandchild\ntext",
			depth:   2,
			want:    "### Grandchild\ntext",
		},
		{
			name:    "mixed headings normalized relative to min",
			content: "## Section\ntext\n#### Subsection",
			depth:   1,
			want:    "## Section\ntext\n#### Subsection",
		},
		{
			name:    "H3 min at depth 1 shifts down to H2",
			content: "### Title\ntext\n##### Sub",
			depth:   1,
			want:    "## Title\ntext\n#### Sub",
		},
		{
			name:    "no headings returns content unchanged",
			content: "just some text\nno headings here",
			depth:   1,
			want:    "just some text\nno headings here",
		},
		{
			name:    "caps at H6",
			content: "##### Deep\n###### Deeper",
			depth:   5,
			// min=5, target=6, delta=+1 → ##### becomes ######, ###### caps at ######
			want: "###### Deep\n###### Deeper",
		},
		{
			name:    "floors at H1",
			content: "### Title\n#### Sub",
			depth:   0,
			// depth 0 wouldn't normally be called, but test the floor
			// target = 1, min = 3, delta = -2
			want: "# Title\n## Sub",
		},
		{
			name:    "user example: siblings get same level",
			content: "# Grandchild 2",
			depth:   2,
			want:    "### Grandchild 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeHeadings(tt.content, tt.depth)
			if got != tt.want {
				t.Errorf("normalizeHeadings() = %q, want %q", got, tt.want)
			}
		})
	}
}

// setupComposeTestVault creates a vault with parent-child notes for compose tests.
// Returns the vault, titles index, and children map.
func setupComposeTestVault(t *testing.T, notes []testNote) (*vault.Vault, *vault.TitlesIndex, map[string][]string) {
	t.Helper()
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	index := &vault.TitlesIndex{Titles: make(map[string]vault.TitleEntry)}

	for _, n := range notes {
		path := filepath.Join(tmpDir, n.filename)
		if err := os.WriteFile(path, []byte(n.raw), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", n.filename, err)
		}
		index.Titles[n.uuid] = vault.TitleEntry{
			Title:  n.title,
			Path:   path,
			Parent: n.parent,
		}
	}

	if err := vlt.SaveTitles(index); err != nil {
		t.Fatalf("failed to save titles: %v", err)
	}

	childrenMap := index.ChildrenMap()

	// Sort children by title for deterministic ordering
	for parent := range childrenMap {
		uuids := childrenMap[parent]
		sortChildUUIDs(vlt, index, uuids, SortField{Field: "title", Ascending: true})
	}

	return vlt, index, childrenMap
}

type testNote struct {
	uuid     string
	title    string
	filename string
	parent   string
	raw      string
}

func TestComposeTextWithSourceMap_SingleRoot(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root

Line two
Line three`,
		},
	})

	text, sm := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)

	// Content should be the note body
	wantContent := "# Root\n\nLine two\nLine three"
	if text != wantContent {
		t.Errorf("composed text = %q, want %q", text, wantContent)
	}

	// Single entry covering all lines
	if len(sm) != 1 {
		t.Fatalf("source_map length = %d, want 1", len(sm))
	}
	if sm[0].UUID != "root-1" {
		t.Errorf("source_map[0].UUID = %q, want %q", sm[0].UUID, "root-1")
	}
	if sm[0].StartLine != 1 {
		t.Errorf("source_map[0].StartLine = %d, want 1", sm[0].StartLine)
	}
	// "# Root\n\nLine two\nLine three" = 4 lines
	if sm[0].EndLine != 4 {
		t.Errorf("source_map[0].EndLine = %d, want 4", sm[0].EndLine)
	}
}

func TestComposeTextWithSourceMap_RootAndChild(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root

Root content`,
		},
		{
			uuid:     "child-1",
			title:    "Child A",
			filename: "Child A.md",
			parent:   "root-1",
			raw: `---
uuid: child-1
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: root-1
---
# Child A

Child content`,
		},
	})

	text, sm := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)

	// Root: "# Root\n\nRoot content"  = lines 1-3
	// separator: line 4 (blank)
	// Child: "## Child A\n\nChild content" = lines 5-7
	lines := strings.Split(text, "\n")
	if len(lines) != 7 {
		t.Fatalf("total lines = %d, want 7, text = %q", len(lines), text)
	}

	if len(sm) != 2 {
		t.Fatalf("source_map length = %d, want 2", len(sm))
	}

	// Root entry
	if sm[0].UUID != "root-1" {
		t.Errorf("sm[0].UUID = %q, want root-1", sm[0].UUID)
	}
	if sm[0].StartLine != 1 || sm[0].EndLine != 3 {
		t.Errorf("sm[0] range = %d-%d, want 1-3", sm[0].StartLine, sm[0].EndLine)
	}

	// Child entry
	if sm[1].UUID != "child-1" {
		t.Errorf("sm[1].UUID = %q, want child-1", sm[1].UUID)
	}
	if sm[1].StartLine != 5 || sm[1].EndLine != 7 {
		t.Errorf("sm[1] range = %d-%d, want 5-7", sm[1].StartLine, sm[1].EndLine)
	}

	// Gap: line 4 should not be covered by any entry
	covered := false
	for _, entry := range sm {
		if 4 >= entry.StartLine && 4 <= entry.EndLine {
			covered = true
		}
	}
	if covered {
		t.Error("line 4 should be a separator gap, but is covered by source_map")
	}
}

func TestComposeTextWithSourceMap_RootAndTwoChildren(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root

Root body`,
		},
		{
			uuid:     "child-a",
			title:    "Alpha",
			filename: "Alpha.md",
			parent:   "root-1",
			raw: `---
uuid: child-a
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: root-1
---
# Alpha

Alpha body`,
		},
		{
			uuid:     "child-b",
			title:    "Beta",
			filename: "Beta.md",
			parent:   "root-1",
			raw: `---
uuid: child-b
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
parent: root-1
---
# Beta

Beta body`,
		},
	})

	_, sm := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)

	if len(sm) != 3 {
		t.Fatalf("source_map length = %d, want 3", len(sm))
	}

	// Root: lines 1-3
	if sm[0].StartLine != 1 || sm[0].EndLine != 3 {
		t.Errorf("root range = %d-%d, want 1-3", sm[0].StartLine, sm[0].EndLine)
	}
	// Alpha: lines 5-7 (gap at line 4)
	if sm[1].StartLine != 5 || sm[1].EndLine != 7 {
		t.Errorf("alpha range = %d-%d, want 5-7", sm[1].StartLine, sm[1].EndLine)
	}
	// Beta: lines 9-11 (gap at line 8)
	if sm[2].StartLine != 9 || sm[2].EndLine != 11 {
		t.Errorf("beta range = %d-%d, want 9-11", sm[2].StartLine, sm[2].EndLine)
	}
}

func TestComposeTextWithSourceMap_NormalizeHeaders(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root

Root body`,
		},
		{
			uuid:     "child-1",
			title:    "Child",
			filename: "Child.md",
			parent:   "root-1",
			raw: `---
uuid: child-1
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: root-1
---
# Child

Child body`,
		},
	})

	// normalizeHeaders should not change the line count
	_, smNorm := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, true)
	_, smPlain := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)

	if len(smNorm) != len(smPlain) {
		t.Fatalf("normalize changed entry count: %d vs %d", len(smNorm), len(smPlain))
	}

	for i := range smNorm {
		if smNorm[i].StartLine != smPlain[i].StartLine || smNorm[i].EndLine != smPlain[i].EndLine {
			t.Errorf("entry %d: normalize range %d-%d != plain range %d-%d",
				i, smNorm[i].StartLine, smNorm[i].EndLine, smPlain[i].StartLine, smPlain[i].EndLine)
		}
	}
}

func TestComposeTextWithSourceMap_MappingCorrectness(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root

Line 2
Line 3`,
		},
		{
			uuid:     "child-1",
			title:    "Child",
			filename: "Child.md",
			parent:   "root-1",
			raw: `---
uuid: child-1
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: root-1
---
# Child

Child line 2
Child line 3`,
		},
	})

	text, sm := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)
	composedLines := strings.Split(text, "\n")

	// Verify mapping formula: original_content_line = (composed_line - start_line) + 1
	for _, entry := range sm {
		for composedLine := entry.StartLine; composedLine <= entry.EndLine; composedLine++ {
			originalLine := (composedLine - entry.StartLine) + 1
			if originalLine < 1 {
				t.Errorf("entry %s: composed line %d maps to invalid original line %d",
					entry.UUID, composedLine, originalLine)
			}
			// Verify the composed line index is valid
			composedIdx := composedLine - 1
			if composedIdx < 0 || composedIdx >= len(composedLines) {
				t.Errorf("entry %s: composed line %d out of range (total lines: %d)",
					entry.UUID, composedLine, len(composedLines))
			}
		}
	}
}

func TestComposeTextWithSourceMap_EmptyContent(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
`,
		},
	})

	text, sm := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)

	// Empty content (after frontmatter) should still produce a single-line entry
	if len(sm) != 1 {
		t.Fatalf("source_map length = %d, want 1", len(sm))
	}
	if sm[0].StartLine != 1 {
		t.Errorf("start_line = %d, want 1", sm[0].StartLine)
	}
	// Empty string = 1 line (the empty line)
	lineCount := strings.Count(text, "\n") + 1
	if sm[0].EndLine != lineCount {
		t.Errorf("end_line = %d, want %d", sm[0].EndLine, lineCount)
	}
}

func TestComposeJSON_WithoutContent(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root

Root body`,
		},
		{
			uuid:     "child-1",
			title:    "Child",
			filename: "Child.md",
			parent:   "root-1",
			raw: `---
uuid: child-1
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: root-1
---
# Child

Child body`,
		},
	})

	// Without includeContent, per-node content should be empty
	tree := composeJSON(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false, false)

	if tree.Content != "" {
		t.Errorf("root content should be empty without --content, got %q", tree.Content)
	}
	if len(tree.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(tree.Children))
	}
	if tree.Children[0].Content != "" {
		t.Errorf("child content should be empty without --content, got %q", tree.Children[0].Content)
	}
}

func TestComposeJSON_WithContent(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root

Root body`,
		},
		{
			uuid:     "child-1",
			title:    "Child",
			filename: "Child.md",
			parent:   "root-1",
			raw: `---
uuid: child-1
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: root-1
---
# Child

Child body`,
		},
	})

	tree := composeJSON(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false, true)

	if tree.Content == "" {
		t.Error("root content should be populated with --content")
	}
	if len(tree.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(tree.Children))
	}
	if tree.Children[0].Content == "" {
		t.Error("child content should be populated with --content")
	}
}

func TestComposeJSON_OutputShape(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root

Root body`,
		},
		{
			uuid:     "child-1",
			title:    "Child",
			filename: "Child.md",
			parent:   "root-1",
			raw: `---
uuid: child-1
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: root-1
---
# Child

Child body`,
		},
	})

	// Simulate what RunE does for --json
	tree := composeJSON(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false, false)
	composedText, sourceMap := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)
	tree.ComposedContent = composedText
	tree.SourceMap = sourceMap

	// Marshal and verify JSON shape
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// composed_content must be present
	if _, ok := raw["composed_content"]; !ok {
		t.Error("composed_content missing from JSON output")
	}

	// source_map must be present
	sm, ok := raw["source_map"]
	if !ok {
		t.Fatal("source_map missing from JSON output")
	}
	smArr, ok := sm.([]any)
	if !ok {
		t.Fatal("source_map is not an array")
	}
	if len(smArr) != 2 {
		t.Errorf("source_map has %d entries, want 2", len(smArr))
	}

	// Per-node content should be absent (omitempty)
	if _, ok := raw["content"]; ok {
		t.Error("content should be absent when --content is not used")
	}
}

func TestComposeJSON_OutputShapeWithContent(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root

Root body`,
		},
		{
			uuid:     "child-1",
			title:    "Child",
			filename: "Child.md",
			parent:   "root-1",
			raw: `---
uuid: child-1
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: root-1
---
# Child

Child body`,
		},
	})

	tree := composeJSON(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false, true)
	composedText, sourceMap := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)
	tree.ComposedContent = composedText
	tree.SourceMap = sourceMap

	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// All fields should be present
	for _, field := range []string{"uuid", "title", "path", "content", "composed_content", "source_map"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("field %q missing from JSON output", field)
		}
	}

	// Children should also have content
	children, ok := raw["children"].([]any)
	if !ok || len(children) == 0 {
		t.Fatal("children missing or empty")
	}
	child := children[0].(map[string]any)
	if _, ok := child["content"]; !ok {
		t.Error("child content missing when --content is used")
	}
}

func TestComposeSourceMapEntryFields(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root Note",
			filename: "Root Note.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root Note

Body`,
		},
	})

	_, sm := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)

	if len(sm) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(sm))
	}

	entry := sm[0]
	if entry.UUID != "root-1" {
		t.Errorf("UUID = %q, want root-1", entry.UUID)
	}
	if entry.Title != "Root Note" {
		t.Errorf("Title = %q, want Root Note", entry.Title)
	}
	if !strings.HasSuffix(entry.Path, "Root Note.md") {
		t.Errorf("Path = %q, should end with Root Note.md", entry.Path)
	}
}

func TestComposePlainTextUnchanged(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root

Root body`,
		},
		{
			uuid:     "child-1",
			title:    "Child",
			filename: "Child.md",
			parent:   "root-1",
			raw: `---
uuid: child-1
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: root-1
---
# Child

Child body`,
		},
	})

	// composeText should produce the same output as composeTextWithSourceMap
	var b strings.Builder
	composeText(vlt, index, childrenMap, "root-1", make(map[string]bool), &b, 0, 0, false, false, false)
	textOld := b.String()

	textNew, _ := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)

	if textOld != textNew {
		t.Errorf("composeText and composeTextWithSourceMap produced different output:\nold: %q\nnew: %q", textOld, textNew)
	}
}

func TestComposeTextWithSourceMap_ListSiblingsNoGap(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root`,
		},
		{
			uuid:     "child-a",
			title:    "Alpha",
			filename: "Alpha.md",
			parent:   "root-1",
			raw: `---
uuid: child-a
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: root-1
---
- item a1
- item a2`,
		},
		{
			uuid:     "child-b",
			title:    "Beta",
			filename: "Beta.md",
			parent:   "root-1",
			raw: `---
uuid: child-b
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
parent: root-1
---
- item b1
- item b2`,
		},
	})

	text, sm := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)

	// Root: "# Root" = line 1
	// separator: \n\n (blank line at line 2, because root is not list-only)
	// Alpha: "- item a1\n- item a2" = lines 3-4
	// separator: \n (NO blank line, both siblings are list-only)
	// Beta: "- item b1\n- item b2" = lines 5-6
	lines := strings.Split(text, "\n")
	if len(lines) != 6 {
		t.Fatalf("total lines = %d, want 6, text = %q", len(lines), text)
	}

	// Verify no blank line between list siblings
	want := "# Root\n\n- item a1\n- item a2\n- item b1\n- item b2"
	if text != want {
		t.Errorf("composed text = %q, want %q", text, want)
	}

	if len(sm) != 3 {
		t.Fatalf("source_map length = %d, want 3", len(sm))
	}

	// Root: line 1
	if sm[0].StartLine != 1 || sm[0].EndLine != 1 {
		t.Errorf("root range = %d-%d, want 1-1", sm[0].StartLine, sm[0].EndLine)
	}
	// Alpha: lines 3-4 (gap at line 2)
	if sm[1].StartLine != 3 || sm[1].EndLine != 4 {
		t.Errorf("alpha range = %d-%d, want 3-4", sm[1].StartLine, sm[1].EndLine)
	}
	// Beta: lines 5-6 (no gap)
	if sm[2].StartLine != 5 || sm[2].EndLine != 6 {
		t.Errorf("beta range = %d-%d, want 5-6", sm[2].StartLine, sm[2].EndLine)
	}
}

func TestComposeTextWithSourceMap_MixedSiblingsKeepGap(t *testing.T) {
	vlt, index, childrenMap := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw: `---
uuid: root-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root`,
		},
		{
			uuid:     "child-a",
			title:    "Alpha",
			filename: "Alpha.md",
			parent:   "root-1",
			raw: `---
uuid: child-a
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: root-1
---
- item a1
- item a2`,
		},
		{
			uuid:     "child-b",
			title:    "Beta",
			filename: "Beta.md",
			parent:   "root-1",
			raw: `---
uuid: child-b
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
parent: root-1
---
Some prose content here.`,
		},
	})

	text, sm := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)

	// Root: "# Root" = line 1
	// separator: \n\n (blank line 2)
	// Alpha: "- item a1\n- item a2" = lines 3-4
	// separator: \n\n (blank line 5, because Beta is not list-only)
	// Beta: "Some prose content here." = line 6
	want := "# Root\n\n- item a1\n- item a2\n\nSome prose content here."
	if text != want {
		t.Errorf("composed text = %q, want %q", text, want)
	}

	if len(sm) != 3 {
		t.Fatalf("source_map length = %d, want 3", len(sm))
	}

	// Beta starts at line 6 (with blank gap at line 5)
	if sm[2].StartLine != 6 || sm[2].EndLine != 6 {
		t.Errorf("beta range = %d-%d, want 6-6", sm[2].StartLine, sm[2].EndLine)
	}
}
