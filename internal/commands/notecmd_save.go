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
		// Full Load fallback — LoadFrontmatterOnly no longer returns tag
		// fields after v0.4.0. The titles entry is the fast path; the loader
		// is the authoritative fallback when a parent's mirror is empty.
		loader := func(path string) (*note.Note, error) {
			return note.Load(path)
		}
		n.InheritedTags = ComputeInheritedTags(n.UUID, titlesIndex, loader)
	}

	filename := determineFilename(n, titleFlag, useH1)
	n.FilePath = filepath.Join(vlt.Path, filename+".md")

	if _, err := os.Stat(n.FilePath); err == nil {
		return fmt.Errorf("file already exists: %s", n.FilePath)
	}

	if err := saveNoteForVault(n, vlt); err != nil {
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

	if err := saveNoteForVault(n, vlt); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	if !normalizedTagsEqual(oldGlobalTags, n.Tags) {
		// Refresh the titles mirror BEFORE cascade so descendants compute
		// inheritance against the just-saved tag set, not the pre-edit one.
		if err := vlt.UpdateTitleEntryFull(n.UUID, n.Title, n.FilePath, n.Parent, n.Tags, n.InlineTags, n.InheritedTags, n.Aliases); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update titles mirror before cascade: %v\n", err)
		}
		if titlesIndex, err := vlt.LoadTitles(); err == nil {
			if err := CascadeInheritedTags(n.UUID, vlt, titlesIndex); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to cascade inherited tags: %v\n", err)
			}
		}
	}

	return nil
}
