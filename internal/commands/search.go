package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
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

	var globalTagsOnly, inlineTagsOnly, everything bool

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
  ruin search --everything -s title:asc -l 20`,
		Args: func(cmd *cobra.Command, args []string) error {
			if everything {
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
			if everything && len(args) == 0 {
				matcher = func(n *note.Note) bool { return true }
			} else {
				query := strings.Join(args, " ")
				var err2 error
				matcher, info, err2 = parseQuery(query, tagScope)
				if err2 != nil {
					return fmt.Errorf("invalid query: %w", err2)
				}
			}

			// Parse sort fields
			var sortFields []SortField
			if flags.Sort != "" {
				var err error
				sortFields, err = parseSort(flags.Sort)
				if err != nil {
					return fmt.Errorf("invalid sort: %w", err)
				}
			}

			// Determine search options for optimization
			// Only enable early termination if limit is set and no sorting requested
			var opts SearchOptions
			if flags.Limit > 0 && len(sortFields) == 0 {
				opts.Limit = flags.Limit
			}
			// Need full note content for output modes that access body/extra
			opts.NeedFullNote = flags.Bulk || flags.First || flags.Edit || flags.Content || flags.Frontmatter != ""

			// Find matching notes
			results, err := searchNotesWithOptions(vlt, matcher, info, opts)
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			// Sort results (if sorting requested)
			if len(sortFields) > 0 {
				sortResults(results, sortFields)

				// Apply limit after sorting (early termination wasn't possible)
				if flags.Limit > 0 && len(results) > flags.Limit {
					results = results[:flags.Limit]
				}
			}

			// No results
			if len(results) == 0 {
				if *jsonOutput {
					fmt.Println("[]")
				}
				return nil
			}

			// Parse frontmatter mode
			fmMode := FrontmatterMode(flags.Frontmatter)
			if flags.Frontmatter != "" && flags.Frontmatter != "none" && flags.Frontmatter != "extra" && flags.Frontmatter != "full" {
				return fmt.Errorf("invalid frontmatter mode: %s (use: none, extra, full)", flags.Frontmatter)
			}

			// Output based on mode
			if flags.Edit {
				// --first limits edit to first match only
				if flags.First && len(results) > 1 {
					results = results[:1]
				}
				return handleEdit(vlt, results, flags.Force, fmMode)
			}

			if flags.Bulk {
				return outputBulk(results, fmMode)
			}

			if flags.First {
				return outputFirst(results, fmMode)
			}

			if *jsonOutput {
				return outputJSON(results, fmMode, flags.Content, flags.StripGlobalTags, flags.StripTitle)
			}

			// Default: list of paths (with optional frontmatter)
			return outputPaths(results, fmMode)
		},
	}

	AddSearchFlags(cmd, &flags, "created:desc")
	cmd.Flags().BoolVar(&everything, "everything", false, "return all notes (no query required)")
	cmd.Flags().BoolVar(&globalTagsOnly, "global-tags", false, "only match global tags (categorization)")
	cmd.Flags().BoolVar(&inlineTagsOnly, "inline-tags", false, "only match inline tags (contextual annotations)")

	return cmd
}
