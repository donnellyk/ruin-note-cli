package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
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

// setParentHelper uses "note set --parent" to set a parent on a note.
func setParentHelper(t *testing.T, vlt *vault.Vault, childID, parentID string) {
	t.Helper()
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", childID, "--parent", parentID, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("note set --parent error = %v", err)
	}
}

func TestNoteSet_Parent_Basic(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-child", "--parent", "uuid-root"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("note set --parent error = %v", err)
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

func TestNoteSet_Parent_SelfReference(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-root", "--parent", "uuid-root"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected self-reference error")
	}
	if got := err.Error(); !containsSubstr(got, "cannot be its own parent") {
		t.Errorf("error = %q, want self-reference error", got)
	}
}

func TestNoteSet_Parent_Cycle(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false

	// Set up chain: child -> root
	setParentHelper(t, vlt, "uuid-child", "uuid-root")

	// Set up: grandchild -> child
	setParentHelper(t, vlt, "uuid-grandchild", "uuid-child")

	// Try to create cycle: root -> grandchild
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-root", "--parent", "uuid-grandchild"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
	if got := err.Error(); !containsSubstr(got, "cycle") {
		t.Errorf("error = %q, want cycle error", got)
	}
}

func TestNoteSet_Parent_OverwriteRequiresForce(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false

	// Set initial parent
	setParentHelper(t, vlt, "uuid-child", "uuid-root")

	// Try to overwrite without --force
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-child", "--parent", "uuid-orphan"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected overwrite error without --force")
	}

	// With --force
	cmd2 := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-child", "--parent", "uuid-orphan", "--force"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("note set --parent --force error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-child")
	if n.Parent != "uuid-orphan" {
		t.Errorf("parent = %q, want %q", n.Parent, "uuid-orphan")
	}
}

func TestParentGet_WithParent(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := true

	// Set parent first via note set
	setParentHelper(t, vlt, "uuid-child", "uuid-root")

	// Get parent
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"get", "uuid-child"})
	err := cmd.Execute()

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

func TestNoteSet_NoParent(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false

	// Set parent
	setParentHelper(t, vlt, "uuid-child", "uuid-root")

	// Remove parent via note set --no-parent
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-child", "--no-parent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("note set --no-parent error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-child")
	if n.Parent != "" {
		t.Errorf("parent = %q, want empty", n.Parent)
	}
}

func TestParentChildren(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := true

	// Set up hierarchy: root -> child, root -> orphan
	setParentHelper(t, vlt, "uuid-child", "uuid-root")
	setParentHelper(t, vlt, "uuid-orphan", "uuid-root")

	// List children
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"children", "uuid-root"})
	err := cmd.Execute()

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
	setParentHelper(t, vlt, "uuid-child", "uuid-root")
	setParentHelper(t, vlt, "uuid-grandchild", "uuid-child")

	// List recursive children of root
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"children", "--recursive", "uuid-root"})
	err := cmd.Execute()

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
	setParentHelper(t, vlt, "uuid-child", "uuid-root")
	setParentHelper(t, vlt, "uuid-grandchild", "uuid-child")

	// Full forest
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"tree"})
	err := cmd.Execute()

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

func TestParentSave_Basic(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"save", "myroot", "uuid-root", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("parent save error = %v", err)
	}

	// Verify saved
	index, err := vlt.LoadParents()
	if err != nil {
		t.Fatalf("LoadParents() error = %v", err)
	}
	if len(index.Parents) != 1 {
		t.Fatalf("got %d parents, want 1", len(index.Parents))
	}
	if index.Parents[0].Name != "myroot" || index.Parents[0].UUID != "uuid-root" {
		t.Errorf("parent = %+v, want {myroot, uuid-root}", index.Parents[0])
	}
}

func TestParentSave_Upsert(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false

	// Save first
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"save", "myroot", "uuid-root", "--force"})
	cmd.Execute()

	// Upsert (change target)
	cmd2 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"save", "myroot", "uuid-child", "--force"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("parent save upsert error = %v", err)
	}

	index, _ := vlt.LoadParents()
	if len(index.Parents) != 1 {
		t.Fatalf("got %d parents, want 1 (upsert, not append)", len(index.Parents))
	}
	if index.Parents[0].UUID != "uuid-child" {
		t.Errorf("UUID = %q, want uuid-child", index.Parents[0].UUID)
	}
}

func TestParentSave_JSON(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := true
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"save", "myroot", "uuid-root", "--force"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("parent save --json error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result struct {
		Name  string `json:"name"`
		UUID  string `json:"uuid"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result.Name != "myroot" {
		t.Errorf("name = %q, want myroot", result.Name)
	}
	if result.UUID != "uuid-root" {
		t.Errorf("uuid = %q, want uuid-root", result.UUID)
	}
}

func TestParentList(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := true

	// Save a couple
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"save", "alpha", "uuid-root", "--force"})
	cmd.Execute()

	cmd2 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"save", "beta", "uuid-child", "--force"})
	cmd2.Execute()

	// List
	cmd3 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd3.SetArgs([]string{"list"})
	err := cmd3.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("parent list error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var entries []struct {
		Name  string `json:"name"`
		UUID  string `json:"uuid"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}
}

func TestParentDelete(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false

	// Save
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"save", "alpha", "uuid-root", "--force"})
	cmd.Execute()

	// Delete
	cmd2 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"delete", "alpha", "--force"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("parent delete error = %v", err)
	}

	index, _ := vlt.LoadParents()
	if len(index.Parents) != 0 {
		t.Errorf("got %d parents after delete, want 0", len(index.Parents))
	}
}

func TestParentDelete_NotFound(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := false
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"delete", "nonexistent", "--force"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !containsSubstr(err.Error(), "not found") {
		t.Errorf("error = %q, want not-found error", err.Error())
	}
}

func TestParentList_FileBookmarkTitle(t *testing.T) {
	vlt := setupParentTestVault(t)
	jsonOut := true

	// Create a compose YML file that references uuid-root as its root
	ymlContent := "root: uuid-root\nchildren:\n  - note: uuid-child\n"
	ymlPath := filepath.Join(vlt.Path, "compose.yml")
	if err := os.WriteFile(ymlPath, []byte(ymlContent), 0644); err != nil {
		t.Fatalf("failed to write compose file: %v", err)
	}

	// Save a file-based bookmark
	cmd := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"save", "mycomp", "--file", ymlPath, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("parent save error = %v", err)
	}

	// List bookmarks
	cmd2 := NewParentCmd(func() *vault.Vault { return vlt }, &jsonOut)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd2.SetArgs([]string{"list"})
	err := cmd2.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("parent list error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var entries []struct {
		Name  string `json:"name"`
		UUID  string `json:"uuid"`
		Title string `json:"title"`
		File  string `json:"file"`
	}
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, buf.String())
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Title != "Root Note" {
		t.Errorf("got title %q, want %q", entries[0].Title, "Root Note")
	}
	if entries[0].File == "" {
		t.Error("expected file field to be set")
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
