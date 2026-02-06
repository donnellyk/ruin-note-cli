package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTitlesIndex_CRUD(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := New(tmpDir)
	os.MkdirAll(vlt.RuinDir(), 0755)

	// Load from nonexistent file returns empty
	idx, err := vlt.LoadTitles()
	if err != nil {
		t.Fatalf("LoadTitles() error = %v", err)
	}
	if len(idx.Titles) != 0 {
		t.Errorf("expected empty titles, got %d", len(idx.Titles))
	}

	// UpdateTitleEntry
	err = vlt.UpdateTitleEntry("uuid-1", "Note One", filepath.Join(tmpDir, "note1.md"), "")
	if err != nil {
		t.Fatalf("UpdateTitleEntry() error = %v", err)
	}

	err = vlt.UpdateTitleEntry("uuid-2", "Note Two", filepath.Join(tmpDir, "note2.md"), "uuid-1")
	if err != nil {
		t.Fatalf("UpdateTitleEntry() error = %v", err)
	}

	// Reload and verify
	idx, err = vlt.LoadTitles()
	if err != nil {
		t.Fatalf("LoadTitles() error = %v", err)
	}
	if len(idx.Titles) != 2 {
		t.Fatalf("expected 2 titles, got %d", len(idx.Titles))
	}
	if idx.Titles["uuid-1"].Title != "Note One" {
		t.Errorf("uuid-1 title = %q, want %q", idx.Titles["uuid-1"].Title, "Note One")
	}
	if idx.Titles["uuid-2"].Parent != "uuid-1" {
		t.Errorf("uuid-2 parent = %q, want %q", idx.Titles["uuid-2"].Parent, "uuid-1")
	}

	// RemoveTitleEntry
	err = vlt.RemoveTitleEntry("uuid-1")
	if err != nil {
		t.Fatalf("RemoveTitleEntry() error = %v", err)
	}

	idx, err = vlt.LoadTitles()
	if err != nil {
		t.Fatalf("LoadTitles() error = %v", err)
	}
	if len(idx.Titles) != 1 {
		t.Errorf("expected 1 title after remove, got %d", len(idx.Titles))
	}
	if _, ok := idx.Titles["uuid-1"]; ok {
		t.Error("uuid-1 should have been removed")
	}
}

func TestTitlesIndex_Rebuild(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := New(tmpDir)
	os.MkdirAll(vlt.RuinDir(), 0755)

	entries := map[string]TitleEntry{
		"uuid-a": {Title: "Alpha", Path: "/alpha.md"},
		"uuid-b": {Title: "Beta", Path: "/beta.md", Parent: "uuid-a"},
	}

	err := vlt.RebuildTitlesIndex(entries)
	if err != nil {
		t.Fatalf("RebuildTitlesIndex() error = %v", err)
	}

	idx, err := vlt.LoadTitles()
	if err != nil {
		t.Fatalf("LoadTitles() error = %v", err)
	}
	if len(idx.Titles) != 2 {
		t.Errorf("expected 2 titles, got %d", len(idx.Titles))
	}
	if idx.Titles["uuid-b"].Parent != "uuid-a" {
		t.Errorf("uuid-b parent = %q, want %q", idx.Titles["uuid-b"].Parent, "uuid-a")
	}
}

func TestTitlesIndex_Upsert(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := New(tmpDir)
	os.MkdirAll(vlt.RuinDir(), 0755)

	// Create entry
	vlt.UpdateTitleEntry("uuid-1", "Original", "/original.md", "")

	// Update same UUID
	vlt.UpdateTitleEntry("uuid-1", "Updated", "/updated.md", "uuid-parent")

	idx, _ := vlt.LoadTitles()
	if idx.Titles["uuid-1"].Title != "Updated" {
		t.Errorf("title = %q, want %q", idx.Titles["uuid-1"].Title, "Updated")
	}
	if idx.Titles["uuid-1"].Parent != "uuid-parent" {
		t.Errorf("parent = %q, want %q", idx.Titles["uuid-1"].Parent, "uuid-parent")
	}
}
