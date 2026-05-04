package commands

import (
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/note"
)

func TestTextMatcher_MatchesContent(t *testing.T) {
	matcher := textMatcher("foo")
	n := &note.Note{
		Content: "This contains foo in the content",
	}

	if !matcher(n) {
		t.Error("textMatcher should match content")
	}
}

func TestTextMatcher_CaseInsensitive(t *testing.T) {
	matcher := textMatcher("FOO")
	n := &note.Note{
		Content: "This contains foo in lowercase",
	}

	if !matcher(n) {
		t.Error("textMatcher should be case-insensitive")
	}
}

func TestTextMatcher_NoMatch(t *testing.T) {
	matcher := textMatcher("xyz")
	n := &note.Note{
		Content: "This does not contain the search term",
	}

	if matcher(n) {
		t.Error("textMatcher should not match when term is absent")
	}
}

func TestTextMatcher_MatchesAlias(t *testing.T) {
	matcher := textMatcher("old")
	n := &note.Note{
		Title:   "Current Name",
		Aliases: []string{"Old Name", "Alternative"},
		Content: "Some content",
	}

	if !matcher(n) {
		t.Error("textMatcher should match aliases")
	}
}

func TestTextMatcher_AliasCaseInsensitive(t *testing.T) {
	matcher := textMatcher("OLD")
	n := &note.Note{
		Title:   "Current Name",
		Aliases: []string{"Old Name"},
		Content: "Some content",
	}

	if !matcher(n) {
		t.Error("textMatcher should match aliases case-insensitively")
	}
}

func TestTitleMatcher_MatchesTitle(t *testing.T) {
	matcher := titleMatcher("sprint")
	n := &note.Note{
		Title: "Sprint Planning",
	}

	if !matcher(n) {
		t.Error("titleMatcher should match title")
	}
}

func TestTitleMatcher_TitleCaseInsensitive(t *testing.T) {
	matcher := titleMatcher("SPRINT")
	n := &note.Note{
		Title: "sprint planning",
	}

	if !matcher(n) {
		t.Error("titleMatcher should be case-insensitive for title")
	}
}

func TestTitleMatcher_MatchesAlias(t *testing.T) {
	matcher := titleMatcher("alternative")
	n := &note.Note{
		Title:   "Sprint Planning",
		Aliases: []string{"Alternative Name", "Old Title"},
	}

	if !matcher(n) {
		t.Error("titleMatcher should match aliases when title doesn't match")
	}
}

func TestTitleMatcher_AliasCaseInsensitive(t *testing.T) {
	matcher := titleMatcher("ALTERNATIVE")
	n := &note.Note{
		Title:   "Sprint Planning",
		Aliases: []string{"Alternative Name"},
	}

	if !matcher(n) {
		t.Error("titleMatcher should match aliases case-insensitively")
	}
}

func TestTitleMatcher_NoMatch(t *testing.T) {
	matcher := titleMatcher("xyz")
	n := &note.Note{
		Title:   "Sprint Planning",
		Aliases: []string{"Alternative Name"},
	}

	if matcher(n) {
		t.Error("titleMatcher should not match when neither title nor aliases match")
	}
}

func TestTitleMatcher_TitleTakesPrecedence(t *testing.T) {
	matcher := titleMatcher("planning")
	n := &note.Note{
		Title:   "Sprint Planning",
		Aliases: []string{"Something Else"},
	}

	if !matcher(n) {
		t.Error("titleMatcher should match title")
	}
}
