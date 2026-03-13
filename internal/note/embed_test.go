package note

import (
	"testing"
)

func TestFindEmbeds(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []EmbedRef
	}{
		{
			name:    "no embeds",
			content: "just some text\nno embeds here",
			want:    nil,
		},
		{
			name:    "simple embed on own line",
			content: "some text\n![[My Note]]\nmore text",
			want:    []EmbedRef{{NoteRef: "My Note", Line: 1}},
		},
		{
			name:    "embed with header",
			content: "![[My Note#Section 1]]",
			want:    []EmbedRef{{NoteRef: "My Note", Header: "Section 1", Line: 0}},
		},
		{
			name:    "embed with leading whitespace",
			content: "  ![[Indented Note]]",
			want:    []EmbedRef{{NoteRef: "Indented Note", Line: 0}},
		},
		{
			name:    "inline embed not matched",
			content: "See ![[My Note]] for details",
			want:    nil,
		},
		{
			name:    "multiple embeds",
			content: "![[Note A]]\nsome text\n![[Note B#Header]]",
			want: []EmbedRef{
				{NoteRef: "Note A", Line: 0},
				{NoteRef: "Note B", Header: "Header", Line: 2},
			},
		},
		{
			name:    "regular wiki link not matched",
			content: "[[Not an embed]]",
			want:    nil,
		},
		{
			name:    "embed with uuid",
			content: "![[d4e5f6a7-b8c9-1234-5678-9abcdef01234]]",
			want:    []EmbedRef{{NoteRef: "d4e5f6a7-b8c9-1234-5678-9abcdef01234", Line: 0}},
		},
		{
			name:    "embed with path",
			content: "![[notes/appendix.md]]",
			want:    []EmbedRef{{NoteRef: "notes/appendix.md", Line: 0}},
		},
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindEmbeds(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("FindEmbeds() returned %d refs, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].NoteRef != tt.want[i].NoteRef {
					t.Errorf("ref[%d].NoteRef = %q, want %q", i, got[i].NoteRef, tt.want[i].NoteRef)
				}
				if got[i].Header != tt.want[i].Header {
					t.Errorf("ref[%d].Header = %q, want %q", i, got[i].Header, tt.want[i].Header)
				}
				if got[i].Line != tt.want[i].Line {
					t.Errorf("ref[%d].Line = %d, want %d", i, got[i].Line, tt.want[i].Line)
				}
			}
		})
	}
}

func TestExtractSection(t *testing.T) {
	tests := []struct {
		name    string
		content string
		header  string
		want    string
		wantErr bool
	}{
		{
			name:    "extract section with subheadings",
			content: "# Title\n\n## Section A\n\nContent A\n\n### Sub A\n\nSub content\n\n## Section B\n\nContent B",
			header:  "Section A",
			want:    "## Section A\n\nContent A\n\n### Sub A\n\nSub content",
		},
		{
			name:    "extract last section",
			content: "# Title\n\n## Section A\n\nContent A\n\n## Section B\n\nContent B",
			header:  "Section B",
			want:    "## Section B\n\nContent B",
		},
		{
			name:    "case insensitive match",
			content: "## My Section\n\nContent here",
			header:  "my section",
			want:    "## My Section\n\nContent here",
		},
		{
			name:    "header not found",
			content: "# Title\n\nContent",
			header:  "Missing",
			wantErr: true,
		},
		{
			name:    "nested heading included",
			content: "## Parent\n\nText\n\n### Child\n\nChild text\n\n#### Grandchild\n\nDeep\n\n## Next",
			header:  "Parent",
			want:    "## Parent\n\nText\n\n### Child\n\nChild text\n\n#### Grandchild\n\nDeep",
		},
		{
			name:    "h1 section",
			content: "# First\n\nContent\n\n# Second\n\nOther",
			header:  "First",
			want:    "# First\n\nContent",
		},
		{
			name:    "trailing blank lines trimmed",
			content: "## Section\n\nContent\n\n\n\n## Next",
			header:  "Section",
			want:    "## Section\n\nContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractSection(tt.content, tt.header)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ExtractSection() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindEmbeds_HeadingAdjustmentInvariant(t *testing.T) {
	content := "## Heading\n![[Embed Note]]\n### Another"
	refs := FindEmbeds(content)
	if len(refs) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(refs))
	}
	if refs[0].NoteRef != "Embed Note" {
		t.Errorf("NoteRef = %q, want %q", refs[0].NoteRef, "Embed Note")
	}
}
