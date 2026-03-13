package commands

import (
	"fmt"
	"strings"

	"kvnd/ruin-note-cli/internal/dateparse"
	"kvnd/ruin-note-cli/internal/note"
)

// isDateTerm returns true if the term is a resolved date term (@YYYY-MM-DD).
func isDateTerm(term string) bool {
	if len(term) != 11 || term[0] != '@' {
		return false
	}
	// Check format: @YYYY-MM-DD
	for i, ch := range term[1:] {
		switch {
		case i == 4 || i == 7:
			if ch != '-' {
				return false
			}
		default:
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return true
}

// dateMatcher returns a matcher that checks if a note has the given date in its dates field.
func dateMatcher(dateStr string) QueryMatcher {
	return func(n *note.Note) bool {
		for _, d := range n.Dates {
			if d == dateStr {
				return true
			}
		}
		return false
	}
}

// tagMatcher returns a matcher that checks if a note has the given tag.
// The scope controls which tag fields are checked.
func tagMatcher(tag string, scope TagScope) QueryMatcher {
	tagNorm := note.NormalizeTag(tag)
	return func(n *note.Note) bool {
		if scope != TagScopeInline {
			for _, t := range n.EffectiveGlobalTags() {
				if note.NormalizeTag(t) == tagNorm {
					return true
				}
			}
		}
		if scope != TagScopeGlobal {
			for _, t := range n.InlineTags {
				if note.NormalizeTag(t) == tagNorm {
					return true
				}
			}
		}
		return false
	}
}

// textMatcher returns a matcher that checks if a note contains the given text.
func textMatcher(text string) QueryMatcher {
	textLower := strings.ToLower(text)
	return func(n *note.Note) bool {
		// Search in content
		if strings.Contains(strings.ToLower(n.Content), textLower) {
			return true
		}
		// Search in title
		if strings.Contains(strings.ToLower(n.Title), textLower) {
			return true
		}
		return false
	}
}

// titleMatcher returns a matcher that checks if a note's title contains the given text.
func titleMatcher(text string) QueryMatcher {
	textLower := strings.ToLower(text)
	return func(n *note.Note) bool {
		return strings.Contains(strings.ToLower(n.Title), textLower)
	}
}

// parentMatcher returns a matcher for parent filter.
// "none" matches notes with no parent. Any other value matches by parent UUID.
func parentMatcher(value string) QueryMatcher {
	if strings.ToLower(value) == "none" {
		return func(n *note.Note) bool {
			return n.Parent == ""
		}
	}
	return func(n *note.Note) bool {
		return n.Parent == value
	}
}

// linkMatcher returns a matcher that checks if a note's URL field contains the given text.
func linkMatcher(text string) QueryMatcher {
	textLower := strings.ToLower(text)
	return func(n *note.Note) bool {
		return strings.Contains(strings.ToLower(n.URL), textLower)
	}
}

// pathMatcher returns a matcher that checks if a note's path contains the given text.
func pathMatcher(text string) QueryMatcher {
	textLower := strings.ToLower(text)
	return func(n *note.Note) bool {
		return strings.Contains(strings.ToLower(n.FilePath), textLower)
	}
}

// createdDateMatcher returns a matcher for created date filter.
// Supports exact dates, months, years, and natural language dates.
func createdDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for created filter: %w", err)
	}
	return func(n *note.Note) bool {
		return r.Contains(n.Created)
	}, nil
}

// updatedDateMatcher returns a matcher for updated date filter.
func updatedDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for updated filter: %w", err)
	}
	return func(n *note.Note) bool {
		return r.Contains(n.Updated)
	}, nil
}

// beforeDateMatcher returns a matcher for notes created before a date.
func beforeDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for before filter: %w", err)
	}
	// Before the start of the parsed range
	return func(n *note.Note) bool {
		return n.Created.Before(r.Start)
	}, nil
}

// afterDateMatcher returns a matcher for notes created after a date.
func afterDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for after filter: %w", err)
	}
	// After the end of the parsed range
	return func(n *note.Note) bool {
		return !n.Created.Before(r.End)
	}, nil
}

// betweenDateMatcher returns a matcher for notes created between two dates.
// Format: DATE,DATE (e.g., "2025-01-01,2025-01-31" or "2025-01,today")
func betweenDateMatcher(value string) (QueryMatcher, error) {
	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("between filter requires two dates separated by comma (e.g., between:2025-01-01,2025-01-31)")
	}

	startRange, err := dateparse.Parse(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid start date for between filter: %w", err)
	}

	endRange, err := dateparse.Parse(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid end date for between filter: %w", err)
	}

	// Range is from start of first date to end of second date (inclusive)
	return func(n *note.Note) bool {
		return !n.Created.Before(startRange.Start) && n.Created.Before(endRange.End)
	}, nil
}
