package commands

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/donnellyk/ruin-note-cli/internal/note"
)

// expandDynamicEmbed dispatches to the appropriate handler for a dynamic embed.
func (w *composeWalker) expandDynamicEmbed(ref note.DynamicEmbedRef, depth int, parentUUID string) *dynamicResult {
	switch ref.Type {
	case "search":
		return w.expandDynamicSearch(ref, depth, parentUUID)
	case "pick":
		return w.expandDynamicPick(ref, depth, parentUUID)
	case "query":
		return w.expandDynamicQuery(ref, depth, parentUUID)
	case "compose":
		return w.expandDynamicCompose(ref, depth)
	default:
		return nil
	}
}

// expandDynamicSearch handles ![[search: query | options]].
func (w *composeWalker) expandDynamicSearch(ref note.DynamicEmbedRef, depth int, parentUUID string) *dynamicResult {
	tagScope := parseDynamicTagScope(ref.Options)
	matcher, info, err := parseQuery(ref.Query, tagScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: dynamic search %q: %v\n", ref.Query, err)
		return nil
	}

	results, err := searchNotes(w.vault, matcher, info)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: dynamic search %q: %v\n", ref.Query, err)
		return nil
	}

	// Exclude the compose root
	results = excludeUUID(results, w.rootUUID)
	// Exclude the parent note containing this embed
	if parentUUID != w.rootUUID {
		results = excludeUUID(results, parentUUID)
	}

	// Apply sort
	if sortStr, ok := ref.Options["sort"]; ok {
		fields, err := parseSort(sortStr)
		if err == nil {
			sortResults(results, fields)
		}
	} else {
		// Default: created:desc
		sortResults(results, []SortField{{Field: "created", Ascending: false}})
	}

	// Apply limit
	if limitStr, ok := ref.Options["limit"]; ok {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(results) {
			results = results[:limit]
		}
	}

	format := ref.Options["format"]
	if format == "" {
		format = "content"
	}

	dynInfo := &dynamicInfo{
		Type:        "search",
		Query:       ref.Query,
		Options:     ref.Options,
		ResultCount: len(results),
	}

	if len(results) == 0 {
		return w.emptyResult(ref, dynInfo)
	}

	switch format {
	case "list":
		text := renderSearchList(results)
		return w.textResult(text, dynInfo, depth)
	case "summary":
		text := renderSearchSummary(results, depth)
		return w.textResult(text, dynInfo, depth)
	default: // "content"
		return w.contentResult(results, dynInfo, depth)
	}
}

// expandDynamicPick handles ![[pick: tags | options]].
func (w *composeWalker) expandDynamicPick(ref note.DynamicEmbedRef, depth int, parentUUID string) *dynamicResult {
	// Parse tags from query (space-separated, may include negation)
	tagArgs := strings.Fields(ref.Query)
	var filter pickTagFilter
	for _, arg := range tagArgs {
		if strings.HasPrefix(arg, "!") {
			filter.exclude = append(filter.exclude, note.NormalizeTag(arg[1:]))
		} else {
			filter.include = append(filter.include, note.NormalizeTag(arg))
		}
	}

	if len(filter.include) == 0 {
		fmt.Fprintf(os.Stderr, "warning: dynamic pick %q: at least one positive tag required\n", ref.Query)
		return nil
	}

	anyMode := ref.Options["any"] == "true"

	df := doneExclude
	if ref.Options["all"] == "true" {
		df = doneInclude
	} else if ref.Options["done"] == "true" {
		df = doneOnly
	}

	// Optional note-level filter
	var filterMatcher QueryMatcher
	if filterStr, ok := ref.Options["filter"]; ok && filterStr != "" {
		m, _, err := parseQuery(filterStr, TagScopeAll)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: dynamic pick filter %q: %v\n", filterStr, err)
			return nil
		}
		filterMatcher = m
	}

	notePaths, err := w.vault.ListNotes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: dynamic pick: %v\n", err)
		return nil
	}

	var pickResults []pickNoteResult

	for _, path := range notePaths {
		fast, err := note.LoadFrontmatterOnly(path)
		if err != nil {
			continue
		}

		// Exclude compose root and parent
		if fast.UUID == w.rootUUID || fast.UUID == parentUUID {
			continue
		}

		// Pre-filter: must have at least one include tag as inline
		if !noteHasInlineTag(fast, filter.include) {
			continue
		}

		if filterMatcher != nil && !filterMatcher(fast) {
			continue
		}

		n, err := note.Load(path)
		if err != nil {
			continue
		}

		matches := pickLinesFromNote(n, filter, nil, anyMode, df, false)
		if len(matches) == 0 {
			continue
		}

		pickResults = append(pickResults, pickNoteResult{
			uuid:    n.UUID,
			title:   n.Title,
			matches: matches,
			created: n.Created,
			updated: n.Updated,
			order:   n.Order,
		})
	}

	// Apply sort
	if sortStr, ok := ref.Options["sort"]; ok {
		fields, _ := parseSort(sortStr)
		sortPickNoteResults(pickResults, fields)
	} else {
		sortPickNoteResults(pickResults, []SortField{{Field: "created", Ascending: false}})
	}

	// Apply limit
	if limitStr, ok := ref.Options["limit"]; ok {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(pickResults) {
			pickResults = pickResults[:limit]
		}
	}

	totalLines := 0
	for _, pr := range pickResults {
		totalLines += len(pr.matches)
	}

	dynInfo := &dynamicInfo{
		Type:        "pick",
		Query:       ref.Query,
		Options:     ref.Options,
		ResultCount: totalLines,
	}

	if len(pickResults) == 0 {
		return w.emptyResult(ref, dynInfo)
	}

	format := ref.Options["format"]
	if format == "" {
		format = "grouped"
	}

	var text string
	switch format {
	case "flat":
		text = renderPickFlat(pickResults, depth)
	default: // "grouped"
		text = renderPickGrouped(pickResults, depth)
	}

	return w.textResult(text, dynInfo, depth)
}

// expandDynamicQuery handles ![[query: name | options]].
func (w *composeWalker) expandDynamicQuery(ref note.DynamicEmbedRef, depth int, parentUUID string) *dynamicResult {
	queries, err := w.vault.LoadQueries()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: dynamic query: failed to load queries: %v\n", err)
		return nil
	}

	var queryStr string
	for _, q := range queries.Queries {
		if q.Name == ref.Query {
			queryStr = q.Query
			break
		}
	}
	if queryStr == "" {
		dynInfo := &dynamicInfo{
			Type:        "query",
			Query:       ref.Query,
			Options:     ref.Options,
			ResultCount: 0,
		}
		tree := &composeTree{
			Depth:    depth + 1,
			Content:  fmt.Sprintf("*Query %q not found*", ref.Query),
			Embedded: true,
			Dynamic:  dynInfo,
		}
		return &dynamicResult{
			segments: []composeSegment{{Embed: tree}},
		}
	}

	// Delegate to search handler with the resolved query
	searchRef := note.DynamicEmbedRef{
		Type:    "search",
		Query:   queryStr,
		Options: ref.Options,
		Line:    ref.Line,
	}
	result := w.expandDynamicSearch(searchRef, depth, parentUUID)
	if result != nil {
		// Update the dynamic info to reflect this was a query: embed
		for i := range result.segments {
			if result.segments[i].Embed != nil && result.segments[i].Embed.Dynamic != nil {
				result.segments[i].Embed.Dynamic.Type = "query"
				result.segments[i].Embed.Dynamic.Query = ref.Query + " → " + queryStr
			}
		}
	}
	return result
}

// expandDynamicCompose handles ![[compose: ref | options]].
func (w *composeWalker) expandDynamicCompose(ref note.DynamicEmbedRef, depth int) *dynamicResult {
	// Resolve the note or bookmark
	resolvedNote, err := ResolveNote(w.vault, ref.Query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: dynamic compose %q: %v\n", ref.Query, err)
		return nil
	}

	if w.visited[resolvedNote.UUID] {
		fmt.Fprintf(os.Stderr, "warning: skipping circular compose embed of %q\n", ref.Query)
		return nil
	}

	// Determine depth budget for sub-compose (own fresh budget)
	subMaxDepth := w.maxDepth
	if depthStr, ok := ref.Options["depth"]; ok {
		if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
			subMaxDepth = d
		}
	}

	// Check for compose file bookmark
	var subChildrenMap map[string][]string
	var subYMLParents map[string]bool
	if bookmark, ok := w.vault.LookupParent(ref.Query); ok && bookmark.File != "" {
		composeFile := LoadComposeFilePath(bookmark.File, w.vault.Path)
		spec, err := ParseComposeFile(composeFile)
		if err == nil {
			result, err := BuildChildrenMapFromSpec(spec, w.vault, w.index)
			if err == nil {
				subChildrenMap = result.ChildrenMap
				subYMLParents = result.YMLParents
			}
		}
	}

	if subChildrenMap == nil {
		subChildrenMap = w.index.ChildrenMap()
	}

	// Create a sub-walker with its own depth budget and visited set,
	// but sharing the parent's visited set for circular detection
	subVisited := make(map[string]bool)
	for k, v := range w.visited {
		subVisited[k] = v
	}
	subWalker := &composeWalker{
		vault:            w.vault,
		index:            w.index,
		childrenMap:      subChildrenMap,
		visited:          subVisited,
		maxDepth:         subMaxDepth,
		stripTitle:       false,
		stripGlobalTags:  w.stripGlobalTags,
		normalizeHeaders: w.normalizeHeaders,
		expandEmbeds:     w.expandEmbeds,
		expandDynamic:    true, // compose: embeds get full dynamic expansion
		rootUUID:         resolvedNote.UUID,
	}

	subTree := subWalker.Walk(resolvedNote.UUID, 0)
	if subTree == nil {
		return nil
	}

	// Adjust heading levels relative to current depth
	adjustTreeDepth(subTree, depth+1)

	// Copy visited UUIDs back so circular detection is global
	for k, v := range subWalker.visited {
		w.visited[k] = v
	}

	dynInfo := &dynamicInfo{
		Type:        "compose",
		Query:       ref.Query,
		Options:     ref.Options,
		ResultCount: countTreeNotes(subTree),
	}
	subTree.Dynamic = dynInfo
	subTree.Embedded = true

	_ = subYMLParents // used during sub-compose construction

	return &dynamicResult{
		segments:      []composeSegment{{Embed: subTree}},
		embeddedUUIDs: collectTreeUUIDs(subTree),
	}
}

// Helper methods for building results

func (w *composeWalker) emptyResult(ref note.DynamicEmbedRef, dynInfo *dynamicInfo) *dynamicResult {
	if ref.Options["empty"] == "hide" {
		return &dynamicResult{}
	}
	tree := &composeTree{
		Depth:    0,
		Content:  fmt.Sprintf("*No results for: %s*", ref.Query),
		Embedded: true,
		Dynamic:  dynInfo,
	}
	return &dynamicResult{
		segments: []composeSegment{{Embed: tree}},
	}
}

func (w *composeWalker) textResult(text string, dynInfo *dynamicInfo, depth int) *dynamicResult {
	tree := &composeTree{
		Depth:    depth + 1,
		Content:  text,
		Embedded: true,
		Dynamic:  dynInfo,
	}
	return &dynamicResult{
		segments: []composeSegment{{Embed: tree}},
	}
}

func (w *composeWalker) contentResult(results []SearchResult, dynInfo *dynamicInfo, depth int) *dynamicResult {
	// Build a virtual container node whose children are the search results
	container := &composeTree{
		Depth:    depth + 1,
		Content:  "",
		Embedded: true,
		Dynamic:  dynInfo,
	}

	var embeddedUUIDs []string
	for _, sr := range results {
		n, err := note.Load(sr.Path)
		if err != nil {
			continue
		}

		content := n.Content
		if w.stripGlobalTags {
			content = note.StripGlobalTags(content, n.InlineTags)
		}
		if w.normalizeHeaders {
			content = normalizeHeadings(content, depth+1)
		} else {
			content = adjustHeadings(content, depth+1)
		}

		child := &composeTree{
			UUID:     sr.UUID,
			Title:    sr.Title,
			Path:     sr.Path,
			Depth:    depth + 1,
			Content:  content,
			Embedded: true,
		}

		// Expand static embeds in search results (but not dynamic)
		if w.expandEmbeds {
			staticWalker := &composeWalker{
				vault:            w.vault,
				index:            w.index,
				childrenMap:      w.childrenMap,
				visited:          make(map[string]bool),
				maxDepth:         w.maxDepth,
				stripGlobalTags:  w.stripGlobalTags,
				normalizeHeaders: w.normalizeHeaders,
				expandEmbeds:     true,
				expandDynamic:    false, // no dynamic expansion in search results
				rootUUID:         w.rootUUID,
			}
			staticWalker.visited[sr.UUID] = true
			staticWalker.expandEmbedsInTree(child, content, depth+1)
		}

		// Expand children if depth option allows
		childDepth := 0
		if depthStr, ok := dynInfo.Options["depth"]; ok {
			childDepth, _ = strconv.Atoi(depthStr)
		}
		if childDepth > 0 {
			for _, childUUID := range w.childrenMap[sr.UUID] {
				grandchild := w.Walk(childUUID, depth+2)
				if grandchild != nil {
					child.Children = append(child.Children, grandchild)
				}
			}
		}

		container.Children = append(container.Children, child)
		embeddedUUIDs = append(embeddedUUIDs, sr.UUID)
	}

	return &dynamicResult{
		segments:      []composeSegment{{Embed: container}},
		embeddedUUIDs: embeddedUUIDs,
	}
}

// Rendering helpers

func renderSearchList(results []SearchResult) string {
	var lines []string
	for _, r := range results {
		lines = append(lines, fmt.Sprintf("- [[%s]]", r.Title))
	}
	return strings.Join(lines, "\n")
}

func renderSearchSummary(results []SearchResult, depth int) string {
	var parts []string
	headingPrefix := strings.Repeat("#", min(depth+2, 6))
	for _, r := range results {
		n, err := note.Load(r.Path)
		if err != nil {
			continue
		}
		dateStr := n.Created.Format("2006-01-02")
		firstLine := firstContentLine(n.Content, n.Title)

		part := fmt.Sprintf("%s %s\n*%s*", headingPrefix, r.Title, dateStr)
		if firstLine != "" {
			part += "\n" + firstLine
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "\n\n")
}

func renderPickGrouped(results []pickNoteResult, depth int) string {
	var parts []string
	headingPrefix := strings.Repeat("#", min(depth+2, 6))
	for _, r := range results {
		var lines []string
		lines = append(lines, fmt.Sprintf("%s %s", headingPrefix, r.title))
		for _, m := range r.matches {
			content := m.Content
			if !strings.HasPrefix(content, "- ") && !strings.HasPrefix(content, "* ") {
				content = "- " + content
			}
			lines = append(lines, content)
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}
	return strings.Join(parts, "\n\n")
}

func renderPickFlat(results []pickNoteResult, _ int) string {
	var lines []string
	for _, r := range results {
		for _, m := range r.matches {
			content := m.Content
			if !strings.HasPrefix(content, "- ") && !strings.HasPrefix(content, "* ") {
				content = "- " + content
			}
			lines = append(lines, fmt.Sprintf("%s (%s)", content, r.title))
		}
	}
	return strings.Join(lines, "\n")
}

// firstContentLine returns the first non-empty, non-title, non-tag line of note content.
func firstContentLine(content, title string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if note.IsHeaderLine(trimmed) {
			continue
		}
		if note.IsTagOnlyLine(trimmed) {
			continue
		}
		return trimmed
	}
	return ""
}

func excludeUUID(results []SearchResult, uuid string) []SearchResult {
	return slices.DeleteFunc(results, func(r SearchResult) bool {
		return r.UUID == uuid
	})
}

func parseDynamicTagScope(opts map[string]string) TagScope {
	switch opts["tag-scope"] {
	case "global":
		return TagScopeGlobal
	case "inline":
		return TagScopeInline
	default:
		return TagScopeAll
	}
}

type pickNoteResult struct {
	uuid    string
	title   string
	matches []PickMatch
	created time.Time
	updated time.Time
	order   *int
}

func sortPickNoteResults(results []pickNoteResult, fields []SortField) {
	sort.Slice(results, func(i, j int) bool {
		for _, f := range fields {
			var cmp int
			switch f.Field {
			case "created":
				if results[i].created.Before(results[j].created) {
					cmp = -1
				} else if results[i].created.After(results[j].created) {
					cmp = 1
				}
			case "updated":
				if results[i].updated.Before(results[j].updated) {
					cmp = -1
				} else if results[i].updated.After(results[j].updated) {
					cmp = 1
				}
			case "title":
				cmp = strings.Compare(strings.ToLower(results[i].title), strings.ToLower(results[j].title))
			}
			if cmp != 0 {
				if f.Ascending {
					return cmp < 0
				}
				return cmp > 0
			}
		}
		return false
	})
}

// adjustTreeDepth recursively adjusts all depth values and heading levels.
func adjustTreeDepth(tree *composeTree, baseDepth int) {
	if baseDepth > 0 && tree.Content != "" {
		tree.Content = adjustHeadings(tree.Content, baseDepth)
	}
	tree.Depth = baseDepth

	for _, seg := range tree.Segments {
		if seg.Embed != nil {
			adjustTreeDepth(seg.Embed, baseDepth+1)
		}
	}
	for _, child := range tree.Children {
		adjustTreeDepth(child, baseDepth+1)
	}
}

func countTreeNotes(tree *composeTree) int {
	count := 0
	if tree.UUID != "" {
		count = 1
	}
	for _, seg := range tree.Segments {
		if seg.Embed != nil {
			count += countTreeNotes(seg.Embed)
		}
	}
	for _, child := range tree.Children {
		count += countTreeNotes(child)
	}
	return count
}

func collectTreeUUIDs(tree *composeTree) []string {
	var uuids []string
	if tree.UUID != "" {
		uuids = append(uuids, tree.UUID)
	}
	for _, seg := range tree.Segments {
		if seg.Embed != nil {
			uuids = append(uuids, collectTreeUUIDs(seg.Embed)...)
		}
	}
	for _, child := range tree.Children {
		uuids = append(uuids, collectTreeUUIDs(child)...)
	}
	return uuids
}
