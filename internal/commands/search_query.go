package commands

import (
	"fmt"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
)

// QueryMatcher tests if a note matches the query.
type QueryMatcher func(n *note.Note) bool

type MatcherInfo struct {
	NeedsBody bool
	// MatchableFromTitles is true when the matcher reads only fields that
	// the synthetic *note.Note built by prefilterPathsViaTitles populates:
	// UUID, Title, FilePath, Parent, Tags, InlineTags, InheritedTags.
	// Setting this true on a matcher that reads any other field
	// (Created, Updated, Order, Dates, URL, Content, Extra, LinkedCards)
	// will silently mis-classify notes during the fast path.
	MatchableFromTitles bool
}

type TagScope int

const (
	TagScopeAll    TagScope = iota // Check both global and inline tags (default)
	TagScopeGlobal                 // Check only global tags (--global-tags)
	TagScopeInline                 // Check only inline tags (--inline-tags)
)

// parseQuery parses a search query: tag search, text search, && (AND), or space (implicit AND).
// Date tokens (@today, @tomorrow, etc.) are resolved before parsing.
func parseQuery(query string, tagScope TagScope) (QueryMatcher, MatcherInfo, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, MatcherInfo{}, fmt.Errorf("empty query")
	}

	query = note.ResolveDateTokensInQuery(query)

	parts := strings.Split(query, "&&")
	var matchers []QueryMatcher
	// MatchableFromTitles starts optimistic and AND-collapses across terms:
	// any single term that reads non-titles fields disqualifies the whole query.
	info := MatcherInfo{MatchableFromTitles: true}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		terms := splitTerms(part)
		for _, term := range terms {
			m, termInfo, err := parseTermMatcher(term, tagScope)
			if err != nil {
				return nil, MatcherInfo{}, err
			}
			matchers = append(matchers, m)
			if termInfo.NeedsBody {
				info.NeedsBody = true
			}
			if !termInfo.MatchableFromTitles {
				info.MatchableFromTitles = false
			}
		}
	}

	if len(matchers) == 0 {
		return nil, MatcherInfo{}, fmt.Errorf("no valid search terms")
	}

	return func(n *note.Note) bool {
		for _, m := range matchers {
			if !m(n) {
				return false
			}
		}
		return true
	}, info, nil
}

// splitTerms splits a query part into terms while preserving spaced tags like "#daily note#".
func splitTerms(part string) []string {
	var terms []string
	var current strings.Builder
	inSpacedTag := false

	for i := 0; i < len(part); i++ {
		ch := part[i]

		if ch == '#' {
			if inSpacedTag {
				current.WriteByte(ch)
				terms = append(terms, current.String())
				current.Reset()
				inSpacedTag = false
				continue
			}

			if current.Len() > 0 && current.String() != "!" {
				terms = append(terms, current.String())
				current.Reset()
			}
			current.WriteByte(ch)

			// A spaced tag is #text with spaces# where the closing # isn't
			// adjacent to a word char (which would make it a separate tag).
			rest := part[i+1:]
			if idx := strings.Index(rest, "#"); idx > 0 {
				potentialContent := rest[:idx]
				hasSpace := strings.ContainsAny(potentialContent, " \t")
				afterClosing := ""
				if idx+1 < len(rest) {
					afterClosing = string(rest[idx+1])
				}
				if hasSpace && (afterClosing == "" || afterClosing == " " || afterClosing == "\t") {
					inSpacedTag = true
				}
			}
			continue
		}

		if ch == ' ' || ch == '\t' {
			if inSpacedTag {
				current.WriteByte(ch)
				continue
			}
			if current.Len() > 0 {
				terms = append(terms, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(ch)
	}

	if current.Len() > 0 {
		terms = append(terms, current.String())
	}

	return terms
}

func negateMatcher(inner QueryMatcher) QueryMatcher {
	return func(n *note.Note) bool {
		return !inner(n)
	}
}

func parseTermMatcher(term string, tagScope TagScope) (QueryMatcher, MatcherInfo, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, MatcherInfo{}, fmt.Errorf("empty term")
	}

	if strings.HasPrefix(term, "!") {
		inner := term[1:]
		if inner == "" {
			return nil, MatcherInfo{}, fmt.Errorf("empty negation term")
		}
		matcher, info, err := parseTermMatcher(inner, tagScope)
		if err != nil {
			return nil, MatcherInfo{}, err
		}
		return negateMatcher(matcher), info, nil
	}

	// fmOnly: matcher reads frontmatter-only fields that aren't all in titles
	// (e.g., Dates, URL, Created). Search engine still has to open files.
	fmOnly := MatcherInfo{NeedsBody: false}
	// titlesOnly: matcher reads only fields that titles.json carries.
	// Search engine can pre-filter from the index without opening files.
	titlesOnly := MatcherInfo{NeedsBody: false, MatchableFromTitles: true}
	needsBody := MatcherInfo{NeedsBody: true}

	// @YYYY-MM-DD matches against the frontmatter dates field.
	if isDateTerm(term) {
		dateStr := term[1:]
		return dateMatcher(dateStr), fmOnly, nil
	}

	if strings.HasPrefix(term, "#") {
		return tagMatcher(term, tagScope), titlesOnly, nil
	}

	if idx := strings.Index(term, ":"); idx > 0 {
		field := strings.ToLower(term[:idx])
		value := term[idx+1:]

		switch field {
		case "created":
			m, err := createdDateMatcher(value)
			return m, fmOnly, err
		case "updated":
			m, err := updatedDateMatcher(value)
			return m, fmOnly, err
		case "before":
			m, err := beforeDateMatcher(value)
			return m, fmOnly, err
		case "after":
			m, err := afterDateMatcher(value)
			return m, fmOnly, err
		case "on":
			m, err := createdDateMatcher(value)
			return m, fmOnly, err
		case "between":
			m, err := betweenDateMatcher(value)
			return m, fmOnly, err
		case "title":
			return titleMatcher(value), titlesOnly, nil
		case "path":
			return pathMatcher(value), titlesOnly, nil
		case "parent":
			return parentMatcher(value), titlesOnly, nil
		case "tags":
			m, err := tagsMatcher(value, tagScope)
			return m, titlesOnly, err
		case "link":
			return linkMatcher(value), fmOnly, nil
		case "todo":
			switch strings.ToLower(value) {
			case "open":
				return func(n *note.Note) bool { return note.HasUncheckedTodos(n.Content) }, needsBody, nil
			case "done":
				return func(n *note.Note) bool { return note.HasCheckedTodos(n.Content) }, needsBody, nil
			case "any":
				return func(n *note.Note) bool { return note.HasAnyTodos(n.Content) }, needsBody, nil
			default:
				return nil, MatcherInfo{}, fmt.Errorf("unknown todo filter %q (use open, done, or any)", value)
			}
		}
		// Unrecognized filter falls through to text search.
	}

	return textMatcher(term), needsBody, nil
}

// parseSort parses "field:direction[,field:direction]".
func parseSort(s string) ([]SortField, error) {
	var fields []SortField

	parts := strings.SplitSeq(s, ",")
	for part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		field := SortField{Ascending: true}

		if idx := strings.Index(part, ":"); idx > 0 {
			field.Field = strings.ToLower(part[:idx])
			dir := strings.ToLower(part[idx+1:])
			switch dir {
			case "asc":
				field.Ascending = true
			case "desc":
				field.Ascending = false
			default:
				return nil, fmt.Errorf("invalid sort direction: %s (use asc or desc)", dir)
			}
		} else {
			field.Field = strings.ToLower(part)
		}

		switch field.Field {
		case "created", "updated", "title", "order":
		default:
			return nil, fmt.Errorf("invalid sort field: %s (use created, updated, title, or order)", field.Field)
		}

		fields = append(fields, field)
	}

	return fields, nil
}
