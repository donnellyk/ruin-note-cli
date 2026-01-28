package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/kevin/ruin-note-cli/internal/note"
	"github.com/kevin/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// ErrNoMatches is returned when a search finds no results.
// This allows the caller to distinguish between errors and no matches.
var ErrNoMatches = fmt.Errorf("no matches found")

// SearchResult represents a single search result.
type SearchResult struct {
	Path  string   `json:"path"`
	UUID  string   `json:"uuid"`
	Title string   `json:"title,omitempty"`
	Tags  []string `json:"tags,omitempty"`
	note  *note.Note
}

// SortField represents a field and direction for sorting.
type SortField struct {
	Field     string
	Ascending bool
}

// NewSearchCmd creates the search command.
func NewSearchCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		bulk   bool
		first  bool
		edit   bool
		sortBy string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for notes",
		Long: `Search for notes matching the given query.

Query syntax (MVP):
  - Tag search: #tagname
  - Text search: word (case-insensitive)
  - AND (explicit): term1 && term2
  - AND (implicit): term1 term2 (space-separated)

Examples:
  ruin search "#daily"
  ruin search "#meeting project-alpha"
  ruin search "#todo && urgent"`,
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
  ruin search "#log" -s created:desc -l 10`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			// Check mutual exclusivity
			modeCount := 0
			if bulk {
				modeCount++
			}
			if first {
				modeCount++
			}
			if edit {
				modeCount++
			}
			if modeCount > 1 {
				return fmt.Errorf("--bulk, --first, and --edit are mutually exclusive")
			}

			// Parse query
			query := strings.Join(args, " ")
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
				// Return special error for no matches (caller can check and set exit code)
				return ErrNoMatches
			}

			// Output based on mode
			if edit {
				return handleEdit(vlt, results)
			}

			if bulk {
				return outputBulk(results)
			}

			if first {
				return outputFirst(results)
			}

			if *jsonOutput {
				return outputJSON(results)
			}

			// Default: list of paths
			for _, r := range results {
				fmt.Println(r.Path)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&bulk, "bulk", "b", false, "output content with %%%% <uuid> %%%% separators")
	cmd.Flags().BoolVarP(&first, "first", "f", false, "output first match content only")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "open matches in $EDITOR")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "", "sort order: field:dir (e.g., created:desc)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "max results (0 = unlimited)")

	return cmd
}

// QueryMatcher is a function that tests if a note matches the query.
type QueryMatcher func(n *note.Note) bool

// parseQuery parses a search query string into a matcher function.
// MVP supports: tag search, text search, && (AND), space (implicit AND)
func parseQuery(query string) (QueryMatcher, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	// Split by && first
	parts := strings.Split(query, "&&")
	var matchers []QueryMatcher

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by space for implicit AND
		terms := splitTerms(part)
		for _, term := range terms {
			m, err := parseTermMatcher(term)
			if err != nil {
				return nil, err
			}
			matchers = append(matchers, m)
		}
	}

	if len(matchers) == 0 {
		return nil, fmt.Errorf("no valid search terms")
	}

	// Combine all matchers with AND
	return func(n *note.Note) bool {
		for _, m := range matchers {
			if !m(n) {
				return false
			}
		}
		return true
	}, nil
}

// splitTerms splits a query part into individual terms.
// Preserves spaced tags like #daily note#
func splitTerms(part string) []string {
	var terms []string
	var current strings.Builder
	inSpacedTag := false

	for i := 0; i < len(part); i++ {
		ch := part[i]

		if ch == '#' {
			if inSpacedTag {
				// End of spaced tag
				current.WriteByte(ch)
				terms = append(terms, current.String())
				current.Reset()
				inSpacedTag = false
				continue
			}

			// Potential start of tag
			if current.Len() > 0 {
				terms = append(terms, current.String())
				current.Reset()
			}
			current.WriteByte(ch)

			// Check if this is a spaced tag
			// A spaced tag is #text with spaces# where the closing # is NOT followed by a word char
			// and NOT preceded by another #
			rest := part[i+1:]
			if idx := strings.Index(rest, "#"); idx > 0 {
				// Check if there's a space in the potential tag content
				// AND the content doesn't contain another # before the closing one
				potentialContent := rest[:idx]
				hasSpace := strings.ContainsAny(potentialContent, " \t")
				// Check what's after the closing #
				afterClosing := ""
				if idx+1 < len(rest) {
					afterClosing = string(rest[idx+1])
				}
				// It's a spaced tag only if:
				// 1. Has space in content
				// 2. Closing # is NOT followed by a word char (which would make it another tag)
				if hasSpace && (afterClosing == "" || afterClosing == " " || afterClosing == "\t") {
					inSpacedTag = true
				}
			}
			continue
		}

		if ch == ' ' || ch == '\t' {
			if inSpacedTag {
				current.WriteByte(ch)
				continue
			}
			if current.Len() > 0 {
				terms = append(terms, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(ch)
	}

	if current.Len() > 0 {
		terms = append(terms, current.String())
	}

	return terms
}

// parseTermMatcher creates a matcher for a single search term.
func parseTermMatcher(term string) (QueryMatcher, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, fmt.Errorf("empty term")
	}

	// Tag search
	if strings.HasPrefix(term, "#") {
		return tagMatcher(term), nil
	}

	// Text search (case-insensitive)
	return textMatcher(term), nil
}

// tagMatcher returns a matcher that checks if a note has the given tag.
func tagMatcher(tag string) QueryMatcher {
	tagNorm := note.NormalizeTag(tag)
	return func(n *note.Note) bool {
		for _, t := range n.Tags {
			if note.NormalizeTag(t) == tagNorm {
				return true
			}
		}
		return false
	}
}

// textMatcher returns a matcher that checks if a note contains the given text.
func textMatcher(text string) QueryMatcher {
	textLower := strings.ToLower(text)
	return func(n *note.Note) bool {
		// Search in content
		if strings.Contains(strings.ToLower(n.Content), textLower) {
			return true
		}
		// Search in title
		if strings.Contains(strings.ToLower(n.Title), textLower) {
			return true
		}
		return false
	}
}

// parseSort parses a sort specification string.
// Format: field:direction[,field:direction]
func parseSort(s string) ([]SortField, error) {
	var fields []SortField

	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		field := SortField{Ascending: true} // default

		if idx := strings.Index(part, ":"); idx > 0 {
			field.Field = strings.ToLower(part[:idx])
			dir := strings.ToLower(part[idx+1:])
			switch dir {
			case "asc":
				field.Ascending = true
			case "desc":
				field.Ascending = false
			default:
				return nil, fmt.Errorf("invalid sort direction: %s (use asc or desc)", dir)
			}
		} else {
			field.Field = strings.ToLower(part)
		}

		// Validate field
		switch field.Field {
		case "created", "updated", "title":
			// valid
		default:
			return nil, fmt.Errorf("invalid sort field: %s (use created, updated, or title)", field.Field)
		}

		fields = append(fields, field)
	}

	return fields, nil
}

// searchNotes finds all notes matching the query.
func searchNotes(vlt *vault.Vault, matcher QueryMatcher) ([]SearchResult, error) {
	notePaths, err := vlt.ListNotes()
	if err != nil {
		return nil, err
	}

	var results []SearchResult

	for _, path := range notePaths {
		n, err := note.Load(path)
		if err != nil {
			// Skip notes that can't be parsed
			continue
		}

		if matcher(n) {
			results = append(results, SearchResult{
				Path:  path,
				UUID:  n.UUID,
				Title: n.Title,
				Tags:  n.Tags,
				note:  n,
			})
		}
	}

	return results, nil
}

// sortResults sorts the results by the given fields.
func sortResults(results []SearchResult, fields []SortField) {
	sort.Slice(results, func(i, j int) bool {
		for _, f := range fields {
			cmp := compareResults(results[i], results[j], f.Field)
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

// compareResults compares two results by the given field.
func compareResults(a, b SearchResult, field string) int {
	switch field {
	case "created":
		if a.note.Created.Before(b.note.Created) {
			return -1
		}
		if a.note.Created.After(b.note.Created) {
			return 1
		}
		return 0
	case "updated":
		if a.note.Updated.Before(b.note.Updated) {
			return -1
		}
		if a.note.Updated.After(b.note.Updated) {
			return 1
		}
		return 0
	case "title":
		return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	}
	return 0
}

// outputJSON outputs results as JSON.
func outputJSON(results []SearchResult) error {
	// Create output without the internal note field
	type jsonResult struct {
		Path  string   `json:"path"`
		UUID  string   `json:"uuid"`
		Title string   `json:"title,omitempty"`
		Tags  []string `json:"tags,omitempty"`
	}

	output := make([]jsonResult, len(results))
	for i, r := range results {
		output[i] = jsonResult{
			Path:  r.Path,
			UUID:  r.UUID,
			Title: r.Title,
			Tags:  r.Tags,
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// outputBulk outputs results in bulk format with %%%% <uuid> %%%% separators.
func outputBulk(results []SearchResult) error {
	entries := make([]note.BulkEntry, len(results))
	for i, r := range results {
		entries[i] = note.BulkEntry{
			UUID:    r.UUID,
			Content: r.note.Content,
		}
	}
	return note.FormatBulk(entries, os.Stdout)
}

// outputFirst outputs the first result's content.
func outputFirst(results []SearchResult) error {
	if len(results) == 0 {
		return nil
	}
	// Output content without frontmatter
	fmt.Print(results[0].note.Content)
	if !strings.HasSuffix(results[0].note.Content, "\n") {
		fmt.Println()
	}
	return nil
}

// handleEdit opens results in $EDITOR and saves changes.
func handleEdit(vlt *vault.Vault, results []SearchResult) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// Create temp file with bulk format
	tmpFile, err := os.CreateTemp("", "ruin-edit-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write original content
	entries := make([]note.BulkEntry, len(results))
	for i, r := range results {
		entries[i] = note.BulkEntry{
			UUID:    r.UUID,
			Content: r.note.Content,
		}
	}

	var original strings.Builder
	if err := note.FormatBulk(entries, &original); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to format bulk content: %w", err)
	}

	if _, err := tmpFile.WriteString(original.String()); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Save original for comparison
	originalContent := original.String()

	// Open editor
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	// Read modified content
	modifiedBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read modified file: %w", err)
	}
	modifiedContent := string(modifiedBytes)

	// If no changes, nothing to do
	if modifiedContent == originalContent {
		fmt.Fprintln(os.Stderr, "No changes made")
		return nil
	}

	// Parse and apply changes
	return applyBulkChanges(vlt, originalContent, modifiedContent, results)
}

// applyBulkChanges applies changes from bulk edit.
func applyBulkChanges(vlt *vault.Vault, original, modified string, results []SearchResult) error {
	// Parse original into uuid -> content map
	originalMap := note.ParseBulk(original)
	modifiedMap := note.ParseBulk(modified)

	// Build uuid -> result map
	resultMap := make(map[string]SearchResult)
	for _, r := range results {
		resultMap[r.UUID] = r
	}

	var modifiedCount, deletedCount int
	var errors []string

	// Check for modifications and deletions
	for uuid, origContent := range originalMap {
		modContent, exists := modifiedMap[uuid]

		if !exists {
			// Note was deleted - we would need --force confirmation
			// For now, just report it
			fmt.Fprintf(os.Stderr, "Note deleted (UUID: %s) - deletion requires --force\n", uuid)
			continue
		}

		if modContent != origContent {
			// Note was modified
			result, ok := resultMap[uuid]
			if !ok {
				errors = append(errors, fmt.Sprintf("UUID not found: %s", uuid))
				continue
			}

			// Update the note
			result.note.Content = modContent
			result.note.RefreshTags()
			result.note.SetTimestamps()

			if err := result.note.Save(); err != nil {
				errors = append(errors, fmt.Sprintf("Failed to save %s: %v", result.Path, err))
				continue
			}

			// Update tags index
			vlt.UpdateTagsIndex(result.note.Tags)
			modifiedCount++
		}
	}

	// Check for new UUIDs (error case)
	for uuid := range modifiedMap {
		if _, exists := originalMap[uuid]; !exists {
			errors = append(errors, fmt.Sprintf("New UUID found: %s (use 'log' to create new notes)", uuid))
		}
	}

	// Report results
	fmt.Fprintf(os.Stderr, "Modified: %d\n", modifiedCount)
	if deletedCount > 0 {
		fmt.Fprintf(os.Stderr, "Deleted: %d (skipped - use update --force)\n", deletedCount)
	}
	if len(errors) > 0 {
		fmt.Fprintf(os.Stderr, "Errors:\n")
		for _, e := range errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
	}

	return nil
}

