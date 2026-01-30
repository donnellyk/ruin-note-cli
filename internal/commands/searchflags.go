package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// errMutuallyExclusive returns an error for mutually exclusive flags.
func errMutuallyExclusive(a, b string) error {
	return fmt.Errorf("%s and %s are mutually exclusive", a, b)
}

// SearchFlags holds the common flags shared by search-like commands.
type SearchFlags struct {
	Bulk        bool
	First       bool
	Edit        bool
	Force       bool
	Frontmatter string
	Sort        string
	Limit       int
}

// AddSearchFlags adds the shared search output flags to a command.
// defaultSort can be empty for no default, or a value like "created:desc".
func AddSearchFlags(cmd *cobra.Command, f *SearchFlags, defaultSort string) {
	cmd.Flags().BoolVarP(&f.Bulk, "bulk", "b", false, "output content with %%%% <uuid> %%%% separators")
	cmd.Flags().BoolVar(&f.First, "first", false, "output first match content only")
	cmd.Flags().BoolVarP(&f.Edit, "edit", "e", false, "open matches in $EDITOR")
	cmd.Flags().BoolVarP(&f.Force, "force", "f", false, "skip confirmation for deletions in edit mode")
	cmd.Flags().StringVar(&f.Frontmatter, "frontmatter", "", "include frontmatter in output (modes: extra, full, none)")
	cmd.Flag("frontmatter").NoOptDefVal = "extra" // --frontmatter without value defaults to "extra"
	cmd.Flags().StringVarP(&f.Sort, "sort", "s", defaultSort, "sort order: field:dir (e.g., created:desc)")
	cmd.Flags().IntVarP(&f.Limit, "limit", "l", 0, "max results (0 = unlimited)")
}

// ValidateSearchFlags checks for mutual exclusivity and other validation rules.
func ValidateSearchFlags(f *SearchFlags, jsonOutput bool) error {
	// Check mutual exclusivity of output formats
	modeCount := 0
	if f.Bulk {
		modeCount++
	}
	if f.First {
		modeCount++
	}
	if modeCount > 1 {
		return errMutuallyExclusive("--bulk", "--first")
	}

	// --edit is orthogonal to format, but incompatible with --json
	if f.Edit && jsonOutput {
		return errMutuallyExclusive("--json", "--edit")
	}

	return nil
}
