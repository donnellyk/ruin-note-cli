package note

import (
	"regexp"
	"sort"
	"strings"

	"kvnd/ruin-note-cli/internal/dateparse"
)

// dateTokenPattern matches @ followed by a potential date token.
// The token is a sequence of word characters plus hyphens (e.g., today, next-week, 2-days, 2026-02-13).
var dateTokenPattern = regexp.MustCompile(`@([\w][\w-]*)`)

// resolvedDatePattern matches @YYYY-MM-DD — an already-resolved date token.
var resolvedDatePattern = regexp.MustCompile(`@(\d{4}-\d{2}-\d{2})`)

// ResolveDateTokens finds @<token> patterns in content and resolves relative
// date tokens to @YYYY-MM-DD. Unrecognized tokens and literal dates are left unchanged.
//
// Token boundary rules:
//   - @ must be preceded by whitespace, start-of-string, or non-alphanumeric (to avoid emails)
//   - Token ends at whitespace, punctuation, or end-of-string
func ResolveDateTokens(content string) string {
	matches := dateTokenPattern.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return content
	}

	var b strings.Builder
	b.Grow(len(content))
	last := 0

	for _, loc := range matches {
		start, end := loc[0], loc[1]

		// Check preceding character — skip if alphanumeric (email-like)
		if start > 0 && isAlphanumeric(content[start-1]) {
			continue
		}

		token := content[start+1 : end] // strip @

		// Already a resolved YYYY-MM-DD? Leave alone.
		if resolvedDatePattern.MatchString(content[start:end]) {
			continue
		}

		// Try to resolve
		resolved, ok := dateparse.ResolveDate(token)
		if !ok {
			continue // Unrecognized, leave as-is
		}

		b.WriteString(content[last:start])
		b.WriteString("@" + resolved.Format("2006-01-02"))
		last = end
	}

	b.WriteString(content[last:])
	return b.String()
}

// ResolveDateTokensInQuery resolves @<token> patterns in a search query string.
// Same logic as ResolveDateTokens but operates on query text.
func ResolveDateTokensInQuery(query string) string {
	return ResolveDateTokens(query)
}

// ExtractDates finds all @YYYY-MM-DD patterns in content and returns the
// date strings (without @ prefix), sorted and deduplicated.
func ExtractDates(content string) []string {
	matches := resolvedDatePattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var dates []string
	for _, m := range matches {
		date := m[1] // capture group without @
		if !seen[date] {
			seen[date] = true
			dates = append(dates, date)
		}
	}

	sort.Strings(dates)
	return dates
}

func isAlphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_'
}
