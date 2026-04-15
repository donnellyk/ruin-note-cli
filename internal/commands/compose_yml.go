package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"gopkg.in/yaml.v3"
)

type ComposeSpec struct {
	Root     string             `yaml:"root"`
	Children []ComposeSpecEntry `yaml:"children"`
}

type ComposeSpecEntry struct {
	Note     string             `yaml:"note"`
	Search   string             `yaml:"search,omitempty"`
	Pick     string             `yaml:"pick,omitempty"`
	Format   string             `yaml:"format,omitempty"`
	Sort     string             `yaml:"sort,omitempty"`
	Limit    int                `yaml:"limit,omitempty"`
	Filter   string             `yaml:"filter,omitempty"`
	Group    string             `yaml:"group,omitempty"`
	Children []ComposeSpecEntry `yaml:"children"`
}

func ParseComposeFile(path string) (*ComposeSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}

	var spec ComposeSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse compose file: %w", err)
	}

	if spec.Root == "" {
		return nil, fmt.Errorf("compose file missing required 'root' field")
	}

	return &spec, nil
}

type ComposeSpecResult struct {
	RootUUID       string
	ChildrenMap    map[string][]string
	YMLParents     map[string]bool
	DynamicEntries map[string][]note.DynamicEmbedRef
}

func BuildChildrenMapFromSpec(spec *ComposeSpec, vlt *vault.Vault, index *vault.TitlesIndex) (*ComposeSpecResult, error) {
	rootNote, err := ResolveNote(vlt, spec.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root %q: %w", spec.Root, err)
	}

	childrenMap := make(map[string][]string)
	ymlParents := make(map[string]bool)
	dynamicEntries := make(map[string][]note.DynamicEmbedRef)

	if len(spec.Children) > 0 {
		childUUIDs, err := resolveSpecChildren(spec.Children, vlt, childrenMap, ymlParents, dynamicEntries, rootNote.UUID)
		if err != nil {
			return nil, err
		}
		childrenMap[rootNote.UUID] = childUUIDs
		ymlParents[rootNote.UUID] = true
	}

	return &ComposeSpecResult{
		RootUUID:       rootNote.UUID,
		ChildrenMap:    childrenMap,
		YMLParents:     ymlParents,
		DynamicEntries: dynamicEntries,
	}, nil
}

func resolveSpecChildren(entries []ComposeSpecEntry, vlt *vault.Vault, childrenMap map[string][]string, ymlParents map[string]bool, dynamicEntries map[string][]note.DynamicEmbedRef, parentUUID string) ([]string, error) {
	var uuids []string
	for _, entry := range entries {
		// Handle dynamic search entries
		if entry.Search != "" {
			ref := buildDynamicRef("search", entry.Search, entry)
			dynamicEntries[parentUUID] = append(dynamicEntries[parentUUID], ref)
			continue
		}

		// Handle dynamic pick entries
		if entry.Pick != "" {
			ref := buildDynamicRef("pick", entry.Pick, entry)
			dynamicEntries[parentUUID] = append(dynamicEntries[parentUUID], ref)
			continue
		}

		n, err := ResolveNote(vlt, entry.Note)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping unresolvable note %q in compose file: %v\n", entry.Note, err)
			continue
		}
		uuids = append(uuids, n.UUID)

		if len(entry.Children) > 0 {
			childUUIDs, err := resolveSpecChildren(entry.Children, vlt, childrenMap, ymlParents, dynamicEntries, n.UUID)
			if err != nil {
				return nil, err
			}
			childrenMap[n.UUID] = childUUIDs
			ymlParents[n.UUID] = true
		}
	}
	return uuids, nil
}

// buildDynamicRef creates a DynamicEmbedRef from a YAML spec entry.
func buildDynamicRef(embedType, query string, entry ComposeSpecEntry) note.DynamicEmbedRef {
	opts := make(map[string]string)
	if entry.Format != "" {
		opts["format"] = entry.Format
	}
	if entry.Sort != "" {
		opts["sort"] = entry.Sort
	}
	if entry.Limit > 0 {
		opts["limit"] = strconv.Itoa(entry.Limit)
	}
	if entry.Filter != "" {
		opts["filter"] = entry.Filter
	}
	if entry.Group != "" {
		opts["group"] = entry.Group
	}
	if len(opts) == 0 {
		opts = nil
	}
	return note.DynamicEmbedRef{
		Type:    embedType,
		Query:   query,
		Options: opts,
	}
}

func ResolveComposeFilePath(filePath string, vaultPath string) (string, error) {
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	absVault, err := filepath.Abs(vaultPath)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(absVault, absFile)
	if err != nil || len(rel) > 1 && rel[:2] == ".." {
		return absFile, nil
	}
	return rel, nil
}

func LoadComposeFilePath(storedPath string, vaultPath string) string {
	if filepath.IsAbs(storedPath) {
		return storedPath
	}
	return filepath.Join(vaultPath, storedPath)
}
