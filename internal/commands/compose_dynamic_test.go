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
