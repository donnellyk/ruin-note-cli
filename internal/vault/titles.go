package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const titlesFile = "titles.json"

type TitleEntry struct {
	Title  string `json:"title"`
	Path   string `json:"path"`
	Parent string `json:"parent,omitempty"`
}

// TitlesIndex is a JSON-based index for O(1) UUID lookups.
type TitlesIndex struct {
	Titles map[string]TitleEntry `json:"titles"`
}

func (v *Vault) TitlesFile() string {
	return filepath.Join(v.RuinDir(), titlesFile)
}

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

func (v *Vault) RemoveTitleEntry(uuid string) error {
	index, err := v.LoadTitles()
	if err != nil {
		return err
	}

	delete(index.Titles, uuid)
	return v.SaveTitles(index)
}

func (v *Vault) RebuildTitlesIndex(entries map[string]TitleEntry) error {
	index := &TitlesIndex{Titles: entries}
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
