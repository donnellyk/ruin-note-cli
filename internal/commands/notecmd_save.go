package commands

import (
	"fmt"
	"os"
	"strings"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
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

// saveWithIndexUpdate performs the shared note-level post-modification flow:
// refresh tags/dates/links, set timestamps, and save the file.
// Callers should follow this with vlt.SaveNote() for index updates.
func saveWithIndexUpdate(n *note.Note, vlt *vault.Vault) error {
	oldGlobalTags := make([]string, len(n.Tags))
	copy(oldGlobalTags, n.Tags)

	n.RefreshTags()
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
