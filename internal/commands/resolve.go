package commands

import (
	"fmt"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

// ResolveNote resolves a note identifier to a loaded Note.
// Resolution order: saved parent bookmark, exact UUID match, title substring, path substring.
func ResolveNote(vlt *vault.Vault, identifier string) (*note.Note, error) {
	if bookmark, ok := vlt.LookupParent(identifier); ok {
		index, err := vlt.LoadTitles()
		if err == nil {
			if entry, ok := index.Titles[bookmark.UUID]; ok {
				n, err := note.Load(entry.Path)
				if err == nil {
					return n, nil
				}
			}
		}
	}

	index, err := vlt.LoadTitles()
	if err != nil {
		return resolveByVaultScan(vlt, identifier)
	}

	if entry, ok := index.Titles[identifier]; ok {
		n, err := note.Load(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("found UUID %s but failed to load %s: %w", identifier, entry.Path, err)
		}
		return n, nil
	}

	identLower := strings.ToLower(identifier)
	var titleMatches []matchCandidate
	for uuid, entry := range index.Titles {
		if strings.Contains(strings.ToLower(entry.Title), identLower) {
			titleMatches = append(titleMatches, matchCandidate{uuid: uuid, entry: entry})
		}
	}
	if len(titleMatches) == 1 {
		n, err := note.Load(titleMatches[0].entry.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", titleMatches[0].entry.Path, err)
		}
		return n, nil
	}
	if len(titleMatches) > 1 {
		return nil, ambiguousError(identifier, titleMatches)
	}

	var pathMatches []matchCandidate
	for uuid, entry := range index.Titles {
		if strings.Contains(strings.ToLower(entry.Path), identLower) {
			pathMatches = append(pathMatches, matchCandidate{uuid: uuid, entry: entry})
		}
	}
	if len(pathMatches) == 1 {
		n, err := note.Load(pathMatches[0].entry.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", pathMatches[0].entry.Path, err)
		}
		return n, nil
	}
	if len(pathMatches) > 1 {
		return nil, ambiguousError(identifier, pathMatches)
	}

	return nil, fmt.Errorf("no note found matching %q", identifier)
}

type matchCandidate struct {
	uuid  string
	entry vault.TitleEntry
}

func ambiguousError(identifier string, candidates []matchCandidate) error {
	var b strings.Builder
	fmt.Fprintf(&b, "ambiguous match for %q: %d notes matched", identifier, len(candidates))
	limit := min(len(candidates), 10)
	for i := range limit {
		c := candidates[i]
		fmt.Fprintf(&b, "\n  %s  %s", c.uuid, c.entry.Title)
	}
	if len(candidates) > 10 {
		fmt.Fprintf(&b, "\n  ... and %d more", len(candidates)-10)
	}
	return fmt.Errorf("%s", b.String())
}

func resolveByVaultScan(vlt *vault.Vault, identifier string) (*note.Note, error) {
	paths, err := vlt.ListNotes()
	if err != nil {
		return nil, err
	}

	identLower := strings.ToLower(identifier)
	var matches []*note.Note

	for _, path := range paths {
		n, err := note.Load(path)
		if err != nil {
			continue
		}

		if n.UUID == identifier {
			return n, nil
		}

		if strings.Contains(strings.ToLower(n.Title), identLower) ||
			strings.Contains(strings.ToLower(n.FilePath), identLower) {
			matches = append(matches, n)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		var candidates []matchCandidate
		for _, n := range matches {
			candidates = append(candidates, matchCandidate{
				uuid:  n.UUID,
				entry: vault.TitleEntry{Title: n.Title, Path: n.FilePath},
			})
		}
		return nil, ambiguousError(identifier, candidates)
	}

	return nil, fmt.Errorf("no note found matching %q", identifier)
}
