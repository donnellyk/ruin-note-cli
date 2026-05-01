package commands

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/donnellyk/ruin-note-cli/internal/dateparse"
	"github.com/donnellyk/ruin-note-cli/internal/note"
)

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

	results = excludeUUID(results, w.rootUUID)
	if parentUUID != w.rootUUID {
		results = excludeUUID(results, parentUUID)
	}

	if sortStr, ok := ref.Options["sort"]; ok {
		fields, err := parseSort(sortStr)
		if err == nil {
			sortResults(results, fields)
		}
	} else {
		sortResults(results, []SortField{{Field: "created", Ascending: false}})
	}

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
		text, attrs := renderSearchList(results)
		return w.textResult(text, dynInfo, depth, attrs)
	case "summary":
		text, attrs := renderSearchSummary(results, depth)
		return w.textResult(text, dynInfo, depth, attrs)
	default: // "content"
		return w.contentResult(results, dynInfo, depth)
	}
}

func (w *composeWalker) expandDynamicPick(ref note.DynamicEmbedRef, depth int, parentUUID string) *dynamicResult {
	resolved := normalizePickQueryCommas(note.ResolveDateTokensInQuery(ref.Query))
	tagArgs := strings.Fields(resolved)
	var filter pickTagFilter
	var dateRanges []dateparse.DateRange
	for _, arg := range tagArgs {
		switch {
		case strings.HasPrefix(arg, "!#"):
			filter.exclude = append(filter.exclude, note.NormalizeStored(arg[1:]))
		case strings.HasPrefix(arg, "#"):
			filter.include = append(filter.include, note.NormalizeStored(arg))
		case strings.HasPrefix(arg, "@between:"):
			dr, err := parsePickBetween(arg, time.Now())
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: dynamic pick: %v\n", err)
				continue
			}
			dateRanges = append(dateRanges, dr)
		case strings.HasPrefix(arg, "@"):
			token := arg[1:]
			dr, err := dateparse.ParseWithReference(token, time.Now())
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: dynamic pick: unrecognized date %s\n", arg)
				continue
			}
			dateRanges = append(dateRanges, dr)
		default:
			filter.include = append(filter.include, note.NormalizeStored(arg))
		}
	}

	if len(filter.include) == 0 && len(dateRanges) == 0 {
		fmt.Fprintf(os.Stderr, "warning: dynamic pick %q: at least one positive tag or @date required\n", ref.Query)
		return nil
	}

	anyMode := ref.Options["any"] == "true"

	df := doneExclude
	if ref.Options["all"] == "true" {
		df = doneInclude
	} else if ref.Options["done"] == "true" {
		df = doneOnly
	}

	var filterMatcher QueryMatcher
	if filterStr, ok := ref.Options["filter"]; ok && filterStr != "" {
		m, _, err := parseQuery(filterStr, TagScopeAll)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: dynamic pick filter %q: %v\n", filterStr, err)
			return nil
		}
		filterMatcher = m
	}

	excludeUUIDs := map[string]bool{w.rootUUID: true, parentUUID: true}
	candidates, err := pickCandidatePaths(w.vault, filter, nil, excludeUUIDs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: dynamic pick: %v\n", err)
		return nil
	}

	var pickResults []pickNoteResult
	for _, path := range candidates {
		if filterMatcher != nil {
			fast, err := note.LoadFrontmatterOnly(path)
			if err != nil {
				continue
			}
			hydrateNoteTagsFromIndex(fast, w.index, path, false)
			if !filterMatcher(fast) {
				continue
			}
		}
		n, err := note.Load(path)
		if err != nil {
			continue
		}

		matches := pickLinesFromNote(n, filter, dateRanges, anyMode, df, false)
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

	if sortStr, ok := ref.Options["sort"]; ok {
		fields, _ := parseSort(sortStr)
		sortPickNoteResults(pickResults, fields)
	} else {
		sortPickNoteResults(pickResults, []SortField{{Field: "created", Ascending: false}})
	}

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
	var attrs []attributionEntry
	switch format {
	case "flat":
		text, attrs = renderPickFlat(pickResults, depth)
	default: // "grouped"
		groupBy := ref.Options["group"]
		if groupBy == "" {
			groupBy = "note"
		}
		groups := w.groupPickResults(pickResults, groupBy, filter.include)
		text, attrs = renderPickGroups(groups, depth)
	}

	return w.textResult(text, dynInfo, depth, attrs)
}

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

	searchRef := note.DynamicEmbedRef{
		Type:    "search",
		Query:   queryStr,
		Options: ref.Options,
		Line:    ref.Line,
	}
	result := w.expandDynamicSearch(searchRef, depth, parentUUID)
	if result != nil {
		for i := range result.segments {
			if result.segments[i].Embed != nil && result.segments[i].Embed.Dynamic != nil {
				result.segments[i].Embed.Dynamic.Type = "query"
				result.segments[i].Embed.Dynamic.Query = ref.Query + " → " + queryStr
			}
		}
	}
	return result
}

func (w *composeWalker) expandDynamicCompose(ref note.DynamicEmbedRef, depth int) *dynamicResult {
	resolvedNote, err := ResolveNote(w.vault, ref.Query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: dynamic compose %q: %v\n", ref.Query, err)
		return nil
	}

	if w.visited[resolvedNote.UUID] {
		fmt.Fprintf(os.Stderr, "warning: skipping circular compose embed of %q\n", ref.Query)
		return nil
	}

	subMaxDepth := w.maxDepth
	if depthStr, ok := ref.Options["depth"]; ok {
		if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
			subMaxDepth = d
		}
	}

	subChildrenMap := w.index.ChildrenMap()

	// Sub-walker shares the parent's visited set for circular detection.
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

	adjustTreeDepth(subTree, depth+1)

	// Copy visited UUIDs back so circular detection is global.
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

	return &dynamicResult{
		segments:      []composeSegment{{Embed: subTree}},
		embeddedUUIDs: collectTreeUUIDs(subTree),
	}
}

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

func (w *composeWalker) textResult(text string, dynInfo *dynamicInfo, depth int, attrs []attributionEntry) *dynamicResult {
	// Dynamic text generators produce headings at a base level (H1);
	// adjust them to the correct depth in the tree. Heading adjustment
	// does not change line counts, so attribution line offsets stay valid.
	if w.normalizeHeaders {
		text = normalizeHeadings(text, depth+1)
	} else {
		text = adjustHeadings(text, depth+1)
	}
	tree := &composeTree{
		Depth:       depth + 1,
		Content:     text,
		Embedded:    true,
		Dynamic:     dynInfo,
		Attribution: attrs,
	}
	return &dynamicResult{
		segments: []composeSegment{{Embed: tree}},
	}
}

func (w *composeWalker) contentResult(results []SearchResult, dynInfo *dynamicInfo, depth int) *dynamicResult {
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

func renderSearchList(results []SearchResult) (string, []attributionEntry) {
	var lines []string
	var attrs []attributionEntry
	for i, r := range results {
		lines = append(lines, fmt.Sprintf("- [[%s]]", r.Title))
		attrs = append(attrs, attributionEntry{
			UUID: r.UUID, Title: r.Title, Path: r.Path,
			LineOffset: i, LineCount: 1,
		})
	}
	return strings.Join(lines, "\n"), attrs
}

func renderSearchSummary(results []SearchResult, _ int) (string, []attributionEntry) {
	var parts []string
	var attrs []attributionEntry
	cumulativeOffset := 0
	headingPrefix := "#" // base level; textResult adjusts to correct depth
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
		partLineCount := strings.Count(part, "\n") + 1
		if len(parts) > 0 {
			cumulativeOffset++ // blank separator line
		}
		attrs = append(attrs, attributionEntry{
			UUID: r.UUID, Title: r.Title, Path: r.Path,
			LineOffset: cumulativeOffset, LineCount: partLineCount,
		})
		cumulativeOffset += partLineCount
		parts = append(parts, part)
	}
	return strings.Join(parts, "\n\n"), attrs
}

type pickGroup struct {
	heading string
	// headingRef is the source note for the heading (empty for group=tag).
	headingRef sourceRef
	matches    []pickMatchWithSource
}

// sourceRef identifies a source note contributing to rendered output.
type sourceRef struct {
	UUID, Title, Path string
}

type pickMatchWithSource struct {
	match  PickMatch
	source sourceRef
}

func (w *composeWalker) groupPickResults(results []pickNoteResult, groupBy string, includeTags []string) []pickGroup {
	switch groupBy {
	case "parent":
		return w.groupByParent(results, false)
	case "root":
		return w.groupByParent(results, true)
	case "tag":
		return groupByTag(results, includeTags)
	default: // "note"
		return w.groupByNote(results)
	}
}

func (w *composeWalker) groupByNote(results []pickNoteResult) []pickGroup {
	var groups []pickGroup
	for _, r := range results {
		ref := w.sourceRefForUUID(r.uuid)
		ref.Title = r.title
		var matches []pickMatchWithSource
		for _, m := range r.matches {
			matches = append(matches, pickMatchWithSource{match: m, source: ref})
		}
		groups = append(groups, pickGroup{heading: r.title, headingRef: ref, matches: matches})
	}
	return groups
}

func (w *composeWalker) groupByParent(results []pickNoteResult, walkToRoot bool) []pickGroup {
	ordered := []sourceRef{}
	seen := map[string]int{}
	grouped := map[string][]pickMatchWithSource{}

	for _, r := range results {
		key := w.resolveParentKey(r.uuid, walkToRoot)
		if _, ok := seen[key.UUID]; !ok {
			seen[key.UUID] = len(ordered)
			ordered = append(ordered, key)
		}
		srcRef := w.sourceRefForUUID(r.uuid)
		if srcRef.Title == "" {
			srcRef.Title = r.title
		}
		for _, m := range r.matches {
			grouped[key.UUID] = append(grouped[key.UUID], pickMatchWithSource{match: m, source: srcRef})
		}
	}

	var groups []pickGroup
	for _, key := range ordered {
		groups = append(groups, pickGroup{heading: key.Title, headingRef: key, matches: grouped[key.UUID]})
	}
	return groups
}

func (w *composeWalker) resolveParentKey(uuid string, walkToRoot bool) sourceRef {
	entry, ok := w.index.Titles[uuid]
	if !ok || entry.Parent == "" {
		return sourceRef{UUID: uuid, Title: entry.Title, Path: entry.Path}
	}

	parentUUID := entry.Parent
	if walkToRoot {
		for range 100 {
			pe, ok := w.index.Titles[parentUUID]
			if !ok || pe.Parent == "" {
				break
			}
			parentUUID = pe.Parent
		}
	}

	if pe, ok := w.index.Titles[parentUUID]; ok {
		return sourceRef{UUID: parentUUID, Title: pe.Title, Path: pe.Path}
	}
	return sourceRef{UUID: uuid, Title: entry.Title, Path: entry.Path}
}

func (w *composeWalker) sourceRefForUUID(uuid string) sourceRef {
	if entry, ok := w.index.Titles[uuid]; ok {
		return sourceRef{UUID: uuid, Title: entry.Title, Path: entry.Path}
	}
	return sourceRef{UUID: uuid}
}

func groupByTag(results []pickNoteResult, includeTags []string) []pickGroup {
	ordered := []string{}
	seen := map[string]int{}
	grouped := map[string][]pickMatchWithSource{}

	for _, r := range results {
		srcRef := sourceRef{UUID: r.uuid, Title: r.title}
		for _, m := range r.matches {
			lineTagsNorm := map[string]bool{}
			for _, lt := range m.Tags {
				lineTagsNorm[note.NormalizeStored(lt)] = true
			}
			matched := false
			for _, it := range includeTags {
				if lineTagsNorm[it] {
					if _, ok := seen[it]; !ok {
						seen[it] = len(ordered)
						ordered = append(ordered, it)
					}
					grouped[it] = append(grouped[it], pickMatchWithSource{match: m, source: srcRef})
					matched = true
				}
			}
			if !matched {
				key := "(untagged)"
				if _, ok := seen[key]; !ok {
					seen[key] = len(ordered)
					ordered = append(ordered, key)
				}
				grouped[key] = append(grouped[key], pickMatchWithSource{match: m, source: srcRef})
			}
		}
	}

	var groups []pickGroup
	for _, tag := range ordered {
		// Tag heading has no source note, so headingRef is empty.
		groups = append(groups, pickGroup{heading: tag, matches: grouped[tag]})
	}
	return groups
}

func renderPickGroups(groups []pickGroup, _ int) (string, []attributionEntry) {
	var parts []string
	var attrs []attributionEntry
	cumulativeOffset := 0
	headingPrefix := "#" // base level; textResult adjusts to correct depth
	for gi, g := range groups {
		if gi > 0 {
			cumulativeOffset++ // blank separator line between groups
		}
		localOffset := 0
		var lines []string
		if g.heading != "" {
			lines = append(lines, fmt.Sprintf("%s %s", headingPrefix, g.heading))
			attrs = append(attrs, attributionEntry{
				UUID:       g.headingRef.UUID,
				Title:      g.headingRef.Title,
				Path:       g.headingRef.Path,
				LineOffset: cumulativeOffset + localOffset,
				LineCount:  1,
			})
			localOffset++
		}
		for _, mw := range g.matches {
			content := mw.match.Content
			if !strings.HasPrefix(content, "- ") && !strings.HasPrefix(content, "* ") {
				content = "- " + content
			}
			lines = append(lines, content)
			attrs = append(attrs, attributionEntry{
				UUID:       mw.source.UUID,
				Title:      mw.source.Title,
				Path:       mw.source.Path,
				LineOffset: cumulativeOffset + localOffset,
				LineCount:  1,
			})
			localOffset++
		}
		parts = append(parts, strings.Join(lines, "\n"))
		cumulativeOffset += localOffset
	}
	return strings.Join(parts, "\n\n"), attrs
}

func renderPickFlat(results []pickNoteResult, _ int) (string, []attributionEntry) {
	var lines []string
	var attrs []attributionEntry
	lineOffset := 0
	for _, r := range results {
		src := sourceRef{UUID: r.uuid, Title: r.title}
		for _, m := range r.matches {
			content := m.Content
			if !strings.HasPrefix(content, "- ") && !strings.HasPrefix(content, "* ") {
				content = "- " + content
			}
			if r.title != "" {
				lines = append(lines, fmt.Sprintf("%s (%s)", content, r.title))
			} else {
				lines = append(lines, content)
			}
			attrs = append(attrs, attributionEntry{
				UUID:       src.UUID,
				Title:      src.Title,
				Path:       src.Path,
				LineOffset: lineOffset,
				LineCount:  1,
			})
			lineOffset++
		}
	}
	return strings.Join(lines, "\n"), attrs
}

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
