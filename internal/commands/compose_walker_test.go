package commands

import (
	"maps"
	"strings"
	"testing"
)

func TestWalkerRenderText_MatchesLegacy(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Root", filename: "Root.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Root\n\nRoot body",
		},
		{
			uuid: "child-a", title: "Alpha", filename: "Alpha.md", parent: "root-1",
			raw: "---\nuuid: child-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\nparent: root-1\n---\n# Alpha\n\nAlpha body",
		},
		{
			uuid: "child-b", title: "Beta", filename: "Beta.md", parent: "root-1",
			raw: "---\nuuid: child-b\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\nparent: root-1\n---\n# Beta\n\nBeta body",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	legacyText, legacySM := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, false)
	tree := walker.Walk("root-1", 0)
	walkerText, walkerSM := renderText(tree)

	if legacyText != walkerText {
		t.Errorf("text mismatch:\nlegacy: %q\nwalker: %q", legacyText, walkerText)
	}

	if len(legacySM) != len(walkerSM) {
		t.Fatalf("source map length: legacy=%d, walker=%d", len(legacySM), len(walkerSM))
	}
	for i := range legacySM {
		if legacySM[i] != walkerSM[i] {
			t.Errorf("source map[%d]: legacy=%+v, walker=%+v", i, legacySM[i], walkerSM[i])
		}
	}
}

func TestWalkerRenderJSON_MatchesLegacy(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Root", filename: "Root.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Root\n\nRoot body",
		},
		{
			uuid: "child-1", title: "Child", filename: "Child.md", parent: "root-1",
			raw: "---\nuuid: child-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\nparent: root-1\n---\n# Child\n\nChild body",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	legacyNode := composeJSON(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false, true)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, false)
	tree := walker.Walk("root-1", 0)
	walkerNode := renderJSON(tree, true)

	if legacyNode.UUID != walkerNode.UUID || legacyNode.Title != walkerNode.Title {
		t.Errorf("root mismatch: legacy=%s/%s, walker=%s/%s", legacyNode.UUID, legacyNode.Title, walkerNode.UUID, walkerNode.Title)
	}
	if legacyNode.Content != walkerNode.Content {
		t.Errorf("root content mismatch:\nlegacy: %q\nwalker: %q", legacyNode.Content, walkerNode.Content)
	}
	if len(legacyNode.Children) != len(walkerNode.Children) {
		t.Fatalf("children count: legacy=%d, walker=%d", len(legacyNode.Children), len(walkerNode.Children))
	}
	for i := range legacyNode.Children {
		if legacyNode.Children[i].UUID != walkerNode.Children[i].UUID {
			t.Errorf("child[%d] UUID mismatch", i)
		}
		if legacyNode.Children[i].Content != walkerNode.Children[i].Content {
			t.Errorf("child[%d] content mismatch:\nlegacy: %q\nwalker: %q", i, legacyNode.Children[i].Content, walkerNode.Children[i].Content)
		}
	}
}

func TestWalkerRenderEditList(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Root", filename: "Root.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Root\n\nRoot body",
		},
		{
			uuid: "child-1", title: "Child", filename: "Child.md", parent: "root-1",
			raw: "---\nuuid: child-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\nparent: root-1\n---\n# Child\n\nChild body",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, false)
	tree := walker.Walk("root-1", 0)
	results := renderEditList(tree)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].UUID != "root-1" {
		t.Errorf("results[0].UUID = %q, want root-1", results[0].UUID)
	}
	if results[1].UUID != "child-1" {
		t.Errorf("results[1].UUID = %q, want child-1", results[1].UUID)
	}
}

func TestWalkerRenderText_ListMerging(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Root", filename: "Root.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Root",
		},
		{
			uuid: "child-a", title: "Alpha", filename: "Alpha.md", parent: "root-1",
			raw: "---\nuuid: child-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\nparent: root-1\n---\n- item a1\n- item a2",
		},
		{
			uuid: "child-b", title: "Beta", filename: "Beta.md", parent: "root-1",
			raw: "---\nuuid: child-b\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\nparent: root-1\n---\n- item b1\n- item b2",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	legacyText, _ := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, false)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, false)
	tree := walker.Walk("root-1", 0)
	walkerText, _ := renderText(tree)

	if legacyText != walkerText {
		t.Errorf("list merging mismatch:\nlegacy: %q\nwalker: %q", legacyText, walkerText)
	}
}

func TestWalkerRenderText_NormalizeHeaders(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Root", filename: "Root.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Root\n\nRoot body",
		},
		{
			uuid: "child-1", title: "Child", filename: "Child.md", parent: "root-1",
			raw: "---\nuuid: child-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\nparent: root-1\n---\n# Child\n\nChild body",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	legacyText, legacySM := composeTextWithSourceMap(vlt, index, childrenMap, "root-1", make(map[string]bool), 0, 0, false, false, true)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, true)
	tree := walker.Walk("root-1", 0)
	walkerText, walkerSM := renderText(tree)

	if legacyText != walkerText {
		t.Errorf("normalize headers text mismatch:\nlegacy: %q\nwalker: %q", legacyText, walkerText)
	}
	if len(legacySM) != len(walkerSM) {
		t.Fatalf("source map length: legacy=%d, walker=%d", len(legacySM), len(walkerSM))
	}
}

func TestWalkerExpandEmbeds_Basic(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\nIntro\n\n![[Arch]]\n\nOutro",
		},
		{
			uuid: "arch-1", title: "Arch", filename: "Arch.md",
			raw: "---\nuuid: arch-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Arch\n\nArchitecture content",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, true)
	walker.expandEmbeds = true
	tree := walker.Walk("root-1", 0)

	if tree == nil {
		t.Fatal("tree is nil")
	}
	if len(tree.Segments) == 0 {
		t.Fatal("expected segments for embed expansion")
	}

	text, _ := renderText(tree)
	if !strings.Contains(text, "Intro") {
		t.Error("missing intro text")
	}
	if !strings.Contains(text, "Architecture content") {
		t.Error("missing embedded content")
	}
	if !strings.Contains(text, "Outro") {
		t.Error("missing outro text")
	}
	if !strings.Contains(text, "## Arch") {
		t.Error("embed heading should be adjusted to H2")
	}
}

func TestWalkerExpandEmbeds_Deduplication(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[Child A]]\n\nMore text",
		},
		{
			uuid: "child-a", title: "Child A", filename: "Child A.md", parent: "root-1",
			raw: "---\nuuid: child-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\nparent: root-1\n---\n# Child A\n\nChild A content",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, true)
	walker.expandEmbeds = true
	tree := walker.Walk("root-1", 0)

	text, _ := renderText(tree)
	count := strings.Count(text, "Child A content")
	if count != 1 {
		t.Errorf("Child A content appears %d times, want 1 (deduplication failed)", count)
	}
}

func TestWalkerExpandEmbeds_HeaderSection(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[Roadmap#Q1 Goals]]",
		},
		{
			uuid: "road-1", title: "Roadmap", filename: "Roadmap.md",
			raw: "---\nuuid: road-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Roadmap\n\n## Q1 Goals\n\n- Launch beta\n- Hire 3 engineers\n\n## Q2 Goals\n\n- Scale to 1000 users",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, true)
	walker.expandEmbeds = true
	tree := walker.Walk("root-1", 0)

	text, _ := renderText(tree)
	if !strings.Contains(text, "Launch beta") {
		t.Error("missing Q1 content")
	}
	if strings.Contains(text, "Scale to 1000") {
		t.Error("Q2 content should not be included")
	}
}

func TestWalkerExpandEmbeds_RecursiveChildren(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[Arch]]",
		},
		{
			uuid: "arch-1", title: "Arch", filename: "Arch.md",
			raw: "---\nuuid: arch-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Arch\n\nArch content",
		},
		{
			uuid: "detail-1", title: "Details", filename: "Details.md", parent: "arch-1",
			raw: "---\nuuid: detail-1\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\nparent: arch-1\n---\n# Details\n\nDetail content",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, true)
	walker.expandEmbeds = true
	tree := walker.Walk("root-1", 0)

	text, _ := renderText(tree)
	if !strings.Contains(text, "Arch content") {
		t.Error("missing embedded arch content")
	}
	if !strings.Contains(text, "Detail content") {
		t.Error("missing recursive child content")
	}
	if !strings.Contains(text, "### Details") {
		t.Error("details heading should be H3 (depth 2)")
	}
}

func TestWalkerExpandEmbeds_UnresolvableLeftAsIs(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[Missing Note]]\n\nAfter",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, false)
	walker.expandEmbeds = true
	tree := walker.Walk("root-1", 0)

	text, _ := renderText(tree)
	if !strings.Contains(text, "![[Missing Note]]") {
		t.Error("unresolvable embed should be left as-is")
	}
}

func TestWalkerExpandEmbeds_NoExpandFlag(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n![[Arch]]",
		},
		{
			uuid: "arch-1", title: "Arch", filename: "Arch.md",
			raw: "---\nuuid: arch-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Arch\n\nArch content",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, false)
	tree := walker.Walk("root-1", 0)

	text, _ := renderText(tree)
	if !strings.Contains(text, "![[Arch]]") {
		t.Error("without --expand-embeds, embed should pass through as-is")
	}
}

func TestWalkerExpandEmbeds_HeadingAdjustmentInvariant(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Hub", filename: "Hub.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Hub\n\n## Section\n\n![[Note]]\n\n### Sub",
		},
		{
			uuid: "note-1", title: "Note", filename: "Note.md",
			raw: "---\nuuid: note-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Note\n\nContent",
		},
	}

	vlt, index, childrenMap := setupComposeTestVault(t, notes)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, false)
	walker.expandEmbeds = true
	tree := walker.Walk("root-1", 0)

	text, _ := renderText(tree)
	if !strings.Contains(text, "## Section") {
		t.Error("original headings should be preserved")
	}
	if !strings.Contains(text, "### Sub") {
		t.Error("original sub-headings should be preserved")
	}
}

func TestWalkerYMLComposition(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Root", filename: "Root.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Root\n\nRoot body",
		},
		{
			uuid: "child-a", title: "Alpha", filename: "Alpha.md",
			raw: "---\nuuid: child-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Alpha\n\nAlpha body",
		},
		{
			uuid: "child-b", title: "Beta", filename: "Beta.md",
			raw: "---\nuuid: child-b\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\n---\n# Beta\n\nBeta body",
		},
	}

	vlt, index, _ := setupComposeTestVault(t, notes)

	spec := &ComposeSpec{
		Root: "Root",
		Children: []ComposeSpecEntry{
			{Note: "Beta"},
			{Note: "Alpha"},
		},
	}

	result, err := BuildChildrenMapFromSpec(spec, vlt, index)
	if err != nil {
		t.Fatal(err)
	}

	childrenMap := index.ChildrenMap()
	maps.Copy(childrenMap, result.ChildrenMap)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, true)
	tree := walker.Walk(result.RootUUID, 0)

	text, _ := renderText(tree)

	betaIdx := strings.Index(text, "Beta body")
	alphaIdx := strings.Index(text, "Alpha body")
	if betaIdx < 0 || alphaIdx < 0 {
		t.Fatal("missing content")
	}
	if betaIdx > alphaIdx {
		t.Error("YML order should put Beta before Alpha")
	}
}

func TestWalkerYMLComposition_HybridFallback(t *testing.T) {
	notes := []testNote{
		{
			uuid: "root-1", title: "Root", filename: "Root.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Root\n\nRoot body",
		},
		{
			uuid: "ch-1", title: "Chapter", filename: "Chapter.md",
			raw: "---\nuuid: ch-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Chapter\n\nChapter body",
		},
		{
			uuid: "sec-1", title: "Section", filename: "Section.md", parent: "ch-1",
			raw: "---\nuuid: sec-1\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\nparent: ch-1\n---\n# Section\n\nSection body",
		},
	}

	vlt, index, _ := setupComposeTestVault(t, notes)

	spec := &ComposeSpec{
		Root: "Root",
		Children: []ComposeSpecEntry{
			{Note: "Chapter"},
		},
	}

	result, err := BuildChildrenMapFromSpec(spec, vlt, index)
	if err != nil {
		t.Fatal(err)
	}

	childrenMap := index.ChildrenMap()
	maps.Copy(childrenMap, result.ChildrenMap)

	walker := newComposeWalker(vlt, index, childrenMap, 0, false, false, true)
	tree := walker.Walk(result.RootUUID, 0)

	text, _ := renderText(tree)
	if !strings.Contains(text, "Section body") {
		t.Error("frontmatter child (Section) should appear via hybrid fallback")
	}
}
