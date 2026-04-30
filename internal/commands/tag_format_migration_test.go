package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

// writeNote writes a single fixture file under the vault.
func writeNote(t *testing.T, vlt *vault.Vault, name, content string) string {
	t.Helper()
	path := filepath.Join(vlt.Path, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// TestSpacedTag_RoundTrip — `#meeting notes#` body syntax stores as
// `meeting notes` and round-trips back through BodyForm.
func TestSpacedTag_RoundTrip(t *testing.T) {
	if got := note.NormalizeStored("#meeting notes#"); got != "meeting notes" {
		t.Fatalf("NormalizeStored body form = %q, want %q", got, "meeting notes")
	}
	if got := note.NormalizeStored("meeting notes"); got != "meeting notes" {
		t.Fatalf("NormalizeStored stored form = %q, want stable", got)
	}
	if got := note.BodyForm("meeting notes"); got != "#meeting notes#" {
		t.Fatalf("BodyForm spaced = %q, want %q", got, "#meeting notes#")
	}
	if got := note.BodyForm("daily"); got != "#daily" {
		t.Fatalf("BodyForm simple = %q, want %q", got, "#daily")
	}
	if got := note.BodyForm("project/alpha"); got != "#project/alpha" {
		t.Fatalf("BodyForm slashed = %q, want %q", got, "#project/alpha")
	}
}

// TestDoctor_TagFormatMigration_PreV040 — a pre-v0.4.0 vault is rewritten
// once and idempotent on subsequent runs.
func TestDoctor_TagFormatMigration_PreV040(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init vault: %v", err)
	}

	// Pre-v0.4.0 frontmatter: `#`-prefixed tags + inline-tags field.
	legacy := `---
uuid: legacy-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#daily"
  - "#work"
inline-tags:
  - "#followup"
inherited-tags:
  - "#project"
parent: parent-1
---
# Daily Note
#daily #work

Some content with a #followup inline.
`
	path := writeNote(t, vlt, "Daily.md", legacy)

	// Detection: HasLegacyTagFrontmatter trips on each marker.
	if !note.HasLegacyTagFrontmatter(legacy) {
		t.Fatal("HasLegacyTagFrontmatter false-negative on pre-v0.4.0 frontmatter")
	}

	// Run doctor. First scan should migrate.
	out, err := RunDoctorFullScan(vlt, false)
	if err != nil {
		t.Fatalf("doctor first run: %v", err)
	}
	if len(out.TagFormatMigrated) != 1 {
		t.Fatalf("first run TagFormatMigrated = %v, want 1 entry", out.TagFormatMigrated)
	}

	// Frontmatter on disk: stripped tags, no inline-tags key.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated note: %v", err)
	}
	gotStr := string(got)
	if strings.Contains(gotStr, `"#daily"`) || strings.Contains(gotStr, `"#work"`) {
		t.Errorf("migrated frontmatter still contains #-prefixed tag literals:\n%s", gotStr)
	}
	if strings.Contains(gotStr, "inline-tags:") {
		t.Errorf("migrated frontmatter still contains inline-tags: key:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "inherited-tags:") {
		t.Errorf("migrated frontmatter dropped inherited-tags:\n%s", gotStr)
	}
	if note.HasLegacyTagFrontmatter(gotStr) {
		t.Error("HasLegacyTagFrontmatter still trips after migration")
	}

	// Second run is a no-op for the migration path.
	out2, err := RunDoctorFullScan(vlt, false)
	if err != nil {
		t.Fatalf("doctor second run: %v", err)
	}
	if len(out2.TagFormatMigrated) != 0 {
		t.Errorf("second run TagFormatMigrated = %v, want empty (idempotent)", out2.TagFormatMigrated)
	}
}

// TestDoctor_ObsidianFixture_NoFrontmatterMutation — an Obsidian vault
// (tags without `#`, no ruin fields) is not mutated by doctor's first run.
func TestDoctor_ObsidianFixture_NoFrontmatterMutation(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init vault: %v", err)
	}

	// Obsidian-style: bare tag values, no `#`, no inline-tags field.
	obsidian := `---
tags:
  - daily
  - work
custom-field: keep-me
---
# Obsidian Note
#daily #work

Some content.
`
	path := writeNote(t, vlt, "Obsidian.md", obsidian)

	if note.HasLegacyTagFrontmatter(obsidian) {
		t.Fatal("HasLegacyTagFrontmatter false-positive on Obsidian-style frontmatter")
	}

	out, err := RunDoctorFullScan(vlt, false)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	// Doctor will still write the file because it adds a UUID and other
	// derived fields. We only need the tag arrays to stay intact.
	if len(out.TagFormatMigrated) != 0 {
		t.Errorf("expected no tag-format migrations on obsidian fixture, got %v", out.TagFormatMigrated)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gotStr := string(got)
	if strings.Contains(gotStr, `"#daily"`) || strings.Contains(gotStr, `"#work"`) {
		t.Errorf("Obsidian frontmatter mutated to add `#`:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "custom-field: keep-me") {
		t.Errorf("custom field dropped from Obsidian frontmatter:\n%s", gotStr)
	}
}

// TestCascadeInheritedTags_UpdatesTitlesMirror — when a parent's tags
// change, descendants' titles.json mirror is updated in the same pass.
func TestCascadeInheritedTags_UpdatesTitlesMirror(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init vault: %v", err)
	}

	parentContent := `---
uuid: parent-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - project
---
# Parent
#project
`
	childContent := `---
uuid: child-1
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
parent: parent-1
inherited-tags:
  - project
---
# Child
Some child note.
`
	writeNote(t, vlt, "Parent.md", parentContent)
	writeNote(t, vlt, "Child.md", childContent)

	if _, err := RunDoctorFullScan(vlt, false); err != nil {
		t.Fatalf("doctor: %v", err)
	}

	// Sanity: child's titles entry mirrors inherited-tags.
	idx, err := vlt.LoadTitles()
	if err != nil {
		t.Fatalf("load titles: %v", err)
	}
	child := idx.Titles["child-1"]
	if !slices.Contains(child.InheritedTags, "project") {
		t.Fatalf("child titles entry missing inherited project: %v", child.InheritedTags)
	}

	// Add a tag to the parent, save, then cascade.
	parent, err := note.Load(filepath.Join(vlt.Path, "Parent.md"))
	if err != nil {
		t.Fatalf("load parent: %v", err)
	}
	parent.Content = "# Parent\n#project #urgent\n"
	parent.RefreshTags()
	if err := parent.Save(); err != nil {
		t.Fatalf("save parent: %v", err)
	}
	vlt.SaveNote(parent, []string{"project"}, nil, "")

	idx2, err := vlt.LoadTitles()
	if err != nil {
		t.Fatalf("reload titles: %v", err)
	}
	if err := CascadeInheritedTags("parent-1", vlt, idx2); err != nil {
		t.Fatalf("cascade: %v", err)
	}

	// Re-check child's titles mirror after cascade.
	idx3, err := vlt.LoadTitles()
	if err != nil {
		t.Fatalf("re-reload titles: %v", err)
	}
	child2 := idx3.Titles["child-1"]
	if !slices.Contains(child2.InheritedTags, "urgent") {
		t.Errorf("after cascade child titles entry missing inherited urgent: %v", child2.InheritedTags)
	}
}

// TestJSONContract_TagsArraysStripped — every JSON tags-array field
// emits stored form (no `#` prefix).
func TestJSONContract_TagsArraysStripped(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init vault: %v", err)
	}

	noteContent := `---
uuid: jsonc-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - daily
  - work
---
# Daily
#daily #work

A note with a #followup inline.
`
	writeNote(t, vlt, "Daily.md", noteContent)
	if _, err := RunDoctorFullScan(vlt, false); err != nil {
		t.Fatalf("doctor: %v", err)
	}

	// titles.json: tags arrays in stored form.
	idx, err := vlt.LoadTitles()
	if err != nil {
		t.Fatalf("load titles: %v", err)
	}
	entry := idx.Titles["jsonc-1"]
	for _, tag := range append(append([]string(nil), entry.Tags...), entry.InlineTags...) {
		if strings.HasPrefix(tag, "#") {
			t.Errorf("titles.json tag %q has `#` prefix", tag)
		}
	}

	// On-disk titles.json has version: 2.
	titlesBytes, err := os.ReadFile(vlt.TitlesFile())
	if err != nil {
		t.Fatalf("read titles.json: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(titlesBytes, &raw); err != nil {
		t.Fatalf("parse titles.json: %v", err)
	}
	if v, _ := raw["version"].(float64); int(v) != 2 {
		t.Errorf("titles.json version = %v, want 2", raw["version"])
	}

	// tags.yml: Name field is stripped form.
	tagsIdx, err := vlt.LoadTags()
	if err != nil {
		t.Fatalf("load tags: %v", err)
	}
	if tagsIdx.Version != 2 {
		t.Errorf("tags.yml version = %d, want 2", tagsIdx.Version)
	}
	for _, te := range tagsIdx.Tags {
		if strings.HasPrefix(te.Name, "#") {
			t.Errorf("tags.yml entry name %q has `#` prefix", te.Name)
		}
	}
}

// TestSearch_FindsUnindexedNote — when a file exists on disk but isn't yet
// in titles.json (e.g., added externally between doctor scans), tag queries
// still find it via the body-classification fallback in
// hydrateNoteTagsFromIndex.
func TestSearch_FindsUnindexedNote(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init vault: %v", err)
	}

	// Write a note WITHOUT touching titles.json — simulates a file added
	// since the last doctor run.
	writeNote(t, vlt, "Stray.md", `---
uuid: stray-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - daily
---
# Stray
#daily

A note ruin hasn't indexed yet.
`)

	matcher, info, err := parseQuery("#daily", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery: %v", err)
	}
	results, err := searchNotesWithOptions(vlt, matcher, info, SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results for #daily on unindexed note, want 1", len(results))
	}
}

// TestTagsRename_UpdatesTitlesMirror — `ruin tags rename` propagates the
// rename to the titles.json mirror so subsequent hot-path tag queries find
// the renamed notes under the new name.
func TestTagsRename_UpdatesTitlesMirror(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	writeNote(t, vlt, "N.md", `---
uuid: rn-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - wip
---
# N
#wip
`)
	if _, err := RunDoctorFullScan(vlt, false); err != nil {
		t.Fatalf("doctor: %v", err)
	}

	// Rename inline via the same code path the CLI uses.
	n, err := note.Load(filepath.Join(vlt.Path, "N.md"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	n.Content = strings.ReplaceAll(n.Content, "#wip", "#in-progress")
	n.RefreshTags()
	n.SetTimestamps()
	if err := n.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := vlt.UpdateTitleEntryFull(n.UUID, n.Title, n.FilePath, n.Parent, n.Tags, n.InlineTags, n.InheritedTags); err != nil {
		t.Fatalf("update titles: %v", err)
	}

	idx, err := vlt.LoadTitles()
	if err != nil {
		t.Fatalf("load titles: %v", err)
	}
	entry := idx.Titles["rn-1"]
	if !slices.Contains(entry.Tags, "in-progress") {
		t.Errorf("titles entry Tags = %v, want to contain in-progress", entry.Tags)
	}
	if slices.Contains(entry.Tags, "wip") {
		t.Errorf("titles entry Tags = %v, still contains stale wip", entry.Tags)
	}
}

// TestIndexVersionRefuse — old binary semantics: a future version on
// titles.json refuses the load with a clear error.
func TestIndexVersionRefuse(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Hand-write a future-version titles.json.
	future := `{"version": 999, "titles": {}}`
	if err := os.WriteFile(vlt.TitlesFile(), []byte(future), 0644); err != nil {
		t.Fatalf("write future titles: %v", err)
	}
	if _, err := vlt.LoadTitles(); err == nil {
		t.Fatal("LoadTitles succeeded on future version; expected refusal")
	}
}
