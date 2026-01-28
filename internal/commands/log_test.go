package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevin/ruin-note-cli/internal/note"
	"github.com/kevin/ruin-note-cli/internal/vault"
)

func TestLogCmd_BasicContent(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	jsonOut := false
	cmd := NewLogCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"Quick thought #idea"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	// Should output a file path
	if !strings.HasSuffix(output, ".md") {
		t.Errorf("output = %q, want .md file path", output)
	}

	// File should exist
	if _, err := os.Stat(output); err != nil {
		t.Errorf("file not created: %v", err)
	}

	// Read and verify content
	data, _ := os.ReadFile(output)
	content := string(data)

	if !strings.Contains(content, "uuid:") {
		t.Error("note should have UUID in frontmatter")
	}
	if !strings.Contains(content, "#idea") {
		t.Error("note should contain the tag")
	}
}

func TestLogCmd_WithTitle(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.Initialize(false)

	jsonOut := false
	cmd := NewLogCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--title", "My Custom Title", "Some content"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	expectedPath := filepath.Join(tmpDir, "My Custom Title.md")
	if output != expectedPath {
		t.Errorf("output = %q, want %q", output, expectedPath)
	}
}

func TestLogCmd_WithH1(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.Initialize(false)

	jsonOut := false
	cmd := NewLogCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	content := "# My H1 Title\n\nSome content here."
	cmd.SetArgs([]string{"--h1", content})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	expectedPath := filepath.Join(tmpDir, "My H1 Title.md")
	if output != expectedPath {
		t.Errorf("output = %q, want %q", output, expectedPath)
	}
}

func TestLogCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.Initialize(false)

	jsonOut := true
	cmd := NewLogCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--title", "JSON Test", "Content"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var output LogOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if output.UUID == "" {
		t.Error("JSON output should have UUID")
	}
	if !strings.HasSuffix(output.Path, "JSON Test.md") {
		t.Errorf("Path = %q, want to end with 'JSON Test.md'", output.Path)
	}
}

func TestLogCmd_UpdatesTagsIndex(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.Initialize(false)

	jsonOut := false
	cmd := NewLogCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--title", "Tagged Note", "Content with #tag1 and #tag2"})
	cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	// Check tags index
	index, err := vlt.LoadTags()
	if err != nil {
		t.Fatalf("failed to load tags: %v", err)
	}

	if len(index.Tags) != 2 {
		t.Errorf("Tags count = %d, want 2", len(index.Tags))
	}

	// Verify tag names
	tagNames := make(map[string]bool)
	for _, tag := range index.Tags {
		tagNames[tag.Name] = true
	}

	if !tagNames["#tag1"] {
		t.Error("tags index should contain #tag1")
	}
	if !tagNames["#tag2"] {
		t.Error("tags index should contain #tag2")
	}
}

func TestLogCmd_NoContent(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.Initialize(false)

	jsonOut := false
	cmd := NewLogCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// Simulate terminal (no stdin pipe) by not providing args
	// This should error because no content is provided
	cmd.SetArgs([]string{})

	// We need to make stdin appear as a terminal for this test
	// Since we can't easily do that, we'll test with empty string arg
	cmd.SetArgs([]string{""})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestLogCmd_FileAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	vlt.Initialize(false)

	// Create a file first
	existingPath := filepath.Join(tmpDir, "Existing.md")
	os.WriteFile(existingPath, []byte("existing content"), 0644)

	jsonOut := false
	cmd := NewLogCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"--title", "Existing", "New content"})

	err := cmd.Execute()

	if err == nil {
		t.Error("expected error when file already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want to contain 'already exists'", err.Error())
	}
}

func TestDetermineFilename(t *testing.T) {
	tests := []struct {
		name      string
		noteTitle string
		titleFlag string
		useH1     bool
		wantMatch string // substring to match
	}{
		{
			name:      "title flag takes precedence",
			noteTitle: "H1 Title",
			titleFlag: "Flag Title",
			useH1:     true,
			wantMatch: "Flag Title",
		},
		{
			name:      "h1 when flag empty",
			noteTitle: "H1 Title",
			titleFlag: "",
			useH1:     true,
			wantMatch: "H1 Title",
		},
		{
			name:      "h1 flag false ignores title",
			noteTitle: "H1 Title",
			titleFlag: "",
			useH1:     false,
			wantMatch: "20", // timestamp starts with year
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &note.Note{Title: tt.noteTitle}
			n.SetTimestamps()

			got := determineFilename(n, tt.titleFlag, tt.useH1)

			if !strings.Contains(got, tt.wantMatch) {
				t.Errorf("determineFilename() = %q, want to contain %q", got, tt.wantMatch)
			}
		})
	}
}
