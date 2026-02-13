package note

import (
	"reflect"
	"testing"
)

func TestExtractTags_SimpleTags(t *testing.T) {
	content := "This has #foo and #bar tags."
	tags := ExtractTags(content)

	expected := []string{"#foo", "#bar"}
	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("ExtractTags() = %v, want %v", tags, expected)
	}
}

func TestExtractTags_SpacedTags(t *testing.T) {
	content := "This has #daily note# tag."
	tags := ExtractTags(content)

	expected := []string{"#daily note#"}
	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("ExtractTags() = %v, want %v", tags, expected)
	}
}

func TestExtractTags_MixedTags(t *testing.T) {
	content := "Has #simple and #spaced tag# here."
	tags := ExtractTags(content)

	expected := []string{"#spaced tag#", "#simple"}
	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("ExtractTags() = %v, want %v", tags, expected)
	}
}

func TestExtractTags_TagWithSlash(t *testing.T) {
	content := "Date tag #2025/may here."
	tags := ExtractTags(content)

	expected := []string{"#2025/may"}
	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("ExtractTags() = %v, want %v", tags, expected)
	}
}

func TestExtractTags_Deduplication(t *testing.T) {
	content := "Has #foo twice: #foo"
	tags := ExtractTags(content)

	expected := []string{"#foo"}
	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("ExtractTags() = %v, want %v (should deduplicate)", tags, expected)
	}
}

func TestExtractTags_CaseInsensitiveDedup(t *testing.T) {
	content := "Has #Foo and #foo"
	tags := ExtractTags(content)

	// Should keep first occurrence
	if len(tags) != 1 {
		t.Errorf("ExtractTags() = %v, want 1 tag (case-insensitive dedup)", tags)
	}
}

func TestExtractTags_NoTags(t *testing.T) {
	content := "No tags here at all."
	tags := ExtractTags(content)

	if len(tags) != 0 {
		t.Errorf("ExtractTags() = %v, want empty", tags)
	}
}

func TestExtractTags_HashInCode(t *testing.T) {
	// This is a known limitation - we don't parse markdown code blocks
	// Tags in code will still be extracted
	content := "Some `#notag` here and #realtag"
	tags := ExtractTags(content)

	// Both will be extracted (limitation)
	if len(tags) < 1 {
		t.Errorf("ExtractTags() = %v, want at least #realtag", tags)
	}
}

func TestExtractTags_SpacedTagEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "spaced tag with extra whitespace",
			content: "#  daily note  #",
			want:    []string{"#daily note#"},
		},
		{
			name:    "broken spaced tag (has # inside)",
			content: "#broken # tag#",
			want:    []string{"#broken"},
		},
		{
			name:    "adjacent simple tags",
			content: "#foo#bar",
			want:    []string{"#foo", "#bar"},
		},
		{
			name:    "simple tag not matched as spaced",
			content: "#simple#",
			want:    []string{"#simple"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTags(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractTags_MarkdownLinks(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "tag-like text inside link text and URL",
			content: `Related: [Issue #53: Streaming support](https://github.com/acme/platform/issues/53) #done`,
			want:    []string{"#done"},
		},
		{
			name:    "tag inside link text only",
			content: `See [#foo docs](https://example.com) for details`,
			want:    nil,
		},
		{
			name:    "tag inside URL only",
			content: `See [docs](https://example.com/#section) #ref`,
			want:    []string{"#ref"},
		},
		{
			name:    "multiple links with real tags",
			content: `[#1](url1) and [#2](url2) #real`,
			want:    []string{"#real"},
		},
		{
			name:    "link with no tag-like content",
			content: `See [docs](https://example.com) #tag`,
			want:    []string{"#tag"},
		},
		{
			name:    "bare hash in link text",
			content: `Check [issue #42](https://github.com/org/repo/issues/42)`,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTags(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyTags(t *testing.T) {
	content := `# My Note
#global1 #global2

This is content with #inline tag.

More content here.

#endtag`

	globalTags, inlineTags := ClassifyTags(content, "My Note")

	// Global tags: those right after title and at the end
	expectedGlobal := []string{"#global1", "#global2", "#endtag"}
	if !reflect.DeepEqual(globalTags, expectedGlobal) {
		t.Errorf("globalTags = %v, want %v", globalTags, expectedGlobal)
	}

	// Inline tags: within content
	expectedInline := []string{"#inline"}
	if !reflect.DeepEqual(inlineTags, expectedInline) {
		t.Errorf("inlineTags = %v, want %v", inlineTags, expectedInline)
	}
}

func TestClassifyTags_AllGlobal(t *testing.T) {
	content := `# Title
#tag1 #tag2`

	globalTags, inlineTags := ClassifyTags(content, "Title")

	if len(globalTags) != 2 {
		t.Errorf("globalTags = %v, want 2 tags", globalTags)
	}

	if len(inlineTags) != 0 {
		t.Errorf("inlineTags = %v, want empty", inlineTags)
	}
}

func TestClassifyTags_AllInline(t *testing.T) {
	content := `# Title

This paragraph has #tag1 and #tag2 inline.`

	globalTags, inlineTags := ClassifyTags(content, "Title")

	if len(globalTags) != 0 {
		t.Errorf("globalTags = %v, want empty", globalTags)
	}

	if len(inlineTags) != 2 {
		t.Errorf("inlineTags = %v, want 2 tags", inlineTags)
	}
}

func TestClassifyTags_CommaSeparatedGlobal(t *testing.T) {
	content := `# Ruin Log

Some content with #inline here.

#log, #ruin`

	globalTags, inlineTags := ClassifyTags(content, "Ruin Log")

	expectedGlobal := []string{"#log", "#ruin"}
	if !reflect.DeepEqual(globalTags, expectedGlobal) {
		t.Errorf("globalTags = %v, want %v", globalTags, expectedGlobal)
	}

	expectedInline := []string{"#inline"}
	if !reflect.DeepEqual(inlineTags, expectedInline) {
		t.Errorf("inlineTags = %v, want %v", inlineTags, expectedInline)
	}
}

func TestClassifyTags_MidContentTagOnlyLine(t *testing.T) {
	content := `# Note
#top

Content paragraph. #inline

#middle

More content.

#bottom`

	globalTags, inlineTags := ClassifyTags(content, "Note")

	expectedGlobal := []string{"#top", "#middle", "#bottom"}
	if !reflect.DeepEqual(globalTags, expectedGlobal) {
		t.Errorf("globalTags = %v, want %v", globalTags, expectedGlobal)
	}

	expectedInline := []string{"#inline"}
	if !reflect.DeepEqual(inlineTags, expectedInline) {
		t.Errorf("inlineTags = %v, want %v", inlineTags, expectedInline)
	}
}

func TestMergeTags(t *testing.T) {
	global := []string{"#a", "#b"}
	inline := []string{"#b", "#c"}

	merged := MergeTags(global, inline)

	expected := []string{"#a", "#b", "#c"}
	if !reflect.DeepEqual(merged, expected) {
		t.Errorf("MergeTags() = %v, want %v", merged, expected)
	}
}

func TestIsTagOnlyLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"#foo #bar", true},
		{"#foo", true},
		{"  #foo  ", true},
		{"#foo, #bar", true},
		{"#foo; #bar", true},
		{"#foo | #bar", true},
		{"#foo, #bar, #baz", true},
		{"Some text #foo", false},
		{"#foo some text", false},
		{"[Issue #53](https://github.com/issues/53)", false},
		{"", false},
		{"   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := IsTagOnlyLine(tt.line); got != tt.want {
				t.Errorf("IsTagOnlyLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}
