package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"kvnd/ruin-note-cli/internal/dateparse"
	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
)

// DateFilterMode specifies which timestamp(s) to match.
type DateFilterMode int

const (
	DateFilterBoth    DateFilterMode = iota // Match created OR updated (default)
	DateFilterCreated                       // Match only created
	DateFilterUpdated                       // Match only updated
)

// NewTodayCmd creates the today command.
func NewTodayCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var flags SearchFlags
	var onlyCreated, onlyUpdated bool

	cmd := &cobra.Command{
		Use:   "today",
		Short: "Show notes from today",
		Long: `Show all notes created or updated today (local timezone).

By default, matches notes where either the created OR updated timestamp is today.
Use --created or --updated to filter to only one timestamp type.

See also:
  ruin yesterday     Show notes from yesterday
  ruin search        Search with custom date filters`,
		Example: `  # List today's notes (created or updated)
  ruin today

  # Only notes created today
  ruin today --created

  # Only notes updated today
  ruin today --updated

  # Bulk export today's notes
  ruin today --bulk

  # JSON output for scripting
  ruin today --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := DateFilterBoth
			if onlyCreated && onlyUpdated {
				return fmt.Errorf("--created and --updated are mutually exclusive")
			}
			if onlyCreated {
				mode = DateFilterCreated
			} else if onlyUpdated {
				mode = DateFilterUpdated
			}

			return runDateCommand(
				getVault,
				jsonOutput,
				dateparse.Today(),
				mode,
				&flags,
			)
		},
	}

	AddSearchFlags(cmd, &flags, "created:desc")
	cmd.Flags().BoolVarP(&onlyCreated, "created", "c", false, "only match notes created on this date")
	cmd.Flags().BoolVarP(&onlyUpdated, "updated", "u", false, "only match notes updated on this date")

	return cmd
}

// NewYesterdayCmd creates the yesterday command.
func NewYesterdayCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var flags SearchFlags
	var onlyCreated, onlyUpdated bool

	cmd := &cobra.Command{
		Use:   "yesterday",
		Short: "Show notes from yesterday",
		Long: `Show all notes created or updated yesterday (local timezone).

By default, matches notes where either the created OR updated timestamp is yesterday.
Use --created or --updated to filter to only one timestamp type.

See also:
  ruin today         Show notes from today
  ruin search        Search with custom date filters`,
		Example: `  # List yesterday's notes (created or updated)
  ruin yesterday

  # Only notes created yesterday
  ruin yesterday --created

  # Only notes updated yesterday
  ruin yesterday --updated

  # Bulk export yesterday's notes
  ruin yesterday --bulk

  # JSON output for scripting
  ruin yesterday --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := DateFilterBoth
			if onlyCreated && onlyUpdated {
				return fmt.Errorf("--created and --updated are mutually exclusive")
			}
			if onlyCreated {
				mode = DateFilterCreated
			} else if onlyUpdated {
				mode = DateFilterUpdated
			}

			return runDateCommand(
				getVault,
				jsonOutput,
				dateparse.Yesterday(),
				mode,
				&flags,
			)
		},
	}

	AddSearchFlags(cmd, &flags, "created:desc")
	cmd.Flags().BoolVarP(&onlyCreated, "created", "c", false, "only match notes created on this date")
	cmd.Flags().BoolVarP(&onlyUpdated, "updated", "u", false, "only match notes updated on this date")

	return cmd
}

// runDateCommand is a helper that runs a search filtered by a date range.
func runDateCommand(
	getVault func() *vault.Vault,
	jsonOutput *bool,
	dateRange dateparse.DateRange,
	mode DateFilterMode,
	flags *SearchFlags,
) error {
	vlt := getVault()
	if vlt == nil {
		return fmt.Errorf("vault not configured")
	}

	// Validate flags
	if err := ValidateSearchFlags(flags, *jsonOutput); err != nil {
		return err
	}

	// Create date matcher based on mode
	var matcher QueryMatcher
	switch mode {
	case DateFilterCreated:
		matcher = func(n *note.Note) bool {
			return dateRange.Contains(n.Created)
		}
	case DateFilterUpdated:
		matcher = func(n *note.Note) bool {
			return dateRange.Contains(n.Updated)
		}
	default: // DateFilterBoth
		matcher = func(n *note.Note) bool {
			return dateRange.Contains(n.Created) || dateRange.Contains(n.Updated)
		}
	}

	info := MatcherInfo{NeedsBody: false}
	return executeSearch(vlt, matcher, info, flags, *jsonOutput, nil)
}
