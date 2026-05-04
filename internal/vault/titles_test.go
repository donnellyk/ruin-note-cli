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

func TestTitlesIndex_FindByAlias(t *testing.T) {
	idx := &TitlesIndex{
		Titles: map[string]TitleEntry{
			"uuid-1": {Title: "Note A", Aliases: []string{"OldName", "AnotherName"}},
			"uuid-2": {Title: "Note B", Aliases: []string{"Different"}},
			"uuid-3": {Title: "Note C", Aliases: nil},
		},
	}

	tests := []struct {
		name    string
		alias   string
		wantUUID string
		wantOK  bool
	}{
		{"exact match", "OldName", "uuid-1", true},
		{"case insensitive", "oldname", "uuid-1", true},
		{"with spaces", "  OldName  ", "uuid-1", true},
		{"different alias", "Different", "uuid-2", true},
		{"not found", "NonExistent", "", false},
		{"empty string", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid, ok := idx.FindByAlias(tt.alias)
			if ok != tt.wantOK || (ok && uuid != tt.wantUUID) {
				t.Errorf("FindByAlias(%q) = (%q, %v), want (%q, %v)", tt.alias, uuid, ok, tt.wantUUID, tt.wantOK)
			}
		})
	}
}

func TestTitlesIndex_FindByTitle_WithAliasFallback(t *testing.T) {
	idx := &TitlesIndex{
		Titles: map[string]TitleEntry{
			"uuid-1": {Title: "Main Title", Aliases: []string{"Alias1"}},
			"uuid-2": {Title: "Another", Aliases: []string{"Alias2"}},
		},
	}

	tests := []struct {
		name    string
		query   string
		wantUUID string
		wantOK  bool
	}{
		{"title match", "Main Title", "uuid-1", true},
		{"title case insensitive", "main title", "uuid-1", true},
		{"alias fallback", "Alias1", "uuid-1", true},
		{"alias case insensitive", "alias1", "uuid-1", true},
		{"title takes precedence", "Another", "uuid-2", true},
		{"not found", "Unknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid, ok := idx.FindByTitle(tt.query)
			if ok != tt.wantOK || (ok && uuid != tt.wantUUID) {
				t.Errorf("FindByTitle(%q) = (%q, %v), want (%q, %v)", tt.query, uuid, ok, tt.wantUUID, tt.wantOK)
			}
		})
	}
}

func TestMakeTitleEntryWithAliases(t *testing.T) {
	entry := MakeTitleEntryWithAliases("Test Note", "/test.md", "parent-uuid", []string{"tag1", "tag2"}, []string{"itag1"}, nil, []string{"Alias1", "Alias2"})

	if entry.Title != "Test Note" {
		t.Errorf("Title = %q, want %q", entry.Title, "Test Note")
	}
	if entry.Path != "/test.md" {
		t.Errorf("Path = %q, want %q", entry.Path, "/test.md")
	}
	if entry.Parent != "parent-uuid" {
		t.Errorf("Parent = %q, want %q", entry.Parent, "parent-uuid")
	}
	if len(entry.Aliases) != 2 || entry.Aliases[0] != "Alias1" {
		t.Errorf("Aliases = %v, want [Alias1 Alias2]", entry.Aliases)
	}
}
