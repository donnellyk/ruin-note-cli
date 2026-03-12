package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
)

var headingPattern = regexp.MustCompile(`(?m)^(#{1,6})\s`)
var listLinePattern = regexp.MustCompile(`^[ \t]*(?:[-*+]|\d+\.)\s`)

// isListOnlyContent returns true if every non-blank line starts with a
// markdown list marker (-, *, +, 1.) including checkbox variants.
func isListOnlyContent(content string) bool {
	if strings.TrimSpace(content) == "" {
		return false
	}
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !listLinePattern.MatchString(line) {
			return false
		}
	}
	return true
}

// NewComposeCmd creates the compose command.
func NewComposeCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		maxDepth         int
		stripTitle       bool
		stripGlobalTag   bool
		sortBy           string
		edit             bool
		force            bool
		content          bool
		normalizeHeaders bool
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
  ruin compose <uuid> --normalize-headers
  ruin compose <uuid> --sort created:desc
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

			sortField, err := parseComposeSort(sortBy)
			if err != nil {
				return err
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
				sortChildUUIDs(vlt, index, uuids, sortField)
			}

			if edit {
				results := collectTreeNotes(vlt, index, childrenMap, root.UUID, make(map[string]bool), maxDepth, 0)
				if len(results) == 0 {
					return fmt.Errorf("no notes found in tree")
				}
				return handleEdit(vlt, results, force, FrontmatterNone)
			}

			if *jsonOutput {
				tree := composeJSON(vlt, index, childrenMap, root.UUID, make(map[string]bool), maxDepth, 0, stripTitle, stripGlobalTag, normalizeHeaders, content)
				composedText, sourceMap := composeTextWithSourceMap(vlt, index, childrenMap, root.UUID, make(map[string]bool), maxDepth, 0, stripTitle, stripGlobalTag, normalizeHeaders)
				tree.ComposedContent = composedText
				tree.SourceMap = sourceMap
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(tree)
			}

			// Text output
			var b strings.Builder
			composeText(vlt, index, childrenMap, root.UUID, make(map[string]bool), &b, maxDepth, 0, stripTitle, stripGlobalTag, normalizeHeaders)
			fmt.Print(b.String())
			return nil
		},
	}

	cmd.Flags().IntVar(&maxDepth, "depth", 0, "max recursion depth (0 = unlimited)")
	cmd.Flags().BoolVar(&stripTitle, "strip-title", false, "remove H1 title from root note")
	cmd.Flags().BoolVar(&stripGlobalTag, "strip-global-tags", false, "remove global tag lines")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "title", "child ordering: field[:dir] (e.g., created:desc)")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "open tree notes in $EDITOR")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation for deletions in edit mode")
	cmd.Flags().BoolVar(&content, "content", false, "include per-node content in JSON output")
	cmd.Flags().BoolVar(&normalizeHeaders, "normalize-headers", false, "normalize child headings so siblings share the same top-level")
	return cmd
}

// parseComposeSort parses a sort string for the compose command.
// Format: field[:direction] where direction is asc or desc.
func parseComposeSort(s string) (SortField, error) {
	field := SortField{Ascending: true}

	if idx := strings.Index(s, ":"); idx > 0 {
		field.Field = strings.ToLower(s[:idx])
		dir := strings.ToLower(s[idx+1:])
		switch dir {
		case "asc":
			field.Ascending = true
		case "desc":
			field.Ascending = false
		default:
			return field, fmt.Errorf("invalid sort direction: %s (use asc or desc)", dir)
		}
	} else {
		field.Field = strings.ToLower(s)
	}

	switch field.Field {
	case "created", "title", "order":
		// valid
	default:
		return field, fmt.Errorf("invalid sort field: %s (use created, title, or order)", field.Field)
	}

	return field, nil
}

func sortChildUUIDs(_ *vault.Vault, index *vault.TitlesIndex, uuids []string, sf SortField) {
	ascending := sf.Ascending
	switch sf.Field {
	case "created":
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
			if ascending {
				return items[i].created < items[j].created
			}
			return items[i].created > items[j].created
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
				if ascending {
					return iSet
				}
				return jSet
			}
			if iSet && *ni.Order != *nj.Order {
				if ascending {
					return *ni.Order < *nj.Order
				}
				return *ni.Order > *nj.Order
			}
			return ei.Title < ej.Title
		})
	default: // "title"
		sort.Slice(uuids, func(i, j int) bool {
			if ascending {
				return index.Titles[uuids[i]].Title < index.Titles[uuids[j]].Title
			}
			return index.Titles[uuids[i]].Title > index.Titles[uuids[j]].Title
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

func composeText(vlt *vault.Vault, index *vault.TitlesIndex, childrenMap map[string][]string, uuid string, visited map[string]bool, b *strings.Builder, maxDepth, depth int, stripTitle, stripGlobalTags, normalizeHeaders bool) {
	text, _ := composeTextWithSourceMap(vlt, index, childrenMap, uuid, visited, maxDepth, depth, stripTitle, stripGlobalTags, normalizeHeaders)
	b.WriteString(text)
}

func composeTextWithSourceMap(vlt *vault.Vault, index *vault.TitlesIndex, childrenMap map[string][]string, uuid string, visited map[string]bool, maxDepth, depth int, stripTitle, stripGlobalTags, normalizeHeaders bool) (string, []sourceEntry) {
	var b strings.Builder
	var sourceMap []sourceEntry
	nextLine := 1

	var prevDepth int
	var prevListOnly bool

	var walk func(uuid string, depth int)
	walk = func(uuid string, depth int) {
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
			if normalizeHeaders {
				content = normalizeHeadings(content, depth)
			} else {
				content = adjustHeadings(content, depth)
			}
		}

		// Add separator between notes (\n\n = line terminator + blank line = 1 gap line)
		// Suppress blank line between list-only siblings at the same depth.
		if b.Len() > 0 {
			listOnly := isListOnlyContent(content)
			if depth == prevDepth && prevListOnly && listOnly {
				b.WriteString("\n")
			} else {
				b.WriteString("\n\n")
				nextLine++
			}
		}

		startLine := nextLine
		lineCount := strings.Count(content, "\n") + 1
		endLine := startLine + lineCount - 1

		sourceMap = append(sourceMap, sourceEntry{
			UUID:      uuid,
			Path:      entry.Path,
			Title:     entry.Title,
			StartLine: startLine,
			EndLine:   endLine,
		})

		b.WriteString(content)
		nextLine = endLine + 1
		prevDepth = depth
		prevListOnly = isListOnlyContent(content)

		// Recurse into children
		if maxDepth > 0 && depth >= maxDepth {
			return
		}

		for _, childUUID := range childrenMap[uuid] {
			walk(childUUID, depth+1)
		}
	}

	walk(uuid, depth)
	return b.String(), sourceMap
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

// normalizeHeadings rebases headings so the minimum heading level in the
// content maps to depth+1. Sibling notes at the same depth will share the
// same top-level heading regardless of their original heading levels.
func normalizeHeadings(content string, depth int) string {
	targetLevel := depth + 1

	// Find the minimum heading level in the content
	minLevel := 7
	for _, match := range headingPattern.FindAllString(content, -1) {
		hashes := 0
		for _, c := range match {
			if c == '#' {
				hashes++
			} else {
				break
			}
		}
		if hashes < minLevel {
			minLevel = hashes
		}
	}

	if minLevel == 7 {
		return content // no headings
	}

	delta := targetLevel - minLevel

	if delta == 0 {
		return content
	}

	return headingPattern.ReplaceAllStringFunc(content, func(match string) string {
		hashes := 0
		for _, c := range match {
			if c == '#' {
				hashes++
			} else {
				break
			}
		}

		newLevel := hashes + delta
		if newLevel < 1 {
			newLevel = 1
		}
		if newLevel > 6 {
			newLevel = 6
		}

		return strings.Repeat("#", newLevel) + match[hashes:]
	})
}

type sourceEntry struct {
	UUID      string `json:"uuid"`
	Path      string `json:"path"`
	Title     string `json:"title"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type composeNode struct {
	UUID            string        `json:"uuid"`
	Title           string        `json:"title"`
	Path            string        `json:"path"`
	Content         string        `json:"content,omitempty"`
	Children        []composeNode `json:"children,omitempty"`
	ComposedContent string        `json:"composed_content,omitempty"`
	SourceMap       []sourceEntry `json:"source_map,omitempty"`
}

func composeJSON(vlt *vault.Vault, index *vault.TitlesIndex, childrenMap map[string][]string, uuid string, visited map[string]bool, maxDepth, depth int, stripTitle, stripGlobalTags, normalizeHeaders, includeContent bool) composeNode {
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

	if includeContent {
		n, err := note.Load(entry.Path)
		if err == nil {
			content := n.Content
			if depth == 0 && stripTitle {
				content = note.StripTitle(content)
			}
			if stripGlobalTags {
				content = note.StripGlobalTags(content, n.InlineTags)
			}
			if depth > 0 {
				if normalizeHeaders {
					content = normalizeHeadings(content, depth)
				} else {
					content = adjustHeadings(content, depth)
				}
			}
			node.Content = content
		}
	}

	if maxDepth > 0 && depth >= maxDepth {
		return node
	}

	for _, childUUID := range childrenMap[uuid] {
		child := composeJSON(vlt, index, childrenMap, childUUID, visited, maxDepth, depth+1, stripTitle, stripGlobalTags, normalizeHeaders, includeContent)
		node.Children = append(node.Children, child)
	}

	return node
}
