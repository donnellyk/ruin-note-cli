package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

func TestComputeInheritedTags_SingleParent(t *testing.T) {
	titlesIndex := &vault.TitlesIndex{
		Titles: map[string]vault.TitleEntry{
			"parent-1": {Title: "Parent", Path: "/tmp/parent.md", Parent: ""},
			"child-1":  {Title: "Child", Path: "/tmp/child.md", Parent: "parent-1"},
		},
	}

	loader := func(path string) (*note.Note, error) {
		if path == "/tmp/parent.md" {
			return &note.Note{UUID: "parent-1", Tags: []string{"#project", "#important"}}, nil
		}
		return &note.Note{}, nil
	}

	inherited := ComputeInheritedTags("child-1", titlesIndex, loader)
	if len(inherited) != 2 {
		t.Fatalf("expected 2 inherited tags, got %d: %v", len(inherited), inherited)
	}
	if inherited[0] != "#project" || inherited[1] != "#important" {
		t.Errorf("unexpected tags: %v", inherited)
	}
}

func TestComputeInheritedTags_Transitive(t *testing.T) {
	titlesIndex := &vault.TitlesIndex{
		Titles: map[string]vault.TitleEntry{
			"grandparent": {Title: "GP", Path: "/tmp/gp.md", Parent: ""},
			"parent-1":    {Title: "Parent", Path: "/tmp/parent.md", Parent: "grandparent"},
			"child-1":     {Title: "Child", Path: "/tmp/child.md", Parent: "parent-1"},
		},
	}

	loader := func(path string) (*note.Note, error) {
		switch path {
		case "/tmp/gp.md":
			return &note.Note{UUID: "grandparent", Tags: []string{"#root"}}, nil
		case "/tmp/parent.md":
			return &note.Note{UUID: "parent-1", Tags: []string{"#mid"}}, nil
		}
		return &note.Note{}, nil
	}

	inherited := ComputeInheritedTags("child-1", titlesIndex, loader)
	if len(inherited) != 2 {
		t.Fatalf("expected 2 inherited tags, got %d: %v", len(inherited), inherited)
	}
	// Parent tags come first, then grandparent
	if inherited[0] != "#mid" || inherited[1] != "#root" {
		t.Errorf("unexpected order: %v", inherited)
	}
}

func TestComputeInheritedTags_CycleDetection(t *testing.T) {
	titlesIndex := &vault.TitlesIndex{
		Titles: map[string]vault.TitleEntry{
			"a": {Title: "A", Path: "/tmp/a.md", Parent: "b"},
			"b": {Title: "B", Path: "/tmp/b.md", Parent: "a"},
		},
	}

	loader := func(path string) (*note.Note, error) {
		switch path {
		case "/tmp/a.md":
			return &note.Note{UUID: "a", Tags: []string{"#tagA"}}, nil
		case "/tmp/b.md":
			return &note.Note{UUID: "b", Tags: []string{"#tagB"}}, nil
		}
		return &note.Note{}, nil
	}

	// Should not infinite loop
	inherited := ComputeInheritedTags("a", titlesIndex, loader)
	// b is parent of a, and b's parent is a (cycle) — should get b's tags only
	if len(inherited) != 1 || inherited[0] != "#tagB" {
		t.Errorf("expected [#tagB], got %v", inherited)
	}
}

func TestComputeInheritedTags_NoParent(t *testing.T) {
	titlesIndex := &vault.TitlesIndex{
		Titles: map[string]vault.TitleEntry{
			"root": {Title: "Root", Path: "/tmp/root.md", Parent: ""},
		},
	}

	loader := func(path string) (*note.Note, error) {
		return &note.Note{UUID: "root", Tags: []string{"#tag"}}, nil
	}

	inherited := ComputeInheritedTags("root", titlesIndex, loader)
	if len(inherited) != 0 {
		t.Errorf("expected no inherited tags, got %v", inherited)
	}
}

func TestComputeInheritedTags_Deduplicated(t *testing.T) {
	titlesIndex := &vault.TitlesIndex{
		Titles: map[string]vault.TitleEntry{
			"gp":    {Title: "GP", Path: "/tmp/gp.md", Parent: ""},
			"p":     {Title: "P", Path: "/tmp/p.md", Parent: "gp"},
			"child": {Title: "C", Path: "/tmp/child.md", Parent: "p"},
		},
	}

	loader := func(path string) (*note.Note, error) {
		switch path {
		case "/tmp/gp.md":
			return &note.Note{UUID: "gp", Tags: []string{"#shared", "#gponly"}}, nil
		case "/tmp/p.md":
			return &note.Note{UUID: "p", Tags: []string{"#shared", "#ponly"}}, nil
		}
		return &note.Note{}, nil
	}

	inherited := ComputeInheritedTags("child", titlesIndex, loader)
	// #shared appears in both parent and grandparent but should only appear once
	seen := make(map[string]bool)
	for _, t := range inherited {
		if seen[t] {
			t = "duplicate: " + t
		}
		seen[t] = true
	}
	if len(inherited) != 3 {
		t.Errorf("expected 3 unique tags, got %d: %v", len(inherited), inherited)
	}
}

func TestCascadeInheritedTags(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	// Create parent note with #project tag
	parentContent := `---
uuid: parent-uuid
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#project"
---
# Parent Note
#project

Some parent content.
`
	parentPath := filepath.Join(tmpDir, "parent.md")
	if err := os.WriteFile(parentPath, []byte(parentContent), 0644); err != nil {
		t.Fatalf("failed to write parent: %v", err)
	}

	// Create child note without inherited tags
	childContent := `---
uuid: child-uuid
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
parent: parent-uuid
---
# Child Note

Some child content.
`
	childPath := filepath.Join(tmpDir, "child.md")
	if err := os.WriteFile(childPath, []byte(childContent), 0644); err != nil {
		t.Fatalf("failed to write child: %v", err)
	}

	// Set up titles index
	titlesIndex := &vault.TitlesIndex{
		Titles: map[string]vault.TitleEntry{
			"parent-uuid": {Title: "Parent Note", Path: parentPath, Parent: ""},
			"child-uuid":  {Title: "Child Note", Path: childPath, Parent: "parent-uuid"},
		},
	}
	if err := vlt.SaveTitles(titlesIndex); err != nil {
		t.Fatalf("failed to save titles: %v", err)
	}

	// Cascade
	if err := CascadeInheritedTags("parent-uuid", vlt, titlesIndex); err != nil {
		t.Fatalf("CascadeInheritedTags() error = %v", err)
	}

	// Verify child now has inherited tags
	child, err := note.Load(childPath)
	if err != nil {
		t.Fatalf("failed to reload child: %v", err)
	}
	if len(child.InheritedTags) != 1 || child.InheritedTags[0] != "#project" {
		t.Errorf("child.InheritedTags = %v, want [#project]", child.InheritedTags)
	}
}

func TestCreateNote_InheritedTags(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	// Create parent note with a global tag
	parentContent := `---
uuid: parent-uuid
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#project"
---
# Parent Note
#project

Some parent content.
`
	parentPath := filepath.Join(tmpDir, "Parent Note.md")
	if err := os.WriteFile(parentPath, []byte(parentContent), 0644); err != nil {
		t.Fatalf("failed to write parent: %v", err)
	}

	// Add parent to titles index so it can be looked up
	titlesIndex := &vault.TitlesIndex{
		Titles: map[string]vault.TitleEntry{
			"parent-uuid": {Title: "Parent Note", Path: parentPath, Parent: ""},
		},
	}
	if err := vlt.SaveTitles(titlesIndex); err != nil {
		t.Fatalf("failed to save titles: %v", err)
	}

	// Create a new child note via createNote
	child := &note.Note{
		Content: "# Child Note\n\nSome child content.\n",
		Parent:  "parent-uuid",
	}
	if err := createNote(child, vlt, "Child Note", false); err != nil {
		t.Fatalf("createNote() error = %v", err)
	}

	// Reload from disk and verify inherited tags
	reloaded, err := note.Load(child.FilePath)
	if err != nil {
		t.Fatalf("failed to reload child: %v", err)
	}
	if len(reloaded.InheritedTags) != 1 || reloaded.InheritedTags[0] != "#project" {
		t.Errorf("child inherited-tags = %v, want [#project]", reloaded.InheritedTags)
	}
}

func TestRefreshInheritedTags_DisabledClearsStale(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.SetTagInheritance(false)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	n := &note.Note{UUID: "test", Parent: "some-parent", InheritedTags: []string{"#stale"}}

	changed, err := RefreshInheritedTags(n, vlt)
	if err != nil {
		t.Fatalf("RefreshInheritedTags() error = %v", err)
	}
	if !changed {
		t.Error("expected changed=true when clearing stale inherited tags")
	}
	if len(n.InheritedTags) != 0 {
		t.Errorf("expected empty inherited tags, got %v", n.InheritedTags)
	}
}

func TestRefreshInheritedTags_DisabledNoOp(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.SetTagInheritance(false)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	n := &note.Note{UUID: "test", Parent: "some-parent"}

	changed, err := RefreshInheritedTags(n, vlt)
	if err != nil {
		t.Fatalf("RefreshInheritedTags() error = %v", err)
	}
	if changed {
		t.Error("expected changed=false when already empty and disabled")
	}
}

func TestCascadeInheritedTags_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.SetTagInheritance(false)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	// Create parent note with a tag
	parentContent := `---
uuid: parent-uuid
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#project"
---
# Parent
#project
`
	parentPath := filepath.Join(tmpDir, "parent.md")
	if err := os.WriteFile(parentPath, []byte(parentContent), 0644); err != nil {
		t.Fatalf("failed to write parent: %v", err)
	}

	// Create child note without inherited tags
	childContent := `---
uuid: child-uuid
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
parent: parent-uuid
---
# Child
`
	childPath := filepath.Join(tmpDir, "child.md")
	if err := os.WriteFile(childPath, []byte(childContent), 0644); err != nil {
		t.Fatalf("failed to write child: %v", err)
	}

	titlesIndex := &vault.TitlesIndex{
		Titles: map[string]vault.TitleEntry{
			"parent-uuid": {Title: "Parent", Path: parentPath, Parent: ""},
			"child-uuid":  {Title: "Child", Path: childPath, Parent: "parent-uuid"},
		},
	}
	if err := vlt.SaveTitles(titlesIndex); err != nil {
		t.Fatalf("failed to save titles: %v", err)
	}

	// Cascade should be a no-op
	if err := CascadeInheritedTags("parent-uuid", vlt, titlesIndex); err != nil {
		t.Fatalf("CascadeInheritedTags() error = %v", err)
	}

	// Child should NOT have inherited tags
	child, err := note.Load(childPath)
	if err != nil {
		t.Fatalf("failed to reload child: %v", err)
	}
	if len(child.InheritedTags) != 0 {
		t.Errorf("child.InheritedTags = %v, want empty (inheritance disabled)", child.InheritedTags)
	}
}

func TestCreateNote_InheritedTags_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.SetTagInheritance(false)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	// Create parent note with a global tag
	parentContent := `---
uuid: parent-uuid
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#project"
---
# Parent Note
#project

Some parent content.
`
	parentPath := filepath.Join(tmpDir, "Parent Note.md")
	if err := os.WriteFile(parentPath, []byte(parentContent), 0644); err != nil {
		t.Fatalf("failed to write parent: %v", err)
	}

	titlesIndex := &vault.TitlesIndex{
		Titles: map[string]vault.TitleEntry{
			"parent-uuid": {Title: "Parent Note", Path: parentPath, Parent: ""},
		},
	}
	if err := vlt.SaveTitles(titlesIndex); err != nil {
		t.Fatalf("failed to save titles: %v", err)
	}

	child := &note.Note{
		Content: "# Child Note\n\nSome child content.\n",
		Parent:  "parent-uuid",
	}
	if err := createNote(child, vlt, "Child Note Disabled", false); err != nil {
		t.Fatalf("createNote() error = %v", err)
	}

	reloaded, err := note.Load(child.FilePath)
	if err != nil {
		t.Fatalf("failed to reload child: %v", err)
	}
	if len(reloaded.InheritedTags) != 0 {
		t.Errorf("child inherited-tags = %v, want empty (inheritance disabled)", reloaded.InheritedTags)
	}
}

func TestRefreshInheritedTags_NoParent(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	n := &note.Note{UUID: "test", InheritedTags: []string{"#stale"}}

	changed, err := RefreshInheritedTags(n, vlt)
	if err != nil {
		t.Fatalf("RefreshInheritedTags() error = %v", err)
	}
	if !changed {
		t.Error("expected changed=true when clearing stale inherited tags")
	}
	if len(n.InheritedTags) != 0 {
		t.Errorf("expected empty inherited tags, got %v", n.InheritedTags)
	}
}
