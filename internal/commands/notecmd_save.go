package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

// frontmatterLineCount returns the number of lines the frontmatter occupies
// in the serialized file (including the --- delimiters and trailing newline).
func frontmatterLineCount(n *note.Note) (int, error) {
	serialized, err := n.Serialize()
	if err != nil {
		return 0, err
	}
	// Content starts where frontmatter ends. Count lines in the frontmatter portion.
	fmLen := len(serialized) - len(n.Content)
	fm := serialized[:fmLen]
	// Count newlines in the frontmatter string
	return strings.Count(fm, "\n"), nil
}

// createNote performs the shared create-note pipeline:
// EnsureUUID, SetTimestamps, save pipeline, filename generation, file write, and index creation.
// The note's Content, URL, Parent, Order, and Title should be set by the caller before calling this.
func createNote(n *note.Note, vlt *vault.Vault, titleFlag string, useH1 bool) error {
	n.EnsureUUID()
	n.SetTimestamps()

	n.RefreshTags()
	if n.EnsureLinkTag() {
		n.RefreshTags()
	}
	n.Content = note.ResolveDateTokens(n.Content)
	n.RefreshDates()

	titlesIndex, titlesErr := vlt.LoadTitles()
	if titlesErr == nil {
		RefreshLinkedCards(n, titlesIndex)
	} else {
		fmt.Fprintf(os.Stderr, "warning: failed to load titles index for linked-cards: %v\n", titlesErr)
	}

	if n.Parent != "" && titlesErr == nil && vlt.TagInheritanceEnabled() {
		// The note isn't in the titles index yet (CreateNote adds it later),
		// so add a temporary entry so ComputeInheritedTags can find the parent.
		titlesIndex.Titles[n.UUID] = vault.TitleEntry{Parent: n.Parent}
		loader := func(path string) (*note.Note, error) {
			return note.LoadFrontmatterOnly(path)
		}
		n.InheritedTags = ComputeInheritedTags(n.UUID, titlesIndex, loader)
	}

	filename := determineFilename(n, titleFlag, useH1)
	n.FilePath = filepath.Join(vlt.Path, filename+".md")

	if _, err := os.Stat(n.FilePath); err == nil {
		return fmt.Errorf("file already exists: %s", n.FilePath)
	}

	if err := n.Save(); err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	vlt.CreateNote(n, fmt.Sprintf("ruin: Create %q", filename))
	return nil
}

// saveWithIndexUpdate performs the shared note-level post-modification flow:
// refresh tags/dates/links, set timestamps, and save the file.
// Callers should follow this with vlt.SaveNote() for index updates.
func saveWithIndexUpdate(n *note.Note, vlt *vault.Vault) error {
	oldGlobalTags := make([]string, len(n.Tags))
	copy(oldGlobalTags, n.Tags)

	n.RefreshTags()
	if n.EnsureLinkTag() {
		n.RefreshTags()
	}
	n.Content = note.ResolveDateTokens(n.Content)
	n.RefreshDates()

	titlesIndex, err := vlt.LoadTitles()
	if err == nil {
		RefreshLinkedCards(n, titlesIndex)
	}

	// Refresh inherited tags from parent chain
	if _, err := RefreshInheritedTags(n, vlt); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to refresh inherited tags: %v\n", err)
	}

	n.SetTimestamps()

	if err := n.Save(); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	// If global tags changed, cascade to descendants
	if !normalizedTagsEqual(oldGlobalTags, n.Tags) {
		if titlesIndex, err := vlt.LoadTitles(); err == nil {
			if err := CascadeInheritedTags(n.UUID, vlt, titlesIndex); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to cascade inherited tags: %v\n", err)
			}
		}
	}

	return nil
}
