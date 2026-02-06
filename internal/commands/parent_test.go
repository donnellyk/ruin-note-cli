package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kvnd/ruin-note-cli/internal/vault"
)

func setupParentTestVault(t *testing.T) *vault.Vault {
	t.Helper()

	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	notes := []struct {
		filename string
		uuid     string
		title    string
		content  string
	}{
		{
			filename: "root.md",
			uuid:     "uuid-root",
			title:    "Root Note",
			content: `---
uuid: uuid-root
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Root Note

The root.`,
		},
		{
			filename: "child.md",
			uuid:     "uuid-child",
			title:    "Child Note",
			content: `---
uuid: uuid-child
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
---
# Child Note

A child note.`,
		},
		{
			filename: "grandchild.md",
			uuid:     "uuid-grandchild",
			title:    "Grandchild Note",
			content: `---
uuid: uuid-grandchild
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
---
# Grandchild Note

A grandchild.`,
		},
		{
			filename: "orphan.md",
			uuid:     "uuid-orphan",
			title:    "Orphan Note",
			content: `---
uuid: uuid-orphan
created: "2025-01-04T10:00:00-05:00"
updated: "2025-01-04T10:00:00-05:00"
---
# Orphan Note

No parent.`,
		},
	}

	for _, n := range notes {
		path := filepath.Join(tmpDir, n.filename)
		if err := os.WriteFile(path, []byte(n.content), 0644); err != nil {
			t.Fatalf("failed to create test note: %v", err)
		}
		vlt.UpdateTitleEntry(n.uuid, n.title, path, "")
	}

	return vlt
}

func TestParentSet_Basic(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-child", "uuid-root"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("parent set error = %v", err)
	}

	// Verify parent was set
	n, _ := ResolveNote(vlt, "uuid-child")
	if n.Parent != "uuid-root" {
		t.Errorf("child parent = %q, want %q", n.Parent, "uuid-root")
	}

	// Verify titles index updated
	index, _ := vlt.LoadTitles()
	if index.Titles["uuid-child"].Parent != "uuid-root" {
		t.Errorf("titles index parent = %q, want %q", index.Titles["uuid-child"].Parent, "uuid-root")
	}
}

func TestParentSet_SelfReference(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-root", "uuid-root"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected self-reference error")
	}
	if got := err.Error(); !containsSubstr(got, "cannot be its own parent") {
		t.Errorf("error = %q, want self-reference error", got)
	}
}

func TestParentSet_Cycle(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// Set up chain: child -> root
	cmd.SetArgs([]string{"set", "uuid-child", "uuid-root"})
	cmd.Execute()

	// Set up: grandchild -> child
	cmd2 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-grandchild", "uuid-child"})
	cmd2.Execute()

	// Try to create cycle: root -> grandchild (would make root -> grandchild -> child -> root)
	cmd3 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd3.SetArgs([]string{"set", "uuid-root", "uuid-grandchild"})
	err := cmd3.Execute()
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
	if got := err.Error(); !containsSubstr(got, "cycle") {
		t.Errorf("error = %q, want cycle error", got)
	}
}

func TestParentSet_OverwriteRequiresForce(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false

	// Set initial parent
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-child", "uuid-root"})
	cmd.Execute()

	// Try to overwrite without --force
	cmd2 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-child", "uuid-orphan"})
	err := cmd2.Execute()
	if err == nil {
		t.Fatal("expected overwrite error without --force")
	}

	// With --force
	cmd3 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd3.SetArgs([]string{"set", "--force", "uuid-child", "uuid-orphan"})
	if err := cmd3.Execute(); err != nil {
		t.Fatalf("parent set --force error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-child")
	if n.Parent != "uuid-orphan" {
		t.Errorf("parent = %q, want %q", n.Parent, "uuid-orphan")
	}
}

func TestParentGet_WithParent(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := true

	// Set parent first
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-child", "uuid-root"})
	cmd.Execute()

	// Get parent
	cmd2 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd2.SetArgs([]string{"get", "uuid-child"})
	err := cmd2.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("parent get error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result struct {
		UUID  string `json:"uuid"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result.UUID != "uuid-root" {
		t.Errorf("parent UUID = %q, want %q", result.UUID, "uuid-root")
	}
}

func TestParentGet_NoParent(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"get", "uuid-root"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected no-parent error")
	}
	if !containsSubstr(err.Error(), "no parent") {
		t.Errorf("error = %q, want no-parent error", err.Error())
	}
}

func TestParentRemove(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false

	// Set parent
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-child", "uuid-root"})
	cmd.Execute()

	// Remove parent
	cmd2 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"remove", "uuid-child"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("parent remove error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-child")
	if n.Parent != "" {
		t.Errorf("parent = %q, want empty", n.Parent)
	}
}

func TestParentRemove_NoParent(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"remove", "uuid-root"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error removing parent from note with no parent")
	}
}

func TestParentChildren(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := true

	// Set up hierarchy: root -> child, root -> orphan
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-child", "uuid-root"})
	cmd.Execute()

	cmd2 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-orphan", "uuid-root", "--force"})
	cmd2.Execute()

	// List children
	cmd3 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd3.SetArgs([]string{"children", "uuid-root"})
	err := cmd3.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("parent children error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var children []childInfo
	if err := json.Unmarshal(buf.Bytes(), &children); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(children) != 2 {
		t.Errorf("found %d children, want 2", len(children))
	}
}

func TestParentChildren_Recursive(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := true

	// root -> child -> grandchild
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-child", "uuid-root"})
	cmd.Execute()

	cmd2 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-grandchild", "uuid-child"})
	cmd2.Execute()

	// List recursive children of root
	cmd3 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd3.SetArgs([]string{"children", "--recursive", "uuid-root"})
	err := cmd3.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("parent children --recursive error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var children []childInfo
	if err := json.Unmarshal(buf.Bytes(), &children); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("found %d direct children, want 1", len(children))
	}
	if len(children[0].Children) != 1 {
		t.Errorf("found %d grandchildren, want 1", len(children[0].Children))
	}
}

func TestParentTree(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false

	// Set up hierarchy
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-child", "uuid-root"})
	cmd.Execute()

	cmd2 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-grandchild", "uuid-child"})
	cmd2.Execute()

	// Full forest
	cmd3 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd3.SetArgs([]string{"tree"})
	err := cmd3.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("parent tree error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should have at least the root and orphan as top-level entries
	if output == "" {
		t.Error("tree output should not be empty")
	}
}

func TestDetectCycle(t *testing.T) {
	index := &vault.TitlesIndex{
		Titles: map[string]vault.TitleEntry{
			"a": {Title: "A", Path: "/a.md", Parent: ""},
			"b": {Title: "B", Path: "/b.md", Parent: "a"},
			"c": {Title: "C", Path: "/c.md", Parent: "b"},
		},
	}

	// No cycle: d -> c (chain: c -> b -> a)
	if err := detectCycle(index, "d", "c"); err != nil {
		t.Errorf("expected no cycle, got: %v", err)
	}

	// Cycle: a -> c (chain: c -> b -> a, and a is the child)
	if err := detectCycle(index, "a", "c"); err == nil {
		t.Error("expected cycle detection")
	}

	// No cycle: new -> a
	if err := detectCycle(index, "new", "a"); err != nil {
		t.Errorf("expected no cycle, got: %v", err)
	}
}
