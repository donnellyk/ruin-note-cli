package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
)

// setupDoctorVault creates a vault with a note whose frontmatter tags are in sync.
func setupDoctorVault(t *testing.T) *vault.Vault {
	t.Helper()

	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	// Create a note with correct frontmatter
	noteContent := `---
uuid: uuid-doc-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#daily"
---
# Doctor Test Note
#daily

Some content here.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "doctor-note.md"), []byte(noteContent), 0644); err != nil {
		t.Fatalf("failed to write note: %v", err)
	}

	// Seed the tags index with the existing tag
	if err := vlt.UpdateTagsIndex([]string{"#daily"}, nil); err != nil {
		t.Fatalf("failed to seed tags index: %v", err)
	}

	// Seed the titles index
	if err := vlt.UpdateTitleEntry("uuid-doc-1", "Doctor Test Note", filepath.Join(tmpDir, "doctor-note.md"), ""); err != nil {
		t.Fatalf("failed to seed titles index: %v", err)
	}

	return vlt
}

func TestDoctorCmd_SingleFile_AddTag(t *testing.T) {
	vlt := setupDoctorVault(t)
	notePath := filepath.Join(vlt.Path, "doctor-note.md")

	// Manually add a tag line to the note body (simulating external edit)
	data, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("failed to read note: %v", err)
	}
	newContent := string(data) + "\n#newtag\n"
	if err := os.WriteFile(notePath, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to write note: %v", err)
	}

	// Run doctor on the single file
	jsonOut := true
	cmd := NewDoctorCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"doctor-note.md"})
	err = cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output DoctorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v (raw: %s)", err, buf.String())
	}

	if output.Scanned != 1 {
		t.Errorf("Scanned = %d, want 1", output.Scanned)
	}
	if len(output.TagsReindexed) != 1 {
		t.Errorf("TagsReindexed = %v, want 1 entry", output.TagsReindexed)
	}
	if !output.TagsYMLUpdated {
		t.Error("TagsYMLUpdated = false, want true")
	}
	if !output.TitlesUpdated {
		t.Error("TitlesUpdated = false, want true")
	}

	// Verify frontmatter was updated
	n, err := note.Load(notePath)
	if err != nil {
		t.Fatalf("failed to load note: %v", err)
	}
	foundNew := false
	for _, tag := range n.Tags {
		if note.NormalizeTag(tag) == "#newtag" {
			foundNew = true
		}
	}
	if !foundNew {
		t.Errorf("frontmatter tags = %v, want #newtag present", n.Tags)
	}

	// Verify tags index was updated incrementally
	tagsIndex, err := vlt.LoadTags()
	if err != nil {
		t.Fatalf("failed to load tags: %v", err)
	}
	tagMap := make(map[string]int)
	for _, te := range tagsIndex.Tags {
		tagMap[strings.ToLower(te.Name)] = te.Count
	}
	if tagMap["#daily"] != 1 {
		t.Errorf("tags index #daily count = %d, want 1", tagMap["#daily"])
	}
	if tagMap["#newtag"] != 1 {
		t.Errorf("tags index #newtag count = %d, want 1", tagMap["#newtag"])
	}

	// Verify titles index was updated
	titlesIndex, err := vlt.LoadTitles()
	if err != nil {
		t.Fatalf("failed to load titles: %v", err)
	}
	entry, ok := titlesIndex.Titles["uuid-doc-1"]
	if !ok {
		t.Fatal("title entry missing for uuid-doc-1")
	}
	if entry.Title != "Doctor Test Note" {
		t.Errorf("title = %q, want %q", entry.Title, "Doctor Test Note")
	}
}

func TestDoctorCmd_SingleFile_RemoveTag(t *testing.T) {
	vlt := setupDoctorVault(t)
	notePath := filepath.Join(vlt.Path, "doctor-note.md")

	// Remove the #daily tag line from the body
	noteContent := `---
uuid: uuid-doc-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#daily"
---
# Doctor Test Note

Some content here, no tags anymore.
`
	if err := os.WriteFile(notePath, []byte(noteContent), 0644); err != nil {
		t.Fatalf("failed to write note: %v", err)
	}

	jsonOut := true
	cmd := NewDoctorCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"doctor-note.md"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output DoctorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(output.TagsReindexed) != 1 {
		t.Errorf("TagsReindexed = %v, want 1 entry", output.TagsReindexed)
	}

	// Verify frontmatter no longer has #daily
	n, err := note.Load(notePath)
	if err != nil {
		t.Fatalf("failed to load note: %v", err)
	}
	if len(n.Tags) != 0 {
		t.Errorf("tags = %v, want empty", n.Tags)
	}

	// Verify tags index: #daily should be removed (count went to 0)
	tagsIndex, err := vlt.LoadTags()
	if err != nil {
		t.Fatalf("failed to load tags: %v", err)
	}
	for _, te := range tagsIndex.Tags {
		if strings.ToLower(te.Name) == "#daily" {
			t.Errorf("tags index still contains #daily with count %d, want removed", te.Count)
		}
	}
}

func TestDoctorCmd_SingleFile_DryRun(t *testing.T) {
	vlt := setupDoctorVault(t)
	notePath := filepath.Join(vlt.Path, "doctor-note.md")

	// Add a new tag to the body
	data, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if err := os.WriteFile(notePath, []byte(string(data)+"\n#newtag\n"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Capture the file content before doctor
	beforeData, _ := os.ReadFile(notePath)

	jsonOut := true
	cmd := NewDoctorCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--dry-run", "doctor-note.md"})
	err = cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output DoctorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Should report changes
	if len(output.TagsReindexed) != 1 {
		t.Errorf("TagsReindexed = %v, want 1 entry", output.TagsReindexed)
	}

	// File should NOT be modified
	afterData, _ := os.ReadFile(notePath)
	if string(beforeData) != string(afterData) {
		t.Error("dry-run modified the file")
	}

	// Tags index should NOT have #newtag
	tagsIndex, err := vlt.LoadTags()
	if err != nil {
		t.Fatalf("failed to load tags: %v", err)
	}
	for _, te := range tagsIndex.Tags {
		if strings.ToLower(te.Name) == "#newtag" {
			t.Error("dry-run updated the tags index")
		}
	}
}

func TestDoctorCmd_SingleFile_NotFound(t *testing.T) {
	vlt := setupDoctorVault(t)

	jsonOut := false
	cmd := NewDoctorCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"nonexistent.md"})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("error = %q, want to contain 'file not found'", err.Error())
	}
}

func TestDoctorCmd_FullScan_NoArgs(t *testing.T) {
	vlt := setupDoctorVault(t)

	jsonOut := true
	cmd := NewDoctorCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output DoctorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if output.Scanned != 1 {
		t.Errorf("Scanned = %d, want 1", output.Scanned)
	}
	if !output.TagsYMLUpdated {
		t.Error("TagsYMLUpdated = false, want true")
	}
	if !output.TitlesUpdated {
		t.Error("TitlesUpdated = false, want true")
	}
}

func TestDoctorCmd_SingleFile_NoChanges(t *testing.T) {
	vlt := setupDoctorVault(t)

	// Note is already in sync, no changes expected
	jsonOut := true
	cmd := NewDoctorCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"doctor-note.md"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output DoctorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if output.Scanned != 1 {
		t.Errorf("Scanned = %d, want 1", output.Scanned)
	}
	if len(output.TagsReindexed) != 0 {
		t.Errorf("TagsReindexed = %v, want empty (no changes)", output.TagsReindexed)
	}
	if len(output.UUIDGenerated) != 0 {
		t.Errorf("UUIDGenerated = %v, want empty", output.UUIDGenerated)
	}
}

func TestDoctorCmd_SingleFile_AbsolutePath(t *testing.T) {
	vlt := setupDoctorVault(t)
	notePath := filepath.Join(vlt.Path, "doctor-note.md")

	// Add a tag
	data, _ := os.ReadFile(notePath)
	os.WriteFile(notePath, []byte(string(data)+"\n#abs\n"), 0644)

	jsonOut := true
	cmd := NewDoctorCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Pass absolute path
	cmd.SetArgs([]string{notePath})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output DoctorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(output.TagsReindexed) != 1 {
		t.Errorf("TagsReindexed = %v, want 1 entry", output.TagsReindexed)
	}
}

func TestDoctorCmd_FullScan_InheritedTags(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	// Create parent with #project tag
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

	// Create child without inherited-tags in frontmatter
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

	// Seed titles index
	if err := vlt.UpdateTitleEntry("parent-uuid", "Parent Note", parentPath, ""); err != nil {
		t.Fatalf("failed to seed parent title: %v", err)
	}
	if err := vlt.UpdateTitleEntry("child-uuid", "Child Note", childPath, "parent-uuid"); err != nil {
		t.Fatalf("failed to seed child title: %v", err)
	}

	// Run full doctor
	jsonOut := true
	cmd := NewDoctorCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output DoctorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v (raw: %s)", err, buf.String())
	}

	// Child should have inherited tags updated
	if len(output.InheritedTagsUpdated) != 1 {
		t.Errorf("InheritedTagsUpdated = %v, want 1 entry", output.InheritedTagsUpdated)
	}

	// Verify child frontmatter has inherited-tags
	child, err := note.Load(childPath)
	if err != nil {
		t.Fatalf("failed to load child: %v", err)
	}
	if len(child.InheritedTags) != 1 || child.InheritedTags[0] != "#project" {
		t.Errorf("child.InheritedTags = %v, want [#project]", child.InheritedTags)
	}
}

func TestDoctorCmd_FullScan_StripInheritedTags(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	// Create parent with #project tag
	parentContent := `---
uuid: parent-uuid
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#project"
---
# Parent Note
#project

Parent content.
`
	parentPath := filepath.Join(tmpDir, "parent.md")
	if err := os.WriteFile(parentPath, []byte(parentContent), 0644); err != nil {
		t.Fatalf("failed to write parent: %v", err)
	}

	// Create child that redundantly has #project in content
	childContent := `---
uuid: child-uuid
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#project"
parent: parent-uuid
---
# Child Note
#project

Child content.
`
	childPath := filepath.Join(tmpDir, "child.md")
	if err := os.WriteFile(childPath, []byte(childContent), 0644); err != nil {
		t.Fatalf("failed to write child: %v", err)
	}

	// Seed titles index
	if err := vlt.UpdateTitleEntry("parent-uuid", "Parent Note", parentPath, ""); err != nil {
		t.Fatalf("failed to seed parent title: %v", err)
	}
	if err := vlt.UpdateTitleEntry("child-uuid", "Child Note", childPath, "parent-uuid"); err != nil {
		t.Fatalf("failed to seed child title: %v", err)
	}

	// Run full doctor
	jsonOut := true
	cmd := NewDoctorCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output DoctorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v (raw: %s)", err, buf.String())
	}

	// Child should have redundant #project stripped from content
	if len(output.InheritedTagsStripped) != 1 {
		t.Errorf("InheritedTagsStripped = %v, want 1 entry", output.InheritedTagsStripped)
	}

	// Verify child content no longer has #project tag-only line
	child, err := note.Load(childPath)
	if err != nil {
		t.Fatalf("failed to load child: %v", err)
	}
	lines := strings.Split(child.Content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "#project" {
			t.Error("child content still has #project tag-only line after doctor strip")
		}
	}
}
