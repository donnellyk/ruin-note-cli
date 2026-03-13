package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type explainDecision struct {
	Type       string `json:"type"`
	Note       string `json:"note,omitempty"`
	UUID       string `json:"uuid,omitempty"`
	Path       string `json:"path,omitempty"`
	Depth      int    `json:"depth,omitempty"`
	Source     string `json:"source,omitempty"`
	SourceNote string `json:"source_note,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

func renderExplain(tree *composeTree, ymlParents map[string]bool, jsonOut bool) error {
	var decisions []explainDecision
	var lines []string

	decisions = append(decisions, explainDecision{
		Type: "root",
		Note: tree.Title,
		UUID: tree.UUID,
		Path: tree.Path,
	})
	lines = append(lines, fmt.Sprintf("ROOT: %q (uuid: %s)", tree.Title, tree.UUID))

	var walk func(node *composeTree, indent int, parentTitle string)
	walk = func(node *composeTree, indent int, parentTitle string) {
		prefix := strings.Repeat("  ", indent)

		if ymlParents != nil && ymlParents[node.UUID] {
			decisions = append(decisions, explainDecision{
				Type: "yml_source",
				Note: node.Title,
				UUID: node.UUID,
			})
			lines = append(lines, fmt.Sprintf("%sSOURCE: children of %q from compose file", prefix, node.Title))
		}

		if len(node.Segments) > 0 {
			for _, seg := range node.Segments {
				if seg.Embed != nil {
					decisions = append(decisions, explainDecision{
						Type:       "embed",
						Note:       seg.Embed.Title,
						UUID:       seg.Embed.UUID,
						Depth:      seg.Embed.Depth,
						SourceNote: node.Title,
					})
					lines = append(lines, fmt.Sprintf("%sEMBED: %q (uuid: %s) -- from ![[...]] in %q, depth %d",
						prefix, seg.Embed.Title, seg.Embed.UUID, node.Title, seg.Embed.Depth))
					walk(seg.Embed, indent+1, node.Title)
				}
			}
		}

		for _, child := range node.Children {
			walkChild(child, indent, node.Title, &decisions, &lines, walk)
		}
	}

	walk(tree, 1, "")

	if jsonOut {
		output := struct {
			Decisions []explainDecision `json:"decisions"`
		}{Decisions: decisions}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

func walkChild(child *composeTree, indent int, parentTitle string, decisions *[]explainDecision, lines *[]string, walk func(*composeTree, int, string)) {
	prefix := strings.Repeat("  ", indent)
	*decisions = append(*decisions, explainDecision{
		Type:       "child",
		Note:       child.Title,
		UUID:       child.UUID,
		Depth:      child.Depth,
		SourceNote: parentTitle,
	})
	*lines = append(*lines, fmt.Sprintf("%sCHILD: %q (uuid: %s) -- child of %q, depth %d",
		prefix, child.Title, child.UUID, parentTitle, child.Depth))
	walk(child, indent+1, parentTitle)
}
