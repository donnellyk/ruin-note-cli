package note

import (
	"net/url"
	"strings"
)

func IsValidURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func (n *Note) IsURLNote() bool {
	if n.URL != "" {
		return true
	}
	return n.ExtractURL() != ""
}

// ExtractURL returns the URL from frontmatter or the first bare URL line in the body.
func (n *Note) ExtractURL() string {
	if n.URL != "" {
		return n.URL
	}
	lines := strings.SplitSeq(n.Content, "\n")
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if IsHeaderLine(trimmed) {
			continue
		}
		if IsTagOnlyLine(trimmed) {
			continue
		}
		if IsValidURL(trimmed) {
			return trimmed
		}
		break // only the first real content line is considered; stop scanning
	}
	return ""
}

// EnsureLinkTag promotes body URLs to frontmatter and adds #link tag if missing.
// Returns true if the note was modified.
func (n *Note) EnsureLinkTag() bool {
	if !n.IsURLNote() {
		return false
	}

	modified := false

	if n.URL == "" {
		extracted := n.ExtractURL()
		if extracted != "" {
			n.URL = extracted
			modified = true
		}
	}

	const linkStored = "link"
	linkBody := BodyForm(linkStored)

	hasLink := false
	for _, t := range n.Tags {
		if NormalizeStored(t) == linkStored {
			hasLink = true
			break
		}
	}
	if !hasLink {
		for _, t := range n.InlineTags {
			if NormalizeStored(t) == linkStored {
				hasLink = true
				break
			}
		}
	}
	if !hasLink {
		for _, t := range ExtractTags(n.Content) {
			if NormalizeStored(t) == linkStored {
				hasLink = true
				break
			}
		}
	}

	if hasLink {
		return modified
	}

	lines := strings.Split(n.Content, "\n")
	inserted := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if IsTagOnlyLine(trimmed) {
			sep := " "
			if strings.Contains(trimmed, ",") {
				sep = ", "
			}
			lines[i] = line + sep + linkBody
			inserted = true
			break
		}
	}

	if !inserted {
		insertIdx := -1
		for i, line := range lines {
			if IsHeaderLine(strings.TrimSpace(line)) {
				insertIdx = i
				break
			}
		}
		if insertIdx >= 0 {
			newLines := make([]string, 0, len(lines)+2)
			newLines = append(newLines, lines[:insertIdx+1]...)
			newLines = append(newLines, "", linkBody)
			newLines = append(newLines, lines[insertIdx+1:]...)
			lines = newLines
		} else {
			lines = append(lines, "", linkBody)
		}
	}

	n.Content = strings.Join(lines, "\n")
	return true
}
