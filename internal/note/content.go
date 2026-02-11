package note

import (
	"regexp"
	"strings"
)

// StripGlobalTags removes all tag-only lines from content.
// A tag-only line is one containing only tags and separator characters
// (whitespace, commas, etc.) with no other content.
func StripGlobalTags(content string, inlineTags []string) string {
	lines := strings.Split(content, "\n")

	var result []string
	for _, line := range lines {
		if IsTagOnlyLine(strings.TrimSpace(line)) {
			continue
		}
		result = append(result, line)
	}

	// Clean up trailing empty lines
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
