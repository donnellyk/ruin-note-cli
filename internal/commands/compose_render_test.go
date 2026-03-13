package commands

import (
	"testing"
)

func TestRenderExplain_BasicTree(t *testing.T) {
	tree := &composeTree{
		UUID:  "root-1",
		Title: "Root",
		Path:  "Root.md",
		Depth: 0,
		Children: []*composeTree{
			{
				UUID:  "child-1",
				Title: "Child",
				Path:  "Child.md",
				Depth: 1,
			},
		},
	}

	err := renderExplain(tree, nil, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderExplain_WithYMLSource(t *testing.T) {
	tree := &composeTree{
		UUID:  "root-1",
		Title: "Root",
		Path:  "Root.md",
		Depth: 0,
		Children: []*composeTree{
			{UUID: "ch-1", Title: "Chapter 1", Path: "ch1.md", Depth: 1},
		},
	}

	ymlParents := map[string]bool{"root-1": true}
	err := renderExplain(tree, ymlParents, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderExplain_WithEmbeds(t *testing.T) {
	tree := &composeTree{
		UUID:  "root-1",
		Title: "Root",
		Path:  "Root.md",
		Depth: 0,
		Segments: []composeSegment{
			{Text: "intro text"},
			{
				Text: "after embed",
				Embed: &composeTree{
					UUID:     "embed-1",
					Title:    "Embedded",
					Path:     "Embedded.md",
					Depth:    1,
					Embedded: true,
				},
			},
		},
	}

	err := renderExplain(tree, nil, false)
	if err != nil {
		t.Fatal(err)
	}
}
