package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const titlesFile = "titles.json"

// TitleEntry represents a note in the titles index.
type TitleEntry struct {
	Title  string `json:"title"`
	Path   string `json:"path"`
	Parent string `json:"parent,omitempty"`
}

// TitlesIndex is a JSON-based index for O(1) UUID lookups.
type TitlesIndex struct {
	Titles map[string]TitleEntry `json:"titles"`
}

// TitlesFile returns the path to the titles.json file.
func (v *Vault) TitlesFile() string {
	return filepath.Join(v.RuinDir(), titlesFile)
}

// LoadTitles reads the titles index from .ruin/titles.json.
// Returns an empty index if the file is missing.
func (v *Vault) LoadTitles() (*TitlesIndex, error) {
	data, err := os.ReadFile(v.TitlesFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &TitlesIndex{Titles: make(map[string]TitleEntry)}, nil
		}
		return nil, fmt.Errorf("failed to read titles file: %w", err)
	}

	var index TitlesIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse titles file: %w", err)
	}

	if index.Titles == nil {
		index.Titles = make(map[string]TitleEntry)
	}

	return &index, nil
}

// SaveTitles writes the titles index to .ruin/titles.json.
func (v *Vault) SaveTitles(index *TitlesIndex) error {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal titles: %w", err)
	}

	if err := os.WriteFile(v.TitlesFile(), data, 0644); err != nil {
		return fmt.Errorf("failed to write titles file: %w", err)
	}

	return nil
}

// UpdateTitleEntry upserts a single entry in the titles index.
func (v *Vault) UpdateTitleEntry(uuid, title, path, parent string) error {
	index, err := v.LoadTitles()
	if err != nil {
		return err
	}

	index.Titles[uuid] = TitleEntry{
		Title:  title,
		Path:   path,
		Parent: parent,
	}

	return v.SaveTitles(index)
}

// RemoveTitleEntry removes a single entry from the titles index.
func (v *Vault) RemoveTitleEntry(uuid string) error {
	index, err := v.LoadTitles()
	if err != nil {
		return err
	}

	delete(index.Titles, uuid)
	return v.SaveTitles(index)
}

// RebuildTitlesIndex replaces the entire titles index.
func (v *Vault) RebuildTitlesIndex(entries map[string]TitleEntry) error {
	index := &TitlesIndex{Titles: entries}
	return v.SaveTitles(index)
}
