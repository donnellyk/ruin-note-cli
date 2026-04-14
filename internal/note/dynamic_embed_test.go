package note

import (
	"testing"
)

func TestFindDynamicEmbeds_Search(t *testing.T) {
	content := `![[search: #daily @today]]`
	refs := FindDynamicEmbeds(content)
	if len(refs) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(refs))
	}
	if refs[0].Type != "search" {
		t.Errorf("type = %q, want search", refs[0].Type)
	}
	if refs[0].Query != "#daily @today" {
		t.Errorf("query = %q, want %q", refs[0].Query, "#daily @today")
	}
	if refs[0].Options != nil {
		t.Errorf("options = %v, want nil", refs[0].Options)
	}
}

func TestFindDynamicEmbeds_Pick(t *testing.T) {
	content := `![[pick: #followup]]`
	refs := FindDynamicEmbeds(content)
	if len(refs) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(refs))
	}
	if refs[0].Type != "pick" {
		t.Errorf("type = %q, want pick", refs[0].Type)
	}
	if refs[0].Query != "#followup" {
		t.Errorf("query = %q, want %q", refs[0].Query, "#followup")
	}
}

func TestFindDynamicEmbeds_Query(t *testing.T) {
	content := `![[query: weekly-review]]`
	refs := FindDynamicEmbeds(content)
	if len(refs) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(refs))
	}
	if refs[0].Type != "query" {
		t.Errorf("type = %q, want query", refs[0].Type)
	}
	if refs[0].Query != "weekly-review" {
		t.Errorf("query = %q, want %q", refs[0].Query, "weekly-review")
	}
}

func TestFindDynamicEmbeds_Compose(t *testing.T) {
	content := `![[compose: project-alpha]]`
	refs := FindDynamicEmbeds(content)
	if len(refs) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(refs))
	}
	if refs[0].Type != "compose" {
		t.Errorf("type = %q, want compose", refs[0].Type)
	}
	if refs[0].Query != "project-alpha" {
		t.Errorf("query = %q, want %q", refs[0].Query, "project-alpha")
	}
}

func TestFindDynamicEmbeds_WithOptions(t *testing.T) {
	content := `![[search: #daily | limit=5, sort=created:desc]]`
	refs := FindDynamicEmbeds(content)
	if len(refs) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(refs))
	}
	if refs[0].Type != "search" {
		t.Errorf("type = %q, want search", refs[0].Type)
	}
	if refs[0].Query != "#daily" {
		t.Errorf("query = %q, want %q", refs[0].Query, "#daily")
	}
	if refs[0].Options == nil {
		t.Fatal("expected non-nil options")
	}
	if refs[0].Options["limit"] != "5" {
		t.Errorf("options[limit] = %q, want %q", refs[0].Options["limit"], "5")
	}
	if refs[0].Options["sort"] != "created:desc" {
		t.Errorf("options[sort] = %q, want %q", refs[0].Options["sort"], "created:desc")
	}
}

func TestFindDynamicEmbeds_BooleanFlag(t *testing.T) {
	content := `![[pick: #todo | any, done]]`
	refs := FindDynamicEmbeds(content)
	if len(refs) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(refs))
	}
	if refs[0].Options == nil {
		t.Fatal("expected non-nil options")
	}
	if refs[0].Options["any"] != "true" {
		t.Errorf("options[any] = %q, want %q", refs[0].Options["any"], "true")
	}
	if refs[0].Options["done"] != "true" {
		t.Errorf("options[done] = %q, want %q", refs[0].Options["done"], "true")
	}
}

func TestFindDynamicEmbeds_UnknownType(t *testing.T) {
	content := `![[foo: bar]]`
	refs := FindDynamicEmbeds(content)
	if len(refs) != 0 {
		t.Errorf("expected 0 embeds for unknown type, got %d", len(refs))
	}
}

func TestFindDynamicEmbeds_StaticEmbed(t *testing.T) {
	content := `![[Note Title]]`
	refs := FindDynamicEmbeds(content)
	if len(refs) != 0 {
		t.Errorf("expected 0 embeds for static embed, got %d", len(refs))
	}
}

func TestFindDynamicEmbeds_InlineIgnored(t *testing.T) {
	content := `text ![[search: #tag]] text`
	refs := FindDynamicEmbeds(content)
	if len(refs) != 0 {
		t.Errorf("expected 0 embeds for inline occurrence, got %d", len(refs))
	}
}

func TestFindDynamicEmbeds_MultipleEmbeds(t *testing.T) {
	content := "![[search: #daily]]\nSome text\n![[pick: #followup]]\n![[query: weekly]]"
	refs := FindDynamicEmbeds(content)
	if len(refs) != 3 {
		t.Fatalf("expected 3 embeds, got %d", len(refs))
	}
	if refs[0].Type != "search" {
		t.Errorf("refs[0].Type = %q, want search", refs[0].Type)
	}
	if refs[0].Line != 0 {
		t.Errorf("refs[0].Line = %d, want 0", refs[0].Line)
	}
	if refs[1].Type != "pick" {
		t.Errorf("refs[1].Type = %q, want pick", refs[1].Type)
	}
	if refs[1].Line != 2 {
		t.Errorf("refs[1].Line = %d, want 2", refs[1].Line)
	}
	if refs[2].Type != "query" {
		t.Errorf("refs[2].Type = %q, want query", refs[2].Type)
	}
	if refs[2].Line != 3 {
		t.Errorf("refs[2].Line = %d, want 3", refs[2].Line)
	}
}

func TestFindDynamicEmbeds_MixedEmbeds(t *testing.T) {
	content := "![[search: #daily]]\n![[Note Title]]\nSome text"
	refs := FindDynamicEmbeds(content)
	if len(refs) != 1 {
		t.Fatalf("expected 1 dynamic embed, got %d", len(refs))
	}
	if refs[0].Type != "search" {
		t.Errorf("type = %q, want search", refs[0].Type)
	}
}

func TestParseDynamicOptions(t *testing.T) {
	opts := ParseDynamicOptions("limit=5, sort=created:desc, format=list")
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if opts["limit"] != "5" {
		t.Errorf("limit = %q, want %q", opts["limit"], "5")
	}
	if opts["sort"] != "created:desc" {
		t.Errorf("sort = %q, want %q", opts["sort"], "created:desc")
	}
	if opts["format"] != "list" {
		t.Errorf("format = %q, want %q", opts["format"], "list")
	}
}

func TestParseDynamicOptions_Empty(t *testing.T) {
	opts := ParseDynamicOptions("")
	if opts != nil {
		t.Errorf("expected nil for empty input, got %v", opts)
	}
}

func TestParseDynamicOptions_BoolFlags(t *testing.T) {
	opts := ParseDynamicOptions("any, done")
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if opts["any"] != "true" {
		t.Errorf("any = %q, want %q", opts["any"], "true")
	}
	if opts["done"] != "true" {
		t.Errorf("done = %q, want %q", opts["done"], "true")
	}
}
