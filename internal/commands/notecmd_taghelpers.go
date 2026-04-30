package commands

import (
	"fmt"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
)

// ensureHashPrefix is retained as a thin wrapper around note.BodyForm so
// callers expressing intent ("the user typed a tag, render it for body
// insertion") read clearly. Spaced tags get `#…#` delimiters.
func ensureHashPrefix(tag string) string {
	return note.BodyForm(tag)
}

func noteHasTag(n *note.Note, tag string) bool {
	normalized := note.NormalizeStored(tag)
	for _, t := range n.AllTags() {
		if note.NormalizeStored(t) == normalized {
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

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if note.IsTagOnlyLine(trimmed) {
			sep := detectSeparator(trimmed)
			lines[i] = trimmed + sep + tag
			return strings.Join(lines, "\n")
		}
	}

	// No tag-only line found — insert one after the title header.
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

func detectSeparator(line string) string {
	if strings.Contains(line, ", ") {
		return ", "
	}
	if strings.Contains(line, ",") {
		return ", "
	}
	return " "
}

func removeTagClean(content, tag string) string {
	normalized := note.NormalizeStored(tag)
	lines := strings.Split(content, "\n")
	var result []string

	for _, line := range lines {
		newLine := removeTagFromLine(line, normalized)
		trimmed := strings.TrimSpace(newLine)
		if trimmed == "" && note.IsTagOnlyLine(strings.TrimSpace(line)) {
			continue
		}
		result = append(result, newLine)
	}

	return strings.Join(result, "\n")
}

func removeTagFromLine(line, normalizedTag string) string {
	matches := note.ExtractTagMatches(line)
	if len(matches) == 0 {
		return line
	}

	var toRemove []note.TagMatch
	for _, m := range matches {
		if note.NormalizeStored(m.Tag) == normalizedTag {
			toRemove = append(toRemove, m)
		}
	}

	if len(toRemove) == 0 {
		return line
	}

	// Remove matches from end to start to preserve positions.
	result := line
	for i := len(toRemove) - 1; i >= 0; i-- {
		m := toRemove[i]
		before := result[:m.Start]
		after := result[m.End:]
		result = before + after
	}

	result = cleanSeparators(result)
	result = strings.TrimRight(result, " \t")

	return result
}

func cleanSeparators(line string) string {
	for strings.Contains(line, "  ") {
		line = strings.ReplaceAll(line, "  ", " ")
	}
	for strings.Contains(line, ",,") {
		line = strings.ReplaceAll(line, ",,", ",")
	}
	for strings.Contains(line, ", ,") {
		line = strings.ReplaceAll(line, ", ,", ",")
	}
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, ", ")
	line = strings.TrimRight(line, ", ")
	return line
}

func insertInlineTag(content, tag string, lineNum int) (string, error) {
	lines := strings.Split(content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return "", fmt.Errorf("--line %d out of range (note has %d content lines)", lineNum, len(lines))
	}
	idx := lineNum - 1
	lines[idx] = strings.TrimRight(lines[idx], " \t") + " " + tag
	return strings.Join(lines, "\n"), nil
}

func removeTagFromLineNum(content, tag string, lineNum int) (string, error) {
	lines := strings.Split(content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return "", fmt.Errorf("--line %d out of range (note has %d content lines)", lineNum, len(lines))
	}
	idx := lineNum - 1
	normalized := note.NormalizeStored(tag)
	newLine := removeTagFromLine(lines[idx], normalized)
	trimmed := strings.TrimSpace(newLine)
	if trimmed == "" && note.IsTagOnlyLine(strings.TrimSpace(lines[idx])) {
		lines = append(lines[:idx], lines[idx+1:]...)
	} else {
		lines[idx] = newLine
	}
	return strings.Join(lines, "\n"), nil
}
