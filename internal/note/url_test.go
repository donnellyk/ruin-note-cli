package note

import (
	"strings"
	"testing"
)

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"https://example.com/path?q=1#frag", true},
		{"http://localhost:8080", true},
		{"ftp://example.com", false},
		{"not-a-url", false},
		{"", false},
		{"example.com", false},
		{"://missing-scheme", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidURL(tt.input)
			if got != tt.want {
				t.Errorf("IsValidURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsURLNote(t *testing.T) {
	t.Run("note with url frontmatter", func(t *testing.T) {
		n := &Note{URL: "https://example.com", Content: "# Title\nSome content"}
		if !n.IsURLNote() {
			t.Error("expected IsURLNote() = true for note with URL field")
		}
	})

	t.Run("note with body URL", func(t *testing.T) {
		n := &Note{Content: "# Title\nhttps://example.com\nSome content"}
		if !n.IsURLNote() {
			t.Error("expected IsURLNote() = true for note with body URL")
		}
	})

	t.Run("note without URL", func(t *testing.T) {
		n := &Note{Content: "# Title\nJust plain text"}
		if n.IsURLNote() {
			t.Error("expected IsURLNote() = false for note without URL")
		}
	})

	t.Run("note with URL after non-URL content", func(t *testing.T) {
		n := &Note{Content: "# Title\nSome text\nhttps://example.com"}
		if n.IsURLNote() {
			t.Error("expected IsURLNote() = false when URL is not the first content line")
		}
	})
}

func TestExtractURL(t *testing.T) {
	t.Run("frontmatter URL takes precedence", func(t *testing.T) {
		n := &Note{
			URL:     "https://frontmatter.com",
			Content: "# Title\nhttps://body.com",
		}
		got := n.ExtractURL()
		if got != "https://frontmatter.com" {
			t.Errorf("ExtractURL() = %q, want %q", got, "https://frontmatter.com")
		}
	})

	t.Run("body URL detection", func(t *testing.T) {
		n := &Note{Content: "# Title\nhttps://example.com\nMore text"}
		got := n.ExtractURL()
		if got != "https://example.com" {
			t.Errorf("ExtractURL() = %q, want %q", got, "https://example.com")
		}
	})

	t.Run("body URL after tags", func(t *testing.T) {
		n := &Note{Content: "# Title\n#tag1 #tag2\nhttps://example.com\nMore text"}
		got := n.ExtractURL()
		if got != "https://example.com" {
			t.Errorf("ExtractURL() = %q, want %q", got, "https://example.com")
		}
	})

	t.Run("no URL", func(t *testing.T) {
		n := &Note{Content: "# Title\nJust text"}
		got := n.ExtractURL()
		if got != "" {
			t.Errorf("ExtractURL() = %q, want empty", got)
		}
	})

	t.Run("non-URL first content line", func(t *testing.T) {
		n := &Note{Content: "# Title\nSome text\nhttps://example.com"}
		got := n.ExtractURL()
		if got != "" {
			t.Errorf("ExtractURL() = %q, want empty (URL not on first content line)", got)
		}
	})
}

func TestEnsureLinkTag(t *testing.T) {
	t.Run("non-URL note returns false", func(t *testing.T) {
		n := &Note{Content: "# Title\nJust text"}
		if n.EnsureLinkTag() {
			t.Error("EnsureLinkTag() should return false for non-URL note")
		}
	})

	t.Run("URL note gets #link added to existing tag line", func(t *testing.T) {
		n := &Note{
			URL:     "https://example.com",
			Content: "# Title\n#tag1 #tag2\nSome content",
			Tags:    []string{"#tag1", "#tag2"},
		}
		if !n.EnsureLinkTag() {
			t.Error("EnsureLinkTag() should return true when #link is added")
		}
		if !strings.Contains(n.Content, "#link") {
			t.Error("content should contain #link tag")
		}
		// Tag line should have #link appended
		lines := strings.Split(n.Content, "\n")
		if lines[1] != "#tag1 #tag2 #link" {
			t.Errorf("tag line = %q, want %q", lines[1], "#tag1 #tag2 #link")
		}
	})

	t.Run("URL note gets #link added after title when no tag line", func(t *testing.T) {
		n := &Note{
			URL:     "https://example.com",
			Content: "# Title\nSome content",
		}
		if !n.EnsureLinkTag() {
			t.Error("EnsureLinkTag() should return true when #link is added")
		}
		if !strings.Contains(n.Content, "#link") {
			t.Error("content should contain #link tag")
		}
		lines := strings.Split(n.Content, "\n")
		// Should be: # Title, "", #link, Some content
		if len(lines) < 4 {
			t.Fatalf("expected at least 4 lines, got %d: %v", len(lines), lines)
		}
		if lines[1] != "" {
			t.Errorf("line 1 = %q, want empty", lines[1])
		}
		if lines[2] != "#link" {
			t.Errorf("line 2 = %q, want %q", lines[2], "#link")
		}
	})

	t.Run("idempotent: already has #link returns false", func(t *testing.T) {
		n := &Note{
			URL:     "https://example.com",
			Content: "# Title\n#link #tag1\nSome content",
			Tags:    []string{"#link", "#tag1"},
		}
		if n.EnsureLinkTag() {
			t.Error("EnsureLinkTag() should return false when #link already present")
		}
	})

	t.Run("idempotent: #link in inline tags", func(t *testing.T) {
		n := &Note{
			URL:        "https://example.com",
			Content:    "# Title\nSome content with #link",
			InlineTags: []string{"#link"},
		}
		if n.EnsureLinkTag() {
			t.Error("EnsureLinkTag() should return false when #link in inline tags")
		}
	})

	t.Run("URL promotion: body URL promoted to n.URL field", func(t *testing.T) {
		n := &Note{
			Content: "# Title\n#tag1\nhttps://example.com\nSome content",
			Tags:    []string{"#tag1"},
		}
		if !n.EnsureLinkTag() {
			t.Error("EnsureLinkTag() should return true for URL promotion")
		}
		if n.URL != "https://example.com" {
			t.Errorf("n.URL = %q, want %q", n.URL, "https://example.com")
		}
	})

	t.Run("comma-separated tags use comma separator", func(t *testing.T) {
		n := &Note{
			URL:     "https://example.com",
			Content: "# Title\n#tag1, #tag2\nSome content",
			Tags:    []string{"#tag1", "#tag2"},
		}
		if !n.EnsureLinkTag() {
			t.Error("EnsureLinkTag() should return true when #link is added")
		}
		lines := strings.Split(n.Content, "\n")
		if lines[1] != "#tag1, #tag2, #link" {
			t.Errorf("tag line = %q, want %q", lines[1], "#tag1, #tag2, #link")
		}
	})

	t.Run("content with #linkedin does not prevent #link from being added", func(t *testing.T) {
		n := &Note{
			URL:     "https://example.com",
			Content: "# Title\n#linkedin\nSome content",
			Tags:    []string{"#linkedin"},
		}
		if !n.EnsureLinkTag() {
			t.Error("EnsureLinkTag() should return true when #link is missing (only #linkedin present)")
		}
		if !strings.Contains(n.Content, "#link") {
			t.Error("content should contain #link tag")
		}
		lines := strings.Split(n.Content, "\n")
		if lines[1] != "#linkedin #link" {
			t.Errorf("tag line = %q, want %q", lines[1], "#linkedin #link")
		}
	})

	t.Run("no title no tags appends at end", func(t *testing.T) {
		n := &Note{
			URL:     "https://example.com",
			Content: "Just some text without a header",
		}
		if !n.EnsureLinkTag() {
			t.Error("EnsureLinkTag() should return true when #link is added")
		}
		if !strings.HasSuffix(n.Content, "\n#link") {
			t.Errorf("content should end with #link, got: %q", n.Content)
		}
	})
}
