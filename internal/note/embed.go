package note

import (
	"fmt"
	"regexp"
	"strings"
)

var embedPattern = regexp.MustCompile(`!\[\[([^\[\]#|]+?)(?:#([^\[\]|]+?))?\]\]`)
var embedBlockPattern = regexp.MustCompile(`!\[\[.+?\]\]`)

// FindEmbedRanges returns the [start, end) byte positions of all ![[...]] blocks.
func FindEmbedRanges(content string) [][2]int {
	locs := embedBlockPattern.FindAllStringIndex(content, -1)
	ranges := make([][2]int, len(locs))
	for i, loc := range locs {
		ranges[i] = [2]int{loc[0], loc[1]}
	}
	return ranges
}

// InsideRanges returns true if position pos falls within any of the given ranges.
func InsideRanges(pos int, ranges [][2]int) bool {
	for _, r := range ranges {
		if pos >= r[0] && pos < r[1] {
			return true
		}
	}
	return false
}

type EmbedRef struct {
	NoteRef string
	Header  string
	Line    int
}

func FindEmbeds(content string) []EmbedRef {
	lines := strings.Split(content, "\n")
	var refs []EmbedRef
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		m := embedPattern.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		if m[0] != trimmed {
			continue
		}
		ref := EmbedRef{
			NoteRef: strings.TrimSpace(m[1]),
			Line:    i,
		}
		if len(m) > 2 && m[2] != "" {
			ref.Header = strings.TrimSpace(m[2])
		}
		refs = append(refs, ref)
	}
	return refs
}

func ExtractSection(content string, header string) (string, error) {
	lines := strings.Split(content, "\n")
	headerLower := strings.ToLower(header)

	headingLevelPattern := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

	startIdx := -1
	startLevel := 0

	for i, line := range lines {
		m := headingLevelPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		headingText := strings.TrimSpace(m[2])
		if strings.ToLower(headingText) == headerLower {
			startIdx = i
			startLevel = len(m[1])
			break
		}
	}

	if startIdx == -1 {
		return "", fmt.Errorf("header %q not found", header)
	}

	endIdx := len(lines)
	for i := startIdx + 1; i < len(lines); i++ {
		m := headingLevelPattern.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		level := len(m[1])
		if level <= startLevel {
			endIdx = i
			break
		}
	}

	for endIdx > startIdx+1 && strings.TrimSpace(lines[endIdx-1]) == "" {
		endIdx--
	}

	return strings.Join(lines[startIdx:endIdx], "\n"), nil
}
