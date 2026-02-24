package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
	// unexported, for sorting only
	created time.Time
	updated time.Time
	order   *int
}

// NewPickCmd creates the pick command.
func NewPickCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		anyMode    bool
		allMode    bool
		doneMode   bool
		todoMode   bool
		filterFlag string
		notesFlag  []string
		parentFlag string
		sortFlag   string
	)

	cmd := &cobra.Command{
		Use:   "pick [inline-tags...] [@date...] [flags]",
		Short: "Pick lines annotated with inline tags or checkboxes",
		Long: `Extract lines annotated with specific inline tags from across the vault.

Inline tags are tags that appear on lines with other content (not tag-only
lines which are treated as global tags). Use pick to collect action items,
follow-ups, and other contextual annotations scattered across notes.

By default, multiple tags are combined with AND (lines must contain all tags).
Use --any for OR mode (lines with any of the given tags).

Lines containing #done are excluded by default, since #done marks a line as
resolved/completed. Use --all to include both open and done lines, or --done
to show only completed lines.

Use @date alone to find all lines with a matching date annotation — no tags
required. Use --todo to also match markdown checkbox lines (- [ ] / - [x]).
When --todo or @date is provided, tags become optional. The done filter applies
uniformly: checked checkboxes ([x]) and #done lines are both treated as "done".

Results are sorted by note creation date (newest first) by default. Use --sort
to change the ordering (e.g., --sort title:asc, --sort updated:desc). Sort
applies at the note level; line order within each note is preserved.

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

  # All open markdown checkboxes
  ruin pick --todo

  # Only completed checkboxes
  ruin pick --todo --done

  # Checkboxes that also have a specific tag
  ruin pick --todo "#daily"

  # Date-only: all lines with a matching date annotation
  ruin pick @today
  ruin pick @2026-03-15

  # Filter lines by inline date (range-based matching)
  ruin pick "#followup" @today
  ruin pick "#followup" @this-week
  ruin pick "#followup" @2026-03
  ruin pick "#followup" @2026-03-01

  # Checkboxes with a date annotation
  ruin pick --todo @today --all

  # Filter notes by metadata (note-level, using search query syntax)
  ruin pick "#followup" --filter "created:today"
  ruin pick "#followup" --filter "@tomorrow"
  ruin pick "#todo" --filter "before:2025-06 after:2025-01"
  ruin pick "#followup" --filter "between:2025-01,2025-06"

  # Pick from specific notes only
  ruin pick "#followup" --notes uuid-1,uuid-2,uuid-3

  # Pick from children of a parent note
  ruin pick "#followup" --parent "Hub Note"

  # Using a saved bookmark
  ruin pick "#todo" --parent hub

  # Combine with existing filters
  ruin pick "#followup" --parent hub --filter "created:today"

  # Sort results by title (alphabetical)
  ruin pick "#followup" --sort title:asc

  # Sort results by most recently updated
  ruin pick "#followup" --sort updated:desc

  # JSON output grouped by note
  ruin pick "#followup" --json`,
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			if allMode && doneMode {
				return fmt.Errorf("--all and --done are mutually exclusive")
			}

			if len(notesFlag) > 0 && parentFlag != "" {
				return fmt.Errorf("--notes and --parent are mutually exclusive")
			}

			// Build allowedPaths if --notes or --parent is set
			var allowedPaths map[string]bool
			if len(notesFlag) > 0 {
				index, err := vlt.LoadTitles()
				if err != nil {
					return fmt.Errorf("failed to load titles index: %w", err)
				}
				allowedPaths = make(map[string]bool)
				for _, uuid := range notesFlag {
					entry, ok := index.Titles[uuid]
					if !ok {
						fmt.Fprintf(os.Stderr, "warning: UUID %s not found in index\n", uuid)
						continue
					}
					allowedPaths[entry.Path] = true
				}
			} else if parentFlag != "" {
				parentNote, err := ResolveNote(vlt, parentFlag)
				if err != nil {
					return fmt.Errorf("failed to resolve parent: %w", err)
				}
				index, err := vlt.LoadTitles()
				if err != nil {
					return fmt.Errorf("failed to load titles index: %w", err)
				}
				allowedPaths = make(map[string]bool)
				// Build parent->children map for recursive descent
				children := make(map[string][]string)
				for uuid, entry := range index.Titles {
					if entry.Parent != "" {
						children[entry.Parent] = append(children[entry.Parent], uuid)
					}
				}
				// BFS to collect all descendants
				queue := children[parentNote.UUID]
				for len(queue) > 0 {
					uuid := queue[0]
					queue = queue[1:]
					if entry, ok := index.Titles[uuid]; ok {
						allowedPaths[entry.Path] = true
					}
					queue = append(queue, children[uuid]...)
				}
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
			if len(tagArgs) == 0 && !todoMode && len(dateRanges) == 0 {
				return fmt.Errorf("at least one inline tag, @date, or --todo required")
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

			// Parse --sort
			sortFields, err := parseSort(sortFlag)
			if err != nil {
				return fmt.Errorf("invalid sort: %w", err)
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
				if allowedPaths != nil && !allowedPaths[path] {
					continue
				}

				// Fast pre-filter: check inline tags from frontmatter only
				fast, err := note.LoadFrontmatterOnly(path)
				if err != nil {
					continue
				}

				// Pre-filter: note must have at least one queried tag as inline
				// Skip when --todo without tags (match all notes with checkboxes)
				if len(queryTags) > 0 && !noteHasInlineTag(fast, queryTags) {
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
				matches := pickLinesFromNote(n, queryTags, dateRanges, anyMode, df, todoMode)
				if len(matches) == 0 {
					continue
				}

				results = append(results, PickResult{
					UUID:    n.UUID,
					Title:   n.Title,
					File:    path,
					Matches: matches,
					created: n.Created,
					updated: n.Updated,
					order:   n.Order,
				})
			}

			if len(results) == 0 {
				if *jsonOutput {
					fmt.Println("[]")
				}
				return ErrNoMatches
			}

			sortPickResults(results, sortFields)

			if *jsonOutput {
				return outputPickJSON(results)
			}

			return outputPickBare(results)
		},
	}

	cmd.Flags().BoolVar(&anyMode, "any", false, "match lines with any of the given tags (OR mode)")
	cmd.Flags().BoolVar(&allMode, "all", false, "include lines marked #done (default: exclude)")
	cmd.Flags().BoolVar(&doneMode, "done", false, "show only lines marked #done")
	cmd.Flags().BoolVar(&todoMode, "todo", false, "also match markdown checkbox lines (- [ ] / - [x])")
	cmd.Flags().StringVar(&filterFlag, "filter", "", "filter notes using search query syntax (e.g., \"created:today\", \"@tomorrow\", \"before:2025-06\")")
	cmd.Flags().StringSliceVar(&notesFlag, "notes", nil, "scope to specific notes by UUID (comma-separated or repeated)")
	cmd.Flags().StringVar(&parentFlag, "parent", "", "scope to all descendants of a parent note (bookmark, UUID, or title)")
	cmd.Flags().StringVarP(&sortFlag, "sort", "s", "created:desc", "sort order (e.g., created:desc, title:asc)")

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
// When todoMode is true, checkbox lines (- [ ] / - [x]) are also matched.
func pickLinesFromNote(n *note.Note, queryTags []string, dateRanges []dateparse.DateRange, anyMode bool, df doneFilter, todoMode bool) []PickMatch {
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

		// Normalize line tags for comparison
		lineTagsNorm := make(map[string]bool)
		for _, lt := range lineTags {
			lineTagsNorm[note.NormalizeTag(lt)] = true
		}

		isCheckbox := note.IsCheckboxLine(trimmed)

		// Determine if this line matches
		lineMatched := false

		if todoMode && isCheckbox {
			if len(queryTags) == 0 {
				// --todo without tags: match all checkbox lines
				lineMatched = true
			} else {
				// --todo with tags: checkbox must also have the tags
				lineMatched = matchesTags(lineTagsNorm, queryTags, anyMode)
			}
		}

		// Also check tag-based matching (non-todo or todo+tags)
		if !lineMatched && len(lineTags) > 0 && len(queryTags) > 0 {
			lineMatched = matchesTags(lineTagsNorm, queryTags, anyMode)
		}

		// Date-only mode: any content line can potentially match if it contains matching dates.
		// The date filter below will do the actual date check.
		if !lineMatched && len(queryTags) == 0 && !todoMode && len(dateRanges) > 0 {
			lineMatched = true
		}

		if !lineMatched {
			continue
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

		// Check done status: #done tag OR checked checkbox [x]
		doneTagNorm := note.NormalizeTag(doneTag)
		isDone := lineTagsNorm[doneTagNorm] || (todoMode && note.IsCheckedLine(trimmed))

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

// matchesTags checks if a line's tags match the query tags in AND or OR mode.
func matchesTags(lineTagsNorm map[string]bool, queryTags []string, anyMode bool) bool {
	if anyMode {
		for _, qt := range queryTags {
			if lineTagsNorm[qt] {
				return true
			}
		}
		return false
	}
	for _, qt := range queryTags {
		if !lineTagsNorm[qt] {
			return false
		}
	}
	return true
}

// sortPickResults sorts pick results by the given fields.
func sortPickResults(results []PickResult, fields []SortField) {
	sort.Slice(results, func(i, j int) bool {
		for _, f := range fields {
			if f.Field == "order" {
				aSet, bSet := results[i].order != nil, results[j].order != nil
				if aSet != bSet {
					return aSet // set before unset
				}
			}
			cmp := comparePickResults(results[i], results[j], f.Field)
			if cmp != 0 {
				if f.Ascending {
					return cmp < 0
				}
				return cmp > 0
			}
		}
		return false
	})
}

// comparePickResults compares two pick results by the given field.
func comparePickResults(a, b PickResult, field string) int {
	switch field {
	case "created":
		if a.created.Before(b.created) {
			return -1
		}
		if a.created.After(b.created) {
			return 1
		}
		return 0
	case "updated":
		if a.updated.Before(b.updated) {
			return -1
		}
		if a.updated.After(b.updated) {
			return 1
		}
		return 0
	case "title":
		return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	case "order":
		aOrd, bOrd := 0, 0
		if a.order != nil {
			aOrd = *a.order
		}
		if b.order != nil {
			bOrd = *b.order
		}
		if aOrd < bOrd {
			return -1
		}
		if aOrd > bOrd {
			return 1
		}
		return 0
	}
	return 0
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
