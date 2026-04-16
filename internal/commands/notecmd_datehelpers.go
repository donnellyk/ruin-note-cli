package commands

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/dateparse"
	"github.com/donnellyk/ruin-note-cli/internal/note"
)

func resolveDateArg(raw string) (string, error) {
	token := strings.TrimPrefix(raw, "@")
	resolved, ok := dateparse.ResolveDate(token)
	if !ok {
		return "", fmt.Errorf("unrecognized date: %q", raw)
	}
	return "@" + resolved.Format("2006-01-02"), nil
}

var resolvedDateRe = regexp.MustCompile(`\s*@\d{4}-\d{2}-\d{2}`)

func specificDateRe(date string) *regexp.Regexp {
	return regexp.MustCompile(`\s*` + regexp.QuoteMeta(date))
}

// insertDateInContent inserts a date reference into content.
// If lineNum==0: insert on tag-only line (like insertGlobalTag).
// Otherwise: append to end of specified line.
func insertDateInContent(content, date string, lineNum int) (string, error) {
	if lineNum == 0 {
		return insertGlobalTag(content, date), nil
	}
	lines := strings.Split(content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return "", fmt.Errorf("--line %d out of range (note has %d content lines)", lineNum, len(lines))
	}
	idx := lineNum - 1
	lines[idx] = strings.TrimRight(lines[idx], " \t") + " " + date
	return strings.Join(lines, "\n"), nil
}

// removeDateFromContent removes date references from content.
// If date is empty, removes ALL @YYYY-MM-DD patterns.
// If date is set (e.g. "@2026-03-15"), removes only that specific date.
// If lineNum==0: operates on all lines. Otherwise on specific line only.
func removeDateFromContent(content, date string, lineNum int) string {
	lines := strings.Split(content, "\n")

	var re *regexp.Regexp
	if date == "" {
		re = resolvedDateRe
	} else {
		re = specificDateRe(date)
	}

	start, end := 0, len(lines)
	if lineNum > 0 && lineNum <= len(lines) {
		start = lineNum - 1
		end = lineNum
	}

	var result []string
	for i, l := range lines {
		if i >= start && i < end {
			newLine := re.ReplaceAllString(l, "")
			newLine = strings.TrimRight(newLine, " \t")
			// Drop tag-only lines that become empty after date removal.
			if strings.TrimSpace(newLine) == "" && note.IsTagOnlyLine(strings.TrimSpace(l)) {
				continue
			}
			result = append(result, newLine)
		} else {
			result = append(result, l)
		}
	}

	return strings.Join(result, "\n")
}
