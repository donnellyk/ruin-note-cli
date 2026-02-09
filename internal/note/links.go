package note

import (
	"regexp"
	"strings"
)

// wikiLinkPattern matches [[title]] and [[title|display text]]
var wikiLinkPattern = regexp.MustCompile(`\[\[([^\[\]|]+?)(?:\|([^\[\]]*?))?\]\]`)

// WikiLink represents a parsed wiki-style link.
type WikiLink struct {
	Title   string // Target title (left of |)
	Display string // Display text (right of |, or empty)
}

// ExtractWikiLinks finds all wiki-style links in the given content.
// Returns deduplicated links by lowercase title, keeping first occurrence.
func ExtractWikiLinks(content string) []WikiLink {
	matches := wikiLinkPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var result []WikiLink

	for _, m := range matches {
		title := strings.TrimSpace(m[1])
		if title == "" {
			continue
		}

		titleLower := strings.ToLower(title)
		if seen[titleLower] {
			continue
		}
		seen[titleLower] = true

		link := WikiLink{Title: title}
		if len(m) > 2 {
			link.Display = strings.TrimSpace(m[2])
		}
		result = append(result, link)
	}

	return result
}

// ExtractWikiLinkTitles returns unique titles from wiki links in content.
// Deduplicated case-insensitively, keeping first occurrence's casing.
func ExtractWikiLinkTitles(content string) []string {
	links := ExtractWikiLinks(content)
	if len(links) == 0 {
		return nil
	}

	titles := make([]string, len(links))
	for i, link := range links {
		titles[i] = link.Title
	}
	return titles
}
