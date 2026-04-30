package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const titlesFile = "titles.json"

// titlesSchemaVersion is the on-disk schema version for titles.json.
// v0.4.0 introduced version: 2 (tag fields). Older or missing version
// triggers migration via doctor.
const titlesSchemaVersion = 2

type TitleEntry struct {
	Title         string   `json:"title"`
	Path          string   `json:"path"`
	Parent        string   `json:"parent,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	InlineTags    []string `json:"inline_tags,omitempty"`
	InheritedTags []string `json:"inherited_tags,omitempty"`
}

// TitlesIndex is a JSON-based index for O(1) UUID lookups.
type TitlesIndex struct {
	Version int                   `json:"version,omitempty"`
	Titles  map[string]TitleEntry `json:"titles"`
}

// sortedCopy returns a sorted copy of in suitable for stable serialization.
// Returns nil for empty input so JSON omitempty works.
func sortedCopy(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}

// MakeTitleEntry constructs a TitleEntry with sorted tag arrays. Use this
// instead of struct literals so cached tag fields stay diff-stable on disk.
func MakeTitleEntry(title, path, parent string, tags, inlineTags, inheritedTags []string) TitleEntry {
	return TitleEntry{
		Title:         title,
		Path:          path,
		Parent:        parent,
		Tags:          sortedCopy(tags),
		InlineTags:    sortedCopy(inlineTags),
		InheritedTags: sortedCopy(inheritedTags),
	}
}

func (v *Vault) TitlesFile() string {
	return filepath.Join(v.RuinDir(), titlesFile)
}

func (v *Vault) LoadTitles() (*TitlesIndex, error) {
	data, err := os.ReadFile(v.TitlesFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &TitlesIndex{Version: titlesSchemaVersion, Titles: make(map[string]TitleEntry)}, nil
		}
		return nil, fmt.Errorf("failed to read titles file: %w", err)
	}

	var index TitlesIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse titles file: %w", err)
	}

	if index.Version > titlesSchemaVersion {
		return nil, fmt.Errorf("titles.json version %d is newer than this binary supports (max %d); upgrade ruin", index.Version, titlesSchemaVersion)
	}

	if index.Titles == nil {
		index.Titles = make(map[string]TitleEntry)
	}

	return &index, nil
}

func (v *Vault) SaveTitles(index *TitlesIndex) error {
	if index.Version == 0 {
		index.Version = titlesSchemaVersion
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal titles: %w", err)
	}

	if err := os.WriteFile(v.TitlesFile(), data, 0644); err != nil {
		return fmt.Errorf("failed to write titles file: %w", err)
	}

	return nil
}

func (v *Vault) UpdateTitleEntry(uuid, title, path, parent string) error {
	index, err := v.LoadTitles()
	if err != nil {
		return err
	}

	// Preserve existing tag fields when callers don't supply them. Doctor and
	// the explicit tag-aware writer below populate them; the simple updater
	// (used by tests, parent reassignment, etc.) leaves them untouched.
	existing := index.Titles[uuid]
	existing.Title = title
	existing.Path = path
	existing.Parent = parent
	index.Titles[uuid] = existing

	return v.SaveTitles(index)
}

// UpdateTitleEntryFull writes a complete entry, including the tag mirror
// fields. SaveNote and CreateNote use this so titles.json stays the source of
// truth for hot-path tag matchers.
func (v *Vault) UpdateTitleEntryFull(uuid, title, path, parent string, tags, inlineTags, inheritedTags []string) error {
	index, err := v.LoadTitles()
	if err != nil {
		return err
	}

	index.Titles[uuid] = MakeTitleEntry(title, path, parent, tags, inlineTags, inheritedTags)
	return v.SaveTitles(index)
}

// UpdateTitleEntryInheritedTags updates only the inherited_tags mirror for a
// single entry. Used by CascadeInheritedTags so descendants stay in sync with
// parent tag changes without rewriting other fields.
func (v *Vault) UpdateTitleEntryInheritedTags(uuid string, inheritedTags []string) error {
	index, err := v.LoadTitles()
	if err != nil {
		return err
	}

	entry, ok := index.Titles[uuid]
	if !ok {
		return nil
	}
	entry.InheritedTags = sortedCopy(inheritedTags)
	index.Titles[uuid] = entry
	return v.SaveTitles(index)
}

func (v *Vault) RemoveTitleEntry(uuid string) error {
	index, err := v.LoadTitles()
	if err != nil {
		return err
	}

	delete(index.Titles, uuid)
	return v.SaveTitles(index)
}

func (v *Vault) RebuildTitlesIndex(entries map[string]TitleEntry) error {
	index := &TitlesIndex{Version: titlesSchemaVersion, Titles: entries}
	return v.SaveTitles(index)
}

// ChildrenMap builds a map from parent UUID to child UUIDs.
func (idx *TitlesIndex) ChildrenMap() map[string][]string {
	children := make(map[string][]string)
	for uuid, entry := range idx.Titles {
		if entry.Parent != "" {
			children[entry.Parent] = append(children[entry.Parent], uuid)
		}
	}
	return children
}

// FindByTitle returns the UUID for a case-insensitive title match.
func (idx *TitlesIndex) FindByTitle(title string) (string, bool) {
	titleLower := strings.ToLower(strings.TrimSpace(title))
	for uuid, entry := range idx.Titles {
		if strings.ToLower(entry.Title) == titleLower {
			return uuid, true
		}
	}
	return "", false
}
