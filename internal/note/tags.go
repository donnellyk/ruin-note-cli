package note

import "github.com/donnellyk/ruin-note-cli/pkg/notetext"

// TagMatch is an alias for notetext.TagMatch so existing callers compile unchanged.
type TagMatch = notetext.TagMatch

// ExtractTags finds all tags in the given content.
// Returns deduplicated tags in order of first occurrence.
func ExtractTags(content string) []string {
	return notetext.ExtractTags(content)
}

// ExtractTagMatches finds all tags with their positions.
func ExtractTagMatches(content string) []TagMatch {
	return notetext.ExtractTagMatches(content)
}

// ClassifyTags separates tags into global tags and inline tags.
func ClassifyTags(content string, title string) (globalTags []string, inlineTags []string) {
	return notetext.ClassifyTags(content, title)
}

// IsHeaderLine returns true if the line is a markdown header (H1 through H6).
func IsHeaderLine(line string) bool {
	return notetext.IsHeaderLine(line)
}

// IsTagOnlyLine returns true if the line contains only tags and separator characters.
func IsTagOnlyLine(line string) bool {
	return notetext.IsTagOnlyLine(line)
}

// NormalizeTag normalizes a tag for comparison (lowercase).
func NormalizeTag(tag string) string {
	return notetext.NormalizeTag(tag)
}

// MergeTags combines global and inline tags, with global tags first.
func MergeTags(globalTags, inlineTags []string) []string {
	return notetext.MergeTags(globalTags, inlineTags)
}

// StripInheritedTagsFromContent removes inherited tags from tag-only lines in content.
func StripInheritedTagsFromContent(content string, inheritedTags []string) string {
	return notetext.StripInheritedTagsFromContent(content, inheritedTags)
}
