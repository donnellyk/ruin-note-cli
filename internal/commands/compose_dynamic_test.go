package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

// setupDynamicTestVault creates a vault with notes for dynamic embed testing.
// Returns the vault, titles index, and children map.
func setupDynamicTestVault(t *testing.T, notes []testNote) (*vault.Vault, *vault.TitlesIndex, map[string][]string) {
	t.Helper()
	return setupComposeTestVault(t, notes)
}

// walkDynamic creates a compose walker with dynamic embeds enabled and walks the root note.
func walkDynamic(vlt *vault.Vault, index *vault.TitlesIndex, childrenMap map[string][]string, rootUUID string) *composeTree {
	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, false)
	walker.expandEmbeds = true
	walker.expandDynamic = true
	walker.rootUUID = rootUUID
	return walker.Walk(rootUUID, 0)
}

// --- Dynamic Search Tests ---

func TestDynamicSearch_Content(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\ntags:\n  - \"#hub\"\n---\n# Hub\n#hub\n\n![[search: #daily]]",
		},
		{
			uuid: "note-a", title: "Day One", filename: "Day One.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day One\n#daily\n\nDay one content here.",
		},
		{
			uuid: "note-b", title: "Day Two", filename: "Day Two.md",
			raw: "---\nuuid: note-b\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day Two\n#daily\n\nDay two content here.",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	if !strings.Contains(text, "Day one content here.") {
		t.Error("expected Day One content in output")
	}
	if !strings.Contains(text, "Day two content here.") {
		t.Error("expected Day Two content in output")
	}
}

func TestDynamicSearch_List(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[search: #daily | format=list]]",
		},
		{
			uuid: "note-a", title: "Day One", filename: "Day One.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day One\n#daily\n\nContent.",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	if !strings.Contains(text, "- [[Day One]]") {
		t.Errorf("expected wiki-link bullet list, got:\n%s", text)
	}
}

func TestDynamicSearch_Summary(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[search: #daily | format=summary]]",
		},
		{
			uuid: "note-a", title: "Day One", filename: "Day One.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day One\n#daily\n\nFirst line of content.",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	if !strings.Contains(text, "Day One") {
		t.Error("expected note title in summary")
	}
	if !strings.Contains(text, "2025-01-02") {
		t.Error("expected date in summary")
	}
	if !strings.Contains(text, "First line of content.") {
		t.Error("expected first content line in summary")
	}
}

func TestDynamicSearch_Limit(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[search: #daily | limit=1, format=list]]",
		},
		{
			uuid: "note-a", title: "Day One", filename: "Day One.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day One\n#daily\n\nContent A.",
		},
		{
			uuid: "note-b", title: "Day Two", filename: "Day Two.md",
			raw: "---\nuuid: note-b\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day Two\n#daily\n\nContent B.",
		},
		{
			uuid: "note-c", title: "Day Three", filename: "Day Three.md",
			raw: "---\nuuid: note-c\ncreated: \"2025-01-04T10:00:00-05:00\"\nupdated: \"2025-01-04T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day Three\n#daily\n\nContent C.",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	// format=list produces "- [[Title]]" lines. With limit=1, only 1 should appear.
	count := strings.Count(text, "- [[")
	if count != 1 {
		t.Errorf("expected 1 list entry (limit=1), got %d in:\n%s", count, text)
	}
}

func TestDynamicSearch_ExcludesRoot(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Hub\n#daily\n\n![[search: #daily | format=list]]",
		},
		{
			uuid: "note-a", title: "Day One", filename: "Day One.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day One\n#daily\n\nContent.",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	// Root note (Hub) has #daily but should be excluded from its own search results
	if strings.Contains(text, "- [[Hub]]") {
		t.Error("root note should be excluded from search results")
	}
	if !strings.Contains(text, "- [[Day One]]") {
		t.Error("expected Day One in results")
	}
}

func TestDynamicSearch_Empty(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[search: #nonexistent]]",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	if !strings.Contains(text, "No results") {
		t.Errorf("expected 'No results' message, got:\n%s", text)
	}
}

func TestDynamicSearch_EmptyHide(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[search: #nonexistent | empty=hide]]",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	if strings.Contains(text, "No results") {
		t.Error("with empty=hide, should not show 'No results' message")
	}
	if strings.Contains(text, "#nonexistent") {
		t.Error("with empty=hide, should not show the query in output")
	}
}

// --- Dynamic Pick Tests ---

func TestDynamicPick_Grouped(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[pick: #followup]]",
		},
		{
			uuid: "note-a", title: "Meeting Notes", filename: "Meeting Notes.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#meeting\"\ninline-tags:\n  - \"#followup\"\n---\n# Meeting Notes\n#meeting\n\n- Review PR #followup\n- Send email #followup",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	// Grouped format should include the note title as a heading
	if !strings.Contains(text, "Meeting Notes") {
		t.Errorf("expected note title in grouped output, got:\n%s", text)
	}
	if !strings.Contains(text, "Review PR") {
		t.Errorf("expected 'Review PR' line, got:\n%s", text)
	}
	if !strings.Contains(text, "Send email") {
		t.Errorf("expected 'Send email' line, got:\n%s", text)
	}
}

func TestDynamicPick_Flat(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[pick: #followup | format=flat]]",
		},
		{
			uuid: "note-a", title: "Meeting Notes", filename: "Meeting Notes.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#meeting\"\ninline-tags:\n  - \"#followup\"\n---\n# Meeting Notes\n#meeting\n\n- Review PR #followup",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	// Flat format appends the note title in parentheses
	if !strings.Contains(text, "(Meeting Notes)") {
		t.Errorf("expected note title in parentheses for flat format, got:\n%s", text)
	}
}

func TestDynamicPick_Negation(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[pick: #followup !#done]]",
		},
		{
			uuid: "note-a", title: "Tasks", filename: "Tasks.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#work\"\ninline-tags:\n  - \"#followup\"\n  - \"#done\"\n---\n# Tasks\n#work\n\n- Open item #followup\n- Closed item #followup #done",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	if !strings.Contains(text, "Open item") {
		t.Errorf("expected open item, got:\n%s", text)
	}
	// Lines with #done should be excluded by the default doneExclude filter
	// and also by explicit !#done negation tag
	if strings.Contains(text, "Closed item") {
		t.Errorf("lines with #done should be excluded, got:\n%s", text)
	}
}

func TestDynamicPick_ExcludesRoot(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\ninline-tags:\n  - \"#followup\"\n---\n# Hub\n\n- Root followup #followup\n\n![[pick: #followup | format=flat]]",
		},
		{
			uuid: "note-a", title: "Other Note", filename: "Other Note.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ninline-tags:\n  - \"#followup\"\n---\n# Other Note\n\n- Other followup #followup",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	// The root note's own #followup line should not appear in pick results
	if strings.Contains(text, "(Hub)") {
		t.Errorf("root note lines should be excluded from pick results, got:\n%s", text)
	}
	if !strings.Contains(text, "(Other Note)") {
		t.Errorf("expected other note's lines in pick results, got:\n%s", text)
	}
}

// --- Dynamic Query Tests ---

func TestDynamicQuery(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[query: my-query | format=list]]",
		},
		{
			uuid: "note-a", title: "Day One", filename: "Day One.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day One\n#daily\n\nContent.",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)

	// Save a query
	queries := &vault.QueriesIndex{
		Queries: []vault.QueryEntry{
			{Name: "my-query", Query: "#daily"},
		},
	}
	if err := vlt.SaveQueries(queries); err != nil {
		t.Fatalf("failed to save queries: %v", err)
	}

	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	if !strings.Contains(text, "- [[Day One]]") {
		t.Errorf("expected query results with Day One, got:\n%s", text)
	}
}

func TestDynamicQuery_NotFound(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[query: nonexistent]]",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' message for missing query, got:\n%s", text)
	}
}

// --- Dynamic Compose Tests ---

func TestDynamicCompose(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\nIntro text\n\n![[compose: Child Note]]",
		},
		{
			uuid: "child-1", title: "Child Note", filename: "Child Note.md",
			raw: "---\nuuid: child-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Child Note\n\nChild body content.",
		},
		{
			uuid: "grandchild-1", title: "Grandchild", filename: "Grandchild.md", parent: "child-1",
			raw: "---\nuuid: grandchild-1\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\nparent: child-1\n---\n# Grandchild\n\nGrandchild body.",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	if !strings.Contains(text, "Intro text") {
		t.Error("expected intro text from root")
	}
	if !strings.Contains(text, "Child body content.") {
		t.Error("expected child note content via dynamic compose")
	}
	if !strings.Contains(text, "Grandchild body.") {
		t.Error("expected grandchild content via dynamic compose (recursive)")
	}
}

func TestDynamicCompose_OwnDepth(t *testing.T) {
	// depth=1 means sub-compose maxDepth=1: root at depth=0 proceeds,
	// direct children at depth=1 are included but THEIR children are not.
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[compose: Child Note | depth=1]]",
		},
		{
			uuid: "child-1", title: "Child Note", filename: "Child Note.md",
			raw: "---\nuuid: child-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Child Note\n\nChild body.",
		},
		{
			uuid: "grandchild-1", title: "Grandchild", filename: "Grandchild.md", parent: "child-1",
			raw: "---\nuuid: grandchild-1\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\nparent: child-1\n---\n# Grandchild\n\nGrandchild body.",
		},
		{
			uuid: "great-gc-1", title: "GreatGrandchild", filename: "GreatGrandchild.md", parent: "grandchild-1",
			raw: "---\nuuid: great-gc-1\ncreated: \"2025-01-04T10:00:00-05:00\"\nupdated: \"2025-01-04T10:00:00-05:00\"\nparent: grandchild-1\n---\n# GreatGrandchild\n\nGreat-grandchild body.",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	if !strings.Contains(text, "Child body.") {
		t.Error("expected child note content")
	}
	// depth=1: grandchild (depth=1 in sub-walker) IS included
	if !strings.Contains(text, "Grandchild body.") {
		t.Error("expected grandchild at depth=1 in sub-walker")
	}
	// depth=1: great-grandchild (depth=2 in sub-walker) should NOT be included
	if strings.Contains(text, "Great-grandchild body.") {
		t.Error("with depth=1, great-grandchild should not be included")
	}
}

// --- Mixed Static and Dynamic Tests ---

func TestDynamicMixedWithStatic(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\nBefore embeds\n\n![[search: #daily | format=list]]\n\n![[Static Note]]\n\nAfter embeds",
		},
		{
			uuid: "note-a", title: "Day One", filename: "Day One.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day One\n#daily\n\nDaily content.",
		},
		{
			uuid: "static-1", title: "Static Note", filename: "Static Note.md",
			raw: "---\nuuid: static-1\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\n---\n# Static Note\n\nStatic embedded content.",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	if !strings.Contains(text, "Before embeds") {
		t.Error("expected text before embeds")
	}
	if !strings.Contains(text, "- [[Day One]]") {
		t.Errorf("expected dynamic search results, got:\n%s", text)
	}
	if !strings.Contains(text, "Static embedded content.") {
		t.Errorf("expected static embed content, got:\n%s", text)
	}
	if !strings.Contains(text, "After embeds") {
		t.Error("expected text after embeds")
	}
}

// --- Helper for creating notes with inline tags for pick tests ---

func createPickTestNote(t *testing.T, dir string, uuid, title, filename string, tags, inlineTags []string, content string, created string) {
	t.Helper()

	var tagsYAML string
	if len(tags) > 0 {
		tagsYAML = "tags:\n"
		for _, tag := range tags {
			tagsYAML += fmt.Sprintf("  - %q\n", tag)
		}
	}

	var inlineTagsYAML string
	if len(inlineTags) > 0 {
		inlineTagsYAML = "inline-tags:\n"
		for _, tag := range inlineTags {
			inlineTagsYAML += fmt.Sprintf("  - %q\n", tag)
		}
	}

	raw := fmt.Sprintf("---\nuuid: %s\ncreated: %q\nupdated: %q\n%s%s---\n%s",
		uuid, created, created, tagsYAML, inlineTagsYAML, content)

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", filename, err)
	}
}

func TestDynamicPick_GroupByParent(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[pick: #followup | group=parent]]",
		},
		{
			uuid: "parent-a", title: "Project Alpha", filename: "Project Alpha.md",
			raw: "---\nuuid: parent-a\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Project Alpha\n\nOverview.",
		},
		{
			uuid: "child-a1", title: "Alpha Task 1", filename: "Alpha Task 1.md", parent: "parent-a",
			raw: "---\nuuid: child-a1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\nparent: parent-a\ninline-tags:\n  - \"#followup\"\n---\n# Alpha Task 1\n\n- Fix the build #followup",
		},
		{
			uuid: "child-a2", title: "Alpha Task 2", filename: "Alpha Task 2.md", parent: "parent-a",
			raw: "---\nuuid: child-a2\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\nparent: parent-a\ninline-tags:\n  - \"#followup\"\n---\n# Alpha Task 2\n\n- Update docs #followup",
		},
		{
			uuid: "orphan-1", title: "Orphan Note", filename: "Orphan Note.md",
			raw: "---\nuuid: orphan-1\ncreated: \"2025-01-04T10:00:00-05:00\"\nupdated: \"2025-01-04T10:00:00-05:00\"\ninline-tags:\n  - \"#followup\"\n---\n# Orphan Note\n\n- Standalone item #followup",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	// Both Alpha children should be grouped under "Project Alpha"
	if !strings.Contains(text, "Project Alpha") {
		t.Errorf("expected parent heading 'Project Alpha', got:\n%s", text)
	}
	if !strings.Contains(text, "Fix the build") && !strings.Contains(text, "Update docs") {
		t.Errorf("expected both alpha items, got:\n%s", text)
	}
	// Orphan should use its own title
	if !strings.Contains(text, "Orphan Note") {
		t.Errorf("expected orphan note heading, got:\n%s", text)
	}
	// Should NOT have separate headings for each alpha child
	if strings.Contains(text, "Alpha Task 1") || strings.Contains(text, "Alpha Task 2") {
		t.Errorf("should group by parent, not individual notes, got:\n%s", text)
	}
}

func TestDynamicPick_GroupByRoot(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[pick: #followup | group=root]]",
		},
		{
			uuid: "grandparent", title: "Top Project", filename: "Top Project.md",
			raw: "---\nuuid: grandparent\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Top Project\n\nOverview.",
		},
		{
			uuid: "parent-a", title: "Sub Module", filename: "Sub Module.md", parent: "grandparent",
			raw: "---\nuuid: parent-a\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\nparent: grandparent\n---\n# Sub Module\n\nDetails.",
		},
		{
			uuid: "child-a1", title: "Deep Task", filename: "Deep Task.md", parent: "parent-a",
			raw: "---\nuuid: child-a1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\nparent: parent-a\ninline-tags:\n  - \"#followup\"\n---\n# Deep Task\n\n- Deep item #followup",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	// Should group under root ancestor "Top Project", not immediate parent "Sub Module"
	if !strings.Contains(text, "Top Project") {
		t.Errorf("expected root ancestor heading 'Top Project', got:\n%s", text)
	}
	if strings.Contains(text, "Sub Module") {
		t.Errorf("should not show intermediate parent, got:\n%s", text)
	}
}

func TestDynamicPick_GroupByTag(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[pick: #todo #followup | any, group=tag]]",
		},
		{
			uuid: "note-a", title: "Work Note", filename: "Work Note.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ninline-tags:\n  - \"#todo\"\n  - \"#followup\"\n---\n# Work Note\n\n- Buy milk #todo\n- Call Alex #followup\n- Fix bug #todo #followup",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)
	// Should have separate sections for #todo and #followup
	if !strings.Contains(text, "#todo") {
		t.Errorf("expected #todo heading, got:\n%s", text)
	}
	if !strings.Contains(text, "#followup") {
		t.Errorf("expected #followup heading, got:\n%s", text)
	}
	// "Fix bug" has both tags and should appear under both
	todoIdx := strings.Index(text, "#todo")
	followupIdx := strings.Index(text, "#followup")
	if todoIdx < 0 || followupIdx < 0 {
		t.Fatal("missing tag headings")
	}
	// "Buy milk" only has #todo
	if !strings.Contains(text, "Buy milk") {
		t.Errorf("expected 'Buy milk', got:\n%s", text)
	}
	// "Call Alex" only has #followup
	if !strings.Contains(text, "Call Alex") {
		t.Errorf("expected 'Call Alex', got:\n%s", text)
	}
}

func TestDynamicPick_NormalizeHeaders(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n## Section\n\n![[pick: #followup]]",
		},
		{
			uuid: "note-a", title: "Tasks", filename: "Tasks.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#work\"\ninline-tags:\n  - \"#followup\"\n---\n# Tasks\n#work\n\n- Fix bug #followup",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)

	// Walk with normalizeHeaders enabled
	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, true)
	walker.expandEmbeds = true
	walker.expandDynamic = true
	walker.rootUUID = "root-1"
	tree := walker.Walk("root-1", 0)
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)

	// The pick embed is preceded by `## Section` (H2), so group headings
	// should nest beneath it at H3.
	if !strings.Contains(text, "### Tasks") {
		t.Errorf("expected pick group heading at ### under ## Section, got:\n%s", text)
	}
	if strings.Contains(text, "\n## Tasks\n") {
		t.Errorf("pick group heading should not be H2 when nested under H2 section, got:\n%s", text)
	}
}

func TestDynamicPick_NestedUnderMultipleSections(t *testing.T) {
	// Reproduction of the reported bug: a root note with multiple pick
	// embeds, each under its own ## heading. Groups should nest at H3.
	notes := []testNote{
		{
			uuid: "root-1", title: "Follow Up", filename: "Follow Up.md",
			raw: "---\nuuid: root-1\ncreated: \"2026-04-14T18:31:53-05:00\"\nupdated: \"2026-04-14T18:31:53-05:00\"\n---\n# Follow Ups\n\n## Ruin\n![[pick: #followup | filter=#ruin]]\n\n## Running\n![[pick: #followup | filter=#runner]]",
		},
		{
			uuid: "r-1", title: "Ruin Daily", filename: "Ruin Daily.md",
			raw: "---\nuuid: r-1\ncreated: \"2026-02-23T10:00:00-05:00\"\nupdated: \"2026-02-23T10:00:00-05:00\"\ntags:\n  - \"#ruin\"\ninline-tags:\n  - \"#followup\"\n---\n# Ruin Daily\n#ruin\n\n- Append mode #followup",
		},
		{
			uuid: "rn-1", title: "Running Notes", filename: "Running Notes.md",
			raw: "---\nuuid: rn-1\ncreated: \"2026-03-26T10:00:00-05:00\"\nupdated: \"2026-03-26T10:00:00-05:00\"\ntags:\n  - \"#runner\"\ninline-tags:\n  - \"#followup\"\n---\n# Running Notes\n#runner\n\n- Stats page #followup",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, true)
	walker.expandEmbeds = true
	walker.expandDynamic = true
	walker.rootUUID = "root-1"
	tree := walker.Walk("root-1", 0)
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)

	// Source literal `## Ruin` and `## Running` stay at H2.
	if !strings.Contains(text, "## Ruin") {
		t.Errorf("expected ## Ruin heading from source, got:\n%s", text)
	}
	if !strings.Contains(text, "## Running") {
		t.Errorf("expected ## Running heading from source, got:\n%s", text)
	}
	// Pick group headings should nest at H3 beneath each H2 section.
	if !strings.Contains(text, "### Ruin Daily") {
		t.Errorf("expected ### Ruin Daily pick group under ## Ruin, got:\n%s", text)
	}
	if !strings.Contains(text, "### Running Notes") {
		t.Errorf("expected ### Running Notes pick group under ## Running, got:\n%s", text)
	}
	// Pick group headings should NOT collide with section heading level.
	if strings.Contains(text, "\n## Ruin Daily") {
		t.Errorf("pick group should not be at H2 (sibling of section), got:\n%s", text)
	}
}

func TestDynamicSearch_NormalizeHeaders(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\ntags:\n  - \"#hub\"\n---\n# Hub\n#hub\n\n![[search: #daily | format=summary]]",
		},
		{
			uuid: "note-a", title: "Day One", filename: "Day One.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day One\n#daily\n\nDay one content.",
		},
	}

	vlt, index, childrenMap := setupDynamicTestVault(t, notes)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, true)
	walker.expandEmbeds = true
	walker.expandDynamic = true
	walker.rootUUID = "root-1"
	tree := walker.Walk("root-1", 0)
	if tree == nil {
		t.Fatal("tree is nil")
	}

	text, _ := renderText(tree)

	// Summary heading should be normalized to H2 (depth 0 → target depth+1=1 → H2 for min H1)
	if !strings.Contains(text, "## Day One") {
		t.Errorf("expected search summary heading normalized to ##, got:\n%s", text)
	}
}

// --- Source Map Completeness Tests ---

// findSourceLines returns content lines from text (1-indexed range) for an entry.
func sourceLinesFor(text string, e sourceEntry) string {
	lines := strings.Split(text, "\n")
	if e.StartLine < 1 || e.EndLine > len(lines) || e.StartLine > e.EndLine {
		return ""
	}
	return strings.Join(lines[e.StartLine-1:e.EndLine], "\n")
}

func TestSourceMap_SearchList(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[search: #daily | format=list, sort=created:asc]]",
		},
		{
			uuid: "note-a", title: "Day One", filename: "Day One.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day One\n#daily\n\nContent A.",
		},
		{
			uuid: "note-b", title: "Day Two", filename: "Day Two.md",
			raw: "---\nuuid: note-b\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day Two\n#daily\n\nContent B.",
		},
	}
	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	text, sm := renderText(tree)

	// Expect sourcemap entries for Day One and Day Two list items.
	found := map[string]string{}
	for _, e := range sm {
		if e.UUID == "note-a" || e.UUID == "note-b" {
			found[e.UUID] = sourceLinesFor(text, e)
		}
	}
	if got := found["note-a"]; got != "- [[Day One]]" {
		t.Errorf("note-a line = %q, want %q", got, "- [[Day One]]")
	}
	if got := found["note-b"]; got != "- [[Day Two]]" {
		t.Errorf("note-b line = %q, want %q", got, "- [[Day Two]]")
	}
}

func TestSourceMap_SearchSummary(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[search: #daily | format=summary, sort=created:asc]]",
		},
		{
			uuid: "note-a", title: "Day One", filename: "Day One.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day One\n#daily\n\nOne content.",
		},
		{
			uuid: "note-b", title: "Day Two", filename: "Day Two.md",
			raw: "---\nuuid: note-b\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\ntags:\n  - \"#daily\"\n---\n# Day Two\n#daily\n\nTwo content.",
		},
	}
	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	text, sm := renderText(tree)

	found := map[string]string{}
	for _, e := range sm {
		if e.UUID == "note-a" || e.UUID == "note-b" {
			found[e.UUID] = sourceLinesFor(text, e)
		}
	}
	if got := found["note-a"]; !strings.Contains(got, "Day One") || !strings.Contains(got, "One content.") {
		t.Errorf("note-a sourcemap block = %q, want it to include Day One and its first line", got)
	}
	if got := found["note-b"]; !strings.Contains(got, "Day Two") || !strings.Contains(got, "Two content.") {
		t.Errorf("note-b sourcemap block = %q, want it to include Day Two and its first line", got)
	}
}

func TestSourceMap_PickFlat(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[pick: #followup | format=flat, sort=created:asc]]",
		},
		{
			uuid: "note-a", title: "Notes A", filename: "Notes A.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ninline-tags:\n  - \"#followup\"\n---\n# Notes A\n\n- Task A1 #followup\n- Task A2 #followup",
		},
		{
			uuid: "note-b", title: "Notes B", filename: "Notes B.md",
			raw: "---\nuuid: note-b\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\ninline-tags:\n  - \"#followup\"\n---\n# Notes B\n\n- Task B1 #followup",
		},
	}
	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	text, sm := renderText(tree)

	// Each flat line must map back to its source note's UUID.
	aCount, bCount := 0, 0
	for _, e := range sm {
		line := sourceLinesFor(text, e)
		switch e.UUID {
		case "note-a":
			if !strings.Contains(line, "(Notes A)") {
				t.Errorf("note-a entry maps to %q, expected a line containing (Notes A)", line)
			}
			aCount++
		case "note-b":
			if !strings.Contains(line, "(Notes B)") {
				t.Errorf("note-b entry maps to %q, expected a line containing (Notes B)", line)
			}
			bCount++
		}
	}
	if aCount != 2 {
		t.Errorf("note-a entries = %d, want 2", aCount)
	}
	if bCount != 1 {
		t.Errorf("note-b entries = %d, want 1", bCount)
	}
}

func TestSourceMap_PickGroupedByNote(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[pick: #followup | sort=created:asc]]",
		},
		{
			uuid: "note-a", title: "Notes A", filename: "Notes A.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ninline-tags:\n  - \"#followup\"\n---\n# Notes A\n\n- Task A1 #followup",
		},
		{
			uuid: "note-b", title: "Notes B", filename: "Notes B.md",
			raw: "---\nuuid: note-b\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\ninline-tags:\n  - \"#followup\"\n---\n# Notes B\n\n- Task B1 #followup",
		},
	}
	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	text, sm := renderText(tree)

	// Heading and match lines should all map to their source note.
	aLines, bLines := []string{}, []string{}
	for _, e := range sm {
		line := sourceLinesFor(text, e)
		switch e.UUID {
		case "note-a":
			aLines = append(aLines, line)
		case "note-b":
			bLines = append(bLines, line)
		}
	}
	// Expect 2 entries for each note: heading + one match line.
	if len(aLines) != 2 {
		t.Errorf("note-a entries = %d lines = %v, want 2", len(aLines), aLines)
	}
	if len(bLines) != 2 {
		t.Errorf("note-b entries = %d lines = %v, want 2", len(bLines), bLines)
	}
}

func TestSourceMap_PickGroupedByParent(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[pick: #followup | group=parent, sort=created:asc]]",
		},
		{
			uuid: "parent-a", title: "Project Alpha", filename: "Project Alpha.md",
			raw: "---\nuuid: parent-a\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Project Alpha\n\nOverview.",
		},
		{
			uuid: "child-1", title: "Task 1", filename: "Task 1.md", parent: "parent-a",
			raw: "---\nuuid: child-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\nparent: parent-a\ninline-tags:\n  - \"#followup\"\n---\n# Task 1\n\n- Do thing 1 #followup",
		},
		{
			uuid: "child-2", title: "Task 2", filename: "Task 2.md", parent: "parent-a",
			raw: "---\nuuid: child-2\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\nparent: parent-a\ninline-tags:\n  - \"#followup\"\n---\n# Task 2\n\n- Do thing 2 #followup",
		},
	}
	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	text, sm := renderText(tree)

	// Heading should map to parent-a; match lines should map to their source notes.
	parentHeading, child1, child2 := "", "", ""
	for _, e := range sm {
		line := sourceLinesFor(text, e)
		switch e.UUID {
		case "parent-a":
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				parentHeading = line
			}
		case "child-1":
			child1 = line
		case "child-2":
			child2 = line
		}
	}
	if !strings.Contains(parentHeading, "Project Alpha") {
		t.Errorf("parent-a heading entry = %q, want Project Alpha heading", parentHeading)
	}
	if !strings.Contains(child1, "Do thing 1") {
		t.Errorf("child-1 entry = %q, want its match line", child1)
	}
	if !strings.Contains(child2, "Do thing 2") {
		t.Errorf("child-2 entry = %q, want its match line", child2)
	}
}

func TestSourceMap_PickGroupedByTag(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[pick: #todo #followup | any, group=tag]]",
		},
		{
			uuid: "note-a", title: "Notes A", filename: "Notes A.md",
			raw: "---\nuuid: note-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\ninline-tags:\n  - \"#todo\"\n---\n# Notes A\n\n- Buy milk #todo",
		},
		{
			uuid: "note-b", title: "Notes B", filename: "Notes B.md",
			raw: "---\nuuid: note-b\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\ninline-tags:\n  - \"#followup\"\n---\n# Notes B\n\n- Call Alex #followup",
		},
	}
	vlt, index, childrenMap := setupDynamicTestVault(t, notes)
	tree := walkDynamic(vlt, index, childrenMap, "root-1")
	text, sm := renderText(tree)

	// Match lines must map to their source notes, even though the group
	// heading (tag) has no UUID.
	aOk, bOk := false, false
	for _, e := range sm {
		line := sourceLinesFor(text, e)
		if e.UUID == "note-a" && strings.Contains(line, "Buy milk") {
			aOk = true
		}
		if e.UUID == "note-b" && strings.Contains(line, "Call Alex") {
			bOk = true
		}
	}
	if !aOk {
		t.Errorf("note-a match line missing from sourcemap, text:\n%s\nentries: %+v", text, sm)
	}
	if !bOk {
		t.Errorf("note-b match line missing from sourcemap, text:\n%s\nentries: %+v", text, sm)
	}
}
