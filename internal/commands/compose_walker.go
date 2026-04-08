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
	embeds := note.FindEmbeds(content)
	if len(embeds) == 0 {
		for _, childUUID := range w.childrenMap[tree.UUID] {
			child := w.Walk(childUUID, depth+1)
			if child != nil {
				tree.Children = append(tree.Children, child)
			}
		}
		return
	}

	embeddedUUIDs := make(map[string]bool)
	lines := strings.Split(content, "\n")

	type resolvedEmbed struct {
		ref  note.EmbedRef
		uuid string
	}
	var resolved []resolvedEmbed
	for _, embed := range embeds {
		uuid := w.resolveEmbedRef(embed.NoteRef)
		if uuid != "" {
			embeddedUUIDs[uuid] = true
		}
		resolved = append(resolved, resolvedEmbed{ref: embed, uuid: uuid})
	}

	segments := splitByEmbedLines(lines, embeds)
	tree.Content = ""

	embedIdx := 0
	for i, seg := range segments {
		segText := seg
		var embedTree *composeTree

		if i < len(resolved) {
			re := resolved[embedIdx]
			embedIdx++

			if re.uuid == "" || w.visited[re.uuid] {
				if re.uuid != "" {
					fmt.Fprintf(os.Stderr, "warning: skipping circular embed of %q\n", re.ref.NoteRef)
				} else {
					fmt.Fprintf(os.Stderr, "warning: ![[%s]] could not resolve, left unexpanded\n", re.ref.NoteRef)
				}
				segText = segText + "\n" + lines[re.ref.Line]
			} else {
				embedTree = w.buildEmbedTree(re.uuid, re.ref.Header, depth+1)
			}
		}

		tree.Segments = append(tree.Segments, composeSegment{
			Text:  segText,
			Embed: embedTree,
		})
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

	var writeContent func(uuid, path, title string, content string, depth int)
	writeContent = func(uuid, path, title string, content string, depth int) {
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
			Path:      path,
			Title:     title,
			StartLine: startLine,
			EndLine:   endLine,
		})

		b.WriteString(content)
		nextLine = endLine + 1
		prevDepth = depth
		prevListOnly = isListOnlyContent(content)
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
		} else {
			writeContent(node.UUID, node.Path, node.Title, node.Content, node.Depth)
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

	var walk func(node *composeTree)
	walk = func(node *composeTree) {
		n, err := note.Load(node.Path)
		if err != nil {
			return
		}
		results = append(results, SearchResult{
			Path:   node.Path,
			UUID:   node.UUID,
			Title:  node.Title,
			Tags:   n.Tags,
			Parent: n.Parent,
			note:   n,
		})
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
