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
	fmLen := len(serialized) - len(n.Content)
	fm := serialized[:fmLen]
	return strings.Count(fm, "\n"), nil
}

// createNote performs the shared create-note pipeline. Callers must set
// Content, URL, Parent, Order, and Title before calling.
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

// saveWithIndexUpdate refreshes derived metadata and saves the file. Callers
// must follow this with vlt.SaveNote() to update indexes.
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

	if _, err := RefreshInheritedTags(n, vlt); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to refresh inherited tags: %v\n", err)
	}

	n.SetTimestamps()

	if err := n.Save(); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	if !normalizedTagsEqual(oldGlobalTags, n.Tags) {
		if titlesIndex, err := vlt.LoadTitles(); err == nil {
			if err := CascadeInheritedTags(n.UUID, vlt, titlesIndex); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to cascade inherited tags: %v\n", err)
			}
		}
	}

	return nil
}
