package commands

import (
	"os"
	"path/filepath"
	"testing"

	"kvnd/ruin-note-cli/internal/vault"
)

func setupResolveTestVault(t *testing.T) *vault.Vault {
	t.Helper()

	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	notes := []struct {
		filename string
		uuid     string
		content  string
	}{
		{
			filename: "Meeting-Notes.md",
			uuid:     "uuid-meet",
			content: `---
uuid: uuid-meet
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Meeting Notes

Some meeting content.`,
		},
		{
			filename: "Project-Alpha.md",
			uuid:     "uuid-alpha",
			content: `---
uuid: uuid-alpha
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
---
# Project Alpha

Alpha project details.`,
		},
		{
			filename: "Project-Beta.md",
			uuid:     "uuid-beta",
			content: `---
uuid: uuid-beta
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
---
# Project Beta

Beta project details.`,
		},
	}

	for _, n := range notes {
		path := filepath.Join(tmpDir, n.filename)
		if err := os.WriteFile(path, []byte(n.content), 0644); err != nil {
			t.Fatalf("failed to create test note: %v", err)
		}
		// Update titles index
		vlt.UpdateTitleEntry(n.uuid, "", path, "")
	}

	// Re-update with titles (simulating doctor)
	vlt.UpdateTitleEntry("uuid-meet", "Meeting Notes", filepath.Join(tmpDir, "Meeting-Notes.md"), "")
	vlt.UpdateTitleEntry("uuid-alpha", "Project Alpha", filepath.Join(tmpDir, "Project-Alpha.md"), "")
	vlt.UpdateTitleEntry("uuid-beta", "Project Beta", filepath.Join(tmpDir, "Project-Beta.md"), "")

	return vlt
}

func TestResolveNote_ExactUUID(t *testing.T) {
	vlt := setupResolveTestVault(t)

	n, err := ResolveNote(vlt, "uuid-alpha")
	if err != nil {
		t.Fatalf("ResolveNote() error = %v", err)
	}
	if n.UUID != "uuid-alpha" {
		t.Errorf("UUID = %q, want %q", n.UUID, "uuid-alpha")
	}
}

func TestResolveNote_TitleSubstring(t *testing.T) {
	vlt := setupResolveTestVault(t)

	n, err := ResolveNote(vlt, "Meeting")
	if err != nil {
		t.Fatalf("ResolveNote() error = %v", err)
	}
	if n.UUID != "uuid-meet" {
		t.Errorf("UUID = %q, want %q", n.UUID, "uuid-meet")
	}
}

func TestResolveNote_PathSubstring(t *testing.T) {
	vlt := setupResolveTestVault(t)

	n, err := ResolveNote(vlt, "Project-Alpha")
	if err != nil {
		t.Fatalf("ResolveNote() error = %v", err)
	}
	if n.UUID != "uuid-alpha" {
		t.Errorf("UUID = %q, want %q", n.UUID, "uuid-alpha")
	}
}

func TestResolveNote_Ambiguous(t *testing.T) {
	vlt := setupResolveTestVault(t)

	_, err := ResolveNote(vlt, "Project")
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	if got := err.Error(); !contains(got, "ambiguous") {
		t.Errorf("error = %q, want to contain 'ambiguous'", got)
	}
}

func TestResolveNote_SavedParentBookmark(t *testing.T) {
	vlt := setupResolveTestVault(t)

	// Save a bookmark
	parents := &vault.ParentsIndex{
		Parents: []vault.ParentEntry{
			{Name: "alpha", UUID: "uuid-alpha"},
		},
	}
	if err := vlt.SaveParents(parents); err != nil {
		t.Fatalf("SaveParents() error = %v", err)
	}

	// Resolve by bookmark name
	n, err := ResolveNote(vlt, "alpha")
	if err != nil {
		t.Fatalf("ResolveNote() error = %v", err)
	}
	if n.UUID != "uuid-alpha" {
		t.Errorf("UUID = %q, want %q", n.UUID, "uuid-alpha")
	}
}

func TestResolveNote_BookmarkTakesPrecedence(t *testing.T) {
	vlt := setupResolveTestVault(t)

	// Save a bookmark named "Meeting" that points to alpha
	parents := &vault.ParentsIndex{
		Parents: []vault.ParentEntry{
			{Name: "Meeting", UUID: "uuid-alpha"},
		},
	}
	vlt.SaveParents(parents)

	// "Meeting" would match "Meeting Notes" by title, but bookmark should win
	n, err := ResolveNote(vlt, "Meeting")
	if err != nil {
		t.Fatalf("ResolveNote() error = %v", err)
	}
	if n.UUID != "uuid-alpha" {
		t.Errorf("UUID = %q, want %q (bookmark should take precedence)", n.UUID, "uuid-alpha")
	}
}

func TestResolveNote_NotFound(t *testing.T) {
	vlt := setupResolveTestVault(t)

	_, err := ResolveNote(vlt, "nonexistent-thing")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if got := err.Error(); !contains(got, "no note found") {
		t.Errorf("error = %q, want to contain 'no note found'", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
