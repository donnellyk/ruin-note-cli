package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	ruinDir     = ".ruin"
	tagsFile    = "tags.yml"
	queriesFile = "queries.yml"
)

// Vault represents a notes vault directory.
type Vault struct {
	Path string
}

// New creates a new Vault instance for the given path.
func New(path string) *Vault {
	return &Vault{Path: path}
}

// RuinDir returns the path to the .ruin metadata directory.
func (v *Vault) RuinDir() string {
	return filepath.Join(v.Path, ruinDir)
}

// TagsFile returns the path to the tags.yml file.
func (v *Vault) TagsFile() string {
	return filepath.Join(v.RuinDir(), tagsFile)
}

// QueriesFile returns the path to the queries.yml file.
func (v *Vault) QueriesFile() string {
	return filepath.Join(v.RuinDir(), queriesFile)
}

// IsInitialized checks if the vault has been initialized (has .ruin directory).
func (v *Vault) IsInitialized() bool {
	info, err := os.Stat(v.RuinDir())
	if err != nil {
		return false
	}
	return info.IsDir()
}

// InitResult contains information about what was created during initialization.
type InitResult struct {
	Created []string
	Existed []string
}

// Initialize creates the .ruin directory and metadata files if they don't exist.
// If force is true, it will overwrite existing files.
func (v *Vault) Initialize(force bool) (*InitResult, error) {
	result := &InitResult{}

	// Create .ruin directory
	ruinPath := v.RuinDir()
	if err := os.MkdirAll(ruinPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .ruin directory: %w", err)
	}

	// Initialize tags.yml
	tagsPath := v.TagsFile()
	if err := v.initMetadataFile(tagsPath, &TagsIndex{Tags: []TagEntry{}}, force, result); err != nil {
		return nil, err
	}

	// Initialize queries.yml
	queriesPath := v.QueriesFile()
	if err := v.initMetadataFile(queriesPath, &QueriesIndex{Queries: []QueryEntry{}}, force, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (v *Vault) initMetadataFile(path string, data interface{}, force bool, result *InitResult) error {
	_, err := os.Stat(path)
	exists := err == nil

	if exists && !force {
		result.Existed = append(result.Existed, filepath.Base(path))
		return nil
	}

	content, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", filepath.Base(path), err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filepath.Base(path), err)
	}

	result.Created = append(result.Created, filepath.Base(path))
	return nil
}

// Exists checks if the vault directory exists.
func (v *Vault) Exists() bool {
	info, err := os.Stat(v.Path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ListNotes returns all markdown files in the vault (excluding .ruin directory).
func (v *Vault) ListNotes() ([]string, error) {
	var notes []string

	err := filepath.WalkDir(v.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .ruin directory
		if d.IsDir() && d.Name() == ruinDir {
			return filepath.SkipDir
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only include .md files
		if filepath.Ext(path) == ".md" {
			notes = append(notes, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}

	return notes, nil
}

// TagEntry represents a tag in the tags index.
type TagEntry struct {
	Name  string `yaml:"name"`
	Count int    `yaml:"count"`
}

// TagsIndex represents the contents of tags.yml.
type TagsIndex struct {
	Tags []TagEntry `yaml:"tags"`
}

// LoadTags reads the tags index from .ruin/tags.yml.
func (v *Vault) LoadTags() (*TagsIndex, error) {
	data, err := os.ReadFile(v.TagsFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &TagsIndex{Tags: []TagEntry{}}, nil
		}
		return nil, fmt.Errorf("failed to read tags file: %w", err)
	}

	var index TagsIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse tags file: %w", err)
	}

	return &index, nil
}

// SaveTags writes the tags index to .ruin/tags.yml.
func (v *Vault) SaveTags(index *TagsIndex) error {
	data, err := yaml.Marshal(index)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	if err := os.WriteFile(v.TagsFile(), data, 0644); err != nil {
		return fmt.Errorf("failed to write tags file: %w", err)
	}

	return nil
}

// QueryEntry represents a saved query.
type QueryEntry struct {
	Name  string `yaml:"name"`
	Query string `yaml:"query"`
}

// QueriesIndex represents the contents of queries.yml.
type QueriesIndex struct {
	Queries []QueryEntry `yaml:"queries"`
}

// LoadQueries reads the queries index from .ruin/queries.yml.
func (v *Vault) LoadQueries() (*QueriesIndex, error) {
	data, err := os.ReadFile(v.QueriesFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &QueriesIndex{Queries: []QueryEntry{}}, nil
		}
		return nil, fmt.Errorf("failed to read queries file: %w", err)
	}

	var index QueriesIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse queries file: %w", err)
	}

	return &index, nil
}

// SaveQueries writes the queries index to .ruin/queries.yml.
func (v *Vault) SaveQueries(index *QueriesIndex) error {
	data, err := yaml.Marshal(index)
	if err != nil {
		return fmt.Errorf("failed to marshal queries: %w", err)
	}

	if err := os.WriteFile(v.QueriesFile(), data, 0644); err != nil {
		return fmt.Errorf("failed to write queries file: %w", err)
	}

	return nil
}
