package note

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

	// Find simple tags, skip if overlapping with spaced tags
	simpleMatches := simpleTagPattern.FindAllStringIndex(content, -1)
	for _, loc := range simpleMatches {
		start, end := loc[0], loc[1]
		if seen[start] {
			continue // Skip if this position is part of a spaced tag
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

// normalizeSpacedTag cleans up a spaced tag.
// Input: "#some text#" -> Output: "#some text#"
// Trims internal whitespace but keeps the delimiters.
func normalizeSpacedTag(tag string) string {
	// Remove the surrounding #s, trim whitespace, add back
	inner := strings.TrimPrefix(tag, "#")
	inner = strings.TrimSuffix(inner, "#")
	inner = strings.TrimSpace(inner)
	return "#" + inner + "#"
}

func deduplicateTags(matches []TagMatch) []string {
	seen := make(map[string]bool)
	var result []string

	for _, m := range matches {
		normalized := normalizeTag(m.Tag)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, m.Tag)
		}
	}

	return result
}

// normalizeTag normalizes a tag for comparison (lowercase).
func normalizeTag(tag string) string {
	return strings.ToLower(tag)
}

// ClassifyTags separates tags into global tags and inline tags.
// Global tags are those appearing:
//   - Directly under the H1 title (before first paragraph)
//   - At the end of the document (after last paragraph)
//
// Inline tags are those appearing within content paragraphs.
func ClassifyTags(content string, title string) (globalTags []string, inlineTags []string) {
	lines := strings.Split(content, "\n")

	// Find title line index
	titleLineIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			titleLineIdx = i
			break
		}
	}

	// Find first content line (non-empty, non-tag-only line after title)
	firstContentIdx := -1
	for i := titleLineIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		// Check if line is only tags
		if isTagOnlyLine(trimmed) {
			continue
		}
		firstContentIdx = i
		break
	}

	// Find last content line (non-empty, non-tag-only line)
	lastContentIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if isTagOnlyLine(trimmed) {
			continue
		}
		lastContentIdx = i
		break
	}

	// Now classify each tag
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
		// Find which line this tag is on
		lineIdx := findLineIndex(match.Start, lineOffsets)

		normalized := normalizeTag(match.Tag)
		isGlobal := false

		// Global if: between title and first content, or after last content
		if firstContentIdx == -1 {
			// No content, all tags are global
			isGlobal = true
		} else if lineIdx > titleLineIdx && lineIdx < firstContentIdx {
			// Between title and first content
			isGlobal = true
		} else if lineIdx > lastContentIdx {
			// After last content
			isGlobal = true
		}

		if isGlobal {
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

// isTagOnlyLine returns true if the line contains only tags and whitespace.
// Empty lines or whitespace-only lines return false.
func isTagOnlyLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	// Extract all tags from this line
	tags := findAllTags(line)
	if len(tags) == 0 {
		return false
	}

	// Remove all matched tags from the line and check if only whitespace remains
	remaining := line
	// Sort by position descending to remove from end first (preserves positions)
	for i := len(tags) - 1; i >= 0; i-- {
		t := tags[i]
		remaining = remaining[:t.Start] + remaining[t.End:]
	}

	return strings.TrimSpace(remaining) == ""
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
		normalized := normalizeTag(tag)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, tag)
		}
	}

	for _, tag := range inlineTags {
		normalized := normalizeTag(tag)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, tag)
		}
	}

	return result
}
