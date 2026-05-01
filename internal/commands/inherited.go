package commands

import (
	"fmt"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

const maxInheritanceDepth = 100

// ComputeInheritedTags walks the ancestor chain via the titles index and
// collects global tags from all ancestors. Returns deduplicated tags (stored
// form, no `#`). Uses a depth limit and visited set for cycle detection.
//
// The loader is used as a fallback when a parent's TitleEntry doesn't have
// its Tags mirror populated yet (e.g. during the v0.4.0 migration before
// doctor finishes the first full scan). Steady-state callers can pass nil.
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

		var parentTags []string
		if parentEntry.Tags != nil {
			parentTags = parentEntry.Tags
		} else if loader != nil {
			parentNote, err := loader(parentEntry.Path)
			if err != nil {
				break
			}
			parentTags = parentNote.Tags
		}

		for _, t := range parentTags {
			norm := note.NormalizeStored(t)
			if !tagSeen[norm] {
				tagSeen[norm] = true
				inherited = append(inherited, norm)
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

	// note.Load (full body classification) since v0.4.0 — LoadFrontmatterOnly
	// no longer returns tag fields. The loader is only consulted as a fallback
	// when the parent's titles entry doesn't yet mirror its Tags (e.g. tests
	// that construct titles indexes by hand or first-time migration).
	loader := func(path string) (*note.Note, error) {
		return note.Load(path)
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

	// note.Load (full body classification) since v0.4.0 — LoadFrontmatterOnly
	// no longer returns tag fields. The loader is only consulted as a fallback
	// when the parent's titles entry doesn't yet mirror its Tags (e.g. tests
	// that construct titles indexes by hand or first-time migration).
	loader := func(path string) (*note.Note, error) {
		return note.Load(path)
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
			if err := saveNoteForVault(n, vlt); err != nil {
				return fmt.Errorf("failed to save %s: %w", entry.Path, err)
			}
			// Mirror the new inherited-tags into titles.json so hot-path matchers
			// (which read from the in-memory titles index) see the post-cascade
			// state without a full reindex.
			if err := vlt.UpdateTitleEntryInheritedTags(uuid, newInherited); err != nil {
				return fmt.Errorf("failed to update titles mirror for %s: %w", entry.Path, err)
			}
			// Reflect the change in the local titles index too so subsequent
			// loop iterations resolving deeper descendants see the latest tags.
			entry.InheritedTags = append([]string(nil), newInherited...)
			titlesIndex.Titles[uuid] = entry
		}

		queue = append(queue, children[uuid]...)
	}

	return nil
}
