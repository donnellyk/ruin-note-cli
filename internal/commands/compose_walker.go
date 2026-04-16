package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

type composeTree struct {
	UUID     string
	Title    string
	Path     string
	Depth    int
	Content  string
	Embedded bool
	Segments []composeSegment
	Children []*composeTree
	Dynamic  *dynamicInfo // non-nil for dynamic embed nodes

	// Attribution maps line ranges within Content back to their source notes.
	// Populated by dynamic embed renderers so the sourcemap has a per-line
	// UUID even when multiple notes contribute to a single rendered block.
	Attribution []attributionEntry
}

type attributionEntry struct {
	UUID       string
	Title      string
	Path       string
	LineOffset int // 0-based line offset within Content
	LineCount  int // number of lines covered by this entry
}

type dynamicInfo struct {
	Type        string            `json:"type"`
	Query       string            `json:"query"`
	Options     map[string]string `json:"options,omitempty"`
	ResultCount int               `json:"result_count"`
}

type composeSegment struct {
	Text  string
	Embed *composeTree
}

type composeWalker struct {
	vault            *vault.Vault
	index            *vault.TitlesIndex
	childrenMap      map[string][]string
	visited          map[string]bool
	maxDepth         int
	stripTitle       bool
	stripGlobalTags  bool
	normalizeHeaders bool
	expandEmbeds     bool
	expandDynamic    bool   // enable dynamic embed expansion (![[search: ...]] etc.)
	rootUUID         string // UUID of the compose root (excluded from dynamic search/pick results)
}

func newComposeWalker(vlt *vault.Vault, index *vault.TitlesIndex, childrenMap map[string][]string, maxDepth int, stripTitle, stripGlobalTags, normalizeHeaders bool) *composeWalker {
	return &composeWalker{
		vault:            vlt,
		index:            index,
		childrenMap:      childrenMap,
		visited:          make(map[string]bool),
		maxDepth:         maxDepth,
		stripTitle:       stripTitle,
		stripGlobalTags:  stripGlobalTags,
		normalizeHeaders: normalizeHeaders,
	}
}

func (w *composeWalker) Walk(uuid string, depth int) *composeTree {
	if w.visited[uuid] {
		return nil
	}
	w.visited[uuid] = true

	entry, ok := w.index.Titles[uuid]
	if !ok {
		return nil
	}

	n, err := note.Load(entry.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load %s: %v\n", entry.Path, err)
		return nil
	}

	content := n.Content

	if depth == 0 && w.stripTitle {
		content = note.StripTitle(content)
	}
	if w.stripGlobalTags {
		content = note.StripGlobalTags(content, n.InlineTags)
	}
	if depth > 0 {
		if w.normalizeHeaders {
			content = normalizeHeadings(content, depth)
		} else {
			content = adjustHeadings(content, depth)
		}
	}

	tree := &composeTree{
		UUID:    uuid,
		Title:   entry.Title,
		Path:    entry.Path,
		Depth:   depth,
		Content: content,
	}

	if w.maxDepth > 0 && depth >= w.maxDepth {
		return tree
	}

	if w.expandEmbeds {
		w.expandEmbedsInTree(tree, content, depth)
	} else {
		for _, childUUID := range w.childrenMap[uuid] {
			child := w.Walk(childUUID, depth+1)
			if child != nil {
				tree.Children = append(tree.Children, child)
			}
		}
	}

	return tree
}

func (w *composeWalker) expandEmbedsInTree(tree *composeTree, content string, depth int) {
	var dynEmbeds []note.DynamicEmbedRef
	if w.expandDynamic {
		dynEmbeds = note.FindDynamicEmbeds(content)
	}
	staticEmbeds := note.FindEmbeds(content)

	// Static detection must skip lines already consumed by dynamic embeds.
	dynLines := make(map[int]bool)
	for _, de := range dynEmbeds {
		dynLines[de.Line] = true
	}
	var filteredStatic []note.EmbedRef
	for _, se := range staticEmbeds {
		if !dynLines[se.Line] {
			filteredStatic = append(filteredStatic, se)
		}
	}

	if len(dynEmbeds) == 0 && len(filteredStatic) == 0 {
		for _, childUUID := range w.childrenMap[tree.UUID] {
			child := w.Walk(childUUID, depth+1)
			if child != nil {
				tree.Children = append(tree.Children, child)
			}
		}
		return
	}

	var allEmbeds []embedLine
	for i, se := range filteredStatic {
		allEmbeds = append(allEmbeds, embedLine{line: se.Line, staticIdx: i})
	}
	for i, de := range dynEmbeds {
		allEmbeds = append(allEmbeds, embedLine{line: de.Line, isDynamic: true, dynIdx: i})
	}
	sortEmbedLines(allEmbeds)

	embeddedUUIDs := make(map[string]bool)
	lines := strings.Split(content, "\n")

	embedLineNums := make([]int, len(allEmbeds))
	for i, el := range allEmbeds {
		embedLineNums[i] = el.line
	}
	segments := splitByLines(lines, embedLineNums)
	tree.Content = ""

	// Track the preceding heading level so embeds nest beneath the most
	// recent heading in the source note, not just the note's depth.
	lastHeadingLevel := 0
	for i, seg := range segments {
		segText := seg
		if lvl, ok := lastHeadingLevelInText(seg); ok {
			lastHeadingLevel = lvl
		}

		if i < len(allEmbeds) {
			el := allEmbeds[i]
			// Base level at which the embed's first heading should render:
			// one below the preceding heading, or depth+1 if none.
			baseLevel := max(depth+1, lastHeadingLevel)

			if el.isDynamic {
				dynRef := dynEmbeds[el.dynIdx]
				// textResult adds +1 internally; pass baseLevel-1 so target lands at baseLevel.
				result := w.expandDynamicEmbed(dynRef, baseLevel-1, tree.UUID)
				if result != nil {
					tree.Segments = append(tree.Segments, composeSegment{Text: segText})
					tree.Segments = append(tree.Segments, result.segments...)
					for _, uuid := range result.embeddedUUIDs {
						embeddedUUIDs[uuid] = true
					}
					continue
				}
				segText = segText + "\n" + lines[el.line]
			} else {
				se := filteredStatic[el.staticIdx]
				uuid := w.resolveEmbedRef(se.NoteRef)
				if uuid != "" {
					embeddedUUIDs[uuid] = true
				}
				if uuid == "" || w.visited[uuid] {
					if uuid != "" {
						fmt.Fprintf(os.Stderr, "warning: skipping circular embed of %q\n", se.NoteRef)
					} else {
						fmt.Fprintf(os.Stderr, "warning: ![[%s]] could not resolve, left unexpanded\n", se.NoteRef)
					}
					segText = segText + "\n" + lines[se.Line]
				} else {
					embedTree := w.buildEmbedTree(uuid, se.Header, baseLevel)
					tree.Segments = append(tree.Segments, composeSegment{
						Text:  segText,
						Embed: embedTree,
					})
					continue
				}
			}
		}

		tree.Segments = append(tree.Segments, composeSegment{Text: segText})
	}

	for _, childUUID := range w.childrenMap[tree.UUID] {
		if embeddedUUIDs[childUUID] {
			continue
		}
		child := w.Walk(childUUID, depth+1)
		if child != nil {
			tree.Children = append(tree.Children, child)
		}
	}
}

type dynamicResult struct {
	segments      []composeSegment
	embeddedUUIDs []string
}

type embedLine struct {
	line      int
	isDynamic bool
	staticIdx int
	dynIdx    int
}

// lastHeadingLevelInText returns the level of the last heading line in text.
// Tracks embed context so dynamic/static embeds nest below the preceding heading.
func lastHeadingLevelInText(text string) (int, bool) {
	matches := headingPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return 0, false
	}
	last := matches[len(matches)-1]
	level := 0
	for _, c := range last {
		if c == '#' {
			level++
		} else {
			break
		}
	}
	return level, true
}

func sortEmbedLines(lines []embedLine) {
	for i := 1; i < len(lines); i++ {
		for j := i; j > 0 && lines[j].line < lines[j-1].line; j-- {
			lines[j], lines[j-1] = lines[j-1], lines[j]
		}
	}
}

func splitByLines(lines []string, lineNums []int) []string {
	if len(lineNums) == 0 {
		return []string{strings.Join(lines, "\n")}
	}
	var segments []string
	prev := 0
	for _, ln := range lineNums {
		segments = append(segments, strings.Join(lines[prev:ln], "\n"))
		prev = ln + 1
	}
	segments = append(segments, strings.Join(lines[prev:], "\n"))
	return segments
}

func (w *composeWalker) resolveEmbedRef(ref string) string {
	n, err := ResolveNote(w.vault, ref)
	if err != nil {
		return ""
	}
	return n.UUID
}

func (w *composeWalker) buildEmbedTree(uuid, header string, depth int) *composeTree {
	if w.visited[uuid] {
		return nil
	}
	w.visited[uuid] = true

	entry, ok := w.index.Titles[uuid]
	if !ok {
		return nil
	}

	n, err := note.Load(entry.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load %s: %v\n", entry.Path, err)
		return nil
	}

	content := n.Content
	if header != "" {
		section, err := note.ExtractSection(content, header)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v in %s\n", err, entry.Title)
			return nil
		}
		content = section
	}

	if w.stripGlobalTags {
		content = note.StripGlobalTags(content, n.InlineTags)
	}
	if w.normalizeHeaders {
		content = normalizeHeadings(content, depth)
	} else {
		content = adjustHeadings(content, depth)
	}

	tree := &composeTree{
		UUID:     uuid,
		Title:    entry.Title,
		Path:     entry.Path,
		Depth:    depth,
		Content:  content,
		Embedded: true,
	}

	if w.maxDepth > 0 && depth >= w.maxDepth {
		return tree
	}

	if w.expandEmbeds {
		w.expandEmbedsInTree(tree, content, depth)
	} else {
		for _, childUUID := range w.childrenMap[uuid] {
			child := w.Walk(childUUID, depth+1)
			if child != nil {
				tree.Children = append(tree.Children, child)
			}
		}
	}

	return tree
}

func splitByEmbedLines(lines []string, embeds []note.EmbedRef) []string {
	if len(embeds) == 0 {
		return []string{strings.Join(lines, "\n")}
	}

	var segments []string
	prev := 0
	for _, embed := range embeds {
		seg := strings.Join(lines[prev:embed.Line], "\n")
		segments = append(segments, seg)
		prev = embed.Line + 1
	}
	segments = append(segments, strings.Join(lines[prev:], "\n"))
	return segments
}

func renderText(tree *composeTree) (string, []sourceEntry) {
	var b strings.Builder
	var sourceMap []sourceEntry
	nextLine := 1
	var prevDepth int
	var prevListOnly bool

	writeBlock := func(content string, depth int) (startLine int) {
		if b.Len() > 0 {
			listOnly := isListOnlyContent(content)
			if depth == prevDepth && prevListOnly && listOnly {
				b.WriteString("\n")
			} else {
				b.WriteString("\n\n")
				nextLine++
			}
		}

		startLine = nextLine
		lineCount := strings.Count(content, "\n") + 1
		endLine := startLine + lineCount - 1

		b.WriteString(content)
		nextLine = endLine + 1
		prevDepth = depth
		prevListOnly = isListOnlyContent(content)
		return startLine
	}

	writeContent := func(uuid, path, title, content string, depth int) {
		start := writeBlock(content, depth)
		lineCount := strings.Count(content, "\n") + 1
		sourceMap = append(sourceMap, sourceEntry{
			UUID:      uuid,
			Path:      path,
			Title:     title,
			StartLine: start,
			EndLine:   start + lineCount - 1,
		})
	}

	writeAttributed := func(node *composeTree) {
		start := writeBlock(node.Content, node.Depth)
		for _, attr := range node.Attribution {
			sourceMap = append(sourceMap, sourceEntry{
				UUID:      attr.UUID,
				Path:      attr.Path,
				Title:     attr.Title,
				StartLine: start + attr.LineOffset,
				EndLine:   start + attr.LineOffset + attr.LineCount - 1,
			})
		}
	}

	var walk func(node *composeTree)
	walk = func(node *composeTree) {
		if len(node.Segments) > 0 {
			for _, seg := range node.Segments {
				if strings.TrimSpace(seg.Text) != "" {
					writeContent(node.UUID, node.Path, node.Title, seg.Text, node.Depth)
				}
				if seg.Embed != nil {
					walk(seg.Embed)
					for _, child := range seg.Embed.Children {
						walk(child)
					}
				}
			}
		} else if strings.TrimSpace(node.Content) != "" {
			if len(node.Attribution) > 0 {
				writeAttributed(node)
			} else {
				writeContent(node.UUID, node.Path, node.Title, node.Content, node.Depth)
			}
		}

		for _, child := range node.Children {
			walk(child)
		}

	}

	walk(tree)
	return b.String(), sourceMap
}

func renderJSON(tree *composeTree, includeContent bool) composeNode {
	node := composeNode{
		UUID:     tree.UUID,
		Title:    tree.Title,
		Path:     tree.Path,
		Embedded: tree.Embedded,
		Dynamic:  tree.Dynamic,
	}

	if includeContent {
		if len(tree.Segments) > 0 {
			var parts []string
			for _, seg := range tree.Segments {
				parts = append(parts, seg.Text)
			}
			node.Content = strings.Join(parts, "\n")
		} else {
			node.Content = tree.Content
		}
	}

	if len(tree.Segments) > 0 {
		for _, seg := range tree.Segments {
			if seg.Embed != nil {
				embedNode := renderJSON(seg.Embed, includeContent)
				node.Children = append(node.Children, embedNode)
				for _, child := range seg.Embed.Children {
					node.Children = append(node.Children, renderJSON(child, includeContent))
				}
			}
		}
	}

	for _, child := range tree.Children {
		node.Children = append(node.Children, renderJSON(child, includeContent))
	}

	return node
}

func renderEditList(tree *composeTree) []SearchResult {
	var results []SearchResult
	seen := make(map[string]bool)

	var walk func(node *composeTree)
	walk = func(node *composeTree) {
		// Skip dynamic containers (no path).
		if node.Path != "" && !seen[node.UUID] {
			n, err := note.Load(node.Path)
			if err == nil {
				seen[node.UUID] = true
				results = append(results, SearchResult{
					Path:   node.Path,
					UUID:   node.UUID,
					Title:  node.Title,
					Tags:   n.Tags,
					Parent: n.Parent,
					note:   n,
				})
			}
		}
		if len(node.Segments) > 0 {
			for _, seg := range node.Segments {
				if seg.Embed != nil {
					walk(seg.Embed)
					for _, child := range seg.Embed.Children {
						walk(child)
					}
				}
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}

	walk(tree)
	return results
}
