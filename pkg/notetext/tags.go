package notetext

import (
	"regexp"
	"strings"
)

// Tag patterns:
// Simple tag: #word or #word/subword (alphanumeric, underscore, forward slash)
// Spaced tag: #text with spaces# (must contain at least one space, ends with # not followed by word char)
var (
	// simpleTagPattern matches #word, #word/sub, #123, etc.
	simpleTagPattern = regexp.MustCompile(`#[\w/]+`)

	// spacedTagPattern is not used as a simple regex due to complexity.
	// Instead, we detect spaced tags programmatically.

	// markdownLinkPattern matches [text](url) to exclude tag-like strings inside links.
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
	seen := make(map[int]bool) // Track start positions to avoid overlaps

	// Find regions to exclude from tag extraction
	linkRanges := findMarkdownLinkRanges(content)
	embedRanges := FindEmbedRanges(content)
	codeRanges := findCodeRanges(content)
	excludedRanges := append(linkRanges, embedRanges...)
	excludedRanges = append(excludedRanges, codeRanges...)

	// Find spaced tags first (they take precedence)
	// A spaced tag: #content with space#
	// - Must start with #
	// - Must contain at least one space
	// - Must end with # not followed by word char
	// - Must NOT contain another # in the content (which would indicate a broken tag)
	for i := 0; i < len(content); i++ {
		if content[i] != '#' {
			continue
		}

		// Skip if inside an excluded range (markdown link or embed)
		if InsideRanges(i, excludedRanges) {
			continue
		}

		// Try to find a closing # for a spaced tag
		spacedTag, end := tryParseSpacedTag(content, i)
		if spacedTag != "" {
			matches = append(matches, TagMatch{
				Tag:   spacedTag,
				Start: i,
				End:   end,
			})
			// Mark these positions as used
			for j := i; j < end; j++ {
				seen[j] = true
			}
			i = end - 1 // -1 because loop will increment
		}
	}

	// Find simple tags, skip if overlapping with spaced tags or inside excluded ranges
	simpleMatches := simpleTagPattern.FindAllStringIndex(content, -1)
	for _, loc := range simpleMatches {
		start, end := loc[0], loc[1]
		if seen[start] || InsideRanges(start, excludedRanges) {
			continue
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

// findMarkdownLinkRanges returns byte ranges for all [text](url) markdown links.
func findMarkdownLinkRanges(content string) [][2]int {
	locs := markdownLinkPattern.FindAllStringIndex(content, -1)
	ranges := make([][2]int, len(locs))
	for i, loc := range locs {
		ranges[i] = [2]int{loc[0], loc[1]}
	}
	return ranges
}

// findCodeRanges returns byte ranges for inline code (`...`) and fenced code blocks (```...```).
func findCodeRanges(content string) [][2]int {
	var ranges [][2]int
	i := 0
	for i < len(content) {
		if content[i] == '`' {
			// Check for fenced code block (``` at start of line)
			if i+2 < len(content) && content[i+1] == '`' && content[i+2] == '`' {
				// Find closing ```
				start := i
				// Skip the opening ``` and any language identifier on the same line
				j := i + 3
				for j < len(content) && content[j] != '\n' {
					j++
				}
				// Search for closing ``` on its own line
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
				// Inline code: find matching closing backtick
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

	// Check for "# x" pattern (hash, single space, then word char) which indicates
	// a broken tag like "#foo # bar#" - the "# bar#" part should not be a tag
	if start+2 < len(content) &&
		content[start+1] == ' ' &&
		isWordChar(content[start+2]) {
		return "", 0
	}

	// Look for closing # on the same line
	hasSpace := false
	for i := start + 1; i < len(content); i++ {
		ch := content[i]

		if ch == '\n' {
			// End of line, not a spaced tag
			return "", 0
		}

		if ch == '#' {
			// Found a potential closing #
			// Check if there was a space in the content
			if !hasSpace {
				// No space means this is NOT a spaced tag (could be #foo#bar)
				return "", 0
			}

			// Check if closing # is followed by a word character
			// If so, it's starting another tag, not closing this one
			if i+1 < len(content) {
				next := content[i+1]
				if isWordChar(next) {
					// This # starts another tag, so our potential spaced tag is invalid
					return "", 0
				}
			}

			// Check if there's another # later on the same line
			// If so, this might be a malformed spaced tag like "#broken # tag#"
			// In this case, treat it as a simple tag instead
			if hasAnotherHashOnLine(content, i+1) {
				return "", 0
			}

			// Valid spaced tag found
			inner := content[start+1 : i]
			normalized := "#" + strings.TrimSpace(inner) + "#"
			return normalized, i + 1
		}

		if ch == ' ' || ch == '\t' {
			hasSpace = true
		}
	}

	// No closing # found
	return "", 0
}

// hasAnotherHashOnLine checks if there's another # character before the next newline
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

// isWordChar returns true if the character is a word character (for tag purposes)
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
		normalized := NormalizeTag(m.Tag)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, m.Tag)
		}
	}

	return result
}

// NormalizeTag normalizes a tag for comparison (lowercase).
func NormalizeTag(tag string) string {
	return strings.ToLower(tag)
}

// ClassifyTags separates tags into global tags and inline tags.
// Global tags are those on tag-only lines (lines containing only tags and
// separator characters like commas/whitespace), regardless of position.
// Inline tags are those on lines that also contain non-tag content.
func ClassifyTags(content string, title string) (globalTags []string, inlineTags []string) {
	lines := strings.Split(content, "\n")

	// Build set of tag-only line indices
	tagOnlyLines := make(map[int]bool)
	for i, line := range lines {
		if IsTagOnlyLine(strings.TrimSpace(line)) {
			tagOnlyLines[i] = true
		}
	}

	allMatches := findAllTags(content)

	// Calculate line offsets for position mapping
	lineOffsets := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		lineOffsets[i] = offset
		offset += len(line) + 1 // +1 for newline
	}

	seenGlobal := make(map[string]bool)
	seenInline := make(map[string]bool)

	for _, match := range allMatches {
		lineIdx := findLineIndex(match.Start, lineOffsets)
		normalized := NormalizeTag(match.Tag)

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
	// Count leading # characters
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	// Must have 1-6 #'s followed by a space
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

	// Extract all tags from this line
	tags := findAllTags(line)
	if len(tags) == 0 {
		return false
	}

	// Remove all matched tags from the line and check if only separators remain
	remaining := line
	// Sort by position descending to remove from end first (preserves positions)
	for i := len(tags) - 1; i >= 0; i-- {
		t := tags[i]
		remaining = remaining[:t.Start] + remaining[t.End:]
	}

	// After removing tags, only whitespace and separator punctuation should remain
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
		normalized := NormalizeTag(tag)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, tag)
		}
	}

	for _, tag := range inlineTags {
		normalized := NormalizeTag(tag)
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

	// Build lookup set
	inheritedSet := make(map[string]bool, len(inheritedTags))
	for _, t := range inheritedTags {
		inheritedSet[NormalizeTag(t)] = true
	}

	lines := strings.Split(content, "\n")
	var result []string
	prevBlank := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Pass through non-tag-only lines unchanged
		if !IsTagOnlyLine(trimmed) {
			if trimmed == "" {
				if prevBlank {
					continue // collapse consecutive blank lines
				}
				prevBlank = true
			} else {
				prevBlank = false
			}
			result = append(result, line)
			continue
		}

		// Extract tags from this line
		tags := findAllTags(line)

		// Separate into keep vs strip
		var keepTags []string
		for _, tm := range tags {
			if !inheritedSet[NormalizeTag(tm.Tag)] {
				keepTags = append(keepTags, tm.Tag)
			}
		}

		if len(keepTags) == 0 {
			// All tags were inherited — remove line
			continue
		}

		if len(keepTags) == len(tags) {
			// Nothing stripped — keep original line
			prevBlank = false
			result = append(result, line)
			continue
		}

		// Reconstruct with remaining tags
		prevBlank = false
		result = append(result, strings.Join(keepTags, ", "))
	}

	return strings.Join(result, "\n")
}
