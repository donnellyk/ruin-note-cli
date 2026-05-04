package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

func setupSuggestVault(t *testing.T) *vault.Vault {
	t.Helper()

	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	// Create notes with aliases
	note1 := `---
uuid: uuid-1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
aliases:
  - "Old Title"
  - "Alternative"
---
# Sprint Planning
Content here.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "sprint.md"), []byte(note1), 0644); err != nil {
		t.Fatalf("failed to write note1: %v", err)
	}

	note2 := `---
uuid: uuid-2
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
aliases:
  - "Arch Review"
---
# Architecture Meeting
Content here.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "arch.md"), []byte(note2), 0644); err != nil {
		t.Fatalf("failed to write note2: %v", err)
	}

	note3 := `---
uuid: uuid-3
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
---
# Project Proposal
Content here.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "project.md"), []byte(note3), 0644); err != nil {
		t.Fatalf("failed to write note3: %v", err)
	}

	// Seed titles index with aliases
	if err := vlt.UpdateTitleEntryFull("uuid-1", "Sprint Planning", filepath.Join(tmpDir, "sprint.md"), "", nil, nil, nil, []string{"Old Title", "Alternative"}); err != nil {
		t.Fatalf("failed to seed note1: %v", err)
	}
	if err := vlt.UpdateTitleEntryFull("uuid-2", "Architecture Meeting", filepath.Join(tmpDir, "arch.md"), "", nil, nil, nil, []string{"Arch Review"}); err != nil {
		t.Fatalf("failed to seed note2: %v", err)
	}
	if err := vlt.UpdateTitleEntry("uuid-3", "Project Proposal", filepath.Join(tmpDir, "project.md"), ""); err != nil {
		t.Fatalf("failed to seed note3: %v", err)
	}

	return vlt
}

func TestSuggestCmd_TitlePrefix(t *testing.T) {
	vlt := setupSuggestVault(t)

	jsonOut := false
	cmd := NewSuggestCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"sprint"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Sprint Planning") {
		t.Errorf("output missing 'Sprint Planning': %s", output)
	}
}

func TestSuggestCmd_CaseInsensitive(t *testing.T) {
	vlt := setupSuggestVault(t)

	jsonOut := false
	cmd := NewSuggestCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"SPRINT"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Sprint Planning") {
		t.Errorf("output should match case-insensitively: %s", output)
	}
}

func TestSuggestCmd_AliasMatching(t *testing.T) {
	vlt := setupSuggestVault(t)

	jsonOut := false
	cmd := NewSuggestCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"old"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "alias: Old Title") {
		t.Errorf("output missing alias notation: %s", output)
	}
}

func TestSuggestCmd_AliasCaseInsensitive(t *testing.T) {
	vlt := setupSuggestVault(t)

	jsonOut := false
	cmd := NewSuggestCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"alt"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Alternative") {
		t.Errorf("output missing alias match: %s", output)
	}
}

func TestSuggestCmd_LimitFlag(t *testing.T) {
	vlt := setupSuggestVault(t)

	jsonOut := false
	cmd := NewSuggestCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"proj", "--limit", "1"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) > 1 {
		t.Errorf("got %d results, want at most 1 due to --limit", len(lines))
	}
}

func TestSuggestCmd_JSONOutput(t *testing.T) {
	vlt := setupSuggestVault(t)

	jsonOut := true
	cmd := NewSuggestCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"arch"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var results []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results, got empty")
	}

	if results[0]["uuid"] != "uuid-2" {
		t.Errorf("uuid = %v, want uuid-2", results[0]["uuid"])
	}
	if results[0]["title"] != "Architecture Meeting" {
		t.Errorf("title = %v, want Architecture Meeting", results[0]["title"])
	}
}

func TestSuggestCmd_NoMatches(t *testing.T) {
	vlt := setupSuggestVault(t)

	jsonOut := false
	cmd := NewSuggestCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
}
