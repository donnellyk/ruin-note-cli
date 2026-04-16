package note

import (
	"regexp"
	"strings"
)

// StripGlobalTags removes all tag-only lines from content.
func StripGlobalTags(content string, inlineTags []string) string {
	lines := strings.Split(content, "\n")

	var result []string
	for _, line := range lines {
		if IsTagOnlyLine(strings.TrimSpace(line)) {
			continue
		}
		result = append(result, line)
	}

	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	return strings.Join(result, "\n")
}

// StripTitle removes the first header line and any following empty lines.
func StripTitle(content string) string {
	lines := strings.Split(content, "\n")

	firstNonEmptyIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			firstNonEmptyIdx = i
			break
		}
	}

	if firstNonEmptyIdx == -1 {
		return content
	}

	headerPattern := regexp.MustCompile(`^#{1,6}\s+.+$`)
	if !headerPattern.MatchString(strings.TrimSpace(lines[firstNonEmptyIdx])) {
		return content
	}

	result := make([]string, 0, len(lines)-1)
	result = append(result, lines[:firstNonEmptyIdx]...)
	result = append(result, lines[firstNonEmptyIdx+1:]...)

	for len(result) > 0 && strings.TrimSpace(result[0]) == "" {
		result = result[1:]
	}

	return strings.Join(result, "\n")
}
