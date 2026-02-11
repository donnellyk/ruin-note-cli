package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ruinDir     = ".ruin"
	tagsFile    = "tags.yml"
	queriesFile = "queries.yml"
	parentsFile = "parents.yml"
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

// ParentsFile returns the path to the parents.yml file.
func (v *Vault) ParentsFile() string {
	return filepath.Join(v.RuinDir(), parentsFile)
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

	// Initialize parents.yml
	parentsPath := v.ParentsFile()
	if err := v.initMetadataFile(parentsPath, &ParentsIndex{Parents: []ParentEntry{}}, force, result); err != nil {
		return nil, err
	}

	// Initialize titles.json
	titlesPath := v.TitlesFile()
	if err := v.initJSONFile(titlesPath, &TitlesIndex{Titles: make(map[string]TitleEntry)}, force, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (v *Vault) initJSONFile(path string, data interface{}, force bool, result *InitResult) error {
	_, err := os.Stat(path)
	exists := err == nil

	if exists && !force {
		result.Existed = append(result.Existed, filepath.Base(path))
		return nil
	}

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", filepath.Base(path), err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filepath.Base(path), err)
	}

	result.Created = append(result.Created, filepath.Base(path))
	return nil
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

// Tag scope constants for tags.yml.
const (
	ScopeGlobal = "global"
	ScopeInline = "inline"
)

// TagEntry represents a tag in the tags index.
type TagEntry struct {
	Name  string   `yaml:"name" json:"name"`
	Count int      `yaml:"count" json:"count"`
	Scope []string `yaml:"scope" json:"scope"`
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

// UpdateTagsIndex updates the tags index with the given global and inline tags.
// It increments the count for existing tags (deduped) and merges scope.
func (v *Vault) UpdateTagsIndex(globalTags, inlineTags []string) error {
	if len(globalTags) == 0 && len(inlineTags) == 0 {
		return nil
	}

	index, err := v.LoadTags()
	if err != nil {
		return err
	}

	// Build a map for quick lookup
	tagMap := make(map[string]*TagEntry)
	for i := range index.Tags {
		tagMap[strings.ToLower(index.Tags[i].Name)] = &index.Tags[i]
	}

	// Track which tags appear in which scope for this note
	globalSet := make(map[string]bool)
	for _, t := range globalTags {
		globalSet[strings.ToLower(t)] = true
	}
	inlineSet := make(map[string]bool)
	for _, t := range inlineTags {
		inlineSet[strings.ToLower(t)] = true
	}

	// Dedup union of all tags for count increment
	allTags := make(map[string]string) // lowercase -> original
	for _, t := range globalTags {
		allTags[strings.ToLower(t)] = t
	}
	for _, t := range inlineTags {
		key := strings.ToLower(t)
		if _, exists := allTags[key]; !exists {
			allTags[key] = t
		}
	}

	for key, tag := range allTags {
		noteScope := scopeFromSets(globalSet[key], inlineSet[key])

		if entry, ok := tagMap[key]; ok {
			entry.Count++
			entry.Scope = mergeScope(entry.Scope, noteScope)
		} else {
			index.Tags = append(index.Tags, TagEntry{
				Name:  tag,
				Count: 1,
				Scope: noteScope,
			})
			tagMap[key] = &index.Tags[len(index.Tags)-1]
		}
	}

	return v.SaveTags(index)
}

// scopeFromSets returns the scope list for a tag based on its presence in global/inline sets.
func scopeFromSets(isGlobal, isInline bool) []string {
	var scope []string
	if isGlobal {
		scope = append(scope, ScopeGlobal)
	}
	if isInline {
		scope = append(scope, ScopeInline)
	}
	return scope
}

// mergeScope widens the scope of a tag entry by adding any new scope values.
func mergeScope(existing, incoming []string) []string {
	has := make(map[string]bool)
	for _, s := range existing {
		has[s] = true
	}
	for _, s := range incoming {
		if !has[s] {
			existing = append(existing, s)
			has[s] = true
		}
	}
	return existing
}

// DecrementTagsIndex decrements the count for the given tags (deduped from global + inline).
// Tags with count <= 0 are removed from the index.
// Scope is not narrowed on decrement; use RebuildTagsIndex for accurate scope.
func (v *Vault) DecrementTagsIndex(globalTags, inlineTags []string) error {
	if len(globalTags) == 0 && len(inlineTags) == 0 {
		return nil
	}

	index, err := v.LoadTags()
	if err != nil {
		return err
	}

	// Build a map for quick lookup
	tagMap := make(map[string]int) // index position
	for i := range index.Tags {
		tagMap[strings.ToLower(index.Tags[i].Name)] = i
	}

	// Dedup union of all tags
	allKeys := make(map[string]bool)
	for _, t := range globalTags {
		allKeys[strings.ToLower(t)] = true
	}
	for _, t := range inlineTags {
		allKeys[strings.ToLower(t)] = true
	}

	// Decrement tags
	toRemove := make(map[int]bool)
	for key := range allKeys {
		if idx, ok := tagMap[key]; ok {
			index.Tags[idx].Count--
			if index.Tags[idx].Count <= 0 {
				toRemove[idx] = true
			}
		}
	}

	// Remove tags with count <= 0
	if len(toRemove) > 0 {
		newTags := make([]TagEntry, 0, len(index.Tags)-len(toRemove))
		for i, tag := range index.Tags {
			if !toRemove[i] {
				newTags = append(newTags, tag)
			}
		}
		index.Tags = newTags
	}

	return v.SaveTags(index)
}

// RebuildTagsIndex rebuilds the entire tags index from all notes.
// totalCounts is the deduped per-note count. globalTags and inlineTags
// are sets indicating which scopes each tag has been seen in.
func (v *Vault) RebuildTagsIndex(totalCounts map[string]int, globalTags, inlineTags map[string]bool) error {
	index := &TagsIndex{Tags: make([]TagEntry, 0, len(totalCounts))}

	for tag, count := range totalCounts {
		if count > 0 {
			index.Tags = append(index.Tags, TagEntry{
				Name:  tag,
				Count: count,
				Scope: scopeFromSets(globalTags[tag], inlineTags[tag]),
			})
		}
	}

	return v.SaveTags(index)
}

// QueryEntry represents a saved query.
type QueryEntry struct {
	Name  string `yaml:"name" json:"name"`
	Query string `yaml:"query" json:"query"`
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

// ParentEntry represents a saved parent bookmark (name -> UUID).
type ParentEntry struct {
	Name string `yaml:"name" json:"name"`
	UUID string `yaml:"uuid" json:"uuid"`
}

// ParentsIndex represents the contents of parents.yml.
type ParentsIndex struct {
	Parents []ParentEntry `yaml:"parents"`
}

// LoadParents reads the parents index from .ruin/parents.yml.
func (v *Vault) LoadParents() (*ParentsIndex, error) {
	data, err := os.ReadFile(v.ParentsFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ParentsIndex{Parents: []ParentEntry{}}, nil
		}
		return nil, fmt.Errorf("failed to read parents file: %w", err)
	}

	var index ParentsIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse parents file: %w", err)
	}

	return &index, nil
}

// SaveParents writes the parents index to .ruin/parents.yml.
func (v *Vault) SaveParents(index *ParentsIndex) error {
	data, err := yaml.Marshal(index)
	if err != nil {
		return fmt.Errorf("failed to marshal parents: %w", err)
	}

	if err := os.WriteFile(v.ParentsFile(), data, 0644); err != nil {
		return fmt.Errorf("failed to write parents file: %w", err)
	}

	return nil
}

// LookupParent finds a saved parent by name and returns its UUID.
func (v *Vault) LookupParent(name string) (string, bool) {
	index, err := v.LoadParents()
	if err != nil {
		return "", false
	}

	for _, p := range index.Parents {
		if p.Name == name {
			return p.UUID, true
		}
	}
	return "", false
}
