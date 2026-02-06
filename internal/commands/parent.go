package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"kvnd/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// NewParentCmd creates the parent command with subcommands.
func NewParentCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parent",
		Short: "Manage parent-child note relationships",
		Long:  `Commands for setting, viewing, and removing parent-child relationships between notes.`,
	}

	cmd.AddCommand(newParentSetCmd(getVault, jsonOutput))
	cmd.AddCommand(newParentGetCmd(getVault, jsonOutput))
	cmd.AddCommand(newParentRemoveCmd(getVault, jsonOutput))
	cmd.AddCommand(newParentChildrenCmd(getVault, jsonOutput))
	cmd.AddCommand(newParentTreeCmd(getVault, jsonOutput))

	return cmd
}

func newParentSetCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "set <child> <parent>",
		Short: "Set the parent of a note",
		Long: `Set a parent-child relationship between two notes.

Both <child> and <parent> can be a UUID, title substring, or path substring.

Validates that:
  - The child and parent are different notes
  - No cycle would be created`,
		Example: `  ruin parent set "My Child Note" "My Parent Note"
  ruin parent set <child-uuid> <parent-uuid>`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			child, err := ResolveNote(vlt, args[0])
			if err != nil {
				return fmt.Errorf("child: %w", err)
			}

			parent, err := ResolveNote(vlt, args[1])
			if err != nil {
				return fmt.Errorf("parent: %w", err)
			}

			// Self-reference check
			if child.UUID == parent.UUID {
				return fmt.Errorf("a note cannot be its own parent")
			}

			// Cycle detection
			index, err := vlt.LoadTitles()
			if err != nil {
				return fmt.Errorf("failed to load titles index: %w", err)
			}
			if err := detectCycle(index, child.UUID, parent.UUID); err != nil {
				return err
			}

			// Check existing parent
			if child.Parent != "" && child.Parent != parent.UUID && !force {
				existingTitle := child.Parent
				if entry, ok := index.Titles[child.Parent]; ok {
					existingTitle = entry.Title
				}
				return fmt.Errorf("note already has parent %q (use --force to overwrite)", existingTitle)
			}

			child.Parent = parent.UUID
			child.SetTimestamps()
			if err := child.Save(); err != nil {
				return fmt.Errorf("failed to save: %w", err)
			}

			// Update titles index
			if err := vlt.UpdateTitleEntry(child.UUID, child.Title, child.FilePath, child.Parent); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to update titles index: %v\n", err)
			}

			if *jsonOutput {
				output := struct {
					Child  string `json:"child"`
					Parent string `json:"parent"`
				}{
					Child:  child.UUID,
					Parent: parent.UUID,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Fprintf(os.Stderr, "Set parent of %q to %q\n", child.Title, parent.Title)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation when overwriting existing parent")
	return cmd
}

func newParentGetCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <note>",
		Short: "Show the parent of a note",
		Example: `  ruin parent get "My Note"
  ruin parent get <uuid>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			n, err := ResolveNote(vlt, args[0])
			if err != nil {
				return err
			}

			if n.Parent == "" {
				return fmt.Errorf("note %q has no parent", n.Title)
			}

			index, err := vlt.LoadTitles()
			if err != nil {
				return fmt.Errorf("failed to load titles index: %w", err)
			}

			entry, ok := index.Titles[n.Parent]
			if !ok {
				fmt.Fprintf(os.Stderr, "warning: parent UUID %s not found in vault (orphaned reference)\n", n.Parent)
				if *jsonOutput {
					output := struct {
						UUID string `json:"uuid"`
					}{UUID: n.Parent}
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(output)
				}
				fmt.Println(n.Parent)
				return nil
			}

			if *jsonOutput {
				output := struct {
					UUID  string `json:"uuid"`
					Title string `json:"title"`
					Path  string `json:"path"`
				}{
					UUID:  n.Parent,
					Title: entry.Title,
					Path:  entry.Path,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Println(entry.Path)
			return nil
		},
	}

	return cmd
}

func newParentRemoveCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <note>",
		Short: "Remove the parent of a note",
		Example: `  ruin parent remove "My Note"
  ruin parent remove <uuid>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			n, err := ResolveNote(vlt, args[0])
			if err != nil {
				return err
			}

			if n.Parent == "" {
				return fmt.Errorf("note %q has no parent", n.Title)
			}

			n.Parent = ""
			n.SetTimestamps()
			if err := n.Save(); err != nil {
				return fmt.Errorf("failed to save: %w", err)
			}

			if err := vlt.UpdateTitleEntry(n.UUID, n.Title, n.FilePath, ""); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to update titles index: %v\n", err)
			}

			if *jsonOutput {
				output := struct {
					UUID    string `json:"uuid"`
					Removed bool   `json:"removed"`
				}{UUID: n.UUID, Removed: true}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Fprintf(os.Stderr, "Removed parent from %q\n", n.Title)
			return nil
		},
	}

	return cmd
}

func newParentChildrenCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:   "children <note>",
		Short: "List children of a note",
		Example: `  ruin parent children "My Note"
  ruin parent children <uuid> --recursive`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			n, err := ResolveNote(vlt, args[0])
			if err != nil {
				return err
			}

			index, err := vlt.LoadTitles()
			if err != nil {
				return fmt.Errorf("failed to load titles index: %w", err)
			}

			if recursive {
				return outputChildrenRecursive(index, n.UUID, *jsonOutput)
			}

			// Direct children only
			var children []childInfo
			for uuid, entry := range index.Titles {
				if entry.Parent == n.UUID {
					children = append(children, childInfo{
						UUID:  uuid,
						Title: entry.Title,
						Path:  entry.Path,
					})
				}
			}

			sort.Slice(children, func(i, j int) bool {
				return children[i].Title < children[j].Title
			})

			if *jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(children)
			}

			for _, c := range children {
				fmt.Println(c.Path)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "include all descendants")
	return cmd
}

type childInfo struct {
	UUID     string      `json:"uuid"`
	Title    string      `json:"title"`
	Path     string      `json:"path"`
	Children []childInfo `json:"children,omitempty"`
}

func outputChildrenRecursive(index *vault.TitlesIndex, parentUUID string, jsonOut bool) error {
	// Build parent->children map
	childrenMap := make(map[string][]string)
	for uuid, entry := range index.Titles {
		if entry.Parent != "" {
			childrenMap[entry.Parent] = append(childrenMap[entry.Parent], uuid)
		}
	}

	// Sort children by title
	for parent := range childrenMap {
		uuids := childrenMap[parent]
		sort.Slice(uuids, func(i, j int) bool {
			return index.Titles[uuids[i]].Title < index.Titles[uuids[j]].Title
		})
	}

	tree := buildChildTree(index, childrenMap, parentUUID, make(map[string]bool))

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(tree)
	}

	printChildTree(tree, 0)
	return nil
}

func buildChildTree(index *vault.TitlesIndex, childrenMap map[string][]string, uuid string, visited map[string]bool) []childInfo {
	if visited[uuid] {
		return nil
	}
	visited[uuid] = true

	uuids := childrenMap[uuid]
	var result []childInfo
	for _, childUUID := range uuids {
		entry := index.Titles[childUUID]
		info := childInfo{
			UUID:     childUUID,
			Title:    entry.Title,
			Path:     entry.Path,
			Children: buildChildTree(index, childrenMap, childUUID, visited),
		}
		result = append(result, info)
	}
	return result
}

func printChildTree(children []childInfo, depth int) {
	for _, c := range children {
		fmt.Printf("%s%s\n", strings.Repeat("  ", depth), c.Path)
		printChildTree(c.Children, depth+1)
	}
}

func newParentTreeCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		maxDepth int
	)

	cmd := &cobra.Command{
		Use:   "tree [note]",
		Short: "Show the parent-child tree",
		Long: `Show a tree of parent-child relationships.

With a note argument, shows the subtree rooted at that note.
Without arguments, shows the full forest (all root notes and their descendants).`,
		Example: `  ruin parent tree
  ruin parent tree <uuid>
  ruin parent tree --depth 2`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			index, err := vlt.LoadTitles()
			if err != nil {
				return fmt.Errorf("failed to load titles index: %w", err)
			}

			// Build parent->children map
			childrenMap := make(map[string][]string)
			for uuid, entry := range index.Titles {
				if entry.Parent != "" {
					childrenMap[entry.Parent] = append(childrenMap[entry.Parent], uuid)
				}
			}

			// Sort children by title
			for parent := range childrenMap {
				uuids := childrenMap[parent]
				sort.Slice(uuids, func(i, j int) bool {
					return index.Titles[uuids[i]].Title < index.Titles[uuids[j]].Title
				})
			}

			if len(args) == 1 {
				// Rooted subtree
				n, err := ResolveNote(vlt, args[0])
				if err != nil {
					return err
				}

				if *jsonOutput {
					tree := buildTreeNode(index, childrenMap, n.UUID, make(map[string]bool), maxDepth, 0)
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(tree)
				}

				entry := index.Titles[n.UUID]
				fmt.Println(entry.Path)
				printTreeNodes(index, childrenMap, n.UUID, make(map[string]bool), maxDepth, 1)
				return nil
			}

			// Full forest: find roots (notes with no parent, or parent not in vault)
			var roots []string
			for uuid, entry := range index.Titles {
				if entry.Parent == "" {
					roots = append(roots, uuid)
				} else if _, ok := index.Titles[entry.Parent]; !ok {
					// Parent doesn't exist in vault - treat as root
					roots = append(roots, uuid)
				}
			}
			sort.Slice(roots, func(i, j int) bool {
				return index.Titles[roots[i]].Title < index.Titles[roots[j]].Title
			})

			if *jsonOutput {
				var forest []treeNode
				visited := make(map[string]bool)
				for _, uuid := range roots {
					node := buildTreeNode(index, childrenMap, uuid, visited, maxDepth, 0)
					forest = append(forest, node)
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(forest)
			}

			visited := make(map[string]bool)
			for _, uuid := range roots {
				entry := index.Titles[uuid]
				fmt.Println(entry.Path)
				printTreeNodes(index, childrenMap, uuid, visited, maxDepth, 1)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&maxDepth, "depth", 0, "max tree depth (0 = unlimited)")
	return cmd
}

type treeNode struct {
	UUID     string     `json:"uuid"`
	Title    string     `json:"title"`
	Path     string     `json:"path"`
	Children []treeNode `json:"children,omitempty"`
}

func buildTreeNode(index *vault.TitlesIndex, childrenMap map[string][]string, uuid string, visited map[string]bool, maxDepth, depth int) treeNode {
	entry := index.Titles[uuid]
	node := treeNode{
		UUID:  uuid,
		Title: entry.Title,
		Path:  entry.Path,
	}

	if visited[uuid] {
		return node
	}
	visited[uuid] = true

	if maxDepth > 0 && depth >= maxDepth {
		return node
	}

	for _, childUUID := range childrenMap[uuid] {
		child := buildTreeNode(index, childrenMap, childUUID, visited, maxDepth, depth+1)
		node.Children = append(node.Children, child)
	}

	return node
}

func printTreeNodes(index *vault.TitlesIndex, childrenMap map[string][]string, uuid string, visited map[string]bool, maxDepth, depth int) {
	if visited[uuid] {
		return
	}
	visited[uuid] = true

	if maxDepth > 0 && depth > maxDepth {
		return
	}

	for _, childUUID := range childrenMap[uuid] {
		entry := index.Titles[childUUID]
		fmt.Printf("%s%s\n", strings.Repeat("  ", depth), entry.Path)
		printTreeNodes(index, childrenMap, childUUID, visited, maxDepth, depth+1)
	}
}

// detectCycle checks if setting proposedParentUUID as the parent of childUUID
// would create a cycle. It walks from proposedParent up the ancestor chain.
func detectCycle(index *vault.TitlesIndex, childUUID, proposedParentUUID string) error {
	current := proposedParentUUID
	for i := 0; i < 100; i++ {
		if current == childUUID {
			return fmt.Errorf("cycle detected: setting this parent would create a circular relationship")
		}
		entry, ok := index.Titles[current]
		if !ok || entry.Parent == "" {
			return nil
		}
		current = entry.Parent
	}
	return fmt.Errorf("parent chain exceeds maximum depth of 100")
}
