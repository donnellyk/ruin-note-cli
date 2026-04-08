package commands

import (
	"fmt"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// ErrNoMatches is returned when a search finds no results.
// This allows the caller to distinguish between errors and no matches.
var ErrNoMatches = fmt.Errorf("no matches found")

// SearchResult represents a single search result.
type SearchResult struct {
	Path   string   `json:"path"`
	UUID   string   `json:"uuid"`
	Title  string   `json:"title,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Parent string   `json:"parent,omitempty"`
	note   *note.Note
}

// SortField represents a field and direction for sorting.
type SortField struct {
	Field     string
	Ascending bool
}

// NewSearchCmd creates the search command.
func NewSearchCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var flags SearchFlags

	var globalTagsOnly, inlineTagsOnly, everything, linkOnly bool
	var notes []string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for notes",
		Long: `Search for notes matching the given query.

Query syntax:
  - Tag search: #tagname (requires # prefix)
  - Date search: @date (matches dates in note body)
  - Text search: word (case-insensitive)
  - AND (explicit): term1 && term2
  - AND (implicit): term1 term2 (space-separated)

Date tokens (@ syntax):
  - @today, @tomorrow, @yesterday
  - @2026-02-13 (exact date)

Date filters (metadata):
  - created:DATE     Notes created on date
  - updated:DATE     Notes updated on date
  - on:DATE          Alias for created:DATE
  - before:DATE      Notes created before date (exclusive)
  - after:DATE       Notes created after date (exclusive)
  - between:D1,D2    Notes created between dates (inclusive)

Date formats:
  - Exact: 2025-01-28, 2025-01, 2025
  - Natural: today, yesterday, tomorrow

Other filters:
  - title:TEXT       Notes with title containing text
  - path:TEXT        Notes with path containing text
  - parent:UUID      Notes with specific parent
  - parent:none      Notes with no parent
  - link:TEXT        Notes with URL containing text

Todo filters:
  - todo:open        Notes with unchecked checkboxes (- [ ])
  - todo:done        Notes with checked checkboxes (- [x])
  - todo:any         Notes with any checkboxes

Tag scope:
  By default, tag searches check both global and inline tags.
  - --global-tags    Only match global tags (categorization)
  - --inline-tags    Only match inline tags (contextual annotations)

See also:
  ruin query save    Save a search as a named query
  ruin today         Shortcut for notes created today
  ruin yesterday     Shortcut for notes created yesterday`,
		Example: `  # Tag search
  ruin search "#daily"

  # Text + tag
  ruin search "#meeting project-alpha"

  # JSON for scripting
  ruin search "#todo" --json

  # Bulk export for editing
  ruin search "#draft" --bulk

  # First match content
  ruin search "#readme" --first

  # Edit matching notes
  ruin search "#blog" --edit

  # Sorted by newest
  ruin search "#log" -s created:desc -l 10

  # Date tokens (in note body)
  ruin search "@tomorrow"
  ruin search "#followup @2025-02-03"

  # Date filters (metadata)
  ruin search "created:today"
  ruin search "created:yesterday"
  ruin search "between:2025-01-01,2025-01-31"

  # Title and path filters
  ruin search "title:meeting"
  ruin search "path:projects/"

  # All notes (no query required)
  ruin search --everything
  ruin search --everything -s title:asc -l 20

  # Link notes
  ruin search --link
  ruin search --link "#project"

  # Constrain to specific notes
  ruin search "#daily" --notes uuid1,uuid2,uuid3`,
		Args: func(cmd *cobra.Command, args []string) error {
			if everything || linkOnly {
				return nil
			}
			return cobra.MinimumNArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			// Validate flags
			if err := ValidateSearchFlags(&flags, *jsonOutput); err != nil {
				return err
			}

			if globalTagsOnly && inlineTagsOnly {
				return fmt.Errorf("--global-tags and --inline-tags are mutually exclusive")
			}

			// Determine tag scope
			tagScope := TagScopeAll
			if globalTagsOnly {
				tagScope = TagScopeGlobal
			} else if inlineTagsOnly {
				tagScope = TagScopeInline
			}

			// Parse query
			var matcher QueryMatcher
			var info MatcherInfo
			if (everything || linkOnly) && len(args) == 0 {
				matcher = func(n *note.Note) bool { return true }
			} else {
				query := strings.Join(args, " ")
				var err2 error
				matcher, info, err2 = parseQuery(query, tagScope)
				if err2 != nil {
					return fmt.Errorf("invalid query: %w", err2)
				}
			}

			// Wrap matcher with --link filter
			if linkOnly {
				baseMatcher := matcher
				linkMatch := linkNoteMatcher()
				matcher = func(n *note.Note) bool {
					return linkMatch(n) && baseMatcher(n)
				}
				info.NeedsBody = true
			}

			return executeSearch(vlt, matcher, info, &flags, *jsonOutput, notes)
		},
	}

	AddSearchFlags(cmd, &flags, "created:desc")
	cmd.Flags().BoolVar(&everything, "everything", false, "return all notes (no query required)")
	cmd.Flags().BoolVar(&globalTagsOnly, "global-tags", false, "only match global tags (categorization)")
	cmd.Flags().BoolVar(&inlineTagsOnly, "inline-tags", false, "only match inline tags (contextual annotations)")
	cmd.Flags().BoolVar(&linkOnly, "link", false, "only match link notes (notes with a URL)")
	cmd.Flags().StringSliceVar(&notes, "notes", nil, "constrain to specific note UUIDs (comma-separated or repeated)")

	return cmd
}

// executeSearch runs the shared search pipeline: sort parsing, option building,
// search execution, and result dispatch. Used by both search and link list.
func executeSearch(vlt *vault.Vault, matcher QueryMatcher, info MatcherInfo, flags *SearchFlags, jsonOutput bool, uuids []string) error {
	var sortFields []SortField
	if flags.Sort != "" {
		var err error
		sortFields, err = parseSort(flags.Sort)
		if err != nil {
			return fmt.Errorf("invalid sort: %w", err)
		}
	}

	var opts SearchOptions
	if flags.Limit > 0 && len(sortFields) == 0 {
		opts.Limit = flags.Limit
	}
	opts.NeedFullNote = flags.Bulk || flags.First || flags.Edit || flags.Content || flags.Frontmatter != ""

	if len(uuids) > 0 {
		opts.UUIDs = uuids
	}

	results, err := searchNotesWithOptions(vlt, matcher, info, opts)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	return dispatchSearchResults(vlt, results, flags, jsonOutput, sortFields)
}
