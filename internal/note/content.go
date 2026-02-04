package note

import (
	"regexp"
	"strings"
)

// StripGlobalTags removes global tags (top/bottom of content) while preserving inline tags.
// Global tags are those on tag-only lines at the beginning (after title) or end of content.
func StripGlobalTags(content string, inlineTags []string) string {
	lines := strings.Split(content, "\n")

	// Build a set of inline tag positions (normalized) for reference
	inlineSet := make(map[string]bool)
	for _, tag := range inlineTags {
		inlineSet[NormalizeTag(tag)] = true
	}

	// Find the title line index
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

	// If no content lines found, remove all tag-only lines
	if firstContentIdx == -1 {
		var result []string
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Keep title line and empty lines
			if i == titleLineIdx || trimmed == "" {
				result = append(result, line)
				continue
			}
			// Skip tag-only lines
			if isTagOnlyLine(trimmed) {
				continue
			}
			result = append(result, line)
		}
		return strings.Join(result, "\n")
	}

	// Build result, removing tag-only lines before firstContentIdx and after lastContentIdx
	var result []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Always keep title line
		if i == titleLineIdx {
			result = append(result, line)
			continue
		}

		// Between title and first content: skip tag-only lines
		if i > titleLineIdx && i < firstContentIdx {
			if trimmed == "" {
				// Keep empty lines only if not all lines between title and content are being removed
				continue
			}
			if isTagOnlyLine(trimmed) {
				continue
			}
		}

		// After last content: skip tag-only lines
		if i > lastContentIdx {
			if isTagOnlyLine(trimmed) {
				continue
			}
		}

		result = append(result, line)
	}

	// Clean up excessive empty lines at the end
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	return strings.Join(result, "\n")
}

// StripTitle removes the first H1 line if present at the start of content.
// Also removes any immediately following empty lines.
func StripTitle(content string) string {
	lines := strings.Split(content, "\n")

	// Find first non-empty line
	firstNonEmptyIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			firstNonEmptyIdx = i
			break
		}
	}

	if firstNonEmptyIdx == -1 {
		return content // All empty
	}

	// Check if first non-empty line is an H1
	h1Pattern := regexp.MustCompile(`^#\s+.+$`)
	if !h1Pattern.MatchString(strings.TrimSpace(lines[firstNonEmptyIdx])) {
		return content // Not starting with H1
	}

	// Remove the H1 line
	result := make([]string, 0, len(lines)-1)
	result = append(result, lines[:firstNonEmptyIdx]...)
	result = append(result, lines[firstNonEmptyIdx+1:]...)

	// Trim leading empty lines from the result
	for len(result) > 0 && strings.TrimSpace(result[0]) == "" {
		result = result[1:]
	}

	return strings.Join(result, "\n")
}
