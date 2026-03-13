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

// ErrFrontmatterTruncated indicates the frontmatter didn't fit in the read buffer.
var ErrFrontmatterTruncated = errors.New("frontmatter truncated")

var bufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 4096)
		return &buf
	},
}

// ParseFrontmatterFast parses frontmatter from a byte slice using a hand-rolled
// line-by-line scanner, avoiding yaml.Unmarshal overhead.
// Returns the parsed Frontmatter, the byte offset where the body begins, and any error.
// Unknown keys are silently skipped (Extra is not populated).
func ParseFrontmatterFast(data []byte) (*Frontmatter, int, error) {
	// Trim leading newlines
	offset := 0
	for offset < len(data) && (data[offset] == '\n' || data[offset] == '\r') {
		offset++
	}

	// Check for opening ---
	if !bytes.HasPrefix(data[offset:], []byte("---")) {
		return &Frontmatter{}, offset, nil
	}

	// Find end of opening delimiter line
	lineEnd := bytes.IndexByte(data[offset:], '\n')
	if lineEnd == -1 {
		return &Frontmatter{}, offset, nil
	}
	fmStart := offset + lineEnd + 1

	// Find closing ---
	closingIdx := bytes.Index(data[fmStart:], []byte("\n---"))
	if closingIdx == -1 {
		return nil, 0, ErrFrontmatterTruncated
	}

	fmBytes := data[fmStart : fmStart+closingIdx]

	// Body starts after closing --- and its newline
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

	return fm, bodyOffset, nil
}

// parseFMLines scans frontmatter lines and populates known fields.
func parseFMLines(data []byte, fm *Frontmatter) error {
	var currentKey string
	var currentList *[]string

	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		// Strip \r
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		if len(line) == 0 {
			continue
		}

		// Check if this is a list item (starts with whitespace + "- ")
		if (line[0] == ' ' || line[0] == '\t') && currentList != nil {
			trimmed := bytes.TrimSpace(line)
			if bytes.HasPrefix(trimmed, []byte("- ")) {
				val := trimmed[2:]
				*currentList = append(*currentList, unquote(val))
				continue
			}
		}

		// Key: value line
		colonIdx := bytes.IndexByte(line, ':')
		if colonIdx <= 0 {
			// Not a key line; could be continuation, skip
			currentKey = ""
			currentList = nil
			continue
		}

		key := string(bytes.TrimSpace(line[:colonIdx]))
		value := bytes.TrimSpace(line[colonIdx+1:])
		currentKey = key
		currentList = nil

		// Check for flow-style list: [a, b, c]
		if len(value) > 0 && value[0] == '[' {
			list := parseFlowList(value)
			switch key {
			case "tags":
				fm.Tags = list
			case "inline-tags":
				fm.InlineTags = list
			case "inherited-tags":
				fm.InheritedTags = list
			case "dates":
				fm.Dates = list
			case "linked-cards":
				fm.LinkedCards = list
			}
			continue
		}

		// If value is empty, this might be a block-style list
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
			default:
				// Unknown key with block list - skip
			}
			continue
		}

		// Scalar value
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
			// Single value on same line as key (unusual but handle it)
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
			// Unknown key - skip
			_ = currentKey
		}
	}

	return nil
}

// unquote strips surrounding quotes from a byte slice and returns a string.
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
	// Strip [ and ]
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

// LoadFrontmatterOnly reads only the frontmatter portion of a note file.
// It reads up to 4KB, parses frontmatter with the fast parser, and extracts
// the title from the beginning of the body. Content is left empty.
// On truncated frontmatter or parse error, falls back to full Load.
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

	note := &Note{
		UUID:          fm.UUID,
		Parent:        fm.Parent,
		Order:         fm.Order,
		LinkedCards:   fm.LinkedCards,
		URL:           fm.URL,
		Tags:          fm.Tags,
		InlineTags:    fm.InlineTags,
		InheritedTags: fm.InheritedTags,
		Dates:         fm.Dates,
		FilePath:      path,
	}

	// Parse timestamps
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

	// Extract title from body portion of the buffer
	if bodyOffset < len(data) {
		note.Title = extractTitleFromBytes(data[bodyOffset:])
	}

	return note, nil
}

// extractTitleFromBytes finds the first markdown header in a byte slice.
func extractTitleFromBytes(data []byte) string {
	// Scan line by line looking for ^#{1,6}
	for len(data) > 0 {
		// Find end of line
		lineEnd := bytes.IndexByte(data, '\n')
		var line []byte
		if lineEnd == -1 {
			line = data
			data = nil
		} else {
			line = data[:lineEnd]
			data = data[lineEnd+1:]
		}

		// Strip \r
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		// Check for header: starts with # followed by more # or space
		if len(line) == 0 || line[0] != '#' {
			continue
		}

		// Count # characters (1-6)
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

		// Extract title text
		title := bytes.TrimSpace(line[hashes:])
		return string(title)
	}
	return ""
}
