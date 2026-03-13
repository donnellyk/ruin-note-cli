package commands

import (
	"fmt"
	"strings"

	"kvnd/ruin-note-cli/internal/note"
)

// QueryMatcher is a function that tests if a note matches the query.
type QueryMatcher func(n *note.Note) bool

// MatcherInfo describes matcher properties for optimization.
type MatcherInfo struct {
	NeedsBody bool // true if the matcher needs full note content (e.g., text search)
}

// TagScope controls which tag fields are checked during tag search.
type TagScope int

const (
	TagScopeAll    TagScope = iota // Check both global and inline tags (default)
	TagScopeGlobal                 // Check only global tags (--global-tags)
	TagScopeInline                 // Check only inline tags (--inline-tags)
)

// parseQuery parses a search query string into a matcher function.
// MVP supports: tag search, text search, && (AND), space (implicit AND)
// Date tokens (@today, @tomorrow, etc.) are resolved before parsing.
func parseQuery(query string, tagScope TagScope) (QueryMatcher, MatcherInfo, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, MatcherInfo{}, fmt.Errorf("empty query")
	}

	// Resolve date tokens in query (@tomorrow → @2026-02-13)
	query = note.ResolveDateTokensInQuery(query)

	// Split by && first
	parts := strings.Split(query, "&&")
	var matchers []QueryMatcher
	info := MatcherInfo{}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by space for implicit AND
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
		}
	}

	if len(matchers) == 0 {
		return nil, MatcherInfo{}, fmt.Errorf("no valid search terms")
	}

	// Combine all matchers with AND
	return func(n *note.Note) bool {
		for _, m := range matchers {
			if !m(n) {
				return false
			}
		}
		return true
	}, info, nil
}

// splitTerms splits a query part into individual terms.
// Preserves spaced tags like #daily note#
func splitTerms(part string) []string {
	var terms []string
	var current strings.Builder
	inSpacedTag := false

	for i := 0; i < len(part); i++ {
		ch := part[i]

		if ch == '#' {
			if inSpacedTag {
				// End of spaced tag
				current.WriteByte(ch)
				terms = append(terms, current.String())
				current.Reset()
				inSpacedTag = false
				continue
			}

			// Potential start of tag
			if current.Len() > 0 {
				terms = append(terms, current.String())
				current.Reset()
			}
			current.WriteByte(ch)

			// Check if this is a spaced tag
			// A spaced tag is #text with spaces# where the closing # is NOT followed by a word char
			// and NOT preceded by another #
			rest := part[i+1:]
			if idx := strings.Index(rest, "#"); idx > 0 {
				// Check if there's a space in the potential tag content
				// AND the content doesn't contain another # before the closing one
				potentialContent := rest[:idx]
				hasSpace := strings.ContainsAny(potentialContent, " \t")
				// Check what's after the closing #
				afterClosing := ""
				if idx+1 < len(rest) {
					afterClosing = string(rest[idx+1])
				}
				// It's a spaced tag only if:
				// 1. Has space in content
				// 2. Closing # is NOT followed by a word char (which would make it another tag)
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

// parseTermMatcher creates a matcher for a single search term.
func parseTermMatcher(term string, tagScope TagScope) (QueryMatcher, MatcherInfo, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, MatcherInfo{}, fmt.Errorf("empty term")
	}

	fmOnly := MatcherInfo{NeedsBody: false}
	needsBody := MatcherInfo{NeedsBody: true}

	// Date search: @YYYY-MM-DD matches against frontmatter dates field
	if isDateTerm(term) {
		dateStr := term[1:] // strip @
		return dateMatcher(dateStr), fmOnly, nil
	}

	// Tag search
	if strings.HasPrefix(term, "#") {
		return tagMatcher(term, tagScope), fmOnly, nil
	}

	// Check for filter prefixes (field:value)
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
			m, err := createdDateMatcher(value) // alias for created:
			return m, fmOnly, err
		case "between":
			m, err := betweenDateMatcher(value)
			return m, fmOnly, err
		case "title":
			return titleMatcher(value), fmOnly, nil
		case "path":
			return pathMatcher(value), fmOnly, nil
		case "parent":
			return parentMatcher(value), fmOnly, nil
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
		// If not a recognized filter, fall through to text search
	}

	// Text search (case-insensitive)
	return textMatcher(term), needsBody, nil
}

// parseSort parses a sort specification string.
// Format: field:direction[,field:direction]
func parseSort(s string) ([]SortField, error) {
	var fields []SortField

	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		field := SortField{Ascending: true} // default

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

		// Validate field
		switch field.Field {
		case "created", "updated", "title", "order":
			// valid
		default:
			return nil, fmt.Errorf("invalid sort field: %s (use created, updated, title, or order)", field.Field)
		}

		fields = append(fields, field)
	}

	return fields, nil
}
