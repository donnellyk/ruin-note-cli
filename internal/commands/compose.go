package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

var headingPattern = regexp.MustCompile(`(?m)^(#{1,6})\s`)

// NewComposeCmd creates the compose command.
func NewComposeCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		maxDepth       int
		stripTitle     bool
		stripGlobalTag bool
		sortBy         string
		edit           bool
		force          bool
		content        bool
	)

	cmd := &cobra.Command{
		Use:   "compose <note>",
		Short: "Assemble a document from a note tree",
		Long: `Recursively compose a document from a note and its children.

Starting from the given note, includes the note's content followed by all
children's content. Headings in child notes are adjusted by depth level.

Children are sorted by title by default.`,
		Example: `  ruin compose <uuid>
  ruin compose "Project Plan" --depth 2
  ruin compose <uuid> --strip-title --strip-global-tags
  ruin compose <uuid> --sort created
  ruin compose <uuid> --json
  ruin compose <uuid> --edit`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			if edit && *jsonOutput {
				return errMutuallyExclusive("--json", "--edit")
			}

			if content && !*jsonOutput {
				return fmt.Errorf("--content requires --json")
			}

			root, err := ResolveNote(vlt, args[0])
			if err != nil {
				return err
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

			// Sort children
			for parent := range childrenMap {
				uuids := childrenMap[parent]
				sortChildUUIDs(vlt, index, uuids, sortBy)
			}

			if edit {
				results := collectTreeNotes(vlt, index, childrenMap, root.UUID, make(map[string]bool), maxDepth, 0)
				if len(results) == 0 {
					return fmt.Errorf("no notes found in tree")
				}
				return handleEdit(vlt, results, force, FrontmatterNone)
			}

			if *jsonOutput {
				tree := composeJSON(vlt, index, childrenMap, root.UUID, make(map[string]bool), maxDepth, 0, stripTitle, stripGlobalTag)
				if content {
					var b strings.Builder
					composeText(vlt, index, childrenMap, root.UUID, make(map[string]bool), &b, maxDepth, 0, stripTitle, stripGlobalTag)
					tree.Content = b.String()
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(tree)
			}

			// Text output
			var b strings.Builder
			composeText(vlt, index, childrenMap, root.UUID, make(map[string]bool), &b, maxDepth, 0, stripTitle, stripGlobalTag)
			fmt.Print(b.String())
			return nil
		},
	}

	cmd.Flags().IntVar(&maxDepth, "depth", 0, "max recursion depth (0 = unlimited)")
	cmd.Flags().BoolVar(&stripTitle, "strip-title", false, "remove H1 title from root note")
	cmd.Flags().BoolVar(&stripGlobalTag, "strip-global-tags", false, "remove global tag lines")
	cmd.Flags().StringVar(&sortBy, "sort", "title", "child ordering: title, created, or order")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "open tree notes in $EDITOR")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation for deletions in edit mode")
	cmd.Flags().BoolVar(&content, "content", false, "include full composed document in JSON content field")
	return cmd
}

func sortChildUUIDs(_ *vault.Vault, index *vault.TitlesIndex, uuids []string, sortBy string) {
	switch sortBy {
	case "created":
		// Need to load notes to get created time
		type uuidTime struct {
			uuid    string
			created string
		}
		var items []uuidTime
		for _, uuid := range uuids {
			entry := index.Titles[uuid]
			n, err := note.Load(entry.Path)
			if err != nil {
				items = append(items, uuidTime{uuid: uuid})
				continue
			}
			items = append(items, uuidTime{uuid: uuid, created: n.Created.Format(note.TimeFormat)})
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].created < items[j].created
		})
		for i, item := range items {
			uuids[i] = item.uuid
		}
	case "order":
		sort.Slice(uuids, func(i, j int) bool {
			ei := index.Titles[uuids[i]]
			ej := index.Titles[uuids[j]]
			ni, erri := note.Load(ei.Path)
			nj, errj := note.Load(ej.Path)
			if erri != nil || errj != nil {
				return uuids[i] < uuids[j]
			}
			iSet, jSet := ni.Order != nil, nj.Order != nil
			if iSet != jSet {
				return iSet // set before unset
			}
			if iSet && *ni.Order != *nj.Order {
				return *ni.Order < *nj.Order
			}
			return ei.Title < ej.Title
		})
	default: // "title"
		sort.Slice(uuids, func(i, j int) bool {
			return index.Titles[uuids[i]].Title < index.Titles[uuids[j]].Title
		})
	}
}

// collectTreeNotes walks the compose tree and returns SearchResults for each note.
// Notes are returned in compose order (parent first, then sorted children).
func collectTreeNotes(vlt *vault.Vault, index *vault.TitlesIndex, childrenMap map[string][]string, uuid string, visited map[string]bool, maxDepth, depth int) []SearchResult {
	if visited[uuid] {
		return nil
	}
	visited[uuid] = true

	entry, ok := index.Titles[uuid]
	if !ok {
		return nil
	}

	n, err := note.Load(entry.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load %s: %v\n", entry.Path, err)
		return nil
	}

	results := []SearchResult{{
		Path:   entry.Path,
		UUID:   n.UUID,
		Title:  n.Title,
		Tags:   n.Tags,
		Parent: n.Parent,
		note:   n,
	}}

	if maxDepth > 0 && depth >= maxDepth {
		return results
	}

	for _, childUUID := range childrenMap[uuid] {
		results = append(results, collectTreeNotes(vlt, index, childrenMap, childUUID, visited, maxDepth, depth+1)...)
	}

	return results
}

func composeText(vlt *vault.Vault, index *vault.TitlesIndex, childrenMap map[string][]string, uuid string, visited map[string]bool, b *strings.Builder, maxDepth, depth int, stripTitle, stripGlobalTags bool) {
	if visited[uuid] {
		return
	}
	visited[uuid] = true

	entry, ok := index.Titles[uuid]
	if !ok {
		return
	}

	n, err := note.Load(entry.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load %s: %v\n", entry.Path, err)
		return
	}

	content := n.Content

	// Strip title from root only
	if depth == 0 && stripTitle {
		content = note.StripTitle(content)
	}

	// Strip global tags
	if stripGlobalTags {
		content = note.StripGlobalTags(content, n.InlineTags)
	}

	// Adjust headings for children
	if depth > 0 {
		content = adjustHeadings(content, depth)
	}

	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString(content)

	// Recurse into children
	if maxDepth > 0 && depth >= maxDepth {
		return
	}

	for _, childUUID := range childrenMap[uuid] {
		composeText(vlt, index, childrenMap, childUUID, visited, b, maxDepth, depth+1, stripTitle, stripGlobalTags)
	}
}

// adjustHeadings adds `depth` additional # to each heading, capping at H6.
func adjustHeadings(content string, depth int) string {
	return headingPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Count existing #
		hashes := 0
		for _, c := range match {
			if c == '#' {
				hashes++
			} else {
				break
			}
		}

		newLevel := hashes + depth
		if newLevel > 6 {
			newLevel = 6
		}

		return strings.Repeat("#", newLevel) + match[hashes:]
	})
}

type composeNode struct {
	UUID     string        `json:"uuid"`
	Title    string        `json:"title"`
	Path     string        `json:"path"`
	Content  string        `json:"content"`
	Children []composeNode `json:"children,omitempty"`
}

func composeJSON(vlt *vault.Vault, index *vault.TitlesIndex, childrenMap map[string][]string, uuid string, visited map[string]bool, maxDepth, depth int, stripTitle, stripGlobalTags bool) composeNode {
	entry := index.Titles[uuid]
	node := composeNode{
		UUID:  uuid,
		Title: entry.Title,
		Path:  entry.Path,
	}

	if visited[uuid] {
		return node
	}
	visited[uuid] = true

	n, err := note.Load(entry.Path)
	if err != nil {
		return node
	}

	content := n.Content
	if depth == 0 && stripTitle {
		content = note.StripTitle(content)
	}
	if stripGlobalTags {
		content = note.StripGlobalTags(content, n.InlineTags)
	}
	if depth > 0 {
		content = adjustHeadings(content, depth)
	}
	node.Content = content

	if maxDepth > 0 && depth >= maxDepth {
		return node
	}

	for _, childUUID := range childrenMap[uuid] {
		child := composeJSON(vlt, index, childrenMap, childUUID, visited, maxDepth, depth+1, stripTitle, stripGlobalTags)
		node.Children = append(node.Children, child)
	}

	return node
}
