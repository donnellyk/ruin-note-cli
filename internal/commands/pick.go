package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"kvnd/ruin-note-cli/internal/dateparse"
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
		anyMode    bool
		allMode    bool
		doneMode   bool
		filterFlag string
	)

	cmd := &cobra.Command{
		Use:   "pick <inline-tags...> [@date...]",
		Short: "Pick lines annotated with inline tags",
		Long: `Extract lines annotated with specific inline tags from across the vault.

Inline tags are tags that appear on lines with other content (not tag-only
lines which are treated as global tags). Use pick to collect action items,
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

  # Filter lines by inline date (range-based matching)
  ruin pick "#followup" @today
  ruin pick "#followup" @this-week
  ruin pick "#followup" @2026-03
  ruin pick "#followup" @2026-03-01

  # Filter notes by metadata (note-level, using search query syntax)
  ruin pick "#followup" --filter "created:today"
  ruin pick "#followup" --filter "@tomorrow"
  ruin pick "#todo" --filter "before:2025-06 after:2025-01"
  ruin pick "#followup" --filter "between:2025-01,2025-06"

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

			// Separate args into tags and date tokens
			var tagArgs []string
			var dateRanges []dateparse.DateRange
			for _, arg := range args {
				switch {
				case strings.HasPrefix(arg, "#"):
					tagArgs = append(tagArgs, arg)
				case strings.HasPrefix(arg, "@"):
					token := arg[1:] // strip @
					dr, err := dateparse.ParseWithReference(token, time.Now())
					if err != nil {
						return fmt.Errorf("unrecognized date: %s", arg)
					}
					dateRanges = append(dateRanges, dr)
				default:
					return fmt.Errorf("invalid argument %q: must start with # or @", arg)
				}
			}
			if len(tagArgs) == 0 {
				return fmt.Errorf("at least one inline tag required")
			}

			// Parse --filter into a QueryMatcher
			var filterMatcher QueryMatcher
			if filterFlag != "" {
				m, _, err := parseQuery(filterFlag, TagScopeAll)
				if err != nil {
					return fmt.Errorf("invalid filter: %w", err)
				}
				filterMatcher = m
			}

			// Determine done filter
			df := doneExclude
			if allMode {
				df = doneInclude
			} else if doneMode {
				df = doneOnly
			}

			// Normalize query tags
			queryTags := make([]string, len(tagArgs))
			for i, arg := range tagArgs {
				queryTags[i] = note.NormalizeTag(arg)
			}

			// Load all notes
			notePaths, err := vlt.ListNotes()
			if err != nil {
				return fmt.Errorf("failed to list notes: %w", err)
			}

			var results []PickResult

			for _, path := range notePaths {
				// Fast pre-filter: check inline tags from frontmatter only
				fast, err := note.LoadFrontmatterOnly(path)
				if err != nil {
					continue
				}

				// Pre-filter: note must have at least one queried tag as inline
				if !noteHasInlineTag(fast, queryTags) {
					continue
				}

				// Pre-filter: apply --filter matcher against frontmatter-only note
				if filterMatcher != nil && !filterMatcher(fast) {
					continue
				}

				// Full load only for notes that pass pre-filter
				n, err := note.Load(path)
				if err != nil {
					continue
				}

				// Extract matching lines from inline zone
				matches := pickLinesFromNote(n, queryTags, dateRanges, anyMode, df)
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
	cmd.Flags().StringVar(&filterFlag, "filter", "", "filter notes using search query syntax (e.g., \"created:today\", \"@tomorrow\", \"before:2025-06\")")

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
// Tag-only lines and the title line are skipped (those contain global tags).
// If dateRanges is non-empty, lines must contain at least one @YYYY-MM-DD date
// that falls within every specified date range.
func pickLinesFromNote(n *note.Note, queryTags []string, dateRanges []dateparse.DateRange, anyMode bool, df doneFilter) []PickMatch {
	lines := strings.Split(n.Content, "\n")

	var matches []PickMatch

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Skip header lines (title)
		if note.IsHeaderLine(trimmed) {
			continue
		}

		// Skip tag-only lines (global tags)
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

		// Check inline date filter: for each date range, the line must
		// contain at least one @YYYY-MM-DD date that falls within the range.
		if len(dateRanges) > 0 {
			lineDates := note.ExtractDates(trimmed)
			if len(lineDates) == 0 {
				continue
			}
			// Parse line dates into time.Time values
			var parsedDates []time.Time
			for _, ds := range lineDates {
				if t, err := time.ParseInLocation("2006-01-02", ds, time.Local); err == nil {
					parsedDates = append(parsedDates, t)
				}
			}
			if len(parsedDates) == 0 {
				continue
			}
			// Every date range must be satisfied by at least one line date
			allRanges := true
			for _, dr := range dateRanges {
				found := false
				for _, pd := range parsedDates {
					if dr.Contains(pd) {
						found = true
						break
					}
				}
				if !found {
					allRanges = false
					break
				}
			}
			if !allRanges {
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
