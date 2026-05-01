package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

// frontmatterHasKey is a thin test wrapper around note.HasFrontmatterKey
// that reads the file from disk.
func frontmatterHasKey(t *testing.T, path, key string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return note.HasFrontmatterKey(string(data), key)
}

// TestTagFrontmatter_Default_EmitsBothKeys — vault save with default flag
// emits tags: and inline-tags:.
func TestTagFrontmatter_Default_EmitsBothKeys(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	n, err := note.Parse(`# Daily
#daily

A line with #followup.
`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	n.UUID = "fm-1"
	n.FilePath = filepath.Join(tmpDir, "Daily.md")
	if err := saveNoteForVault(n, vlt); err != nil {
		t.Fatalf("save: %v", err)
	}

	if !frontmatterHasKey(t, n.FilePath, "tags") {
		t.Errorf("default flag should emit tags:")
	}
	if !frontmatterHasKey(t, n.FilePath, "inline-tags") {
		t.Errorf("default flag should emit inline-tags:")
	}
}

// TestTagFrontmatter_Off_OmitsBothKeys — vault save with flag=false omits
// both tags: and inline-tags: but keeps inherited-tags:.
func TestTagFrontmatter_Off_OmitsBothKeys(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.SetTagFrontmatter(false)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	n, err := note.Parse(`# Daily
#daily

A line with #followup.
`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	n.UUID = "fm-2"
	n.InheritedTags = []string{"project"}
	n.FilePath = filepath.Join(tmpDir, "Daily.md")
	if err := saveNoteForVault(n, vlt); err != nil {
		t.Fatalf("save: %v", err)
	}

	if frontmatterHasKey(t, n.FilePath, "tags") {
		t.Errorf("flag=false should omit tags:")
	}
	if frontmatterHasKey(t, n.FilePath, "inline-tags") {
		t.Errorf("flag=false should omit inline-tags:")
	}
	if !frontmatterHasKey(t, n.FilePath, "inherited-tags") {
		t.Errorf("flag=false should still emit inherited-tags:")
	}
}

// TestTagFrontmatter_FlipStripsKeys — flipping the flag from true to false
// and re-saving strips existing tags:/inline-tags: keys without touching
// other frontmatter.
func TestTagFrontmatter_FlipStripsKeys(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	n, err := note.Parse(`# Daily
#daily

A line with #followup.
`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	n.UUID = "fm-3"
	n.FilePath = filepath.Join(tmpDir, "Daily.md")

	// First save with flag=true.
	if err := saveNoteForVault(n, vlt); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if !frontmatterHasKey(t, n.FilePath, "inline-tags") {
		t.Fatalf("first save should have emitted inline-tags:")
	}

	// Re-load (so the parser captures the source node), flip flag, re-save.
	reloaded, err := note.Load(n.FilePath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	vlt.SetTagFrontmatter(false)
	if err := saveNoteForVault(reloaded, vlt); err != nil {
		t.Fatalf("second save: %v", err)
	}

	if frontmatterHasKey(t, n.FilePath, "tags") {
		t.Errorf("after flip, tags: should be stripped")
	}
	if frontmatterHasKey(t, n.FilePath, "inline-tags") {
		t.Errorf("after flip, inline-tags: should be stripped")
	}
}

// TestTagFrontmatter_TitlesIndexUnaffected — the flag controls frontmatter
// only; titles.json mirror always carries own + inline + inherited tags.
func TestTagFrontmatter_TitlesIndexUnaffected(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.SetTagFrontmatter(false)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	writeNote(t, vlt, "Note.md", `---
uuid: ti-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Note
#daily

A line with #followup.
`)
	if _, err := RunDoctorFullScan(vlt, false); err != nil {
		t.Fatalf("doctor: %v", err)
	}

	idx, err := vlt.LoadTitles()
	if err != nil {
		t.Fatalf("load titles: %v", err)
	}
	entry := idx.Titles["ti-1"]
	if len(entry.Tags) == 0 || entry.Tags[0] != "daily" {
		t.Errorf("titles entry Tags = %v, want [daily]", entry.Tags)
	}
	if len(entry.InlineTags) == 0 || entry.InlineTags[0] != "followup" {
		t.Errorf("titles entry InlineTags = %v, want [followup]", entry.InlineTags)
	}
}

// TestTagFrontmatter_DoctorMigratesLegacyWithFlagOff — pre-v0.4.0 vault +
// flag=false: doctor migrates and strips own-tag keys in the same commit.
func TestTagFrontmatter_DoctorMigratesLegacyWithFlagOff(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.SetTagFrontmatter(false)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	legacy := `---
uuid: legacy-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#daily"
inline-tags:
  - "#followup"
---
# Legacy
#daily

A line with #followup.
`
	path := writeNote(t, vlt, "Legacy.md", legacy)

	out, err := RunDoctorFullScan(vlt, false)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if len(out.TagFormatMigrated) != 1 {
		t.Errorf("TagFormatMigrated = %v, want 1 entry", out.TagFormatMigrated)
	}

	if frontmatterHasKey(t, path, "tags") {
		t.Errorf("flag=false migration should strip tags:")
	}
	if frontmatterHasKey(t, path, "inline-tags") {
		t.Errorf("flag=false migration should strip inline-tags:")
	}
}

// TestTagFrontmatter_OwnTagMirrorChangedReported — doctor's
// OwnTagMirrorChanged counter fires on flag flips.
func TestTagFrontmatter_OwnTagMirrorChangedReported(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Default flag (true): seed a note that already has frontmatter tags.
	writeNote(t, vlt, "N.md", `---
uuid: omc-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - daily
---
# N
#daily
`)
	if _, err := RunDoctorFullScan(vlt, false); err != nil {
		t.Fatalf("first doctor: %v", err)
	}

	// Flip flag — doctor should now report own-tag mirror change.
	vlt.SetTagFrontmatter(false)
	out, err := RunDoctorFullScan(vlt, false)
	if err != nil {
		t.Fatalf("flip doctor: %v", err)
	}
	if len(out.OwnTagMirrorChanged) != 1 {
		t.Errorf("OwnTagMirrorChanged = %v, want 1 entry", out.OwnTagMirrorChanged)
	}
}
