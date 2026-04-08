package commands

import (
	"fmt"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
)

// ensureHashPrefix adds # prefix if missing.
func ensureHashPrefix(tag string) string {
	if !strings.HasPrefix(tag, "#") {
		return "#" + tag
	}
	return tag
}

// noteHasTag checks if a note already has a tag (case-insensitive).
func noteHasTag(n *note.Note, tag string) bool {
	normalized := note.NormalizeTag(tag)
	for _, t := range n.AllTags() {
		if note.NormalizeTag(t) == normalized {
			return true
		}
	}
	return false
}

// insertGlobalTag inserts a global tag into note content.
// If a tag-only line exists, appends to the first one using the same separator.
// Otherwise, inserts a new tag-only line after the title header (or at line 0).
func insertGlobalTag(content, tag string) string {
	lines := strings.Split(content, "\n")

	// Find the first tag-only line
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if note.IsTagOnlyLine(trimmed) {
			sep := detectSeparator(trimmed)
			lines[i] = trimmed + sep + tag
			return strings.Join(lines, "\n")
		}
	}

	// No tag-only line found — insert one after the title header
	insertIdx := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if note.IsHeaderLine(trimmed) {
			insertIdx = i + 1
			break
		}
	}

	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, tag)
	newLines = append(newLines, lines[insertIdx:]...)
	return strings.Join(newLines, "\n")
}

// detectSeparator returns the separator used between tags on a tag-only line.
// Returns ", " if commas are used, otherwise " ".
func detectSeparator(line string) string {
	if strings.Contains(line, ", ") {
		return ", "
	}
	if strings.Contains(line, ",") {
		return ", "
	}
	return " "
}

// removeTagClean removes a tag from content with proper cleanup.
// - Removes trailing/leading separators
// - Removes lines that become empty after tag removal
// - Trims trailing whitespace on lines where inline tag was removed
func removeTagClean(content, tag string) string {
	normalized := note.NormalizeTag(tag)
	lines := strings.Split(content, "\n")
	var result []string

	for _, line := range lines {
		newLine := removeTagFromLine(line, normalized)
		// If a tag-only line is now empty, skip it
		trimmed := strings.TrimSpace(newLine)
		if trimmed == "" && note.IsTagOnlyLine(strings.TrimSpace(line)) {
			continue
		}
		result = append(result, newLine)
	}

	return strings.Join(result, "\n")
}

// removeTagFromLine removes all occurrences of a tag from a single line.
func removeTagFromLine(line, normalizedTag string) string {
	matches := note.ExtractTagMatches(line)
	if len(matches) == 0 {
		return line
	}

	// Find which matches to remove (by normalized comparison)
	var toRemove []note.TagMatch
	for _, m := range matches {
		if note.NormalizeTag(m.Tag) == normalizedTag {
			toRemove = append(toRemove, m)
		}
	}

	if len(toRemove) == 0 {
		return line
	}

	// Remove matches from end to start to preserve positions
	result := line
	for i := len(toRemove) - 1; i >= 0; i-- {
		m := toRemove[i]
		before := result[:m.Start]
		after := result[m.End:]
		result = before + after
	}

	// Clean up separators: remove double commas, leading/trailing commas
	result = cleanSeparators(result)
	// Trim trailing whitespace
	result = strings.TrimRight(result, " \t")

	return result
}

// cleanSeparators cleans up leftover separator characters after tag removal.
func cleanSeparators(line string) string {
	// Collapse multiple spaces
	for strings.Contains(line, "  ") {
		line = strings.ReplaceAll(line, "  ", " ")
	}
	// Remove ", ," or ",," patterns
	for strings.Contains(line, ",,") {
		line = strings.ReplaceAll(line, ",,", ",")
	}
	for strings.Contains(line, ", ,") {
		line = strings.ReplaceAll(line, ", ,", ",")
	}
	// Remove leading/trailing commas (with optional whitespace)
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, ", ")
	line = strings.TrimRight(line, ", ")
	// If trimming removed meaningful whitespace, keep at least the trimmed form
	return line
}

// --- Line-targeted tag helpers ---

// insertInlineTag appends a tag to the end of a specific content line.
func insertInlineTag(content, tag string, lineNum int) (string, error) {
	lines := strings.Split(content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return "", fmt.Errorf("--line %d out of range (note has %d content lines)", lineNum, len(lines))
	}
	idx := lineNum - 1
	lines[idx] = strings.TrimRight(lines[idx], " \t") + " " + tag
	return strings.Join(lines, "\n"), nil
}

// removeTagFromLineNum removes a tag from a specific content line only.
func removeTagFromLineNum(content, tag string, lineNum int) (string, error) {
	lines := strings.Split(content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return "", fmt.Errorf("--line %d out of range (note has %d content lines)", lineNum, len(lines))
	}
	idx := lineNum - 1
	normalized := note.NormalizeTag(tag)
	newLine := removeTagFromLine(lines[idx], normalized)
	// If a tag-only line becomes empty, remove it
	trimmed := strings.TrimSpace(newLine)
	if trimmed == "" && note.IsTagOnlyLine(strings.TrimSpace(lines[idx])) {
		lines = append(lines[:idx], lines[idx+1:]...)
	} else {
		lines[idx] = newLine
	}
	return strings.Join(lines, "\n"), nil
}
