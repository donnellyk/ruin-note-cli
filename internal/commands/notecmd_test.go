package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
)

func setupNoteTestVault(t *testing.T) *vault.Vault {
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
		parent   string
		content  string
	}{
		{
			filename: "tagged.md",
			uuid:     "uuid-tagged",
			title:    "Tagged Note",
			content: `---
uuid: uuid-tagged
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
tags:
  - "#daily"
  - "#work"
---
# Tagged Note
#daily, #work

Some content here.`,
		},
		{
			filename: "plain.md",
			uuid:     "uuid-plain",
			title:    "Plain Note",
			content: `---
uuid: uuid-plain
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
---
# Plain Note

No tags on this note.`,
		},
		{
			filename: "source.md",
			uuid:     "uuid-source",
			title:    "Source Note",
			content: `---
uuid: uuid-source
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
tags:
  - "#idea"
---
# Source Note
#idea

Source content to merge.`,
		},
		{
			filename: "target.md",
			uuid:     "uuid-target",
			title:    "Target Note",
			content: `---
uuid: uuid-target
created: "2025-01-04T10:00:00-05:00"
updated: "2025-01-04T10:00:00-05:00"
tags:
  - "#project"
---
# Target Note
#project

Target content.`,
		},
		{
			filename: "child-of-source.md",
			uuid:     "uuid-child-src",
			title:    "Child of Source",
			parent:   "uuid-source",
			content: `---
uuid: uuid-child-src
created: "2025-01-05T10:00:00-05:00"
updated: "2025-01-05T10:00:00-05:00"
parent: uuid-source
---
# Child of Source

I'm a child of the source note.`,
		},
	}

	for _, n := range notes {
		path := filepath.Join(tmpDir, n.filename)
		if err := os.WriteFile(path, []byte(n.content), 0644); err != nil {
			t.Fatalf("failed to create test note: %v", err)
		}
		vlt.UpdateTitleEntry(n.uuid, n.title, path, n.parent)
	}

	return vlt
}

// --- note set tests ---

func TestNoteSet_AddTag(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := true
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-tag", "#urgent"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("note set --add-tag error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result noteSetOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, buf.String())
	}

	if len(result.Changes) != 1 {
		t.Fatalf("got %d changes, want 1", len(result.Changes))
	}
	if result.Changes[0].Action != "added" {
		t.Errorf("action = %q, want added", result.Changes[0].Action)
	}

	// Verify tag was actually added to content
	n, _ := ResolveNote(vlt, "uuid-tagged")
	allTags := n.AllTags()
	found := false
	for _, tag := range allTags {
		if note.NormalizeTag(tag) == "#urgent" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tag #urgent not found in note tags: %v", allTags)
	}
}

func TestNoteSet_AddTag_NoopIfExists(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := true
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-tag", "#daily"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result noteSetOutput
	json.Unmarshal(buf.Bytes(), &result)

	if len(result.Changes) != 0 {
		t.Errorf("got %d changes, want 0 (tag already exists)", len(result.Changes))
	}
}

func TestNoteSet_AddTag_ToPlainNote(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-plain", "--add-tag", "#new"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	// Verify: tag should be inserted after the title
	n, _ := ResolveNote(vlt, "uuid-plain")
	lines := strings.Split(n.Content, "\n")

	// First line should be the title, second should be the tag
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	if lines[0] != "# Plain Note" {
		t.Errorf("line 0 = %q, want title", lines[0])
	}
	if strings.TrimSpace(lines[1]) != "#new" {
		t.Errorf("line 1 = %q, want #new", lines[1])
	}
}

func TestNoteSet_RemoveTag(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-tagged", "--remove-tag", "#work"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-tagged")
	for _, tag := range n.AllTags() {
		if note.NormalizeTag(tag) == "#work" {
			t.Errorf("tag #work should have been removed, still found in: %v", n.AllTags())
		}
	}
}

func TestNoteSet_RemoveTag_NoopIfNotPresent(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := true
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"set", "uuid-tagged", "--remove-tag", "#nonexistent"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result noteSetOutput
	json.Unmarshal(buf.Bytes(), &result)

	if len(result.Changes) != 0 {
		t.Errorf("got %d changes, want 0 (tag not present)", len(result.Changes))
	}
}

func TestNoteSet_Order(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-plain", "--order", "5"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-plain")
	if n.Order == nil || *n.Order != 5 {
		t.Errorf("order = %v, want 5", n.Order)
	}
}

func TestNoteSet_NoOrder(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false

	// Set order first
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-plain", "--order", "3"})
	cmd.Execute()

	// Unset order
	cmd2 := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-plain", "--no-order"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-plain")
	if n.Order != nil {
		t.Errorf("order = %v, want nil", *n.Order)
	}
}

func TestNoteSet_OrderAndNoOrder_MutuallyExclusive(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-plain", "--order", "1", "--no-order"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected mutually exclusive error")
	}
	if !containsSubstr(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want mutually exclusive", err.Error())
	}
}

func TestNoteSet_Field(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-plain", "--field", "status=active"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-plain")
	if n.Extra["status"] != "active" {
		t.Errorf("extra[status] = %v, want active", n.Extra["status"])
	}
}

func TestNoteSet_FieldDelete(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false

	// Set field
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-plain", "--field", "status=active"})
	cmd.Execute()

	// Delete field (empty value)
	cmd2 := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-plain", "--field", "status="})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-plain")
	if _, exists := n.Extra["status"]; exists {
		t.Errorf("extra[status] should be deleted, got %v", n.Extra["status"])
	}
}

func TestNoteSet_NoMutationFlag(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-plain"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no mutation flags")
	}
	if !containsSubstr(err.Error(), "at least one mutation") {
		t.Errorf("error = %q, want mutation required", err.Error())
	}
}

func TestNoteSet_AddTagAutoPrefix(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// Without # prefix
	cmd.SetArgs([]string{"set", "uuid-plain", "--add-tag", "newtag"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-plain")
	found := false
	for _, tag := range n.AllTags() {
		if note.NormalizeTag(tag) == "#newtag" {
			found = true
		}
	}
	if !found {
		t.Error("tag #newtag not found after adding without # prefix")
	}
}

func TestNoteSet_CombinedFlags(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := true
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-tag", "#new", "--remove-tag", "#work", "--order", "2"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result noteSetOutput
	json.Unmarshal(buf.Bytes(), &result)

	if len(result.Changes) != 3 {
		t.Errorf("got %d changes, want 3", len(result.Changes))
	}
}

// --- note set --line tests ---

func TestNoteSet_AddTag_InlineWithLine(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// tagged.md content lines:
	// 1: # Tagged Note
	// 2: #daily, #work
	// 3: (empty)
	// 4: Some content here.
	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-tag", "#inlinetest", "--line", "4"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-tagged")
	lines := strings.Split(n.Content, "\n")
	if !strings.Contains(lines[3], "#inlinetest") {
		t.Errorf("line 4 = %q, expected #inlinetest appended", lines[3])
	}
	if !strings.HasPrefix(lines[3], "Some content here.") {
		t.Errorf("line 4 = %q, original content should be preserved", lines[3])
	}
}

func TestNoteSet_RemoveTag_FromSpecificLine(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false

	// First add an inline tag to line 4
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-tag", "#inlinetest", "--line", "4"})
	cmd.Execute()

	// Now remove it from line 4 only
	cmd2 := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-tagged", "--remove-tag", "#inlinetest", "--line", "4"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-tagged")
	lines := strings.Split(n.Content, "\n")
	if strings.Contains(lines[3], "#inlinetest") {
		t.Errorf("line 4 = %q, #inlinetest should have been removed", lines[3])
	}
}

func TestNoteSet_RemoveTag_LineDoesNotAffectOtherLines(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false

	// tagged.md has #daily on line 2 (the tag-only line).
	// Removing #daily with --line 4 should NOT remove it from line 2.
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-tagged", "--remove-tag", "#daily", "--line", "4"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-tagged")
	// #daily should still exist on the tag-only line
	found := false
	for _, tag := range n.Tags {
		if note.NormalizeTag(tag) == "#daily" {
			found = true
		}
	}
	if !found {
		t.Error("#daily should still exist on the tag-only line (not targeted by --line 4)")
	}
}

func TestNoteSet_AddDate_Global(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := true
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-date", "today"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result noteSetOutput
	json.Unmarshal(buf.Bytes(), &result)

	if len(result.Changes) != 1 {
		t.Fatalf("got %d changes, want 1", len(result.Changes))
	}
	if result.Changes[0].Field != "date" || result.Changes[0].Action != "added" {
		t.Errorf("change = %+v, want date/added", result.Changes[0])
	}

	// Verify date was added to content
	n, _ := ResolveNote(vlt, "uuid-tagged")
	if !strings.Contains(n.Content, "@") {
		t.Error("expected @YYYY-MM-DD date in content")
	}
}

func TestNoteSet_AddDate_WithLine(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-date", "2026-03-15", "--line", "4"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-tagged")
	lines := strings.Split(n.Content, "\n")
	if !strings.Contains(lines[3], "@2026-03-15") {
		t.Errorf("line 4 = %q, expected @2026-03-15", lines[3])
	}
}

func TestNoteSet_RemoveDate_AllLines(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false

	// Add a date first
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-date", "2026-03-15", "--line", "4"})
	cmd.Execute()

	// Remove that specific date from all lines
	cmd2 := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-tagged", "--remove-date", "@2026-03-15"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-tagged")
	if strings.Contains(n.Content, "@2026-03-15") {
		t.Errorf("@2026-03-15 should have been removed, content:\n%s", n.Content)
	}
}

func TestNoteSet_RemoveDate_SpecificLine(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false

	// Add dates on two lines
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-date", "2026-03-15", "--line", "2"})
	cmd.Execute()

	cmd2 := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-tagged", "--add-date", "2026-03-15", "--line", "4"})
	cmd2.Execute()

	// Remove from line 4 only
	cmd3 := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd3.SetArgs([]string{"set", "uuid-tagged", "--remove-date", "@2026-03-15", "--line", "4"})
	if err := cmd3.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-tagged")
	lines := strings.Split(n.Content, "\n")
	if strings.Contains(lines[3], "@2026-03-15") {
		t.Errorf("line 4 should not have @2026-03-15")
	}
	if !strings.Contains(lines[1], "@2026-03-15") {
		t.Errorf("line 2 should still have @2026-03-15")
	}
}

func TestNoteSet_RemoveDates_All(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false

	// Add dates to two lines
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-date", "2026-03-15", "--line", "2"})
	cmd.Execute()

	cmd2 := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-tagged", "--add-date", "2026-04-01", "--line", "4"})
	cmd2.Execute()

	// Remove all dates
	cmd3 := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd3.SetArgs([]string{"set", "uuid-tagged", "--remove-dates"})
	if err := cmd3.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-tagged")
	if strings.Contains(n.Content, "@2026-03-15") || strings.Contains(n.Content, "@2026-04-01") {
		t.Errorf("all dates should have been removed, content:\n%s", n.Content)
	}
}

func TestNoteSet_RemoveDates_SpecificLine(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false

	// Add dates to two lines
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-date", "2026-03-15", "--line", "2"})
	cmd.Execute()

	cmd2 := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd2.SetArgs([]string{"set", "uuid-tagged", "--add-date", "2026-04-01", "--line", "4"})
	cmd2.Execute()

	// Remove all dates from line 4 only
	cmd3 := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd3.SetArgs([]string{"set", "uuid-tagged", "--remove-dates", "--line", "4"})
	if err := cmd3.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-tagged")
	lines := strings.Split(n.Content, "\n")
	if strings.Contains(lines[3], "@2026-04-01") {
		t.Errorf("line 4 should not have @2026-04-01")
	}
	if !strings.Contains(lines[1], "@2026-03-15") {
		t.Errorf("line 2 should still have @2026-03-15")
	}
}

func TestNoteSet_LineValidation_OutOfRange(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-tag", "#test", "--line", "999"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected out-of-range error")
	}
	if !containsSubstr(err.Error(), "out of range") {
		t.Errorf("error = %q, want out of range", err.Error())
	}
}

func TestNoteSet_LineValidation_WithoutMutationFlag(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-tagged", "--line", "1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --line without mutation flag")
	}
	if !containsSubstr(err.Error(), "--line requires") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestNoteSet_AddDate_InvalidToken(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"set", "uuid-tagged", "--add-date", "gibberish-not-a-date"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid date token")
	}
	if !containsSubstr(err.Error(), "unrecognized date") {
		t.Errorf("error = %q", err.Error())
	}
}

// --- note append tests ---

func TestNoteAppend_AtEnd(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := true
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"append", "uuid-plain", "New paragraph"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result noteAppendOutput
	json.Unmarshal(buf.Bytes(), &result)

	if result.Action != "appended" {
		t.Errorf("action = %q, want appended", result.Action)
	}

	// Verify content
	n, _ := ResolveNote(vlt, "uuid-plain")
	if !strings.Contains(n.Content, "New paragraph") {
		t.Error("appended text not found in content")
	}
	if !strings.HasSuffix(n.Content, "New paragraph\n") {
		t.Errorf("content should end with 'New paragraph\\n', got: %q", n.Content[len(n.Content)-30:])
	}
}

func TestNoteAppend_InsertBeforeLine(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"append", "uuid-plain", "Inserted line", "--line", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-plain")
	lines := strings.Split(n.Content, "\n")
	if lines[0] != "Inserted line" {
		t.Errorf("line 0 = %q, want 'Inserted line'", lines[0])
	}
}

func TestNoteAppend_Suffix(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"append", "uuid-plain", " (modified)", "--line", "1", "--suffix"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ := ResolveNote(vlt, "uuid-plain")
	lines := strings.Split(n.Content, "\n")
	if !strings.HasSuffix(lines[0], "(modified)") {
		t.Errorf("line 0 = %q, want suffix '(modified)'", lines[0])
	}
}

func TestNoteAppend_SuffixWithoutLine(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"append", "uuid-plain", "text", "--suffix"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --suffix without --line")
	}
	if !containsSubstr(err.Error(), "--suffix requires --line") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestNoteAppend_LineOutOfRange(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"append", "uuid-plain", "text", "--line", "999"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected out-of-range error")
	}
	if !containsSubstr(err.Error(), "out of range") {
		t.Errorf("error = %q, want out of range", err.Error())
	}
}

func TestNoteAppend_NoText(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"append", "uuid-plain"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no text")
	}
	if !containsSubstr(err.Error(), "text required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestNoteAppend_RawLine(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false

	// plain.md has frontmatter. Figure out what raw line the content starts at.
	n, _ := ResolveNote(vlt, "uuid-plain")
	serialized, _ := n.Serialize()
	fmLen := len(serialized) - len(n.Content)
	fmLineCount := strings.Count(serialized[:fmLen], "\n")

	// Insert at first content line using raw-line numbering
	rawLineNum := fmLineCount + 1

	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"append", "uuid-plain", "RAW INSERTED", "--line", fmt.Sprintf("%d", rawLineNum), "--raw-line"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	n, _ = ResolveNote(vlt, "uuid-plain")
	lines := strings.Split(n.Content, "\n")
	if lines[0] != "RAW INSERTED" {
		t.Errorf("line 0 = %q, want 'RAW INSERTED'", lines[0])
	}
}

func TestNoteAppend_RawLineInFrontmatter(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// Line 1 is the opening --- delimiter, which is in the frontmatter
	cmd.SetArgs([]string{"append", "uuid-plain", "text", "--line", "1", "--raw-line"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for line pointing into frontmatter")
	}
	if !containsSubstr(err.Error(), "points into frontmatter") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestNoteAppend_RawLineWithoutLine(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"append", "uuid-plain", "text", "--raw-line"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --raw-line without --line")
	}
	if !containsSubstr(err.Error(), "--raw-line requires --line") {
		t.Errorf("error = %q", err.Error())
	}
}

// --- note merge tests ---

func TestNoteMerge_Basic(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := true
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"merge", "uuid-target", "uuid-source", "--force"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result noteMergeOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, buf.String())
	}

	if result.TargetUUID != "uuid-target" {
		t.Errorf("target uuid = %q, want uuid-target", result.TargetUUID)
	}
	if result.SourceUUID != "uuid-source" {
		t.Errorf("source uuid = %q, want uuid-source", result.SourceUUID)
	}

	// Verify target has source's content
	target, _ := ResolveNote(vlt, "uuid-target")
	if !strings.Contains(target.Content, "Source content to merge.") {
		t.Error("target should contain source content after merge")
	}

	// Verify #idea was merged
	if len(result.TagsMerged) == 0 {
		t.Error("expected tags_merged to include #idea")
	}
}

func TestNoteMerge_StripTitle(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"merge", "uuid-target", "uuid-source", "--strip-title", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	target, _ := ResolveNote(vlt, "uuid-target")
	if strings.Contains(target.Content, "# Source Note") {
		t.Error("source title should be stripped from merged content")
	}
	if !strings.Contains(target.Content, "Source content to merge.") {
		t.Error("source content should still be present")
	}
}

func TestNoteMerge_DeleteSource(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// Get source path before merge
	source, _ := ResolveNote(vlt, "uuid-source")
	sourcePath := source.FilePath

	cmd.SetArgs([]string{"merge", "uuid-target", "uuid-source", "--delete-source", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error = %v", err)
	}

	// Source file should be deleted
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Error("source file should be deleted")
	}
}

func TestNoteMerge_SelfMerge(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := false
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	cmd.SetArgs([]string{"merge", "uuid-target", "uuid-target", "--force"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected self-merge error")
	}
	if !containsSubstr(err.Error(), "itself") {
		t.Errorf("error = %q, want self-merge error", err.Error())
	}
}

func TestNoteMerge_ReparentChildren(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := true
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"merge", "uuid-target", "uuid-source", "--force"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result noteMergeOutput
	json.Unmarshal(buf.Bytes(), &result)

	if result.ChildrenMoved != 1 {
		t.Errorf("children_moved = %d, want 1", result.ChildrenMoved)
	}

	// Verify child was reparented
	child, _ := ResolveNote(vlt, "uuid-child-src")
	if child.Parent != "uuid-target" {
		t.Errorf("child parent = %q, want uuid-target", child.Parent)
	}
}

func TestNoteMerge_DryRun(t *testing.T) {
	vlt := setupNoteTestVault(t)
	jsonOut := true
	cmd := NewNoteCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"merge", "uuid-target", "uuid-source", "--dry-run"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result noteMergeOutput
	json.Unmarshal(buf.Bytes(), &result)

	// In dry-run, nothing should actually change
	target, _ := ResolveNote(vlt, "uuid-target")
	if strings.Contains(target.Content, "Source content to merge.") {
		t.Error("dry-run should not modify target")
	}
}

// --- helper tests ---

func TestInsertGlobalTag_ExistingTagLine(t *testing.T) {
	content := "# Title\n#daily, #work\n\nSome text."
	result := insertGlobalTag(content, "#urgent")

	if !strings.Contains(result, "#daily, #work, #urgent") {
		t.Errorf("expected comma-separated tag append, got:\n%s", result)
	}
}

func TestInsertGlobalTag_SpaceSeparated(t *testing.T) {
	content := "# Title\n#daily #work\n\nSome text."
	result := insertGlobalTag(content, "#urgent")

	if !strings.Contains(result, "#daily #work #urgent") {
		t.Errorf("expected space-separated tag append, got:\n%s", result)
	}
}

func TestInsertGlobalTag_NoExistingTags(t *testing.T) {
	content := "# Title\n\nSome text."
	result := insertGlobalTag(content, "#new")

	lines := strings.Split(result, "\n")
	if lines[1] != "#new" {
		t.Errorf("expected #new after title, got:\n%s", result)
	}
}

func TestRemoveTagClean_CommaSeparated(t *testing.T) {
	content := "#daily, #work, #urgent"
	result := removeTagClean(content, "#work")

	// Should not have double commas or leading/trailing commas
	if strings.Contains(result, ",,") || strings.Contains(result, ", ,") {
		t.Errorf("leftover separators in: %q", result)
	}
	if strings.Contains(result, "#work") {
		t.Errorf("tag #work should be removed from: %q", result)
	}
}

func TestRemoveTagClean_EmptyLineRemoval(t *testing.T) {
	content := "# Title\n#only\n\nSome text."
	result := removeTagClean(content, "#only")

	// The line that was only #only should be removed
	if strings.Contains(result, "\n\n\n") {
		t.Errorf("empty lines should be cleaned up: %q", result)
	}
}

func TestDetectSeparator(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"#a, #b", ", "},
		{"#a #b", " "},
		{"#a,#b", ", "}, // comma without space normalizes to ", "
		{"#single", " "},
	}

	for _, tt := range tests {
		got := detectSeparator(tt.line)
		if got != tt.want {
			t.Errorf("detectSeparator(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}
