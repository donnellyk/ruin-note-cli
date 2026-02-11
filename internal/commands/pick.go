package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"

	"github.com/spf13/cobra"
)

// doneTag is the special tag that marks a line as resolved/completed.
const doneTag = "#done"

// doneFilter controls how #done lines are handled.
type doneFilter int

const (
	doneExclude doneFilter = iota // default: hide lines with #done
	doneInclude                   // --all: show everything
	doneOnly                      // --done: show only completed lines
)

// PickMatch represents a single matching line from a note.
type PickMatch struct {
	Line    int      `json:"line"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
	Done    bool     `json:"done"`
}

// PickResult groups matches by note.
type PickResult struct {
	UUID    string      `json:"uuid"`
	Title   string      `json:"title,omitempty"`
	File    string      `json:"file"`
	Matches []PickMatch `json:"matches"`
}

// NewPickCmd creates the pick command.
func NewPickCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		anyMode  bool
		allMode  bool
		doneMode bool
	)

	cmd := &cobra.Command{
		Use:   "pick <inline-tags...>",
		Short: "Pick lines annotated with inline tags",
		Long: `Extract lines annotated with specific inline tags from across the vault.

Inline tags are tags that appear within content paragraphs (not the global
tag lines at the top or bottom of a note). Use pick to collect action items,
follow-ups, and other contextual annotations scattered across notes.

By default, multiple tags are combined with AND (lines must contain all tags).
Use --any for OR mode (lines with any of the given tags).

Lines containing #done are excluded by default, since #done marks a line as
resolved/completed. Use --all to include both open and done lines, or --done
to show only completed lines.

The command pre-filters notes using the inline-tags frontmatter field for
fast lookups, then extracts matching lines from the content body.`,
		Example: `  # Find all open followup items (excludes #done lines)
  ruin pick "#followup"

  # Lines with both tags (AND)
  ruin pick "#followup" "#urgent"

  # Lines with either tag (OR)
  ruin pick "#followup" "#todo" --any

  # Include completed lines
  ruin pick "#followup" --all

  # Show only completed lines
  ruin pick "#followup" --done

  # JSON output grouped by note
  ruin pick "#followup" --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			if allMode && doneMode {
				return fmt.Errorf("--all and --done are mutually exclusive")
			}

			// Validate all args are tags
			for _, arg := range args {
				if !strings.HasPrefix(arg, "#") {
					return fmt.Errorf("invalid tag %q: must start with #", arg)
				}
			}

			// Determine done filter
			df := doneExclude
			if allMode {
				df = doneInclude
			} else if doneMode {
				df = doneOnly
			}

			// Normalize query tags
			queryTags := make([]string, len(args))
			for i, arg := range args {
				queryTags[i] = note.NormalizeTag(arg)
			}

			// Load all notes
			notePaths, err := vlt.ListNotes()
			if err != nil {
				return fmt.Errorf("failed to list notes: %w", err)
			}

			var results []PickResult

			for _, path := range notePaths {
				n, err := note.Load(path)
				if err != nil {
					continue
				}

				// Pre-filter: note must have at least one queried tag as inline
				if !noteHasInlineTag(n, queryTags) {
					continue
				}

				// Extract matching lines from inline zone
				matches := pickLinesFromNote(n, queryTags, anyMode, df)
				if len(matches) == 0 {
					continue
				}

				results = append(results, PickResult{
					UUID:    n.UUID,
					Title:   n.Title,
					File:    path,
					Matches: matches,
				})
			}

			if len(results) == 0 {
				if *jsonOutput {
					fmt.Println("[]")
				}
				return ErrNoMatches
			}

			if *jsonOutput {
				return outputPickJSON(results)
			}

			return outputPickBare(results)
		},
	}

	cmd.Flags().BoolVar(&anyMode, "any", false, "match lines with any of the given tags (OR mode)")
	cmd.Flags().BoolVar(&allMode, "all", false, "include lines marked #done (default: exclude)")
	cmd.Flags().BoolVar(&doneMode, "done", false, "show only lines marked #done")

	return cmd
}

// noteHasInlineTag returns true if the note has at least one of the queried
// tags in its InlineTags field.
func noteHasInlineTag(n *note.Note, queryTags []string) bool {
	for _, it := range n.InlineTags {
		itNorm := note.NormalizeTag(it)
		for _, qt := range queryTags {
			if itNorm == qt {
				return true
			}
		}
	}
	return false
}

// pickLinesFromNote extracts content lines that match the queried inline tags.
// Only lines within the "inline zone" (between global tag regions) are considered.
func pickLinesFromNote(n *note.Note, queryTags []string, anyMode bool, df doneFilter) []PickMatch {
	lines := strings.Split(n.Content, "\n")

	// Determine inline zone boundaries (same logic as ClassifyTags)
	titleLineIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			titleLineIdx = i
			break
		}
	}

	firstContentIdx := -1
	for i := titleLineIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if note.IsTagOnlyLine(trimmed) {
			continue
		}
		firstContentIdx = i
		break
	}

	lastContentIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if note.IsTagOnlyLine(trimmed) {
			continue
		}
		lastContentIdx = i
		break
	}

	// No content body found
	if firstContentIdx == -1 {
		return nil
	}

	var matches []PickMatch

	for i := firstContentIdx; i <= lastContentIdx; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Skip tag-only lines (these are global-style tags, not inline annotations)
		if note.IsTagOnlyLine(trimmed) {
			continue
		}

		// Extract tags from this line
		lineTags := note.ExtractTags(line)
		if len(lineTags) == 0 {
			continue
		}

		// Normalize line tags for comparison
		lineTagsNorm := make(map[string]bool)
		for _, lt := range lineTags {
			lineTagsNorm[note.NormalizeTag(lt)] = true
		}

		// Check match based on AND/OR mode
		if anyMode {
			// OR: line must contain at least one queried tag
			matched := false
			for _, qt := range queryTags {
				if lineTagsNorm[qt] {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		} else {
			// AND: line must contain all queried tags
			allFound := true
			for _, qt := range queryTags {
				if !lineTagsNorm[qt] {
					allFound = false
					break
				}
			}
			if !allFound {
				continue
			}
		}

		// Check #done status and apply filter
		doneTagNorm := note.NormalizeTag(doneTag)
		isDone := lineTagsNorm[doneTagNorm]

		switch df {
		case doneExclude:
			if isDone {
				continue
			}
		case doneOnly:
			if !isDone {
				continue
			}
		}

		matches = append(matches, PickMatch{
			Line:    i + 1, // 1-indexed
			Content: trimmed,
			Tags:    lineTags,
			Done:    isDone,
		})
	}

	return matches
}


func outputPickJSON(results []PickResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func outputPickBare(results []PickResult) error {
	for _, r := range results {
		for _, m := range r.Matches {
			fmt.Println(m.Content)
		}
	}
	return nil
}
