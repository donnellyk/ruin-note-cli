package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

func dumpJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestWalkerExpandEmbeds_NoChildDuplication(t *testing.T) {
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
	if n := strings.Count(text, "Detail content"); n != 1 {
		t.Errorf("Detail content appears %d times in text, want 1\n%s", n, text)
	}

	json := renderJSON(tree, true)
	jsonStr := dumpJSON(t, json)
	if n := strings.Count(jsonStr, "\"detail-1\""); n != 1 {
		t.Errorf("detail-1 appears %d times in JSON node tree, want 1\n%s", n, jsonStr)
	}
}
