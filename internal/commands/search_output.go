package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
)

type FrontmatterMode string

const (
	FrontmatterNone  FrontmatterMode = "none"  // Hide frontmatter (default)
	FrontmatterExtra FrontmatterMode = "extra" // Show only user-defined fields
	FrontmatterFull  FrontmatterMode = "full"  // Show complete frontmatter
)

func outputPaths(results []SearchResult, fmMode FrontmatterMode) error {
	for _, r := range results {
		fmt.Println(r.Path)
		if fmMode == FrontmatterExtra && len(r.note.Extra) > 0 {
			fmt.Printf("  %s\n", formatExtraFields(r.note.Extra))
		} else if fmMode == FrontmatterFull {
			fmt.Printf("  uuid=%s, created=%s, updated=%s\n",
				r.UUID,
				r.note.Created.Format("2006-01-02"),
				r.note.Updated.Format("2006-01-02"))
			if len(r.note.Extra) > 0 {
				fmt.Printf("  %s\n", formatExtraFields(r.note.Extra))
			}
		}
	}
	return nil
}

func formatExtraFields(extra map[string]any) string {
	if len(extra) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(extra))
	for k, v := range extra {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(pairs, ", ")
}

func outputJSON(results []SearchResult, fmMode FrontmatterMode, includeContent, stripGlobalTags, stripTitle bool) error {
	type jsonResult struct {
		Path          string         `json:"path"`
		UUID          string         `json:"uuid"`
		Title         string         `json:"title,omitempty"`
		Tags          []string       `json:"tags,omitempty"`
		InlineTags    []string       `json:"inline_tags,omitempty"`
		InheritedTags []string       `json:"inherited_tags,omitempty"`
		Parent        string         `json:"parent,omitempty"`
		Created       string         `json:"created,omitempty"`
		Updated       string         `json:"updated,omitempty"`
		Extra         map[string]any `json:"extra,omitempty"`
		Content       string         `json:"content,omitempty"`
	}

	output := make([]jsonResult, len(results))
	for i, r := range results {
		jr := jsonResult{
			Path:          r.Path,
			UUID:          r.UUID,
			Title:         r.Title,
			Tags:          r.note.EffectiveGlobalTags(),
			InlineTags:    r.note.InlineTags,
			InheritedTags: r.note.InheritedTags,
			Parent:        r.note.Parent,
			Created:       r.note.Created.Format(note.TimeFormat),
			Updated:       r.note.Updated.Format(note.TimeFormat),
		}

		if fmMode == FrontmatterExtra && len(r.note.Extra) > 0 {
			jr.Extra = r.note.Extra
		} else if fmMode == FrontmatterFull && len(r.note.Extra) > 0 {
			jr.Extra = r.note.Extra
		}

		if includeContent {
			content := r.note.Content

			if fmMode == FrontmatterExtra || fmMode == FrontmatterFull {
				if serialized, err := r.note.Serialize(); err == nil {
					content = serialized
				}
			}

			if stripTitle {
				content = note.StripTitle(content)
			}
			if stripGlobalTags {
				content = note.StripGlobalTags(content, r.note.InlineTags)
			}

			jr.Content = content
		}

		output[i] = jr
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputBulk(results []SearchResult, fmMode FrontmatterMode) error {
	entries := make([]note.BulkEntry, len(results))
	for i, r := range results {
		content := r.note.Content
		if fmMode == FrontmatterFull {
			serialized, err := r.note.Serialize()
			if err == nil {
				content = serialized
			}
		}
		entries[i] = note.BulkEntry{
			UUID:    r.UUID,
			Content: content,
		}
	}
	return note.FormatBulk(entries, os.Stdout)
}

func outputFirst(results []SearchResult, fmMode FrontmatterMode) error {
	if len(results) == 0 {
		return nil
	}

	var output string
	if fmMode == FrontmatterFull {
		serialized, err := results[0].note.Serialize()
		if err == nil {
			output = serialized
		} else {
			output = results[0].note.Content
		}
	} else {
		output = results[0].note.Content
	}

	fmt.Print(output)
	if !strings.HasSuffix(output, "\n") {
		fmt.Println()
	}
	return nil
}
