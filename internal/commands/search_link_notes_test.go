package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

func TestSearchCmd_NotesFlag(t *testing.T) {
	vlt := setupTestVault(t)

	// Build titles index so UUID resolution works
	for _, n := range []struct {
		uuid, title, filename string
	}{
		{"uuid-1", "Daily Note 1", "note1.md"},
		{"uuid-2", "Daily Note 2", "note2.md"},
		{"uuid-3", "Project Alpha", "note3.md"},
		{"uuid-4", "Alpha Sub-task", "note4.md"},
		{"uuid-5", "Orphan Idea", "note5.md"},
	} {
		vlt.UpdateTitleEntry(n.uuid, n.title, filepath.Join(vlt.Path, n.filename), "")
	}

	t.Run("notes constrains results", func(t *testing.T) {
		jsonOut := true
		cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"--everything", "--notes", "uuid-1,uuid-3"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []struct {
			UUID string `json:"uuid"`
		}
		if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v\noutput: %s", err, buf.String())
		}

		if len(results) != 2 {
			t.Fatalf("got %d results, want 2", len(results))
		}

		uuids := make(map[string]bool)
		for _, r := range results {
			uuids[r.UUID] = true
		}
		if !uuids["uuid-1"] || !uuids["uuid-3"] {
			t.Errorf("expected uuid-1 and uuid-3, got %v", uuids)
		}
	})

	t.Run("notes with query", func(t *testing.T) {
		jsonOut := true
		cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// uuid-1 has #daily, uuid-3 has #project — only uuid-1 should match
		cmd.SetArgs([]string{"#daily", "--notes", "uuid-1,uuid-3"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []struct {
			UUID string `json:"uuid"`
		}
		json.Unmarshal(buf.Bytes(), &results)

		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].UUID != "uuid-1" {
			t.Errorf("got UUID %q, want uuid-1", results[0].UUID)
		}
	})

	t.Run("unknown UUIDs ignored", func(t *testing.T) {
		jsonOut := true
		cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"--everything", "--notes", "uuid-nonexistent,uuid-1"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []struct {
			UUID string `json:"uuid"`
		}
		json.Unmarshal(buf.Bytes(), &results)

		if len(results) != 1 {
			t.Errorf("got %d results, want 1 (unknown UUID ignored)", len(results))
		}
	})
}

func TestSearchCmd_LinkFlag(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	linkNote := `---
uuid: uuid-link1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
url: "https://go.dev/blog/go1.22"
tags:
  - "#link"
---
# Go 1.22
#link

https://go.dev/blog/go1.22
`
	linkNote2 := `---
uuid: uuid-link2
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
url: "https://example.com/article"
tags:
  - "#link"
  - "#project"
---
# Example Article
#link #project

https://example.com/article
`
	plainNote := `---
uuid: uuid-plain1
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
tags:
  - "#daily"
---
# Daily Note
#daily

Just a regular note.
`

	for name, content := range map[string]string{
		"link1.md": linkNote,
		"link2.md": linkNote2,
		"plain.md": plainNote,
	} {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test note: %v", err)
		}
	}

	t.Run("link without query", func(t *testing.T) {
		jsonOut := true
		cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd.SetArgs([]string{"--link"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []struct {
			UUID string `json:"uuid"`
		}
		json.Unmarshal(buf.Bytes(), &results)

		if len(results) != 2 {
			t.Errorf("got %d results, want 2 (both link notes)\noutput: %s", len(results), buf.String())
		}
	})

	t.Run("link with query", func(t *testing.T) {
		jsonOut := true
		cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Only link notes with #project tag
		cmd.SetArgs([]string{"--link", "#project"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []struct {
			UUID string `json:"uuid"`
		}
		json.Unmarshal(buf.Bytes(), &results)

		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].UUID != "uuid-link2" {
			t.Errorf("got UUID %q, want uuid-link2", results[0].UUID)
		}
	})

	t.Run("link combined with notes", func(t *testing.T) {
		// Build titles index
		vlt.UpdateTitleEntry("uuid-link1", "Go 1.22", filepath.Join(tmpDir, "link1.md"), "")
		vlt.UpdateTitleEntry("uuid-link2", "Example Article", filepath.Join(tmpDir, "link2.md"), "")
		vlt.UpdateTitleEntry("uuid-plain1", "Daily Note", filepath.Join(tmpDir, "plain.md"), "")

		jsonOut := true
		cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// --link + --notes: only link notes within the UUID set
		cmd.SetArgs([]string{"--link", "--notes", "uuid-link1,uuid-plain1"})
		err := cmd.Execute()

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var results []struct {
			UUID string `json:"uuid"`
		}
		json.Unmarshal(buf.Bytes(), &results)

		// uuid-plain1 is not a link note, so only uuid-link1 should match
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].UUID != "uuid-link1" {
			t.Errorf("got UUID %q, want uuid-link1", results[0].UUID)
		}
	})
}

func TestLinkNoteMatcher(t *testing.T) {
	matcher := linkNoteMatcher()

	// Note with URL in frontmatter
	n1 := &note.Note{URL: "https://example.com"}
	if !matcher(n1) {
		t.Error("expected match for note with URL")
	}

	// Note without URL
	n2 := &note.Note{}
	if matcher(n2) {
		t.Error("expected no match for note without URL")
	}
}

func TestLinkFilter_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	// Create a link note with url frontmatter
	linkNote := `---
uuid: uuid-link1
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
url: "https://go.dev/blog/go1.22"
tags:
  - "#link"
---
# Go 1.22
#link

https://go.dev/blog/go1.22
`
	// Create a non-link note
	plainNote := `---
uuid: uuid-plain1
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
tags:
  - "#daily"
---
# Daily Note
#daily

Just a regular note.
`

	for name, content := range map[string]string{
		"link-note.md":  linkNote,
		"plain-note.md": plainNote,
	} {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test note: %v", err)
		}
	}

	jsonOut := true
	cmd := NewSearchCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"link:go.dev"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var results []json.RawMessage
	json.Unmarshal(buf.Bytes(), &results)

	if len(results) != 1 {
		t.Errorf("got %d results, want 1\noutput: %s", len(results), buf.String())
	}
}
