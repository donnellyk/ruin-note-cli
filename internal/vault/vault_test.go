package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVault_Paths(t *testing.T) {
	v := New("/my/vault")

	if got := v.RuinDir(); got != "/my/vault/.ruin" {
		t.Errorf("RuinDir() = %q, want %q", got, "/my/vault/.ruin")
	}

	if got := v.TagsFile(); got != "/my/vault/.ruin/tags.yml" {
		t.Errorf("TagsFile() = %q, want %q", got, "/my/vault/.ruin/tags.yml")
	}

	if got := v.QueriesFile(); got != "/my/vault/.ruin/queries.yml" {
		t.Errorf("QueriesFile() = %q, want %q", got, "/my/vault/.ruin/queries.yml")
	}

	if got := v.TitlesFile(); got != "/my/vault/.ruin/titles.json" {
		t.Errorf("TitlesFile() = %q, want %q", got, "/my/vault/.ruin/titles.json")
	}

	if got := v.ParentsFile(); got != "/my/vault/.ruin/parents.yml" {
		t.Errorf("ParentsFile() = %q, want %q", got, "/my/vault/.ruin/parents.yml")
	}
}

func TestVault_IsInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	v := New(tmpDir)

	// Not initialized yet
	if v.IsInitialized() {
		t.Error("IsInitialized() = true, want false before init")
	}

	// Initialize
	if _, err := v.Initialize(false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Now initialized
	if !v.IsInitialized() {
		t.Error("IsInitialized() = false, want true after init")
	}
}

func TestVault_Initialize(t *testing.T) {
	tmpDir := t.TempDir()
	v := New(tmpDir)

	result, err := v.Initialize(false)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Check created files
	if len(result.Created) != 4 {
		t.Errorf("Created = %v, want 4 files", result.Created)
	}

	// Check .ruin directory exists
	if _, err := os.Stat(v.RuinDir()); err != nil {
		t.Errorf(".ruin directory not created: %v", err)
	}

	// Check tags.yml exists
	if _, err := os.Stat(v.TagsFile()); err != nil {
		t.Errorf("tags.yml not created: %v", err)
	}

	// Check queries.yml exists
	if _, err := os.Stat(v.QueriesFile()); err != nil {
		t.Errorf("queries.yml not created: %v", err)
	}

	// Check parents.yml exists
	if _, err := os.Stat(v.ParentsFile()); err != nil {
		t.Errorf("parents.yml not created: %v", err)
	}

	// Check titles.json exists
	if _, err := os.Stat(v.TitlesFile()); err != nil {
		t.Errorf("titles.json not created: %v", err)
	}
}

func TestVault_Initialize_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	v := New(tmpDir)

	// First init
	result1, err := v.Initialize(false)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if len(result1.Created) != 4 {
		t.Errorf("First init: Created = %d, want 4", len(result1.Created))
	}

	// Second init (without force)
	result2, err := v.Initialize(false)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if len(result2.Created) != 0 {
		t.Errorf("Second init: Created = %d, want 0", len(result2.Created))
	}
	if len(result2.Existed) != 4 {
		t.Errorf("Second init: Existed = %d, want 4", len(result2.Existed))
	}
}

func TestVault_Initialize_Force(t *testing.T) {
	tmpDir := t.TempDir()
	v := New(tmpDir)

	// First init
	if _, err := v.Initialize(false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Second init with force
	result, err := v.Initialize(true)
	if err != nil {
		t.Fatalf("Initialize(force=true) error = %v", err)
	}
	if len(result.Created) != 4 {
		t.Errorf("Force init: Created = %d, want 4", len(result.Created))
	}
}

func TestVault_ListNotes(t *testing.T) {
	tmpDir := t.TempDir()
	v := New(tmpDir)

	// Initialize vault
	if _, err := v.Initialize(false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Create some test notes
	notes := []string{"note1.md", "note2.md", "subdir/note3.md"}
	for _, note := range notes {
		path := filepath.Join(tmpDir, note)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(path, []byte("# Test"), 0644); err != nil {
			t.Fatalf("failed to create note: %v", err)
		}
	}

	// Create a non-md file (should be ignored)
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("text"), 0644); err != nil {
		t.Fatalf("failed to create txt file: %v", err)
	}

	// List notes
	found, err := v.ListNotes()
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}

	if len(found) != 3 {
		t.Errorf("ListNotes() returned %d notes, want 3", len(found))
	}

	// Verify .ruin files are not included
	for _, f := range found {
		if filepath.Base(filepath.Dir(f)) == ".ruin" {
			t.Errorf("ListNotes() included .ruin file: %s", f)
		}
	}
}

func TestVault_TagsRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	v := New(tmpDir)

	if _, err := v.Initialize(false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Create tags
	tags := &TagsIndex{
		Tags: []TagEntry{
			{Name: "#ruin", Count: 5},
			{Name: "#log", Count: 3},
		},
	}

	// Save
	if err := v.SaveTags(tags); err != nil {
		t.Fatalf("SaveTags() error = %v", err)
	}

	// Load
	loaded, err := v.LoadTags()
	if err != nil {
		t.Fatalf("LoadTags() error = %v", err)
	}

	if len(loaded.Tags) != 2 {
		t.Errorf("LoadTags() returned %d tags, want 2", len(loaded.Tags))
	}

	if loaded.Tags[0].Name != "#ruin" || loaded.Tags[0].Count != 5 {
		t.Errorf("Tags[0] = %+v, want {Name:#ruin, Count:5}", loaded.Tags[0])
	}
}

func TestVault_QueriesRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	v := New(tmpDir)

	if _, err := v.Initialize(false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Create queries
	queries := &QueriesIndex{
		Queries: []QueryEntry{
			{Name: "daily", Query: "#daily && #2025"},
		},
	}

	// Save
	if err := v.SaveQueries(queries); err != nil {
		t.Fatalf("SaveQueries() error = %v", err)
	}

	// Load
	loaded, err := v.LoadQueries()
	if err != nil {
		t.Fatalf("LoadQueries() error = %v", err)
	}

	if len(loaded.Queries) != 1 {
		t.Errorf("LoadQueries() returned %d queries, want 1", len(loaded.Queries))
	}

	if loaded.Queries[0].Name != "daily" {
		t.Errorf("Queries[0].Name = %q, want %q", loaded.Queries[0].Name, "daily")
	}
}

func TestVault_ParentsRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	v := New(tmpDir)

	if _, err := v.Initialize(false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	parents := &ParentsIndex{
		Parents: []ParentEntry{
			{Name: "alpha", UUID: "uuid-alpha"},
			{Name: "docs", UUID: "uuid-docs"},
		},
	}

	if err := v.SaveParents(parents); err != nil {
		t.Fatalf("SaveParents() error = %v", err)
	}

	loaded, err := v.LoadParents()
	if err != nil {
		t.Fatalf("LoadParents() error = %v", err)
	}

	if len(loaded.Parents) != 2 {
		t.Errorf("LoadParents() returned %d parents, want 2", len(loaded.Parents))
	}

	if loaded.Parents[0].Name != "alpha" || loaded.Parents[0].UUID != "uuid-alpha" {
		t.Errorf("Parents[0] = %+v, want {Name:alpha, UUID:uuid-alpha}", loaded.Parents[0])
	}
}

func TestVault_LookupParent(t *testing.T) {
	tmpDir := t.TempDir()
	v := New(tmpDir)

	if _, err := v.Initialize(false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	parents := &ParentsIndex{
		Parents: []ParentEntry{
			{Name: "alpha", UUID: "uuid-alpha"},
		},
	}
	v.SaveParents(parents)

	// Found
	bookmark, ok := v.LookupParent("alpha")
	if !ok || bookmark.UUID != "uuid-alpha" {
		t.Errorf("LookupParent(alpha) = %+v, %v; want UUID=uuid-alpha, true", bookmark, ok)
	}

	// Not found
	_, ok = v.LookupParent("nonexistent")
	if ok {
		t.Error("LookupParent(nonexistent) = true, want false")
	}
}

func TestParentEntry_OmitEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	v := New(tmpDir)
	if _, err := v.Initialize(false); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	parents := &ParentsIndex{
		Parents: []ParentEntry{
			{Name: "note-bm", UUID: "uuid-123"},
			{Name: "other-bm", UUID: "uuid-456"},
		},
	}
	if err := v.SaveParents(parents); err != nil {
		t.Fatalf("SaveParents() error = %v", err)
	}

	loaded, err := v.LoadParents()
	if err != nil {
		t.Fatalf("LoadParents() error = %v", err)
	}
	if len(loaded.Parents) != 2 {
		t.Fatalf("got %d parents, want 2", len(loaded.Parents))
	}

	if loaded.Parents[0].UUID != "uuid-123" {
		t.Errorf("bookmark[0] = %+v, want UUID=uuid-123", loaded.Parents[0])
	}
	if loaded.Parents[1].UUID != "uuid-456" {
		t.Errorf("bookmark[1] = %+v, want UUID=uuid-456", loaded.Parents[1])
	}
}
