package commands

import (
	"fmt"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

const maxInheritanceDepth = 100

// ComputeInheritedTags walks the ancestor chain via the titles index and
// collects global tags from all ancestors. Returns deduplicated tags.
// Uses a depth limit and visited set for cycle detection.
func ComputeInheritedTags(noteUUID string, titlesIndex *vault.TitlesIndex, loader func(path string) (*note.Note, error)) []string {
	entry, ok := titlesIndex.Titles[noteUUID]
	if !ok || entry.Parent == "" {
		return nil
	}

	seen := make(map[string]bool)
	seen[noteUUID] = true

	var inherited []string
	tagSeen := make(map[string]bool)

	currentUUID := entry.Parent
	for range maxInheritanceDepth {
		if seen[currentUUID] {
			break // cycle detected
		}
		seen[currentUUID] = true

		parentEntry, ok := titlesIndex.Titles[currentUUID]
		if !ok {
			break // orphaned parent
		}

		parentNote, err := loader(parentEntry.Path)
		if err != nil {
			break
		}

		for _, t := range parentNote.Tags {
			norm := note.NormalizeStored(t)
			if !tagSeen[norm] {
				tagSeen[norm] = true
				inherited = append(inherited, t)
			}
		}

		if parentEntry.Parent == "" {
			break
		}
		currentUUID = parentEntry.Parent
	}

	return inherited
}

// RefreshInheritedTags computes and sets inherited tags for a single note.
// Returns true if the inherited tags changed.
// When tag inheritance is disabled, clears any existing inherited tags.
func RefreshInheritedTags(n *note.Note, vlt *vault.Vault) (bool, error) {
	if !vlt.TagInheritanceEnabled() {
		if len(n.InheritedTags) > 0 {
			n.InheritedTags = nil
			return true, nil
		}
		return false, nil
	}

	if n.Parent == "" {
		if len(n.InheritedTags) > 0 {
			n.InheritedTags = nil
			return true, nil
		}
		return false, nil
	}

	titlesIndex, err := vlt.LoadTitles()
	if err != nil {
		return false, fmt.Errorf("failed to load titles index: %w", err)
	}

	loader := func(path string) (*note.Note, error) {
		return note.LoadFrontmatterOnly(path)
	}

	newInherited := ComputeInheritedTags(n.UUID, titlesIndex, loader)
	if normalizedTagsEqual(n.InheritedTags, newInherited) {
		return false, nil
	}

	n.InheritedTags = newInherited
	return true, nil
}

// CascadeInheritedTags recomputes inherited tags for all descendants of the
// given parent UUID. It saves any notes whose inherited tags changed.
// No-op when tag inheritance is disabled.
func CascadeInheritedTags(parentUUID string, vlt *vault.Vault, titlesIndex *vault.TitlesIndex) error {
	if !vlt.TagInheritanceEnabled() {
		return nil
	}

	children := titlesIndex.ChildrenMap()

	queue := children[parentUUID]
	visited := make(map[string]bool)
	visited[parentUUID] = true

	loader := func(path string) (*note.Note, error) {
		return note.LoadFrontmatterOnly(path)
	}

	for len(queue) > 0 {
		uuid := queue[0]
		queue = queue[1:]

		if visited[uuid] {
			continue
		}
		visited[uuid] = true

		entry, ok := titlesIndex.Titles[uuid]
		if !ok {
			continue
		}

		newInherited := ComputeInheritedTags(uuid, titlesIndex, loader)

		n, err := note.Load(entry.Path)
		if err != nil {
			continue
		}

		if !normalizedTagsEqual(n.InheritedTags, newInherited) {
			n.InheritedTags = newInherited
			if err := n.Save(); err != nil {
				return fmt.Errorf("failed to save %s: %w", entry.Path, err)
			}
		}

		queue = append(queue, children[uuid]...)
	}

	return nil
}
