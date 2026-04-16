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

	// Find all ![[...]] ranges so we can skip @tokens inside dynamic embeds.
	// Without this, ![[pick: #followup @today]] would resolve @today on save,
	// turning the dynamic embed into a fixed date.
	embedRanges := FindEmbedRanges(content)

	var b strings.Builder
	b.Grow(len(content))
	last := 0

	for _, loc := range matches {
		start, end := loc[0], loc[1]

		// Skip if preceding char is alphanumeric (email-like).
		if start > 0 && isAlphanumeric(content[start-1]) {
			continue
		}

		if InsideRanges(start, embedRanges) {
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
func ExtractDates(content string) []string {
	matches := resolvedDatePattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var dates []string
	for _, m := range matches {
		date := m[1]
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
