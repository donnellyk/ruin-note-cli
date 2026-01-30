package commands

import (
	"fmt"

	"github.com/kevin/ruin-note-cli/internal/dateparse"
	"github.com/kevin/ruin-note-cli/internal/note"
	"github.com/kevin/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// NewTodayCmd creates the today command.
func NewTodayCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		bulk        bool
		first       bool
		edit        bool
		force       bool
		frontmatter string
		sortBy      string
		limit       int
		useUpdated  bool
	)

	cmd := &cobra.Command{
		Use:   "today",
		Short: "Show notes created today",
		Long: `Show all notes where the created timestamp is today (local timezone).

Use --updated to match on the updated timestamp instead of created.`,
		Example: `  # List today's notes
  ruin today

  # Bulk export today's notes
  ruin today --bulk

  # Notes updated today
  ruin today --updated

  # JSON output for scripting
  ruin today --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDateCommand(
				getVault,
				jsonOutput,
				dateparse.Today(),
				useUpdated,
				bulk,
				first,
				edit,
				force,
				frontmatter,
				sortBy,
				limit,
			)
		},
	}

	cmd.Flags().BoolVarP(&bulk, "bulk", "b", false, "output content with %%%% <uuid> %%%% separators")
	cmd.Flags().BoolVarP(&first, "first", "f", false, "output first match content only")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "open matches in $EDITOR")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation for deletions in edit mode")
	cmd.Flags().StringVar(&frontmatter, "frontmatter", "", "include frontmatter in output (modes: extra, full, none)")
	cmd.Flag("frontmatter").NoOptDefVal = "extra"
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "created:desc", "sort order: field:dir (default newest first)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "max results (0 = unlimited)")
	cmd.Flags().BoolVarP(&useUpdated, "updated", "u", false, "match on updated timestamp instead of created")

	return cmd
}

// NewYesterdayCmd creates the yesterday command.
func NewYesterdayCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		bulk        bool
		first       bool
		edit        bool
		force       bool
		frontmatter string
		sortBy      string
		limit       int
		useUpdated  bool
	)

	cmd := &cobra.Command{
		Use:   "yesterday",
		Short: "Show notes created yesterday",
		Long: `Show all notes where the created timestamp is yesterday (local timezone).

Use --updated to match on the updated timestamp instead of created.`,
		Example: `  # List yesterday's notes
  ruin yesterday

  # Bulk export yesterday's notes
  ruin yesterday --bulk

  # Notes updated yesterday
  ruin yesterday --updated

  # JSON output for scripting
  ruin yesterday --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDateCommand(
				getVault,
				jsonOutput,
				dateparse.Yesterday(),
				useUpdated,
				bulk,
				first,
				edit,
				force,
				frontmatter,
				sortBy,
				limit,
			)
		},
	}

	cmd.Flags().BoolVarP(&bulk, "bulk", "b", false, "output content with %%%% <uuid> %%%% separators")
	cmd.Flags().BoolVarP(&first, "first", "f", false, "output first match content only")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "open matches in $EDITOR")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation for deletions in edit mode")
	cmd.Flags().StringVar(&frontmatter, "frontmatter", "", "include frontmatter in output (modes: extra, full, none)")
	cmd.Flag("frontmatter").NoOptDefVal = "extra"
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "created:desc", "sort order: field:dir (default newest first)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "max results (0 = unlimited)")
	cmd.Flags().BoolVarP(&useUpdated, "updated", "u", false, "match on updated timestamp instead of created")

	return cmd
}

// runDateCommand is a helper that runs a search filtered by a date range.
func runDateCommand(
	getVault func() *vault.Vault,
	jsonOutput *bool,
	dateRange dateparse.DateRange,
	useUpdated bool,
	bulk, first, edit, force bool,
	frontmatter string,
	sortBy string,
	limit int,
) error {
	vlt := getVault()
	if vlt == nil {
		return fmt.Errorf("vault not configured")
	}

	// Check mutual exclusivity of output formats
	modeCount := 0
	if bulk {
		modeCount++
	}
	if first {
		modeCount++
	}
	if modeCount > 1 {
		return fmt.Errorf("--bulk and --first are mutually exclusive")
	}

	// --edit is orthogonal to format, but incompatible with --json
	if edit && *jsonOutput {
		return fmt.Errorf("--json and --edit are incompatible")
	}

	// Parse frontmatter mode
	fmMode := FrontmatterMode(frontmatter)
	if frontmatter != "" && frontmatter != "none" && frontmatter != "extra" && frontmatter != "full" {
		return fmt.Errorf("invalid frontmatter mode: %s (use: none, extra, full)", frontmatter)
	}

	// Create date matcher
	var matcher QueryMatcher
	if useUpdated {
		matcher = func(n *note.Note) bool {
			return dateRange.Contains(n.Updated)
		}
	} else {
		matcher = func(n *note.Note) bool {
			return dateRange.Contains(n.Created)
		}
	}

	// Parse sort fields
	var sortFields []SortField
	var err error
	if sortBy != "" {
		sortFields, err = parseSort(sortBy)
		if err != nil {
			return fmt.Errorf("invalid sort: %w", err)
		}
	}

	// Find matching notes
	results, err := searchNotes(vlt, matcher)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Sort results
	if len(sortFields) > 0 {
		sortResults(results, sortFields)
	}

	// Apply limit
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	// No results
	if len(results) == 0 {
		if *jsonOutput {
			fmt.Println("[]")
		}
		return ErrNoMatches
	}

	// Output based on mode
	if edit {
		// --first limits edit to first match only
		if first && len(results) > 1 {
			results = results[:1]
		}
		return handleEdit(vlt, results, force, fmMode)
	}

	if bulk {
		return outputBulk(results, fmMode)
	}

	if first {
		return outputFirst(results, fmMode)
	}

	if *jsonOutput {
		return outputJSON(results, fmMode)
	}

	// Default: list of paths (with optional frontmatter)
	return outputPaths(results, fmMode)
}
