package commands

import (
	"testing"
	"time"

	"github.com/donnellyk/ruin-note-cli/internal/note"
)

func TestNegateTag(t *testing.T) {
	matcher, _, err := parseQuery("!#done", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	withDone := &note.Note{Tags: []string{"#done", "#work"}}
	withoutDone := &note.Note{Tags: []string{"#work"}}

	if matcher(withDone) {
		t.Error("expected !#done to NOT match note with #done tag")
	}
	if !matcher(withoutDone) {
		t.Error("expected !#done to match note without #done tag")
	}
}

func TestNegateText(t *testing.T) {
	matcher, _, err := parseQuery("!meeting", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	withMeeting := &note.Note{
		Title:   "Meeting Notes",
		Content: "# Meeting Notes\nDiscussed the roadmap.",
	}
	withoutMeeting := &note.Note{
		Title:   "Code Review",
		Content: "# Code Review\nFixed the bug.",
	}

	if matcher(withMeeting) {
		t.Error("expected !meeting to NOT match note with 'meeting' in content")
	}
	if !matcher(withoutMeeting) {
		t.Error("expected !meeting to match note without 'meeting'")
	}
}

func TestNegateDate(t *testing.T) {
	matcher, _, err := parseQuery("!created:2026-04-14", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	onDate := &note.Note{
		Created: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
	}
	offDate := &note.Note{
		Created: time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC),
	}

	if matcher(onDate) {
		t.Error("expected !created:2026-04-14 to NOT match note created on 2026-04-14")
	}
	if !matcher(offDate) {
		t.Error("expected !created:2026-04-14 to match note created on 2026-04-13")
	}
}

func TestNegateTitle(t *testing.T) {
	matcher, _, err := parseQuery("!title:draft", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	draftNote := &note.Note{Title: "draft notes", Content: "# draft notes\n"}
	finalNote := &note.Note{Title: "final notes", Content: "# final notes\n"}

	if matcher(draftNote) {
		t.Error("expected !title:draft to NOT match note titled 'draft notes'")
	}
	if !matcher(finalNote) {
		t.Error("expected !title:draft to match note titled 'final notes'")
	}
}

func TestNegatePath(t *testing.T) {
	matcher, _, err := parseQuery("!path:archive/", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	archived := &note.Note{FilePath: "archive/old.md"}
	active := &note.Note{FilePath: "notes/new.md"}

	if matcher(archived) {
		t.Error("expected !path:archive/ to NOT match note at archive/old.md")
	}
	if !matcher(active) {
		t.Error("expected !path:archive/ to match note at notes/new.md")
	}
}

func TestNegateParent(t *testing.T) {
	matcher, _, err := parseQuery("!parent:none", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	noParent := &note.Note{Parent: ""}
	hasParent := &note.Note{Parent: "some-uuid"}

	if matcher(noParent) {
		t.Error("expected !parent:none to NOT match note with empty parent")
	}
	if !matcher(hasParent) {
		t.Error("expected !parent:none to match note with parent UUID")
	}
}

func TestNegateParentUUID(t *testing.T) {
	matcher, _, err := parseQuery("!parent:abc-123", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	matchingParent := &note.Note{Parent: "abc-123"}
	differentParent := &note.Note{Parent: "xyz-789"}

	if matcher(matchingParent) {
		t.Error("expected !parent:abc-123 to NOT match note with parent abc-123")
	}
	if !matcher(differentParent) {
		t.Error("expected !parent:abc-123 to match note with different parent")
	}
}

func TestTagsNone(t *testing.T) {
	matcher, _, err := parseQuery("tags:none", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	untagged := &note.Note{}
	withGlobal := &note.Note{Tags: []string{"#work"}}
	withInline := &note.Note{InlineTags: []string{"#followup"}}
	withBoth := &note.Note{Tags: []string{"#work"}, InlineTags: []string{"#followup"}}
	withInherited := &note.Note{InheritedTags: []string{"#project"}}

	if !matcher(untagged) {
		t.Error("expected tags:none to match note with no tags")
	}
	if matcher(withGlobal) {
		t.Error("expected tags:none to NOT match note with global tags")
	}
	if matcher(withInline) {
		t.Error("expected tags:none to NOT match note with inline tags")
	}
	if matcher(withBoth) {
		t.Error("expected tags:none to NOT match note with both tag types")
	}
	if matcher(withInherited) {
		t.Error("expected tags:none to NOT match note with inherited tags (mirrors tagMatcher scope)")
	}
}

func TestTagsNoneGlobalScope(t *testing.T) {
	matcher, _, err := parseQuery("tags:none", TagScopeGlobal)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	noGlobalOnlyInline := &note.Note{InlineTags: []string{"#followup"}}
	withGlobal := &note.Note{Tags: []string{"#work"}}

	if !matcher(noGlobalOnlyInline) {
		t.Error("expected tags:none (global scope) to match note with only inline tags")
	}
	if matcher(withGlobal) {
		t.Error("expected tags:none (global scope) to NOT match note with global tags")
	}
}

func TestTagsNoneInlineScope(t *testing.T) {
	matcher, _, err := parseQuery("tags:none", TagScopeInline)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	noInlineOnlyGlobal := &note.Note{Tags: []string{"#work"}}
	withInline := &note.Note{InlineTags: []string{"#followup"}}

	if !matcher(noInlineOnlyGlobal) {
		t.Error("expected tags:none (inline scope) to match note with only global tags")
	}
	if matcher(withInline) {
		t.Error("expected tags:none (inline scope) to NOT match note with inline tags")
	}
}

func TestTagsUnknownValue(t *testing.T) {
	_, _, err := parseQuery("tags:work", TagScopeAll)
	if err == nil {
		t.Error("expected error for tags:work (only tags:none is supported)")
	}
}

func TestNegateTagsNone(t *testing.T) {
	matcher, _, err := parseQuery("!tags:none", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	untagged := &note.Note{}
	tagged := &note.Note{Tags: []string{"#work"}}

	if matcher(untagged) {
		t.Error("expected !tags:none to NOT match untagged note")
	}
	if !matcher(tagged) {
		t.Error("expected !tags:none to match tagged note")
	}
}

func TestNegateCombined(t *testing.T) {
	matcher, _, err := parseQuery("#project !#archived", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	projectOnly := &note.Note{Tags: []string{"#project"}}
	projectAndArchived := &note.Note{Tags: []string{"#project", "#archived"}}
	archivedOnly := &note.Note{Tags: []string{"#archived"}}

	if !matcher(projectOnly) {
		t.Error("expected '#project !#archived' to match note with #project only")
	}
	if matcher(projectAndArchived) {
		t.Error("expected '#project !#archived' to NOT match note with both tags")
	}
	if matcher(archivedOnly) {
		t.Error("expected '#project !#archived' to NOT match note with only #archived")
	}
}

func TestNegateMultiple(t *testing.T) {
	matcher, _, err := parseQuery("!#archived !#draft", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	withArchived := &note.Note{Tags: []string{"#archived", "#work"}}
	withDraft := &note.Note{Tags: []string{"#draft", "#work"}}
	withNeither := &note.Note{Tags: []string{"#work", "#project"}}
	withBoth := &note.Note{Tags: []string{"#archived", "#draft"}}

	if matcher(withArchived) {
		t.Error("expected '!#archived !#draft' to NOT match note with #archived")
	}
	if matcher(withDraft) {
		t.Error("expected '!#archived !#draft' to NOT match note with #draft")
	}
	if !matcher(withNeither) {
		t.Error("expected '!#archived !#draft' to match note with neither tag")
	}
	if matcher(withBoth) {
		t.Error("expected '!#archived !#draft' to NOT match note with both tags")
	}
}

func TestDoubleNegate(t *testing.T) {
	// Double negation via parseTermMatcher directly (splitTerms splits "!!#tag"
	// into "!!" and "#tag", so parseQuery can't handle it, but parseTermMatcher can).
	matcher, _, err := parseTermMatcher("!!#tag", TagScopeAll)
	if err != nil {
		t.Fatalf("parseTermMatcher error: %v", err)
	}

	withTag := &note.Note{Tags: []string{"#tag"}}
	withoutTag := &note.Note{Tags: []string{"#other"}}

	if !matcher(withTag) {
		t.Error("expected !!#tag (double negate) to match note with #tag")
	}
	if matcher(withoutTag) {
		t.Error("expected !!#tag (double negate) to NOT match note without #tag")
	}

	// Verify parseQuery errors on "!!#tag" since splitTerms separates it
	_, _, err = parseQuery("!!#tag", TagScopeAll)
	if err == nil {
		t.Error("expected parseQuery to error on '!!#tag' (splitTerms separates !! from #tag)")
	}
}

func TestNegateOnlyQuery(t *testing.T) {
	matcher, _, err := parseQuery("!#archived", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	withArchived := &note.Note{Tags: []string{"#archived"}}
	withoutArchived := &note.Note{Tags: []string{"#work"}}

	if matcher(withArchived) {
		t.Error("expected !#archived to NOT match note with #archived")
	}
	if !matcher(withoutArchived) {
		t.Error("expected !#archived to match note without #archived")
	}
}

func TestNegateEmptyTerm(t *testing.T) {
	_, _, err := parseQuery("!", TagScopeAll)
	if err == nil {
		t.Error("expected error for bare '!' query")
	}
}

func TestNegateWithScope(t *testing.T) {
	matcher, _, err := parseQuery("!#done", TagScopeGlobal)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	// #done as global tag
	globalDone := &note.Note{
		Tags:       []string{"#done"},
		InlineTags: []string{},
	}
	// #done as inline tag only
	inlineDone := &note.Note{
		Tags:       []string{"#work"},
		InlineTags: []string{"#done"},
	}

	if matcher(globalDone) {
		t.Error("expected !#done (global scope) to NOT match note where #done is a global tag")
	}
	if !matcher(inlineDone) {
		t.Error("expected !#done (global scope) to match note where #done is only an inline tag")
	}
}

func TestNegateWithText(t *testing.T) {
	matcher, _, err := parseQuery("text !#done", TagScopeAll)
	if err != nil {
		t.Fatalf("parseQuery error: %v", err)
	}

	matchingNote := &note.Note{
		Title:   "Some Note",
		Content: "# Some Note\nSome text here.",
		Tags:    []string{"#work"},
	}
	doneNote := &note.Note{
		Title:   "Done Note",
		Content: "# Done Note\nSome text here.",
		Tags:    []string{"#done"},
	}
	noTextNote := &note.Note{
		Title:   "Other",
		Content: "# Other\nNo matching word.",
		Tags:    []string{"#work"},
	}

	if !matcher(matchingNote) {
		t.Error("expected 'text !#done' to match note with 'text' and without #done")
	}
	if matcher(doneNote) {
		t.Error("expected 'text !#done' to NOT match note with #done tag")
	}
	if matcher(noTextNote) {
		t.Error("expected 'text !#done' to NOT match note without 'text' in content")
	}
}

func TestNegateNeedsBody(t *testing.T) {
	_, info, err := parseTermMatcher("!meeting", TagScopeAll)
	if err != nil {
		t.Fatalf("parseTermMatcher error: %v", err)
	}
	if !info.NeedsBody {
		t.Error("expected NeedsBody=true for negated text matcher")
	}

	// Tag-based negation should not need body
	_, tagInfo, err := parseTermMatcher("!#done", TagScopeAll)
	if err != nil {
		t.Fatalf("parseTermMatcher error: %v", err)
	}
	if tagInfo.NeedsBody {
		t.Error("expected NeedsBody=false for negated tag matcher")
	}
}

func TestSplitTermsNegation(t *testing.T) {
	t.Run("negated tag", func(t *testing.T) {
		got := splitTerms("!#tag rest")
		want := []string{"!#tag", "rest"}
		if len(got) != len(want) {
			t.Fatalf("splitTerms() = %v, want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("splitTerms()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("negated spaced tag", func(t *testing.T) {
		got := splitTerms("!#spaced tag# rest")
		want := []string{"!#spaced tag#", "rest"}
		if len(got) != len(want) {
			t.Fatalf("splitTerms() = %v, want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("splitTerms()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}
