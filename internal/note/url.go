package note

import (
	"net/url"
	"strings"
)

// IsValidURL returns true if s is a valid HTTP or HTTPS URL.
func IsValidURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// IsURLNote returns true if the note has a URL (in frontmatter or body).
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
	// Check first non-empty, non-title, non-tag-only line
	lines := strings.Split(n.Content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip title lines (headers)
		if IsHeaderLine(trimmed) {
			continue
		}
		// Skip tag-only lines
		if IsTagOnlyLine(trimmed) {
			continue
		}
		// Check if this line is a bare URL
		if IsValidURL(trimmed) {
			return trimmed
		}
		break // first content line is not a URL
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

	// Promote body URL to frontmatter if not already set
	if n.URL == "" {
		extracted := n.ExtractURL()
		if extracted != "" {
			n.URL = extracted
			modified = true
		}
	}

	// Check if #link tag already present in tags or inline-tags
	hasLink := false
	for _, t := range n.Tags {
		if NormalizeTag(t) == "#link" {
			hasLink = true
			break
		}
	}
	if !hasLink {
		for _, t := range n.InlineTags {
			if NormalizeTag(t) == "#link" {
				hasLink = true
				break
			}
		}
	}
	// Also check content directly for #link
	if !hasLink {
		for _, t := range ExtractTags(n.Content) {
			if NormalizeTag(t) == "#link" {
				hasLink = true
				break
			}
		}
	}

	if hasLink {
		return modified
	}

	// Need to add #link tag
	// Find first tag-only line and append
	lines := strings.Split(n.Content, "\n")
	inserted := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if IsTagOnlyLine(trimmed) {
			// Determine separator: if line uses commas, use ", ", else use " "
			sep := " "
			if strings.Contains(trimmed, ",") {
				sep = ", "
			}
			lines[i] = line + sep + "#link"
			inserted = true
			break
		}
	}

	if !inserted {
		// Insert after title header or at end
		insertIdx := -1
		for i, line := range lines {
			if IsHeaderLine(strings.TrimSpace(line)) {
				insertIdx = i
				break
			}
		}
		if insertIdx >= 0 {
			// Insert after the title line
			newLines := make([]string, 0, len(lines)+2)
			newLines = append(newLines, lines[:insertIdx+1]...)
			newLines = append(newLines, "", "#link")
			newLines = append(newLines, lines[insertIdx+1:]...)
			lines = newLines
		} else {
			// Append at end
			lines = append(lines, "", "#link")
		}
	}

	n.Content = strings.Join(lines, "\n")
	return true
}
