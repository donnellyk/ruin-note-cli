package note

import "testing"

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
