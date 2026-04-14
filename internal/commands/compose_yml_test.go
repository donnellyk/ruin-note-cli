package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseComposeFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		check   func(*testing.T, *ComposeSpec)
	}{
		{
			name: "basic spec",
			content: `root: "Project Hub"
children:
  - note: "Chapter 1"
  - note: "Chapter 2"
`,
			check: func(t *testing.T, spec *ComposeSpec) {
				if spec.Root != "Project Hub" {
					t.Errorf("Root = %q, want %q", spec.Root, "Project Hub")
				}
				if len(spec.Children) != 2 {
					t.Fatalf("len(Children) = %d, want 2", len(spec.Children))
				}
				if spec.Children[0].Note != "Chapter 1" {
					t.Errorf("Children[0].Note = %q, want %q", spec.Children[0].Note, "Chapter 1")
				}
			},
		},
		{
			name: "nested children",
			content: `root: "Root"
children:
  - note: "Parent"
    children:
      - note: "Child A"
      - note: "Child B"
`,
			check: func(t *testing.T, spec *ComposeSpec) {
				if len(spec.Children[0].Children) != 2 {
					t.Fatalf("nested children = %d, want 2", len(spec.Children[0].Children))
				}
			},
		},
		{
			name: "root only",
			content: `root: "Just Root"
`,
			check: func(t *testing.T, spec *ComposeSpec) {
				if spec.Root != "Just Root" {
					t.Errorf("Root = %q", spec.Root)
				}
				if len(spec.Children) != 0 {
					t.Errorf("expected no children")
				}
			},
		},
		{
			name: "search entry",
			content: `root: "Project"
children:
  - note: "Intro"
  - search: "#daily"
    format: list
    limit: 5
  - note: "Conclusion"
`,
			check: func(t *testing.T, spec *ComposeSpec) {
				if len(spec.Children) != 3 {
					t.Fatalf("len(Children) = %d, want 3", len(spec.Children))
				}
				if spec.Children[1].Search != "#daily" {
					t.Errorf("Children[1].Search = %q, want %q", spec.Children[1].Search, "#daily")
				}
				if spec.Children[1].Format != "list" {
					t.Errorf("Children[1].Format = %q, want %q", spec.Children[1].Format, "list")
				}
				if spec.Children[1].Limit != 5 {
					t.Errorf("Children[1].Limit = %d, want 5", spec.Children[1].Limit)
				}
			},
		},
		{
			name: "pick entry",
			content: `root: "Project"
children:
  - pick: "#followup"
    format: flat
    sort: "created:desc"
`,
			check: func(t *testing.T, spec *ComposeSpec) {
				if len(spec.Children) != 1 {
					t.Fatalf("len(Children) = %d, want 1", len(spec.Children))
				}
				if spec.Children[0].Pick != "#followup" {
					t.Errorf("Children[0].Pick = %q, want %q", spec.Children[0].Pick, "#followup")
				}
				if spec.Children[0].Format != "flat" {
					t.Errorf("Children[0].Format = %q, want %q", spec.Children[0].Format, "flat")
				}
				if spec.Children[0].Sort != "created:desc" {
					t.Errorf("Children[0].Sort = %q, want %q", spec.Children[0].Sort, "created:desc")
				}
			},
		},
		{
			name:    "missing root",
			content: `children:\n  - note: "orphan"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "compose.yml")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}
			spec, err := ParseComposeFile(tmpFile)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.check != nil {
				tt.check(t, spec)
			}
		})
	}
}

func TestBuildChildrenMapFromSpec(t *testing.T) {
	vlt, index, _ := setupComposeTestVault(t, []testNote{
		{
			uuid:     "root-1",
			title:    "Root",
			filename: "Root.md",
			raw:      "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Root\n\nRoot body",
		},
		{
			uuid:     "child-a",
			title:    "Alpha",
			filename: "Alpha.md",
			raw:      "---\nuuid: child-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Alpha\n\nAlpha body",
		},
		{
			uuid:     "child-b",
			title:    "Beta",
			filename: "Beta.md",
			raw:      "---\nuuid: child-b\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\n---\n# Beta\n\nBeta body",
		},
	})
	_ = index

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

	if result.RootUUID != "root-1" {
		t.Errorf("RootUUID = %q, want %q", result.RootUUID, "root-1")
	}

	children := result.ChildrenMap["root-1"]
	if len(children) != 2 {
		t.Fatalf("children count = %d, want 2", len(children))
	}
	if children[0] != "child-b" {
		t.Errorf("children[0] = %q, want child-b", children[0])
	}
	if children[1] != "child-a" {
		t.Errorf("children[1] = %q, want child-a", children[1])
	}

	if !result.YMLParents["root-1"] {
		t.Error("root-1 should be in YMLParents")
	}
}

func TestBuildChildrenMapFromSpec_Nested(t *testing.T) {
	vlt, index, _ := setupComposeTestVault(t, []testNote{
		{
			uuid: "root-1", title: "Root", filename: "Root.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Root",
		},
		{
			uuid: "ch-1", title: "Chapter 1", filename: "Chapter 1.md",
			raw: "---\nuuid: ch-1\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Chapter 1",
		},
		{
			uuid: "sec-1", title: "Section 1.1", filename: "Section 1.1.md",
			raw: "---\nuuid: sec-1\ncreated: \"2025-01-03T10:00:00-05:00\"\nupdated: \"2025-01-03T10:00:00-05:00\"\n---\n# Section 1.1",
		},
	})
	_ = index

	spec := &ComposeSpec{
		Root: "Root",
		Children: []ComposeSpecEntry{
			{
				Note: "Chapter 1",
				Children: []ComposeSpecEntry{
					{Note: "Section 1.1"},
				},
			},
		},
	}

	result, err := BuildChildrenMapFromSpec(spec, vlt, index)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.ChildrenMap["root-1"]) != 1 {
		t.Fatalf("root children = %d, want 1", len(result.ChildrenMap["root-1"]))
	}
	if len(result.ChildrenMap["ch-1"]) != 1 {
		t.Fatalf("chapter children = %d, want 1", len(result.ChildrenMap["ch-1"]))
	}
	if result.ChildrenMap["ch-1"][0] != "sec-1" {
		t.Errorf("chapter child = %q, want sec-1", result.ChildrenMap["ch-1"][0])
	}
	if !result.YMLParents["ch-1"] {
		t.Error("ch-1 should be in YMLParents")
	}
}

func TestBuildChildrenMapFromSpec_DynamicSearch(t *testing.T) {
	vlt, index, _ := setupComposeTestVault(t, []testNote{
		{
			uuid: "root-1", title: "Root", filename: "Root.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Root\n\nRoot body",
		},
		{
			uuid: "child-a", title: "Alpha", filename: "Alpha.md",
			raw: "---\nuuid: child-a\ncreated: \"2025-01-02T10:00:00-05:00\"\nupdated: \"2025-01-02T10:00:00-05:00\"\n---\n# Alpha\n\nAlpha body",
		},
	})
	_ = index

	spec := &ComposeSpec{
		Root: "Root",
		Children: []ComposeSpecEntry{
			{Note: "Alpha"},
			{Search: "#daily", Format: "list", Limit: 3},
		},
	}

	result, err := BuildChildrenMapFromSpec(spec, vlt, index)
	if err != nil {
		t.Fatal(err)
	}

	// Note children should only include Alpha
	children := result.ChildrenMap["root-1"]
	if len(children) != 1 {
		t.Fatalf("children count = %d, want 1 (search not a child)", len(children))
	}
	if children[0] != "child-a" {
		t.Errorf("children[0] = %q, want child-a", children[0])
	}

	// Dynamic entries should have the search
	dynEntries := result.DynamicEntries["root-1"]
	if len(dynEntries) != 1 {
		t.Fatalf("dynamic entries count = %d, want 1", len(dynEntries))
	}
	if dynEntries[0].Type != "search" {
		t.Errorf("dynamic type = %q, want search", dynEntries[0].Type)
	}
	if dynEntries[0].Query != "#daily" {
		t.Errorf("dynamic query = %q, want #daily", dynEntries[0].Query)
	}
	if dynEntries[0].Options["format"] != "list" {
		t.Errorf("dynamic format = %q, want list", dynEntries[0].Options["format"])
	}
	if dynEntries[0].Options["limit"] != "3" {
		t.Errorf("dynamic limit = %q, want 3", dynEntries[0].Options["limit"])
	}
}

func TestBuildChildrenMapFromSpec_DynamicPick(t *testing.T) {
	vlt, index, _ := setupComposeTestVault(t, []testNote{
		{
			uuid: "root-1", title: "Root", filename: "Root.md",
			raw: "---\nuuid: root-1\ncreated: \"2025-01-01T10:00:00-05:00\"\nupdated: \"2025-01-01T10:00:00-05:00\"\n---\n# Root\n\nRoot body",
		},
	})
	_ = index

	spec := &ComposeSpec{
		Root: "Root",
		Children: []ComposeSpecEntry{
			{Pick: "#followup", Format: "flat", Sort: "title"},
		},
	}

	result, err := BuildChildrenMapFromSpec(spec, vlt, index)
	if err != nil {
		t.Fatal(err)
	}

	// No note children
	children := result.ChildrenMap["root-1"]
	if len(children) != 0 {
		t.Fatalf("children count = %d, want 0", len(children))
	}

	// Dynamic entries should have the pick
	dynEntries := result.DynamicEntries["root-1"]
	if len(dynEntries) != 1 {
		t.Fatalf("dynamic entries count = %d, want 1", len(dynEntries))
	}
	if dynEntries[0].Type != "pick" {
		t.Errorf("dynamic type = %q, want pick", dynEntries[0].Type)
	}
	if dynEntries[0].Query != "#followup" {
		t.Errorf("dynamic query = %q, want #followup", dynEntries[0].Query)
	}
}

func TestResolveComposeFilePath(t *testing.T) {
	tests := []struct {
		name      string
		filePath  string
		vaultPath string
		want      string
	}{
		{
			name:      "file inside vault",
			filePath:  "/vault/compose.yml",
			vaultPath: "/vault",
			want:      "compose.yml",
		},
		{
			name:      "file in subdirectory",
			filePath:  "/vault/specs/compose.yml",
			vaultPath: "/vault",
			want:      "specs/compose.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveComposeFilePath(tt.filePath, tt.vaultPath)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("ResolveComposeFilePath() = %q, want %q", got, tt.want)
			}
		})
	}
}
