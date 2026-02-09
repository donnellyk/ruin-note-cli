package note

import (
	"testing"
)

func TestExtractWikiLinks(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []WikiLink
	}{
		{
			name:    "basic link",
			content: "See [[Project Alpha]] for details.",
			want:    []WikiLink{{Title: "Project Alpha"}},
		},
		{
			name:    "link with display text",
			content: "Check [[Project Alpha|the alpha project]] out.",
			want:    []WikiLink{{Title: "Project Alpha", Display: "the alpha project"}},
		},
		{
			name:    "multiple links",
			content: "See [[Alpha]] and [[Beta]] and [[Gamma]].",
			want: []WikiLink{
				{Title: "Alpha"},
				{Title: "Beta"},
				{Title: "Gamma"},
			},
		},
		{
			name:    "case-insensitive dedup",
			content: "See [[Alpha]] and [[alpha]] and [[ALPHA]].",
			want:    []WikiLink{{Title: "Alpha"}},
		},
		{
			name:    "whitespace trimming",
			content: "See [[ Project Alpha ]] for details.",
			want:    []WikiLink{{Title: "Project Alpha"}},
		},
		{
			name:    "whitespace trimming with display",
			content: "See [[ Alpha | the project ]] here.",
			want:    []WikiLink{{Title: "Alpha", Display: "the project"}},
		},
		{
			name:    "empty brackets",
			content: "See [[]] for nothing.",
			want:    nil,
		},
		{
			name:    "whitespace only brackets",
			content: "See [[   ]] for nothing.",
			want:    nil,
		},
		{
			name:    "no links",
			content: "Just plain text with no links.",
			want:    nil,
		},
		{
			name:    "special chars in title",
			content: "See [[Project Alpha - v2.0]] for details.",
			want:    []WikiLink{{Title: "Project Alpha - v2.0"}},
		},
		{
			name:    "link with empty display",
			content: "See [[Alpha|]] here.",
			want:    []WikiLink{{Title: "Alpha"}},
		},
		{
			name:    "mixed links and display",
			content: "See [[Alpha]] and [[Beta|beta project]].",
			want: []WikiLink{
				{Title: "Alpha"},
				{Title: "Beta", Display: "beta project"},
			},
		},
		{
			name:    "links in multiline content",
			content: "# My Note\n\nSee [[Alpha]] for context.\n\nAlso check [[Beta]].\n",
			want: []WikiLink{
				{Title: "Alpha"},
				{Title: "Beta"},
			},
		},
		{
			name:    "nested brackets ignored",
			content: "See [[[Alpha]]] for details.",
			want:    []WikiLink{{Title: "Alpha"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractWikiLinks(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractWikiLinks() returned %d links, want %d\ngot: %v", len(got), len(tt.want), got)
			}
			for i, link := range got {
				if link.Title != tt.want[i].Title {
					t.Errorf("link[%d].Title = %q, want %q", i, link.Title, tt.want[i].Title)
				}
				if link.Display != tt.want[i].Display {
					t.Errorf("link[%d].Display = %q, want %q", i, link.Display, tt.want[i].Display)
				}
			}
		})
	}
}

func TestExtractWikiLinkTitles(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "basic",
			content: "See [[Alpha]] and [[Beta]].",
			want:    []string{"Alpha", "Beta"},
		},
		{
			name:    "dedup",
			content: "See [[Alpha]] and [[alpha]].",
			want:    []string{"Alpha"},
		},
		{
			name:    "no links",
			content: "Plain text.",
			want:    nil,
		},
		{
			name:    "display text stripped",
			content: "See [[Alpha|the project]].",
			want:    []string{"Alpha"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractWikiLinkTitles(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractWikiLinkTitles() returned %d titles, want %d", len(got), len(tt.want))
			}
			for i, title := range got {
				if title != tt.want[i] {
					t.Errorf("title[%d] = %q, want %q", i, title, tt.want[i])
				}
			}
		})
	}
}
