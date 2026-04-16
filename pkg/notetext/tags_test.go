package notetext

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

func TestExtractTags_KebabCase(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "single kebab tag",
			content: "marked as #done-later",
			want:    []string{"#done-later"},
		},
		{
			name:    "follow-up style",
			content: "needs #follow-up soon",
			want:    []string{"#follow-up"},
		},
		{
			name:    "multi-dash tag",
			content: "#kebab-case-tag here",
			want:    []string{"#kebab-case-tag"},
		},
		{
			name:    "mixed kebab and simple",
			content: "#in-progress #followup",
			want:    []string{"#in-progress", "#followup"},
		},
		{
			name:    "trailing dash stripped",
			content: "follow up #done- later",
			want:    []string{"#done"},
		},
		{
			name:    "leading dash does not match",
			content: "#-leading is not a tag",
			want:    nil,
		},
		{
			name:    "slash combined with dash",
			content: "tagged #date/2026-q2 today",
			want:    []string{"#date/2026-q2"},
		},
		{
			name:    "dash between adjacent tags",
			content: "#foo-#bar",
			want:    []string{"#foo", "#bar"},
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

func TestStripInheritedTagsFromContent(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		inheritedTags []string
		want          string
	}{
		{
			name:          "all tags removed from line",
			content:       "# Title\n#project\n\nSome content.",
			inheritedTags: []string{"#project"},
			want:          "# Title\n\nSome content.",
		},
		{
			name:          "partial removal with formatting",
			content:       "# Title\n#project, #local\n\nSome content.",
			inheritedTags: []string{"#project"},
			want:          "# Title\n#local\n\nSome content.",
		},
		{
			name:          "no match leaves content unchanged",
			content:       "# Title\n#local\n\nSome content.",
			inheritedTags: []string{"#project"},
			want:          "# Title\n#local\n\nSome content.",
		},
		{
			name:          "inline tags not affected",
			content:       "# Title\nSome content with #project tag.",
			inheritedTags: []string{"#project"},
			want:          "# Title\nSome content with #project tag.",
		},
		{
			name:          "empty inherited tags",
			content:       "# Title\n#project\n\nContent.",
			inheritedTags: nil,
			want:          "# Title\n#project\n\nContent.",
		},
		{
			name:          "multiple lines stripped",
			content:       "# Title\n#a\n#b\n\nContent.",
			inheritedTags: []string{"#a", "#b"},
			want:          "# Title\n\nContent.",
		},
		{
			name:          "case insensitive match",
			content:       "# Title\n#Project\n\nContent.",
			inheritedTags: []string{"#project"},
			want:          "# Title\n\nContent.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripInheritedTagsFromContent(tt.content, tt.inheritedTags)
			if got != tt.want {
				t.Errorf("StripInheritedTagsFromContent() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestExtractTags_Embeds(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "tags inside static embed ignored",
			content: "![[Note Title]]\n#real",
			want:    []string{"#real"},
		},
		{
			name:    "tags inside dynamic search embed ignored",
			content: "![[search: #daily @today | limit=5]]\n#actual",
			want:    []string{"#actual"},
		},
		{
			name:    "tags inside dynamic pick embed ignored",
			content: "![[pick: #followup !#done]]",
			want:    nil,
		},
		{
			name:    "tags inside dynamic query embed ignored",
			content: "text #before\n![[query: weekly-review]]\n#after",
			want:    []string{"#before", "#after"},
		},
		{
			name:    "tags inside compose embed ignored",
			content: "![[compose: project-alpha]]",
			want:    nil,
		},
		{
			name:    "tag outside embed is kept",
			content: "Some text #tag\n![[search: #daily]]\nMore #other",
			want:    []string{"#tag", "#other"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTags(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractTags() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if NormalizeTag(got[i]) != NormalizeTag(tt.want[i]) {
					t.Errorf("ExtractTags()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestClassifyTags_Embeds(t *testing.T) {
	content := "# Title\n#global\n\nSome text #inline\n![[pick: #followup !#done]]\n#another-global"
	global, inline := ClassifyTags(content, "Title")

	// #followup and #done inside the embed should not appear
	for _, tag := range append(global, inline...) {
		norm := NormalizeTag(tag)
		if norm == "#followup" || norm == "#done" {
			t.Errorf("tag %q inside embed should not be classified, got global=%v inline=%v", tag, global, inline)
		}
	}

	// #global and #another-global should be global, #inline should be inline
	hasGlobal := false
	hasAnotherGlobal := false
	for _, tag := range global {
		switch NormalizeTag(tag) {
		case "#global":
			hasGlobal = true
		case "#another-global":
			hasAnotherGlobal = true
		}
	}
	if !hasGlobal {
		t.Errorf("expected #global in global tags, got %v", global)
	}
	if !hasAnotherGlobal {
		t.Errorf("expected #another-global in global tags, got %v", global)
	}

	hasInline := false
	for _, tag := range inline {
		if NormalizeTag(tag) == "#inline" {
			hasInline = true
		}
	}
	if !hasInline {
		t.Errorf("expected #inline in inline tags, got %v", inline)
	}
}

func TestNormalizeTag(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"#Foo", "#foo"},
		{"#BAR", "#bar"},
		{"#already", "#already"},
		{"#Mixed Case#", "#mixed case#"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := NormalizeTag(tt.input); got != tt.want {
				t.Errorf("NormalizeTag(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsHeaderLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"# Title", true},
		{"## Section", true},
		{"### Subsection", true},
		{"#tag", false},
		{"###### H6", true},
		{"####### Too many", false},
		{"Not a header", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := IsHeaderLine(tt.line); got != tt.want {
				t.Errorf("IsHeaderLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestExtractTags_CodeBlocks(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "inline code excluded",
			content: "about `#done` in backticks",
			want:    nil,
		},
		{
			name:    "real tag outside inline code",
			content: "about `#done` and #real",
			want:    []string{"#real"},
		},
		{
			name:    "fenced code block excluded",
			content: "before\n```\n#done in fenced\n```\n#real outside",
			want:    []string{"#real"},
		},
		{
			name:    "fenced with language",
			content: "```go\nfmt.Println(\"#notag\")\n```\n#actual",
			want:    []string{"#actual"},
		},
		{
			name:    "multiple inline code spans",
			content: "`#a` and `#b` but #c is real",
			want:    []string{"#c"},
		},
		{
			name:    "tag right after inline code",
			content: "`code` #tag",
			want:    []string{"#tag"},
		},
		{
			name:    "unclosed backtick not greedy",
			content: "a ` orphan #tag",
			want:    []string{"#tag"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTags(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractTags() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if NormalizeTag(got[i]) != NormalizeTag(tt.want[i]) {
					t.Errorf("ExtractTags()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFindEmbedRanges(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int // expected number of ranges
	}{
		{"no embeds", "just text", 0},
		{"one embed", "text ![[Note]] more", 1},
		{"two embeds", "![[A]] and ![[B]]", 2},
		{"embed with header", "![[Note#Section]]", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := FindEmbedRanges(tt.content)
			if len(ranges) != tt.want {
				t.Errorf("FindEmbedRanges() returned %d ranges, want %d", len(ranges), tt.want)
			}
		})
	}
}

func TestInsideRanges(t *testing.T) {
	ranges := [][2]int{{5, 10}, {20, 25}}

	tests := []struct {
		pos  int
		want bool
	}{
		{0, false},
		{5, true},
		{7, true},
		{9, true},
		{10, false},
		{15, false},
		{20, true},
		{24, true},
		{25, false},
	}

	for _, tt := range tests {
		if got := InsideRanges(tt.pos, ranges); got != tt.want {
			t.Errorf("InsideRanges(%d) = %v, want %v", tt.pos, got, tt.want)
		}
	}
}
