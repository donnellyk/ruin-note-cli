package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// NewParentCmd creates the parent command with subcommands.
func NewParentCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parent",
		Short: "Query parent-child relationships and manage bookmarks",
		Long:  `Commands for viewing parent-child relationships and managing named parent bookmarks.`,
	}

	cmd.AddCommand(newParentGetCmd(getVault, jsonOutput))
	cmd.AddCommand(newParentChildrenCmd(getVault, jsonOutput))
	cmd.AddCommand(newParentTreeCmd(getVault, jsonOutput))
	cmd.AddCommand(newParentSaveCmd(getVault, jsonOutput))
	cmd.AddCommand(newParentListCmd(getVault, jsonOutput))
	cmd.AddCommand(newParentDeleteCmd(getVault, jsonOutput))

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
	childrenMap := index.ChildrenMap()

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

			childrenMap := index.ChildrenMap()

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

func newParentSaveCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "save <name> <note>",
		Short: "Save a named parent bookmark",
		Long: `Save a named bookmark that maps a short name to a note UUID.

The bookmark can then be used anywhere a note reference is accepted
(e.g., --parent, compose, parent set/get/remove/children/tree).`,
		Example: `  ruin parent save alpha "Project Alpha Hub"
  ruin parent save docs "Documentation Root" --force`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			name := args[0]

			index, err := vlt.LoadParents()
			if err != nil {
				return fmt.Errorf("failed to load parents: %w", err)
			}

			existingIdx := -1
			for i, p := range index.Parents {
				if p.Name == name {
					existingIdx = i
					break
				}
			}

			if existingIdx >= 0 && !force {
				if !isTerminal(os.Stderr) {
					return fmt.Errorf("non-interactive mode requires --force")
				}

				existing := index.Parents[existingIdx]
				fmt.Fprintf(os.Stderr, "Bookmark %q already exists (UUID: %s). Overwrite? [y/N] ", name, existing.UUID)
				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read response: %w", err)
				}
				response = strings.TrimSpace(strings.ToLower(response))
				if response != "y" && response != "yes" {
					return ErrUserAborted
				}
			}

			n, err := ResolveNote(vlt, args[1])
			if err != nil {
				return fmt.Errorf("note: %w", err)
			}

			if existingIdx >= 0 {
				index.Parents[existingIdx].UUID = n.UUID
			} else {
				index.Parents = append(index.Parents, vault.ParentEntry{
					Name: name,
					UUID: n.UUID,
				})
			}

			if err := vlt.SaveParents(index); err != nil {
				return fmt.Errorf("failed to save parents: %w", err)
			}

			if *jsonOutput {
				output := struct {
					Name  string `json:"name"`
					UUID  string `json:"uuid"`
					Title string `json:"title"`
				}{
					Name:  name,
					UUID:  n.UUID,
					Title: n.Title,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Fprintf(os.Stderr, "Saved parent %q -> %s (%s)\n", name, n.Title, n.UUID)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation when overwriting")
	return cmd
}

func newParentListCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved parent bookmarks",
		Example: `  ruin parent list
  ruin parent list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			index, err := vlt.LoadParents()
			if err != nil {
				return fmt.Errorf("failed to load parents: %w", err)
			}

			titles, _ := vlt.LoadTitles()

			if *jsonOutput {
				type listEntry struct {
					Name  string `json:"name"`
					UUID  string `json:"uuid,omitempty"`
					Title string `json:"title,omitempty"`
				}
				var entries []listEntry
				for _, p := range index.Parents {
					title := ""
					if titles != nil {
						if te, ok := titles.Titles[p.UUID]; ok {
							title = te.Title
						}
					}
					entries = append(entries, listEntry{
						Name:  p.Name,
						UUID:  p.UUID,
						Title: title,
					})
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			if len(index.Parents) == 0 {
				fmt.Println("No saved parent bookmarks")
				return nil
			}

			for _, p := range index.Parents {
				title := p.UUID
				if titles != nil {
					if te, ok := titles.Titles[p.UUID]; ok {
						title = te.Title
					}
				}
				fmt.Printf("%s: %s (%s)\n", p.Name, title, p.UUID)
			}
			return nil
		},
	}

	return cmd
}

func newParentDeleteCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved parent bookmark",
		Example: `  ruin parent delete alpha
  ruin parent delete alpha --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			name := args[0]

			index, err := vlt.LoadParents()
			if err != nil {
				return fmt.Errorf("failed to load parents: %w", err)
			}

			found := -1
			for i, p := range index.Parents {
				if p.Name == name {
					found = i
					break
				}
			}

			if found == -1 {
				return fmt.Errorf("parent bookmark not found: %s", name)
			}

			if !force {
				if !isTerminal(os.Stderr) {
					return fmt.Errorf("non-interactive mode requires --force")
				}

				fmt.Fprintf(os.Stderr, "Delete bookmark %q? [y/N] ", name)
				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read response: %w", err)
				}
				response = strings.TrimSpace(strings.ToLower(response))
				if response != "y" && response != "yes" {
					return ErrUserAborted
				}
			}

			index.Parents = append(index.Parents[:found], index.Parents[found+1:]...)

			if err := vlt.SaveParents(index); err != nil {
				return fmt.Errorf("failed to save parents: %w", err)
			}

			if *jsonOutput {
				output := struct {
					Deleted string `json:"deleted"`
				}{Deleted: name}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Fprintf(os.Stderr, "Deleted parent bookmark %q\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	return cmd
}

// detectCycle checks if setting proposedParentUUID as the parent of childUUID
// would create a cycle. It walks from proposedParent up the ancestor chain.
func detectCycle(index *vault.TitlesIndex, childUUID, proposedParentUUID string) error {
	current := proposedParentUUID
	for range 100 {
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
