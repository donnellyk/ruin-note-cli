package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kevin/ruin-note-cli/internal/note"
	"github.com/kevin/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// ErrUserAborted is returned when the user declines a confirmation prompt.
var ErrUserAborted = fmt.Errorf("user aborted")

// NewQueryCmd creates the query command with subcommands.
func NewQueryCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Manage saved queries",
		Long: `Manage saved search queries.

Subcommands:
  save    Save a named query
  list    List all saved queries
  delete  Delete a saved query
  run     Run a saved query`,
	}

	cmd.AddCommand(newQuerySaveCmd(getVault, jsonOutput))
	cmd.AddCommand(newQueryListCmd(getVault, jsonOutput))
	cmd.AddCommand(newQueryDeleteCmd(getVault, jsonOutput))
	cmd.AddCommand(newQueryRunCmd(getVault, jsonOutput))

	return cmd
}

// newQuerySaveCmd creates the "query save" subcommand.
func newQuerySaveCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "save <name> <query>",
		Short: "Save a named query",
		Long: `Save a search query with a name for later use.

Before saving, the query is tested and the number of matching notes is displayed.
You will be prompted to confirm unless --force is used.`,
		Example: `  # Save a query (interactive)
  ruin query save daily-work "#daily && #work"

  # Save without confirmation (for scripts)
  ruin query save daily-work "#daily && #work" --force

  # Save with JSON output
  ruin query save daily-work "#daily && #work" -f --json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			name := args[0]
			query := args[1]

			// Validate the query by parsing it
			matcher, err := parseQuery(query)
			if err != nil {
				return fmt.Errorf("invalid query: %w", err)
			}

			// Test the query to count matching notes
			matchCount, err := countMatches(vlt, matcher)
			if err != nil {
				return fmt.Errorf("failed to test query: %w", err)
			}

			// Show match count
			fmt.Fprintf(os.Stderr, "Query %q matches %d notes.\n", query, matchCount)

			// Confirmation logic
			if !force {
				if !isTerminal(os.Stderr) {
					return fmt.Errorf("non-interactive mode requires --force")
				}

				fmt.Fprintf(os.Stderr, "Save as %q? [y/N] ", name)
				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read response: %w", err)
				}

				response = strings.TrimSpace(strings.ToLower(response))
				if response != "y" && response != "yes" {
					return ErrUserAborted
				}
			}

			// Load existing queries
			index, err := vlt.LoadQueries()
			if err != nil {
				return fmt.Errorf("failed to load queries: %w", err)
			}

			// Check if name already exists and update or add
			found := false
			for i, q := range index.Queries {
				if q.Name == name {
					index.Queries[i].Query = query
					found = true
					break
				}
			}

			if !found {
				index.Queries = append(index.Queries, vault.QueryEntry{
					Name:  name,
					Query: query,
				})
			}

			// Save queries
			if err := vlt.SaveQueries(index); err != nil {
				return fmt.Errorf("failed to save query: %w", err)
			}

			// Output
			if *jsonOutput {
				output := struct {
					Name    string `json:"name"`
					Query   string `json:"query"`
					Matches int    `json:"matches"`
				}{
					Name:    name,
					Query:   query,
					Matches: matchCount,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Fprintf(os.Stderr, "Saved query %q\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}

// newQueryListCmd creates the "query list" subcommand.
func newQueryListCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all saved queries",
		Long:  `List all saved queries from .ruin/queries.yml.`,
		Example: `  # List queries
  ruin query list

  # List as JSON
  ruin query list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			index, err := vlt.LoadQueries()
			if err != nil {
				return fmt.Errorf("failed to load queries: %w", err)
			}

			if *jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(index.Queries)
			}

			if len(index.Queries) == 0 {
				fmt.Println("No saved queries")
				return nil
			}

			for _, q := range index.Queries {
				fmt.Printf("%s: %s\n", q.Name, q.Query)
			}
			return nil
		},
	}

	return cmd
}

// newQueryDeleteCmd creates the "query delete" subcommand.
func newQueryDeleteCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved query",
		Long:  `Delete a saved query by name.`,
		Example: `  # Delete a query (interactive)
  ruin query delete daily-work

  # Delete without confirmation
  ruin query delete daily-work --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			name := args[0]

			index, err := vlt.LoadQueries()
			if err != nil {
				return fmt.Errorf("failed to load queries: %w", err)
			}

			// Find the query
			found := -1
			for i, q := range index.Queries {
				if q.Name == name {
					found = i
					break
				}
			}

			if found == -1 {
				return fmt.Errorf("query not found: %s", name)
			}

			// Confirmation
			if !force {
				if !isTerminal(os.Stderr) {
					return fmt.Errorf("non-interactive mode requires --force")
				}

				fmt.Fprintf(os.Stderr, "Delete query %q? [y/N] ", name)
				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read response: %w", err)
				}

				response = strings.TrimSpace(strings.ToLower(response))
				if response != "y" && response != "yes" {
					return ErrUserAborted
				}
			}

			// Remove the query
			index.Queries = append(index.Queries[:found], index.Queries[found+1:]...)

			if err := vlt.SaveQueries(index); err != nil {
				return fmt.Errorf("failed to save queries: %w", err)
			}

			if *jsonOutput {
				output := struct {
					Deleted string `json:"deleted"`
				}{
					Deleted: name,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Printf("Deleted query %q\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}

// newQueryRunCmd creates the "query run" subcommand.
func newQueryRunCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		bulk        bool
		first       bool
		edit        bool
		force       bool
		frontmatter string
		sortBy      string
		limit       int
	)

	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Run a saved query",
		Long: `Run a saved query by name.

This is equivalent to running "ruin search <query>" with the saved query string.`,
		Example: `  # Run a saved query
  ruin query run daily-work

  # Run with output options
  ruin query run daily-work --bulk
  ruin query run daily-work --first
  ruin query run daily-work --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			name := args[0]

			// Load queries and find the named one
			index, err := vlt.LoadQueries()
			if err != nil {
				return fmt.Errorf("failed to load queries: %w", err)
			}

			var query string
			for _, q := range index.Queries {
				if q.Name == name {
					query = q.Query
					break
				}
			}

			if query == "" {
				return fmt.Errorf("query not found: %s", name)
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

			// Parse query
			matcher, err := parseQuery(query)
			if err != nil {
				return fmt.Errorf("invalid query: %w", err)
			}

			// Parse sort fields
			var sortFields []SortField
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
		},
	}

	cmd.Flags().BoolVarP(&bulk, "bulk", "b", false, "output content with %%%% <uuid> %%%% separators")
	cmd.Flags().BoolVarP(&first, "first", "f", false, "output first match content only")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "open matches in $EDITOR")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation for deletions in edit mode")
	cmd.Flags().StringVar(&frontmatter, "frontmatter", "", "include frontmatter in output (modes: extra, full, none)")
	cmd.Flag("frontmatter").NoOptDefVal = "extra"
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "", "sort order: field:dir (e.g., created:desc)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "max results (0 = unlimited)")

	return cmd
}

// countMatches counts how many notes match the given query.
func countMatches(vlt *vault.Vault, matcher QueryMatcher) (int, error) {
	notePaths, err := vlt.ListNotes()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, path := range notePaths {
		n, err := note.Load(path)
		if err != nil {
			continue
		}

		if matcher(n) {
			count++
		}
	}

	return count, nil
}

