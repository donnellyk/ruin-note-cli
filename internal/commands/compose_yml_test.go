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
