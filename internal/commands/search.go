package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"

	"kvnd/ruin-note-cli/internal/dateparse"
	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// ErrNoMatches is returned when a search finds no results.
// This allows the caller to distinguish between errors and no matches.
var ErrNoMatches = fmt.Errorf("no matches found")

// FrontmatterMode controls how frontmatter is included in output.
type FrontmatterMode string

const (
	FrontmatterNone  FrontmatterMode = "none"  // Hide frontmatter (default)
	FrontmatterExtra FrontmatterMode = "extra" // Show only user-defined fields
	FrontmatterFull  FrontmatterMode = "full"  // Show complete frontmatter
)

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

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for notes",
		Long: `Search for notes matching the given query.

Query syntax:
  - Tag search: #tagname (requires # prefix)
  - Text search: word (case-insensitive)
  - AND (explicit): term1 && term2
  - AND (implicit): term1 term2 (space-separated)

Date filters:
  - created:DATE     Notes created on date
  - updated:DATE     Notes updated on date
  - on:DATE          Alias for created:DATE
  - before:DATE      Notes created before date (exclusive)
  - after:DATE       Notes created after date (exclusive)
  - between:D1,D2    Notes created between dates (inclusive)

Date formats:
  - Exact: 2025-01-28, 2025-01, 2025
  - Natural: today, yesterday, tomorrow
  - Relative: this-week, last-week, this-month, last-month
  - Duration: 7d, 2w, 3m (last N days/weeks/months)

Other filters:
  - title:TEXT       Notes with title containing text
  - path:TEXT        Notes with path containing text

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

  # Date filters
  ruin search "created:today"
  ruin search "#daily && created:this-week"
  ruin search "updated:7d"
  ruin search "between:2025-01-01,2025-01-31"
  ruin search "before:last-month"

  # Title and path filters
  ruin search "title:meeting"
  ruin search "path:projects/"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			// Validate flags
			if err := ValidateSearchFlags(&flags, *jsonOutput); err != nil {
				return err
			}

			// Parse query
			query := strings.Join(args, " ")
			matcher, err := parseQuery(query)
			if err != nil {
				return fmt.Errorf("invalid query: %w", err)
			}

			// Parse sort fields
			var sortFields []SortField
			if flags.Sort != "" {
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

			// Find matching notes
			results, err := searchNotesWithOptions(vlt, matcher, opts)
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

	// Check for filter prefixes (field:value)
	if idx := strings.Index(term, ":"); idx > 0 {
		field := strings.ToLower(term[:idx])
		value := term[idx+1:]

		switch field {
		case "created":
			return createdDateMatcher(value)
		case "updated":
			return updatedDateMatcher(value)
		case "before":
			return beforeDateMatcher(value)
		case "after":
			return afterDateMatcher(value)
		case "on":
			return createdDateMatcher(value) // alias for created:
		case "between":
			return betweenDateMatcher(value)
		case "title":
			return titleMatcher(value), nil
		case "path":
			return pathMatcher(value), nil
		case "parent":
			return parentMatcher(value), nil
		}
		// If not a recognized filter, fall through to text search
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

// titleMatcher returns a matcher that checks if a note's title contains the given text.
func titleMatcher(text string) QueryMatcher {
	textLower := strings.ToLower(text)
	return func(n *note.Note) bool {
		return strings.Contains(strings.ToLower(n.Title), textLower)
	}
}

// parentMatcher returns a matcher for parent filter.
// "none" matches notes with no parent. Any other value matches by parent UUID.
func parentMatcher(value string) QueryMatcher {
	if strings.ToLower(value) == "none" {
		return func(n *note.Note) bool {
			return n.Parent == ""
		}
	}
	return func(n *note.Note) bool {
		return n.Parent == value
	}
}

// pathMatcher returns a matcher that checks if a note's path contains the given text.
func pathMatcher(text string) QueryMatcher {
	textLower := strings.ToLower(text)
	return func(n *note.Note) bool {
		return strings.Contains(strings.ToLower(n.FilePath), textLower)
	}
}

// createdDateMatcher returns a matcher for created date filter.
// Supports exact dates, months, years, and natural language dates.
func createdDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for created filter: %w", err)
	}
	return func(n *note.Note) bool {
		return r.Contains(n.Created)
	}, nil
}

// updatedDateMatcher returns a matcher for updated date filter.
func updatedDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for updated filter: %w", err)
	}
	return func(n *note.Note) bool {
		return r.Contains(n.Updated)
	}, nil
}

// beforeDateMatcher returns a matcher for notes created before a date.
func beforeDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for before filter: %w", err)
	}
	// Before the start of the parsed range
	return func(n *note.Note) bool {
		return n.Created.Before(r.Start)
	}, nil
}

// afterDateMatcher returns a matcher for notes created after a date.
func afterDateMatcher(value string) (QueryMatcher, error) {
	r, err := dateparse.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid date for after filter: %w", err)
	}
	// After the end of the parsed range
	return func(n *note.Note) bool {
		return !n.Created.Before(r.End)
	}, nil
}

// betweenDateMatcher returns a matcher for notes created between two dates.
// Format: DATE,DATE (e.g., "2025-01-01,2025-01-31" or "last-month,today")
func betweenDateMatcher(value string) (QueryMatcher, error) {
	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("between filter requires two dates separated by comma (e.g., between:2025-01-01,2025-01-31)")
	}

	startRange, err := dateparse.Parse(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid start date for between filter: %w", err)
	}

	endRange, err := dateparse.Parse(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid end date for between filter: %w", err)
	}

	// Range is from start of first date to end of second date (inclusive)
	return func(n *note.Note) bool {
		return !n.Created.Before(startRange.Start) && n.Created.Before(endRange.End)
	}, nil
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
		case "created", "updated", "title", "order":
			// valid
		default:
			return nil, fmt.Errorf("invalid sort field: %s (use created, updated, title, or order)", field.Field)
		}

		fields = append(fields, field)
	}

	return fields, nil
}

// SearchOptions controls search behavior for performance optimizations.
type SearchOptions struct {
	// Limit is the maximum number of results to return (0 = unlimited).
	// When set and no sorting is requested, enables early termination.
	Limit int
}

// searchNotes finds all notes matching the query.
func searchNotes(vlt *vault.Vault, matcher QueryMatcher) ([]SearchResult, error) {
	return searchNotesWithOptions(vlt, matcher, SearchOptions{})
}

// searchNotesWithOptions finds notes with performance optimizations.
// Uses concurrent file reading for improved performance.
func searchNotesWithOptions(vlt *vault.Vault, matcher QueryMatcher, opts SearchOptions) ([]SearchResult, error) {
	notePaths, err := vlt.ListNotes()
	if err != nil {
		return nil, err
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8 // Cap at 8 workers to avoid excessive parallelism
	}
	if len(notePaths) < numWorkers {
		numWorkers = len(notePaths)
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	// Channel for paths to process
	pathsChan := make(chan string, len(notePaths))
	for _, path := range notePaths {
		pathsChan <- path
	}
	close(pathsChan)

	// Channel for results
	resultsChan := make(chan SearchResult, len(notePaths))

	// Worker function
	var wg sync.WaitGroup
	processNote := func() {
		defer wg.Done()
		for path := range pathsChan {
			n, err := note.Load(path)
			if err != nil {
				continue
			}

			if matcher(n) {
				resultsChan <- SearchResult{
					Path:   path,
					UUID:   n.UUID,
					Title:  n.Title,
					Tags:   n.Tags,
					Parent: n.Parent,
					note:   n,
				}
			}
		}
	}

	// Start workers
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go processNote()
	}

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var results []SearchResult
	for result := range resultsChan {
		results = append(results, result)

		// Early termination (only effective when limit is set and no sorting)
		if opts.Limit > 0 && len(results) >= opts.Limit {
			// Note: We can't truly stop workers early with buffered channels,
			// but we stop collecting more results than needed
			break
		}
	}

	// If we broke early due to limit, drain remaining results
	// This ensures goroutines can finish
	if opts.Limit > 0 && len(results) >= opts.Limit {
		go func() {
			for range resultsChan {
				// Drain
			}
		}()
	}

	return results, nil
}

// sortResults sorts the results by the given fields.
func sortResults(results []SearchResult, fields []SortField) {
	sort.Slice(results, func(i, j int) bool {
		for _, f := range fields {
			// For order field, unset (nil) always sorts last regardless of direction
			if f.Field == "order" {
				aSet, bSet := results[i].note.Order != nil, results[j].note.Order != nil
				if aSet != bSet {
					return aSet // set before unset
				}
			}
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
	case "order":
		// nil check is handled in sortResults (unset always sorts last)
		aOrd, bOrd := 0, 0
		if a.note.Order != nil {
			aOrd = *a.note.Order
		}
		if b.note.Order != nil {
			bOrd = *b.note.Order
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

// outputPaths outputs results as paths with optional frontmatter info.
func outputPaths(results []SearchResult, fmMode FrontmatterMode) error {
	for _, r := range results {
		fmt.Println(r.Path)
		if fmMode == FrontmatterExtra && len(r.note.Extra) > 0 {
			fmt.Printf("  %s\n", formatExtraFields(r.note.Extra))
		} else if fmMode == FrontmatterFull {
			fmt.Printf("  uuid=%s, created=%s, updated=%s\n",
				r.UUID,
				r.note.Created.Format("2006-01-02"),
				r.note.Updated.Format("2006-01-02"))
			if len(r.note.Extra) > 0 {
				fmt.Printf("  %s\n", formatExtraFields(r.note.Extra))
			}
		}
	}
	return nil
}

// formatExtraFields formats extra frontmatter fields as key=value pairs.
func formatExtraFields(extra map[string]interface{}) string {
	if len(extra) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(extra))
	for k, v := range extra {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(pairs, ", ")
}

// outputJSON outputs results as JSON.
func outputJSON(results []SearchResult, fmMode FrontmatterMode, includeContent, stripGlobalTags, stripTitle bool) error {
	// Create output with optional frontmatter fields
	type jsonResult struct {
		Path    string                 `json:"path"`
		UUID    string                 `json:"uuid"`
		Title   string                 `json:"title,omitempty"`
		Tags    []string               `json:"tags,omitempty"`
		Parent  string                 `json:"parent,omitempty"`
		Created string                 `json:"created,omitempty"`
		Updated string                 `json:"updated,omitempty"`
		Extra   map[string]interface{} `json:"extra,omitempty"`
		Content string                 `json:"content,omitempty"`
	}

	output := make([]jsonResult, len(results))
	for i, r := range results {
		jr := jsonResult{
			Path:    r.Path,
			UUID:    r.UUID,
			Title:   r.Title,
			Tags:    r.Tags,
			Parent:  r.note.Parent,
			Created: r.note.Created.Format(note.TimeFormat),
			Updated: r.note.Updated.Format(note.TimeFormat),
		}

		if fmMode == FrontmatterExtra && len(r.note.Extra) > 0 {
			jr.Extra = r.note.Extra
		} else if fmMode == FrontmatterFull && len(r.note.Extra) > 0 {
			jr.Extra = r.note.Extra
		}

		// Include content if requested
		if includeContent {
			content := r.note.Content

			// Include frontmatter in content if extra or full mode
			if fmMode == FrontmatterExtra || fmMode == FrontmatterFull {
				if serialized, err := r.note.Serialize(); err == nil {
					content = serialized
				}
			}

			// Apply stripping options
			if stripTitle {
				content = note.StripTitle(content)
			}
			if stripGlobalTags {
				content = note.StripGlobalTags(content, r.note.InlineTags)
			}

			jr.Content = content
		}

		output[i] = jr
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// outputBulk outputs results in bulk format with %%%% <uuid> %%%% separators.
func outputBulk(results []SearchResult, fmMode FrontmatterMode) error {
	entries := make([]note.BulkEntry, len(results))
	for i, r := range results {
		content := r.note.Content
		if fmMode == FrontmatterFull {
			// Include full frontmatter in the content
			serialized, err := r.note.Serialize()
			if err == nil {
				content = serialized
			}
		}
		entries[i] = note.BulkEntry{
			UUID:    r.UUID,
			Content: content,
		}
	}
	return note.FormatBulk(entries, os.Stdout)
}

// outputFirst outputs the first result's content.
func outputFirst(results []SearchResult, fmMode FrontmatterMode) error {
	if len(results) == 0 {
		return nil
	}

	var output string
	if fmMode == FrontmatterFull {
		serialized, err := results[0].note.Serialize()
		if err == nil {
			output = serialized
		} else {
			output = results[0].note.Content
		}
	} else {
		output = results[0].note.Content
	}

	fmt.Print(output)
	if !strings.HasSuffix(output, "\n") {
		fmt.Println()
	}
	return nil
}

// handleEdit opens results in $EDITOR and saves changes.
func handleEdit(vlt *vault.Vault, results []SearchResult, force bool, fmMode FrontmatterMode) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// Single note: use simple format without bulk separators
	if len(results) == 1 {
		return handleEditSingle(vlt, results[0], force, fmMode, editor)
	}

	// Multiple notes: use bulk format
	return handleEditBulk(vlt, results, force, fmMode, editor)
}

// handleEditSingle handles editing a single note without bulk separators.
func handleEditSingle(vlt *vault.Vault, result SearchResult, force bool, fmMode FrontmatterMode, editor string) error {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "ruin-edit-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Prepare content
	content := result.note.Content
	if fmMode == FrontmatterFull {
		serialized, err := result.note.Serialize()
		if err == nil {
			content = serialized
		}
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	originalContent := content

	// Open editor
	cmd := exec.Command("sh", "-c", editor+" \"$1\"", "sh", tmpPath)
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

	// Check for deletion (empty content)
	if strings.TrimSpace(modifiedContent) == "" {
		if !force {
			if !isTerminal(os.Stderr) {
				return fmt.Errorf("deletion requires --force in non-interactive mode")
			}

			fmt.Fprintf(os.Stderr, "The following 1 note(s) will be deleted:\n")
			fmt.Fprintf(os.Stderr, "  - %s\n", result.Path)
			fmt.Fprint(os.Stderr, "Continue? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}

		if err := os.Remove(result.Path); err != nil {
			return fmt.Errorf("failed to delete %s: %w", result.Path, err)
		}
		vlt.DecrementTagsIndex(result.note.Tags)
		vlt.RemoveTitleEntry(result.note.UUID)
		fmt.Fprintf(os.Stderr, "Modified: 0, Deleted: 1\n")
		return nil
	}

	// Apply changes
	if strings.HasPrefix(strings.TrimLeft(modifiedContent, "\n\r"), "---") {
		fm, body, err := note.ParseFrontmatter(modifiedContent)
		if err != nil {
			return fmt.Errorf("failed to parse frontmatter: %w", err)
		}

		if fm.UUID != "" && fm.UUID != result.note.UUID {
			return fmt.Errorf("cannot change UUID")
		}

		if len(fm.Extra) > 0 {
			if result.note.Extra == nil {
				result.note.Extra = make(map[string]interface{})
			}
			for k, v := range fm.Extra {
				result.note.Extra[k] = v
			}
		}

		if len(fm.Tags) > 0 {
			result.note.Tags = fm.Tags
		}

		result.note.Content = body
	} else {
		result.note.Content = modifiedContent
		result.note.RefreshTags()
	}

	result.note.SetTimestamps()

	if err := result.note.Save(); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	vlt.UpdateTagsIndex(result.note.Tags)
	vlt.UpdateTitleEntry(result.note.UUID, result.note.Title, result.note.FilePath, result.note.Parent)
	fmt.Fprintf(os.Stderr, "Modified: 1, Deleted: 0\n")
	return nil
}

// handleEditBulk handles editing multiple notes with bulk format.
func handleEditBulk(vlt *vault.Vault, results []SearchResult, force bool, fmMode FrontmatterMode, editor string) error {
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
		content := r.note.Content
		if fmMode == FrontmatterFull {
			// Include full frontmatter in the content for editing
			serialized, err := r.note.Serialize()
			if err == nil {
				content = serialized
			}
		}
		entries[i] = note.BulkEntry{
			UUID:    r.UUID,
			Content: content,
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

	// Open editor - use shell to handle $EDITOR with arguments (e.g., "code --wait")
	cmd := exec.Command("sh", "-c", editor+" \"$1\"", "sh", tmpPath)
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
	return applyBulkChanges(vlt, originalContent, modifiedContent, results, force)
}

// applyBulkChanges applies changes from bulk edit.
func applyBulkChanges(vlt *vault.Vault, original, modified string, results []SearchResult, force bool) error {
	// Parse original into uuid -> content map
	originalMap := note.ParseBulk(original)
	modifiedMap := note.ParseBulk(modified)

	// Build uuid -> result map
	resultMap := make(map[string]SearchResult)
	for _, r := range results {
		resultMap[r.UUID] = r
	}

	// First pass: collect modifications and deletions
	var toModify []string
	var toDelete []string

	for uuid, origContent := range originalMap {
		modContent, exists := modifiedMap[uuid]

		if !exists {
			toDelete = append(toDelete, uuid)
		} else if modContent != origContent {
			toModify = append(toModify, uuid)
		}
	}

	// Check for new UUIDs (error case)
	var errors []string
	for uuid := range modifiedMap {
		if _, exists := originalMap[uuid]; !exists {
			errors = append(errors, fmt.Sprintf("New UUID found: %s (use 'log' to create new notes)", uuid))
		}
	}

	// Handle deletions - require confirmation or --force
	if len(toDelete) > 0 && !force {
		// Check if stderr is a TTY for interactive confirmation
		if !isTerminal(os.Stderr) {
			return fmt.Errorf("deletions require --force in non-interactive mode")
		}

		fmt.Fprintf(os.Stderr, "The following %d note(s) will be deleted:\n", len(toDelete))
		for _, uuid := range toDelete {
			result, ok := resultMap[uuid]
			if ok {
				fmt.Fprintf(os.Stderr, "  - %s\n", result.Path)
			} else {
				fmt.Fprintf(os.Stderr, "  - UUID: %s (path not found)\n", uuid)
			}
		}
		fmt.Fprint(os.Stderr, "Continue? [y/N]: ")

		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
	}

	// Apply modifications
	var modifiedCount int
	for _, uuid := range toModify {
		result, ok := resultMap[uuid]
		if !ok {
			errors = append(errors, fmt.Sprintf("UUID not found: %s", uuid))
			continue
		}

		modContent := modifiedMap[uuid]

		// Check if modified content includes frontmatter
		if strings.HasPrefix(strings.TrimLeft(modContent, "\n\r"), "---") {
			// Parse frontmatter from modified content
			fm, body, err := note.ParseFrontmatter(modContent)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to parse frontmatter for %s: %v", result.Path, err))
				continue
			}

			// Protect immutable fields
			if fm.UUID != "" && fm.UUID != result.note.UUID {
				errors = append(errors, fmt.Sprintf("Cannot change UUID for %s", result.Path))
				continue
			}

			// Apply allowed frontmatter changes
			// Extra fields can be modified
			if len(fm.Extra) > 0 {
				if result.note.Extra == nil {
					result.note.Extra = make(map[string]interface{})
				}
				for k, v := range fm.Extra {
					result.note.Extra[k] = v
				}
			}

			// Tags from frontmatter override extracted tags if explicitly set
			if len(fm.Tags) > 0 {
				result.note.Tags = fm.Tags
			}

			// Set content (without frontmatter)
			result.note.Content = body
		} else {
			// No frontmatter - just update content
			result.note.Content = modContent
		}

		// Refresh tags from content (unless overridden by frontmatter)
		if !strings.HasPrefix(strings.TrimLeft(modContent, "\n\r"), "---") {
			result.note.RefreshTags()
		}
		result.note.SetTimestamps()

		if err := result.note.Save(); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to save %s: %v", result.Path, err))
			continue
		}

		// Update tags index
		vlt.UpdateTagsIndex(result.note.Tags)
		// Update titles index
		vlt.UpdateTitleEntry(result.note.UUID, result.note.Title, result.note.FilePath, result.note.Parent)
		modifiedCount++
	}

	// Apply deletions
	var deletedCount int
	for _, uuid := range toDelete {
		result, ok := resultMap[uuid]
		if !ok {
			errors = append(errors, fmt.Sprintf("UUID not found in vault: %s", uuid))
			continue
		}

		if err := os.Remove(result.Path); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to delete %s: %v", result.Path, err))
			continue
		}

		// Decrement tags for deleted note
		vlt.DecrementTagsIndex(result.note.Tags)
		vlt.RemoveTitleEntry(result.note.UUID)
		deletedCount++
	}

	// Report results
	fmt.Fprintf(os.Stderr, "Modified: %d, Deleted: %d\n", modifiedCount, deletedCount)
	if len(errors) > 0 {
		fmt.Fprintf(os.Stderr, "Errors:\n")
		for _, e := range errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
	}

	return nil
}

