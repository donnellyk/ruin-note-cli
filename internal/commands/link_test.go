package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

func TestSanitizeTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Simple Title", "Simple Title"},
		{"Title\nWith\nNewlines", "Title With Newlines"},
		{"Title\r\nWith\r\nCRLF", "Title With CRLF"},
		{"  Extra   Spaces  ", "Extra Spaces"},
		{"Mixed\n  Whitespace\r\n  Issues  ", "Mixed Whitespace Issues"},
		{"", ""},
		{"NoChange", "NoChange"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeTitle(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWarnDuplicateURL(t *testing.T) {
	vlt := setupTestVault(t)

	// Create a note with a URL
	n, err := note.Parse("https://example.com\n\n#link\n")
	if err != nil {
		t.Fatalf("failed to parse note: %v", err)
	}
	n.URL = "https://example.com"
	n.EnsureUUID()
	n.SetTimestamps()
	n.RefreshTags()
	n.FilePath = filepath.Join(vlt.Path, "link-note.md")
	if err := n.Save(); err != nil {
		t.Fatalf("failed to save note: %v", err)
	}

	// warnDuplicateURL should not panic and should write to stderr for duplicates
	// (we just verify it doesn't crash; stderr output is a side effect)
	warnDuplicateURL(vlt, "https://example.com", "different-uuid")

	// With the same UUID excluded, no warning should be emitted
	warnDuplicateURL(vlt, "https://example.com", n.UUID)

	// Non-matching URL should not warn
	warnDuplicateURL(vlt, "https://other.com", "any-uuid")
}

func TestLinkNewCreatesNote(t *testing.T) {
	vlt := setupTestVault(t)
	jsonOut := false

	cmd := NewLinkCmd(func() *vault.Vault { return vlt }, &jsonOut)

	// Use --no-fetch to avoid network calls
	cmd.SetArgs([]string{"new", "--no-fetch", "https://example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link new failed: %v", err)
	}

	// Verify a note was created
	notes, err := vlt.ListNotes()
	if err != nil {
		t.Fatalf("failed to list notes: %v", err)
	}

	var found bool
	for _, p := range notes {
		n, err := note.Load(p)
		if err != nil {
			continue
		}
		if n.URL == "https://example.com" {
			found = true
			// Verify it has the #link tag
			hasLink := false
			for _, tag := range n.AllTags() {
				if note.NormalizeStored(tag) == "link" {
					hasLink = true
					break
				}
			}
			if !hasLink {
				t.Error("link note missing #link tag")
			}
			// Verify the URL is in the content
			if !strings.Contains(n.Content, "https://example.com") {
				t.Error("link note content doesn't contain URL")
			}
			break
		}
	}
	if !found {
		t.Error("no note with URL https://example.com was created")
	}
}

func TestLinkNewWithTitle(t *testing.T) {
	vlt := setupTestVault(t)
	jsonOut := false

	cmd := NewLinkCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"new", "--no-fetch", "--title", "My Bookmark", "https://example.com/page"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link new with title failed: %v", err)
	}

	notes, err := vlt.ListNotes()
	if err != nil {
		t.Fatalf("failed to list notes: %v", err)
	}

	var found bool
	for _, p := range notes {
		n, err := note.Load(p)
		if err != nil {
			continue
		}
		if n.URL == "https://example.com/page" {
			found = true
			if n.Title != "My Bookmark" {
				t.Errorf("expected title 'My Bookmark', got %q", n.Title)
			}
			// Verify filename uses the title (--h1 is implicitly true in createNote call)
			base := filepath.Base(p)
			if !strings.Contains(base, "My Bookmark") {
				t.Errorf("expected filename to contain 'My Bookmark', got %q", base)
			}
			break
		}
	}
	if !found {
		t.Error("no note with URL https://example.com/page was created")
	}
}

func TestLinkNewWithTags(t *testing.T) {
	vlt := setupTestVault(t)
	jsonOut := false

	cmd := NewLinkCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"new", "--no-fetch", "--tags", "golang,reading", "https://go.dev"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link new with tags failed: %v", err)
	}

	notes, err := vlt.ListNotes()
	if err != nil {
		t.Fatalf("failed to list notes: %v", err)
	}

	for _, p := range notes {
		n, err := note.Load(p)
		if err != nil {
			continue
		}
		if n.URL == "https://go.dev" {
			allTags := n.AllTags()
			tagSet := make(map[string]bool)
			for _, tag := range allTags {
				tagSet[note.NormalizeStored(tag)] = true
			}
			for _, expected := range []string{"link", "golang", "reading"} {
				if !tagSet[expected] {
					t.Errorf("missing expected tag #%s, got tags: %v", expected, allTags)
				}
			}
			return
		}
	}
	t.Error("no note with URL https://go.dev was created")
}

func TestLinkNewWithComment(t *testing.T) {
	vlt := setupTestVault(t)
	jsonOut := false

	cmd := NewLinkCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"new", "--no-fetch", "--comment", "Great article about Go", "https://go.dev/blog"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link new with comment failed: %v", err)
	}

	notes, err := vlt.ListNotes()
	if err != nil {
		t.Fatalf("failed to list notes: %v", err)
	}

	for _, p := range notes {
		n, err := note.Load(p)
		if err != nil {
			continue
		}
		if n.URL == "https://go.dev/blog" {
			if !strings.Contains(n.Content, "Great article about Go") {
				t.Error("link note content doesn't contain comment")
			}
			return
		}
	}
	t.Error("no note with URL https://go.dev/blog was created")
}

func TestLinkNewInvalidURL(t *testing.T) {
	vlt := setupTestVault(t)
	jsonOut := false

	cmd := NewLinkCmd(func() *vault.Vault { return vlt }, &jsonOut)
	// Suppress usage output during test
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"new", "--no-fetch", "not-a-url"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("expected 'invalid URL' error, got: %v", err)
	}
}

func TestLinkNewJSON(t *testing.T) {
	vlt := setupTestVault(t)
	jsonOut := true

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewLinkCmd(func() *vault.Vault { return vlt }, &jsonOut)
	cmd.SetArgs([]string{"new", "--no-fetch", "https://json-test.com"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("link new --json failed: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, `"url": "https://json-test.com"`) {
		t.Errorf("JSON output missing url field, got: %s", output)
	}
	if !strings.Contains(output, `"path"`) {
		t.Errorf("JSON output missing path field, got: %s", output)
	}
	if !strings.Contains(output, `"uuid"`) {
		t.Errorf("JSON output missing uuid field, got: %s", output)
	}
}

func TestLinkList(t *testing.T) {
	vlt := setupTestVault(t)
	jsonOut := false

	// Create a link note first
	linkCmd := NewLinkCmd(func() *vault.Vault { return vlt }, &jsonOut)
	linkCmd.SetArgs([]string{"new", "--no-fetch", "https://list-test.com"})
	if err := linkCmd.Execute(); err != nil {
		t.Fatalf("failed to create link note: %v", err)
	}

	// Now list link notes
	// Capture stdout to verify output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	listCmd := NewLinkCmd(func() *vault.Vault { return vlt }, &jsonOut)
	listCmd.SetArgs([]string{"list"})
	err := listCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("link list failed: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, ".md") {
		t.Errorf("link list output should contain .md file paths, got: %s", output)
	}
}

func TestLinkList_FindsByURL(t *testing.T) {
	tmpDir := t.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		t.Fatalf("failed to initialize vault: %v", err)
	}

	// Note with URL in frontmatter but NO #link tag
	urlNoTag := `---
uuid: uuid-url-notag
created: "2025-01-01T10:00:00-05:00"
updated: "2025-01-01T10:00:00-05:00"
url: "https://example.com/article"
tags:
  - "#reading"
---
# Example Article
#reading

https://example.com/article
`
	// Note with #link tag but NO url
	tagNoURL := `---
uuid: uuid-tag-nourl
created: "2025-01-02T10:00:00-05:00"
updated: "2025-01-02T10:00:00-05:00"
tags:
  - "#link"
---
# Miscategorized
#link

Not actually a link note.
`
	// Plain note — neither URL nor #link
	plain := `---
uuid: uuid-plain
created: "2025-01-03T10:00:00-05:00"
updated: "2025-01-03T10:00:00-05:00"
tags:
  - "#daily"
---
# Daily
#daily

Just a note.
`

	for name, content := range map[string]string{
		"url-notag.md": urlNoTag,
		"tag-nourl.md": tagNoURL,
		"plain.md":     plain,
	} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	jsonOut := true
	cmd := NewLinkCmd(func() *vault.Vault { return vlt }, &jsonOut)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"list"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("link list failed: %v", err)
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	// Should find the note with a URL but no #link tag
	if !strings.Contains(output, "uuid-url-notag") {
		t.Errorf("expected to find note with URL (no #link tag), got: %s", output)
	}
	// Should NOT find the note with #link tag but no URL
	if strings.Contains(output, "uuid-tag-nourl") {
		t.Errorf("should not find note with #link tag but no URL, got: %s", output)
	}
	// Should NOT find the plain note
	if strings.Contains(output, "uuid-plain") {
		t.Errorf("should not find plain note, got: %s", output)
	}
}
