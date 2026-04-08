package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"gopkg.in/yaml.v3"
)

type ComposeSpec struct {
	Root     string             `yaml:"root"`
	Children []ComposeSpecEntry `yaml:"children"`
}

type ComposeSpecEntry struct {
	Note     string             `yaml:"note"`
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
	RootUUID    string
	ChildrenMap map[string][]string
	YMLParents  map[string]bool
}

func BuildChildrenMapFromSpec(spec *ComposeSpec, vlt *vault.Vault, index *vault.TitlesIndex) (*ComposeSpecResult, error) {
	rootNote, err := ResolveNote(vlt, spec.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root %q: %w", spec.Root, err)
	}

	childrenMap := make(map[string][]string)
	ymlParents := make(map[string]bool)

	if len(spec.Children) > 0 {
		childUUIDs, err := resolveSpecChildren(spec.Children, vlt, childrenMap, ymlParents)
		if err != nil {
			return nil, err
		}
		childrenMap[rootNote.UUID] = childUUIDs
		ymlParents[rootNote.UUID] = true
	}

	return &ComposeSpecResult{
		RootUUID:    rootNote.UUID,
		ChildrenMap: childrenMap,
		YMLParents:  ymlParents,
	}, nil
}

func resolveSpecChildren(entries []ComposeSpecEntry, vlt *vault.Vault, childrenMap map[string][]string, ymlParents map[string]bool) ([]string, error) {
	var uuids []string
	for _, entry := range entries {
		n, err := ResolveNote(vlt, entry.Note)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping unresolvable note %q in compose file: %v\n", entry.Note, err)
			continue
		}
		uuids = append(uuids, n.UUID)

		if len(entry.Children) > 0 {
			childUUIDs, err := resolveSpecChildren(entry.Children, vlt, childrenMap, ymlParents)
			if err != nil {
				return nil, err
			}
			childrenMap[n.UUID] = childUUIDs
			ymlParents[n.UUID] = true
		}
	}
	return uuids, nil
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
