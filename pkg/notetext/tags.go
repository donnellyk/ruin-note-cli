package notetext

import (
	"regexp"
	"strings"
)

var (
	// simpleTagPattern matches #word, #word/sub, #kebab-case, #date/2026-q2, etc.
	// The first character after # must be a word character or slash (not a dash),
	// which avoids spurious matches like #- or #----.
	// Trailing dashes are stripped programmatically after matching.
	simpleTagPattern = regexp.MustCompile(`#[\w/][\w/-]*`)

	markdownLinkPattern = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)
)

// TagMatch represents a found tag with its position.
type TagMatch struct {
	Tag   string // The full tag including #
	Start int    // Start position in content
	End   int    // End position in content
}

// ExtractTags finds all tags in the given content.
// Returns deduplicated tags in order of first occurrence.
func ExtractTags(content string) []string {
	matches := findAllTags(content)
	return deduplicateTags(matches)
}

// ExtractTagMatches finds all tags with their positions.
func ExtractTagMatches(content string) []TagMatch {
	return findAllTags(content)
}

func findAllTags(content string) []TagMatch {
	var matches []TagMatch
	seen := make(map[int]bool)

	linkRanges := findMarkdownLinkRanges(content)
	embedRanges := FindEmbedRanges(content)
	codeRanges := FindCodeRanges(content)
	excludedRanges := append(linkRanges, embedRanges...)
	excludedRanges = append(excludedRanges, codeRanges...)

	// Spaced tags take precedence over simple tags on the same positions.
	for i := 0; i < len(content); i++ {
		if content[i] != '#' {
			continue
		}

		if InsideRanges(i, excludedRanges) {
			continue
		}

		spacedTag, end := tryParseSpacedTag(content, i)
		if spacedTag != "" {
			matches = append(matches, TagMatch{
				Tag:   spacedTag,
				Start: i,
				End:   end,
			})
			for j := i; j < end; j++ {
				seen[j] = true
			}
			i = end - 1
		}
	}

	simpleMatches := simpleTagPattern.FindAllStringIndex(content, -1)
	for _, loc := range simpleMatches {
		start, end := loc[0], loc[1]
		if seen[start] || InsideRanges(start, excludedRanges) {
			continue
		}
		// Strip trailing dashes. The regex allows dashes inside the tag but a
		// trailing dash (e.g. in prose: "follow up #done- later") is treated
		// as punctuation, not part of the tag.
		for end > start+1 && content[end-1] == '-' {
			end--
		}
		tag := content[start:end]
		matches = append(matches, TagMatch{
			Tag:   tag,
			Start: start,
			End:   end,
		})
	}

	return matches
}

func findMarkdownLinkRanges(content string) [][2]int {
	locs := markdownLinkPattern.FindAllStringIndex(content, -1)
	ranges := make([][2]int, len(locs))
	for i, loc := range locs {
		ranges[i] = [2]int{loc[0], loc[1]}
	}
	return ranges
}

// FindCodeRanges returns byte ranges for inline code (`...`) and fenced code blocks (```...```).
func FindCodeRanges(content string) [][2]int {
	var ranges [][2]int
	i := 0
	for i < len(content) {
		if content[i] == '`' {
			if i+2 < len(content) && content[i+1] == '`' && content[i+2] == '`' {
				start := i
				j := i + 3
				for j < len(content) && content[j] != '\n' {
					j++
				}
				for j < len(content) {
					if content[j] == '\n' && j+3 < len(content) && content[j+1] == '`' && content[j+2] == '`' && content[j+3] == '`' {
						end := j + 4
						ranges = append(ranges, [2]int{start, end})
						i = end
						break
					}
					j++
				}
				if j >= len(content) {
					// Unclosed fenced block — treat rest as code
					ranges = append(ranges, [2]int{start, len(content)})
					break
				}
			} else {
				start := i
				j := i + 1
				for j < len(content) && content[j] != '`' && content[j] != '\n' {
					j++
				}
				if j < len(content) && content[j] == '`' {
					ranges = append(ranges, [2]int{start, j + 1})
					i = j + 1
				} else {
					i++
				}
			}
		} else {
			i++
		}
	}
	return ranges
}

// tryParseSpacedTag attempts to parse a spaced tag starting at position start.
// Returns the normalized tag and end position, or empty string if not a valid spaced tag.
func tryParseSpacedTag(content string, start int) (string, int) {
	if start >= len(content) || content[start] != '#' {
		return "", 0
	}

	// "# x" pattern (hash, single space, then word char) indicates a broken tag
	// like "#foo # bar#" — the "# bar#" part must not be treated as a spaced tag.
	if start+2 < len(content) &&
		content[start+1] == ' ' &&
		isWordChar(content[start+2]) {
		return "", 0
	}

	hasSpace := false
	for i := start + 1; i < len(content); i++ {
		ch := content[i]

		if ch == '\n' {
			return "", 0
		}

		if ch == '#' {
			if !hasSpace {
				return "", 0
			}

			// A word char after the closing # means it starts another tag rather than closing this one.
			if i+1 < len(content) {
				next := content[i+1]
				if isWordChar(next) {
					return "", 0
				}
			}

			// Another # later on the same line indicates a malformed spaced tag
			// like "#broken # tag#"; fall through to simple-tag handling.
			if hasAnotherHashOnLine(content, i+1) {
				return "", 0
			}

			inner := content[start+1 : i]
			normalized := "#" + strings.TrimSpace(inner) + "#"
			return normalized, i + 1
		}

		if ch == ' ' || ch == '\t' {
			hasSpace = true
		}
	}

	return "", 0
}

func hasAnotherHashOnLine(content string, start int) bool {
	for i := start; i < len(content); i++ {
		if content[i] == '\n' {
			return false
		}
		if content[i] == '#' {
			return true
		}
	}
	return false
}

// isWordChar treats `/` as a word character so nested tags like #foo/bar
// are recognized as a single token during spaced-tag boundary checks.
func isWordChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '_' || ch == '/'
}

func deduplicateTags(matches []TagMatch) []string {
	seen := make(map[string]bool)
	var result []string

	for _, m := range matches {
		normalized := NormalizeStored(m.Tag)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, m.Tag)
		}
	}

	return result
}

// NormalizeStored returns the storage form of a tag: lowercased, with one
// leading and one trailing `#` stripped if present. Suitable for tag values
// that live in `.ruin/` indexes, frontmatter `tags:` arrays, and equality
// comparisons. Body-text references (`#tag`, `#meeting notes#`) are converted
// to storage form by stripping the delimiters.
//
// Idempotent on stored-form inputs and on tags produced by ExtractTags or
// BodyForm (both emit at most one `#` on each side). Inputs with multiple
// stacked `#` characters are not normalized to a fixed point.
func NormalizeStored(tag string) string {
	tag = strings.ToLower(tag)
	tag = strings.TrimPrefix(tag, "#")
	tag = strings.TrimSuffix(tag, "#")
	return tag
}

// BodyForm returns the body-text form of a tag: lowercased, with the
// appropriate delimiters re-added. Tags containing whitespace (spaced
// tags) are wrapped in `#…#`; other tags get a leading `#` only.
//
// Use this when emitting tags into note body content (rename rewrites,
// EnsureLinkTag, compose tag rendering). Idempotent: input already in
// body form round-trips.
func BodyForm(tag string) string {
	tag = NormalizeStored(tag)
	if tag == "" {
		return ""
	}
	if strings.ContainsAny(tag, " \t") {
		return "#" + tag + "#"
	}
	return "#" + tag
}

// ClassifyTags separates tags into global tags and inline tags.
// Global tags are those on tag-only lines (lines containing only tags and
// separator characters like commas/whitespace), regardless of position.
// Inline tags are those on lines that also contain non-tag content.
func ClassifyTags(content string, title string) (globalTags []string, inlineTags []string) {
	lines := strings.Split(content, "\n")

	tagOnlyLines := make(map[int]bool)
	for i, line := range lines {
		if IsTagOnlyLine(strings.TrimSpace(line)) {
			tagOnlyLines[i] = true
		}
	}

	allMatches := findAllTags(content)

	lineOffsets := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		lineOffsets[i] = offset
		offset += len(line) + 1
	}

	seenGlobal := make(map[string]bool)
	seenInline := make(map[string]bool)

	for _, match := range allMatches {
		lineIdx := findLineIndex(match.Start, lineOffsets)
		normalized := NormalizeStored(match.Tag)

		if tagOnlyLines[lineIdx] {
			if !seenGlobal[normalized] {
				seenGlobal[normalized] = true
				globalTags = append(globalTags, match.Tag)
			}
		} else {
			if !seenInline[normalized] {
				seenInline[normalized] = true
				inlineTags = append(inlineTags, match.Tag)
			}
		}
	}

	return globalTags, inlineTags
}

// IsHeaderLine returns true if the line is a markdown header (H1 through H6).
func IsHeaderLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	return i >= 1 && i <= 6 && i < len(trimmed) && trimmed[i] == ' '
}

// IsTagOnlyLine returns true if the line contains only tags and separator characters.
// Separator characters are whitespace and commas (e.g. "#log, #ruin").
// Empty lines or whitespace-only lines return false.
func IsTagOnlyLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	tags := findAllTags(line)
	if len(tags) == 0 {
		return false
	}

	remaining := line
	// Remove from end first so earlier positions stay valid.
	for i := len(tags) - 1; i >= 0; i-- {
		t := tags[i]
		remaining = remaining[:t.Start] + remaining[t.End:]
	}

	return strings.Trim(remaining, " \t,;|·•–—/") == ""
}

func findLineIndex(pos int, lineOffsets []int) int {
	for i := len(lineOffsets) - 1; i >= 0; i-- {
		if pos >= lineOffsets[i] {
			return i
		}
	}
	return 0
}

// MergeTags combines global and inline tags, with global tags first.
// Returns deduplicated list.
func MergeTags(globalTags, inlineTags []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, tag := range globalTags {
		normalized := NormalizeStored(tag)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, tag)
		}
	}

	for _, tag := range inlineTags {
		normalized := NormalizeStored(tag)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, tag)
		}
	}

	return result
}

// StripInheritedTagsFromContent removes inherited tags from tag-only lines in content.
// If all tags on a line are inherited, the line is removed entirely.
// If some tags remain, the line is reconstructed with clean formatting.
// Non-tag-only lines are never modified.
func StripInheritedTagsFromContent(content string, inheritedTags []string) string {
	if len(inheritedTags) == 0 {
		return content
	}

	inheritedSet := make(map[string]bool, len(inheritedTags))
	for _, t := range inheritedTags {
		inheritedSet[NormalizeStored(t)] = true
	}

	lines := strings.Split(content, "\n")
	var result []string
	prevBlank := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !IsTagOnlyLine(trimmed) {
			if trimmed == "" {
				if prevBlank {
					continue
				}
				prevBlank = true
			} else {
				prevBlank = false
			}
			result = append(result, line)
			continue
		}

		tags := findAllTags(line)

		var keepTags []string
		for _, tm := range tags {
			if !inheritedSet[NormalizeStored(tm.Tag)] {
				keepTags = append(keepTags, tm.Tag)
			}
		}

		if len(keepTags) == 0 {
			continue
		}

		if len(keepTags) == len(tags) {
			prevBlank = false
			result = append(result, line)
			continue
		}

		prevBlank = false
		result = append(result, strings.Join(keepTags, ", "))
	}

	return strings.Join(result, "\n")
}
