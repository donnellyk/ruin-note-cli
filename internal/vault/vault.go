package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/versioning"
	"gopkg.in/yaml.v3"
)

const (
	ruinDir     = ".ruin"
	tagsFile    = "tags.yml"
	queriesFile = "queries.yml"
	parentsFile = "parents.yml"
)

type Vault struct {
	Path           string
	versioning     *versioning.GitVersioning
	tagInheritance bool
	tagFrontmatter bool
}

func New(path string) *Vault {
	return &Vault{Path: path, tagInheritance: true, tagFrontmatter: true}
}

func (v *Vault) SetVersioning(g *versioning.GitVersioning) {
	v.versioning = g
}

func (v *Vault) SetTagInheritance(enabled bool) {
	v.tagInheritance = enabled
}

func (v *Vault) TagInheritanceEnabled() bool {
	return v.tagInheritance
}

func (v *Vault) SetTagFrontmatter(enabled bool) {
	v.tagFrontmatter = enabled
}

func (v *Vault) TagFrontmatterEnabled() bool {
	return v.tagFrontmatter
}

// Commit creates a git commit if versioning is enabled. No-op if versioning is
// nil or msg is empty. Errors are logged to stderr as warnings.
func (v *Vault) Commit(msg string) {
	if v.versioning == nil || msg == "" {
		return
	}
	if err := v.versioning.Commit(msg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: git commit failed: %v\n", err)
	}
}

// SaveNote updates vault indexes after a note has been modified and saved.
// Does not call n.Save() — that's the caller's responsibility.
func (v *Vault) SaveNote(n *note.Note, oldGlobalTags, oldInlineTags []string, commitMsg string) {
	v.DecrementTagsIndex(oldGlobalTags, oldInlineTags)
	v.UpdateTagsIndex(n.Tags, n.InlineTags)
	v.UpdateTitleEntryFull(n.UUID, n.Title, n.FilePath, n.Parent, n.Tags, n.InlineTags, n.InheritedTags, n.Aliases)
	v.Commit(commitMsg)
}

// DeleteNote removes a note file and cleans up its vault indexes.
func (v *Vault) DeleteNote(n *note.Note, commitMsg string) error {
	if err := os.Remove(n.FilePath); err != nil {
		return fmt.Errorf("failed to delete %s: %w", n.FilePath, err)
	}
	v.DecrementTagsIndex(n.Tags, n.InlineTags)
	v.RemoveTitleEntry(n.UUID)
	v.Commit(commitMsg)
	return nil
}

// CreateNote indexes a newly created note. Index errors are non-fatal (logged
// to stderr as warnings).
func (v *Vault) CreateNote(n *note.Note, commitMsg string) {
	if err := v.UpdateTagsIndex(n.Tags, n.InlineTags); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to update tags index: %v\n", err)
	}
	if err := v.UpdateTitleEntryFull(n.UUID, n.Title, n.FilePath, n.Parent, n.Tags, n.InlineTags, n.InheritedTags, n.Aliases); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to update titles index: %v\n", err)
	}
	v.Commit(commitMsg)
}

func (v *Vault) RuinDir() string {
	return filepath.Join(v.Path, ruinDir)
}

func (v *Vault) TagsFile() string {
	return filepath.Join(v.RuinDir(), tagsFile)
}

func (v *Vault) QueriesFile() string {
	return filepath.Join(v.RuinDir(), queriesFile)
}

func (v *Vault) ParentsFile() string {
	return filepath.Join(v.RuinDir(), parentsFile)
}

func (v *Vault) IsInitialized() bool {
	info, err := os.Stat(v.RuinDir())
	if err != nil {
		return false
	}
	return info.IsDir()
}

type InitResult struct {
	Created []string
	Existed []string
}

// Initialize creates the .ruin directory and metadata files if they don't exist.
// If force is true, overwrites existing files.
func (v *Vault) Initialize(force bool) (*InitResult, error) {
	result := &InitResult{}

	ruinPath := v.RuinDir()
	if err := os.MkdirAll(ruinPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .ruin directory: %w", err)
	}

	tagsPath := v.TagsFile()
	if err := v.initMetadataFile(tagsPath, &TagsIndex{Version: indexSchemaVersion, Tags: []TagEntry{}}, force, result); err != nil {
		return nil, err
	}

	queriesPath := v.QueriesFile()
	if err := v.initMetadataFile(queriesPath, &QueriesIndex{Version: indexSchemaVersion, Queries: []QueryEntry{}}, force, result); err != nil {
		return nil, err
	}

	parentsPath := v.ParentsFile()
	if err := v.initMetadataFile(parentsPath, &ParentsIndex{Version: indexSchemaVersion, Parents: []ParentEntry{}}, force, result); err != nil {
		return nil, err
	}

	titlesPath := v.TitlesFile()
	if err := v.initJSONFile(titlesPath, &TitlesIndex{Version: titlesSchemaVersion, Titles: make(map[string]TitleEntry)}, force, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (v *Vault) initJSONFile(path string, data any, force bool, result *InitResult) error {
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

func (v *Vault) initMetadataFile(path string, data any, force bool, result *InitResult) error {
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

		if d.IsDir() && d.Name() == ruinDir {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

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

const (
	ScopeGlobal = "global"
	ScopeInline = "inline"
)

// indexSchemaVersion is the on-disk schema version for the `.ruin/` YAML
// indexes. v0.4.0 introduced version: 2 alongside the tag-format migration.
// Older binaries reading a v0.4.0 vault see the unknown version and refuse
// rather than silently giving wrong answers.
const indexSchemaVersion = 2

type TagEntry struct {
	Name  string   `yaml:"name" json:"name"`
	Count int      `yaml:"count" json:"count"`
	Scope []string `yaml:"scope" json:"scope"`
}

type TagsIndex struct {
	Version int        `yaml:"version,omitempty"`
	Tags    []TagEntry `yaml:"tags"`
}

func checkIndexVersion(file string, version int) error {
	if version > indexSchemaVersion {
		return fmt.Errorf("%s version %d is newer than this binary supports (max %d); upgrade ruin", file, version, indexSchemaVersion)
	}
	return nil
}

func (v *Vault) LoadTags() (*TagsIndex, error) {
	data, err := os.ReadFile(v.TagsFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &TagsIndex{Version: indexSchemaVersion, Tags: []TagEntry{}}, nil
		}
		return nil, fmt.Errorf("failed to read tags file: %w", err)
	}

	var index TagsIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse tags file: %w", err)
	}
	if err := checkIndexVersion("tags.yml", index.Version); err != nil {
		return nil, err
	}

	return &index, nil
}

func (v *Vault) SaveTags(index *TagsIndex) error {
	if index.Version == 0 {
		index.Version = indexSchemaVersion
	}
	data, err := yaml.Marshal(index)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	if err := os.WriteFile(v.TagsFile(), data, 0644); err != nil {
		return fmt.Errorf("failed to write tags file: %w", err)
	}

	return nil
}

// UpdateTagsIndex increments counts for deduped tags (global + inline union)
// and merges scope.
func (v *Vault) UpdateTagsIndex(globalTags, inlineTags []string) error {
	if len(globalTags) == 0 && len(inlineTags) == 0 {
		return nil
	}

	index, err := v.LoadTags()
	if err != nil {
		return err
	}

	tagMap := make(map[string]*TagEntry)
	for i := range index.Tags {
		tagMap[strings.ToLower(index.Tags[i].Name)] = &index.Tags[i]
	}

	globalSet := make(map[string]bool)
	for _, t := range globalTags {
		globalSet[strings.ToLower(t)] = true
	}
	inlineSet := make(map[string]bool)
	for _, t := range inlineTags {
		inlineSet[strings.ToLower(t)] = true
	}

	allTags := make(map[string]string)
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

// DecrementTagsIndex decrements counts for deduped tags (global + inline union).
// Tags with count <= 0 are removed. Scope is not narrowed; use RebuildTagsIndex
// for accurate scope.
func (v *Vault) DecrementTagsIndex(globalTags, inlineTags []string) error {
	if len(globalTags) == 0 && len(inlineTags) == 0 {
		return nil
	}

	index, err := v.LoadTags()
	if err != nil {
		return err
	}

	tagMap := make(map[string]int)
	for i := range index.Tags {
		tagMap[strings.ToLower(index.Tags[i].Name)] = i
	}

	allKeys := make(map[string]bool)
	for _, t := range globalTags {
		allKeys[strings.ToLower(t)] = true
	}
	for _, t := range inlineTags {
		allKeys[strings.ToLower(t)] = true
	}

	toRemove := make(map[int]bool)
	for key := range allKeys {
		if idx, ok := tagMap[key]; ok {
			index.Tags[idx].Count--
			if index.Tags[idx].Count <= 0 {
				toRemove[idx] = true
			}
		}
	}

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

// RebuildTagsIndex rebuilds the entire tags index from all notes. totalCounts
// is the deduped per-note count; globalTags and inlineTags indicate which
// scopes each tag has been seen in.
func (v *Vault) RebuildTagsIndex(totalCounts map[string]int, globalTags, inlineTags map[string]bool) error {
	index := &TagsIndex{Version: indexSchemaVersion, Tags: make([]TagEntry, 0, len(totalCounts))}

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

type QueryEntry struct {
	Name  string `yaml:"name" json:"name"`
	Query string `yaml:"query" json:"query"`
}

type QueriesIndex struct {
	Version int          `yaml:"version,omitempty"`
	Queries []QueryEntry `yaml:"queries"`
}

func (v *Vault) LoadQueries() (*QueriesIndex, error) {
	data, err := os.ReadFile(v.QueriesFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &QueriesIndex{Version: indexSchemaVersion, Queries: []QueryEntry{}}, nil
		}
		return nil, fmt.Errorf("failed to read queries file: %w", err)
	}

	var index QueriesIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse queries file: %w", err)
	}
	if err := checkIndexVersion("queries.yml", index.Version); err != nil {
		return nil, err
	}

	return &index, nil
}

func (v *Vault) SaveQueries(index *QueriesIndex) error {
	if index.Version == 0 {
		index.Version = indexSchemaVersion
	}
	data, err := yaml.Marshal(index)
	if err != nil {
		return fmt.Errorf("failed to marshal queries: %w", err)
	}

	if err := os.WriteFile(v.QueriesFile(), data, 0644); err != nil {
		return fmt.Errorf("failed to write queries file: %w", err)
	}

	return nil
}

type ParentEntry struct {
	Name string `yaml:"name" json:"name"`
	UUID string `yaml:"uuid,omitempty" json:"uuid,omitempty"`
}

type ParentsIndex struct {
	Version int           `yaml:"version,omitempty"`
	Parents []ParentEntry `yaml:"parents"`
}

func (v *Vault) LoadParents() (*ParentsIndex, error) {
	data, err := os.ReadFile(v.ParentsFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ParentsIndex{Version: indexSchemaVersion, Parents: []ParentEntry{}}, nil
		}
		return nil, fmt.Errorf("failed to read parents file: %w", err)
	}

	var index ParentsIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse parents file: %w", err)
	}
	if err := checkIndexVersion("parents.yml", index.Version); err != nil {
		return nil, err
	}

	return &index, nil
}

func (v *Vault) SaveParents(index *ParentsIndex) error {
	if index.Version == 0 {
		index.Version = indexSchemaVersion
	}
	data, err := yaml.Marshal(index)
	if err != nil {
		return fmt.Errorf("failed to marshal parents: %w", err)
	}

	if err := os.WriteFile(v.ParentsFile(), data, 0644); err != nil {
		return fmt.Errorf("failed to write parents file: %w", err)
	}

	return nil
}

type ParentBookmark struct {
	Name string
	UUID string
}

func (v *Vault) LookupParent(name string) (ParentBookmark, bool) {
	index, err := v.LoadParents()
	if err != nil {
		return ParentBookmark{}, false
	}

	for _, p := range index.Parents {
		if p.Name == name {
			return ParentBookmark{
				Name: p.Name,
				UUID: p.UUID,
			}, true
		}
	}
	return ParentBookmark{}, false
}
