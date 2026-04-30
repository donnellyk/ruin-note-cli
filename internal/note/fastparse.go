package note

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrFrontmatterTruncated = errors.New("frontmatter truncated")

var bufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 4096)
		return &buf
	},
}

// ParseFrontmatterFast parses frontmatter from a byte slice using a hand-rolled
// line-by-line scanner, avoiding yaml.Unmarshal overhead. Unknown keys are
// silently skipped (Extra is not populated).
func ParseFrontmatterFast(data []byte) (*Frontmatter, int, error) {
	offset := 0
	for offset < len(data) && (data[offset] == '\n' || data[offset] == '\r') {
		offset++
	}

	if !bytes.HasPrefix(data[offset:], []byte("---")) {
		return &Frontmatter{}, offset, nil
	}

	lineEnd := bytes.IndexByte(data[offset:], '\n')
	if lineEnd == -1 {
		return &Frontmatter{}, offset, nil
	}
	fmStart := offset + lineEnd + 1

	closingIdx := bytes.Index(data[fmStart:], []byte("\n---"))
	if closingIdx == -1 {
		return nil, 0, ErrFrontmatterTruncated
	}

	fmBytes := data[fmStart : fmStart+closingIdx]

	bodyOffset := fmStart + closingIdx + 4 // len("\n---")
	if bodyOffset < len(data) && data[bodyOffset] == '\n' {
		bodyOffset++
	} else if bodyOffset+1 < len(data) && data[bodyOffset] == '\r' && data[bodyOffset+1] == '\n' {
		bodyOffset += 2
	}

	fm := &Frontmatter{}
	if err := parseFMLines(fmBytes, fm); err != nil {
		return nil, 0, err
	}

	// Block-scalar lists are appended raw inside parseFMLines (currentList path).
	// Normalize after the fact so both flow and block forms strip the `#` prefix.
	fm.Tags = normalizeTagSlice(fm.Tags)
	fm.InlineTags = normalizeTagSlice(fm.InlineTags)
	fm.InheritedTags = normalizeTagSlice(fm.InheritedTags)

	return fm, bodyOffset, nil
}

func parseFMLines(data []byte, fm *Frontmatter) error {
	var currentKey string
	var currentList *[]string

	lines := bytes.SplitSeq(data, []byte("\n"))
	for line := range lines {
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		if len(line) == 0 {
			continue
		}

		if (line[0] == ' ' || line[0] == '\t') && currentList != nil {
			trimmed := bytes.TrimSpace(line)
			if bytes.HasPrefix(trimmed, []byte("- ")) {
				val := trimmed[2:]
				*currentList = append(*currentList, unquote(val))
				continue
			}
		}

		colonIdx := bytes.IndexByte(line, ':')
		if colonIdx <= 0 {
			currentKey = ""
			currentList = nil
			continue
		}

		key := string(bytes.TrimSpace(line[:colonIdx]))
		value := bytes.TrimSpace(line[colonIdx+1:])
		currentKey = key
		currentList = nil

		if len(value) > 0 && value[0] == '[' {
			list := parseFlowList(value)
			switch key {
			case "tags":
				fm.Tags = normalizeTagSlice(list)
			case "inline-tags":
				fm.InlineTags = normalizeTagSlice(list)
			case "inherited-tags":
				fm.InheritedTags = normalizeTagSlice(list)
			case "dates":
				fm.Dates = list
			case "linked-cards":
				fm.LinkedCards = list
			}
			continue
		}

		if len(value) == 0 {
			switch key {
			case "tags":
				currentList = &fm.Tags
			case "inline-tags":
				currentList = &fm.InlineTags
			case "inherited-tags":
				currentList = &fm.InheritedTags
			case "dates":
				currentList = &fm.Dates
			case "linked-cards":
				currentList = &fm.LinkedCards
			}
			continue
		}

		strVal := unquote(value)
		switch key {
		case "uuid":
			fm.UUID = strVal
		case "created":
			fm.Created = strVal
		case "updated":
			fm.Updated = strVal
		case "url":
			fm.URL = strVal
		case "parent":
			fm.Parent = strVal
		case "order":
			if n, err := strconv.Atoi(strVal); err == nil {
				fm.Order = &n
			}
		case "tags":
			fm.Tags = []string{strVal}
		case "inline-tags":
			fm.InlineTags = []string{strVal}
		case "inherited-tags":
			fm.InheritedTags = []string{strVal}
		case "dates":
			fm.Dates = []string{strVal}
		case "linked-cards":
			fm.LinkedCards = []string{strVal}
		default:
			_ = currentKey
		}
	}

	return nil
}

func unquote(b []byte) string {
	s := string(bytes.TrimSpace(b))
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// parseFlowList parses a YAML flow-style list like [a, b, c] or ["a", "b"].
func parseFlowList(data []byte) []string {
	s := string(bytes.TrimSpace(data))
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil
	}
	inner := s[1 : len(s)-1]
	if strings.TrimSpace(inner) == "" {
		return nil
	}

	parts := strings.Split(inner, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		result = append(result, unquote([]byte(p)))
	}
	return result
}

// LoadFrontmatterOnly reads only the frontmatter portion of a note file (up to
// 4KB) and extracts the title from the beginning of the body. Content is left
// empty. On truncated frontmatter, falls back to full Load.
func LoadFrontmatterOnly(path string) (*Note, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	bufPtr := bufPool.Get().(*[]byte)
	defer bufPool.Put(bufPtr)
	buf := *bufPtr

	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	data := buf[:n]

	fm, bodyOffset, err := ParseFrontmatterFast(data)
	if err != nil {
		if errors.Is(err, ErrFrontmatterTruncated) {
			return Load(path)
		}
		return nil, err
	}

	// Tags/InlineTags/InheritedTags are intentionally not populated here from
	// v0.4.0 onward. The titles.json mirror is the source of truth for hot-path
	// tag matchers; callers that need tags hydrate via hydrateNoteTagsFromIndex
	// at the worker boundary. Frontmatter values are still parsed (for doctor's
	// migration-detection scan) but discarded on the returned *Note.
	note := &Note{
		UUID:        fm.UUID,
		Parent:      fm.Parent,
		Order:       fm.Order,
		LinkedCards: fm.LinkedCards,
		URL:         fm.URL,
		Dates:       fm.Dates,
		FilePath:    path,
	}

	if fm.Created != "" {
		if t, err := time.Parse(TimeFormat, fm.Created); err == nil {
			note.Created = t
		}
	}
	if fm.Updated != "" {
		if t, err := time.Parse(TimeFormat, fm.Updated); err == nil {
			note.Updated = t
		}
	}

	if bodyOffset < len(data) {
		note.Title = extractTitleFromBytes(data[bodyOffset:])
	}

	return note, nil
}

func extractTitleFromBytes(data []byte) string {
	for len(data) > 0 {
		lineEnd := bytes.IndexByte(data, '\n')
		var line []byte
		if lineEnd == -1 {
			line = data
			data = nil
		} else {
			line = data[:lineEnd]
			data = data[lineEnd+1:]
		}

		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		if len(line) == 0 || line[0] != '#' {
			continue
		}

		hashes := 0
		for hashes < len(line) && line[hashes] == '#' {
			hashes++
		}
		if hashes > 6 || hashes >= len(line) {
			continue
		}
		if line[hashes] != ' ' && line[hashes] != '\t' {
			continue
		}

		title := bytes.TrimSpace(line[hashes:])
		return string(title)
	}
	return ""
}
