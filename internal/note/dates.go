package note

import (
	"regexp"
	"sort"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/dateparse"
)

// dateTokenPattern matches @ followed by a potential date token (e.g., today,
// next-week, 2-days, 2026-02-13).
var dateTokenPattern = regexp.MustCompile(`@([\w][\w-]*)`)

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

	// Skip @tokens inside ![[...]] embeds (so ![[pick: @today]] isn't rewritten
	// on save into a fixed date) and inside code spans / fenced blocks (so
	// example commands in prose aren't rewritten).
	excludedRanges := append(FindEmbedRanges(content), FindCodeRanges(content)...)

	var b strings.Builder
	b.Grow(len(content))
	last := 0

	for _, loc := range matches {
		start, end := loc[0], loc[1]

		// Skip if preceding char is alphanumeric (email-like).
		if start > 0 && isAlphanumeric(content[start-1]) {
			continue
		}

		if InsideRanges(start, excludedRanges) {
			continue
		}

		token := content[start+1 : end]

		if resolvedDatePattern.MatchString(content[start:end]) {
			continue
		}

		resolved, ok := dateparse.ResolveDate(token)
		if !ok {
			continue
		}

		b.WriteString(content[last:start])
		b.WriteString("@" + resolved.Format("2006-01-02"))
		last = end
	}

	b.WriteString(content[last:])
	return b.String()
}

func ResolveDateTokensInQuery(query string) string {
	return ResolveDateTokens(query)
}

// ExtractDates returns @YYYY-MM-DD dates (without @ prefix), sorted and deduplicated.
// Dates inside ![[...]] embeds and code spans / fenced blocks are ignored.
func ExtractDates(content string) []string {
	locs := resolvedDatePattern.FindAllStringSubmatchIndex(content, -1)
	if len(locs) == 0 {
		return nil
	}

	excludedRanges := append(FindEmbedRanges(content), FindCodeRanges(content)...)

	seen := make(map[string]bool)
	var dates []string
	for _, loc := range locs {
		if InsideRanges(loc[0], excludedRanges) {
			continue
		}
		date := content[loc[2]:loc[3]]
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
