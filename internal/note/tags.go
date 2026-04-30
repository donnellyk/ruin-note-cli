package note

import "github.com/donnellyk/ruin-note-cli/pkg/notetext"

type TagMatch = notetext.TagMatch

func ExtractTags(content string) []string {
	return notetext.ExtractTags(content)
}

func ExtractTagMatches(content string) []TagMatch {
	return notetext.ExtractTagMatches(content)
}

func ClassifyTags(content string, title string) (globalTags []string, inlineTags []string) {
	return notetext.ClassifyTags(content, title)
}

func IsHeaderLine(line string) bool {
	return notetext.IsHeaderLine(line)
}

func IsTagOnlyLine(line string) bool {
	return notetext.IsTagOnlyLine(line)
}

func NormalizeStored(tag string) string {
	return notetext.NormalizeStored(tag)
}

func BodyForm(tag string) string {
	return notetext.BodyForm(tag)
}

func MergeTags(globalTags, inlineTags []string) []string {
	return notetext.MergeTags(globalTags, inlineTags)
}

func StripInheritedTagsFromContent(content string, inheritedTags []string) string {
	return notetext.StripInheritedTagsFromContent(content, inheritedTags)
}
