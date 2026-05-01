package commands

import (
	"os"
	"path/filepath"
	"strings"
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

// TestTagFrontmatter_FlipPreservesTagsStripsInlineTags — flipping the flag
// from true to false and re-saving preserves the existing `tags:` (frozen
// at the moment of the flip) but strips `inline-tags:` (still ruin-managed).
func TestTagFrontmatter_FlipPreservesTagsStripsInlineTags(t *testing.T) {
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

	if err := saveNoteForVault(n, vlt); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if !frontmatterHasKey(t, n.FilePath, "inline-tags") {
		t.Fatalf("first save should have emitted inline-tags:")
	}

	reloaded, err := note.Load(n.FilePath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	vlt.SetTagFrontmatter(false)
	if err := saveNoteForVault(reloaded, vlt); err != nil {
		t.Fatalf("second save: %v", err)
	}

	if !frontmatterHasKey(t, n.FilePath, "tags") {
		t.Errorf("after flip, tags: should be preserved (Obsidian-managed surface)")
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

	// Legacy `tags: [#daily]` is migrated to stripped form `tags: [daily]`
	// and preserved (not stripped) — `tags:` is user-managed surface in
	// flag=false mode.
	if !frontmatterHasKey(t, path, "tags") {
		t.Errorf("flag=false migration should preserve tags: (with format normalized)")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(data), "#daily") && strings.Contains(strings.SplitN(string(data), "---", 3)[1], "#daily") {
		t.Errorf("legacy `#`-prefixed format not migrated:\n%s", data)
	}
	if frontmatterHasKey(t, path, "inline-tags") {
		t.Errorf("flag=false migration should strip inline-tags:")
	}
}

// TestTagFrontmatter_Off_PreservesExistingFrontmatterTags — with flag=false,
// existing on-disk `tags:` (e.g. set via Obsidian) survives a ruin save
// untouched, even when body tags differ. `inline-tags:` is still ruin-managed
// and stripped.
func TestTagFrontmatter_Off_PreservesExistingFrontmatterTags(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.SetTagFrontmatter(false)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	path := writeNote(t, vlt, "Daily.md", `---
uuid: pres-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - project
  - status/done
inline-tags:
  - legacy-inline
---
# Daily
#daily

A line with #followup.
`)

	n, err := note.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := saveNoteForVault(n, vlt); err != nil {
		t.Fatalf("save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	fm, _, err := note.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := fm.Tags
	want := []string{"project", "status/done"}
	if len(got) != len(want) {
		t.Fatalf("preserved tags: %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("preserved tags[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
	if frontmatterHasKey(t, path, "inline-tags") {
		t.Errorf("flag=false should still strip inline-tags:")
	}
}

// TestTagFrontmatter_Off_NoExistingTagsKey — with flag=false and no `tags:`
// on disk, ruin doesn't introduce one.
func TestTagFrontmatter_Off_NoExistingTagsKey(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.SetTagFrontmatter(false)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	path := writeNote(t, vlt, "Plain.md", `---
uuid: pres-2
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Plain
#daily
`)

	n, err := note.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := saveNoteForVault(n, vlt); err != nil {
		t.Fatalf("save: %v", err)
	}
	if frontmatterHasKey(t, path, "tags") {
		t.Errorf("flag=false with no on-disk tags: should not introduce one")
	}
}

// TestTagFrontmatter_OwnTagMirrorChangedReported — doctor's
// OwnTagMirrorChanged counter fires when a flag flip causes a frontmatter
// delta. With flag=false, `tags:` is preserved verbatim, so the delta the
// counter detects is `inline-tags:` going from present to absent.
func TestTagFrontmatter_OwnTagMirrorChangedReported(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	writeNote(t, vlt, "N.md", `---
uuid: omc-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - daily
inline-tags:
  - followup
---
# N
#daily

A line with #followup.
`)
	if _, err := RunDoctorFullScan(vlt, false); err != nil {
		t.Fatalf("first doctor: %v", err)
	}

	vlt.SetTagFrontmatter(false)
	out, err := RunDoctorFullScan(vlt, false)
	if err != nil {
		t.Fatalf("flip doctor: %v", err)
	}
	if len(out.OwnTagMirrorChanged) != 1 {
		t.Errorf("OwnTagMirrorChanged = %v, want 1 entry (inline-tags: stripped)", out.OwnTagMirrorChanged)
	}
}
