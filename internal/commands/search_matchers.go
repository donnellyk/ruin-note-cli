package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/dateparse"
	"github.com/donnellyk/ruin-note-cli/internal/note"
)

// isDateTerm returns true if the term is a resolved date term (@YYYY-MM-DD).
func isDateTerm(term string) bool {
	if len(term) != 11 || term[0] != '@' {
		return false
	}
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

func dateMatcher(dateStr string) QueryMatcher {
	return func(n *note.Note) bool {
		return slices.Contains(n.Dates, dateStr)
	}
}

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

func textMatcher(text string) QueryMatcher {
	textLower := strings.ToLower(text)
	return func(n *note.Note) bool {
		if strings.Contains(strings.ToLower(n.Content), textLower) {
			return true
		}
		if strings.Contains(strings.ToLower(n.Title), textLower) {
			return true
		}
		return false
	}
}

func titleMatcher(text string) QueryMatcher {
	textLower := strings.ToLower(text)
	return func(n *note.Note) bool {
		return strings.Contains(strings.ToLower(n.Title), textLower)
	}
}

// parentMatcher: "none" matches notes with no parent; any other value matches by parent UUID.
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

func linkNoteMatcher() QueryMatcher {
	return func(n *note.Note) bool {
		return n.IsURLNote()
	}
}

func linkMatcher(text string) QueryMatcher {
	textLower := strings.ToLower(text)
	return func(n *note.Note) bool {
		return strings.Contains(strings.ToLower(n.URL), textLower)
	}
}

func pathMatcher(text string) QueryMatcher {
	textLower := strings.ToLower(text)
	return func(n *note.Note) bool {
		return strings.Contains(strings.ToLower(n.FilePath), textLower)
	}
}

func createdDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for created filter: %w", err)
	}
	return func(n *note.Note) bool {
		return r.Contains(n.Created)
	}, nil
}

func updatedDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for updated filter: %w", err)
	}
	return func(n *note.Note) bool {
		return r.Contains(n.Updated)
	}, nil
}

func beforeDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for before filter: %w", err)
	}
	return func(n *note.Note) bool {
		return n.Created.Before(r.Start)
	}, nil
}

func afterDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for after filter: %w", err)
	}
	return func(n *note.Note) bool {
		return !n.Created.Before(r.End)
	}, nil
}

// betweenDateMatcher parses "DATE,DATE" (e.g., "2025-01-01,2025-01-31" or "2025-01,today").
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

	// Range spans from start of first date to end of second date (inclusive).
	return func(n *note.Note) bool {
		return !n.Created.Before(startRange.Start) && n.Created.Before(endRange.End)
	}, nil
}
