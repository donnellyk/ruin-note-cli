package note

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

var BulkSeparatorPattern = regexp.MustCompile(`%%%% ([a-zA-Z0-9-]+) %%%%\n?`)

type BulkEntry struct {
	UUID    string
	Content string
}

func FormatBulk(entries []BulkEntry, w io.Writer) error {
	for i, entry := range entries {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%%%%%%%% %s %%%%%%%%\n", entry.UUID)
		fmt.Fprint(w, entry.Content)
		if !strings.HasSuffix(entry.Content, "\n") {
			fmt.Fprintln(w)
		}
	}
	return nil
}

func ParseBulk(content string) map[string]string {
	result := make(map[string]string)

	matches := BulkSeparatorPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return result
	}

	for i, match := range matches {
		uuid := content[match[2]:match[3]]
		contentStart := match[1]

		var contentEnd int
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		} else {
			contentEnd = len(content)
		}

		noteContent := content[contentStart:contentEnd]
		noteContent = strings.TrimSuffix(noteContent, "\n")

		result[uuid] = noteContent
	}

	return result
}

func ParseBulkOrdered(content string) []BulkEntry {
	var entries []BulkEntry

	matches := BulkSeparatorPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return entries
	}

	for i, match := range matches {
		uuid := content[match[2]:match[3]]
		contentStart := match[1]

		var contentEnd int
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		} else {
			contentEnd = len(content)
		}

		noteContent := content[contentStart:contentEnd]
		noteContent = strings.TrimSuffix(noteContent, "\n")

		entries = append(entries, BulkEntry{
			UUID:    uuid,
			Content: noteContent,
		})
	}

	return entries
}
