package commands

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

// SearchOptions controls search behavior for performance optimizations.
type SearchOptions struct {
	// Limit caps the number of results (0 = unlimited). With no sort, enables early termination.
	Limit int
	// NeedFullNote forces a full load for matches even when the matcher only needs frontmatter.
	NeedFullNote bool
	// UUIDs constrains the search to only these UUIDs (empty = no constraint).
	UUIDs []string
}

func searchNotes(vlt *vault.Vault, matcher QueryMatcher, info MatcherInfo) ([]SearchResult, error) {
	return searchNotesWithOptions(vlt, matcher, info, SearchOptions{})
}

func searchNotesWithOptions(vlt *vault.Vault, matcher QueryMatcher, info MatcherInfo, opts SearchOptions) ([]SearchResult, error) {
	notePaths, err := vlt.ListNotes()
	if err != nil {
		return nil, err
	}

	if len(opts.UUIDs) > 0 {
		titles, err := vlt.LoadTitles()
		if err != nil {
			return nil, fmt.Errorf("failed to load titles for UUID filter: %w", err)
		}
		allowedPaths := make(map[string]bool, len(opts.UUIDs))
		for _, uuid := range opts.UUIDs {
			if entry, ok := titles.Titles[uuid]; ok {
				allowedPaths[entry.Path] = true
			}
		}
		filtered := notePaths[:0]
		for _, p := range notePaths {
			if allowedPaths[p] {
				filtered = append(filtered, p)
			}
		}
		notePaths = filtered
	}

	// Cap at 8 workers to avoid excessive parallelism.
	numWorkers := max(min(len(notePaths), min(runtime.NumCPU(), 8)), 1)

	pathsChan := make(chan string, len(notePaths))
	for _, path := range notePaths {
		pathsChan <- path
	}
	close(pathsChan)

	resultsChan := make(chan SearchResult, len(notePaths))

	var wg sync.WaitGroup
	processNote := func() {
		defer wg.Done()
		for path := range pathsChan {
			var n *note.Note
			var loadErr error

			if !info.NeedsBody {
				n, loadErr = note.LoadFrontmatterOnly(path)
			} else {
				n, loadErr = note.Load(path)
			}
			if loadErr != nil {
				continue
			}

			if matcher(n) {
				if !info.NeedsBody && opts.NeedFullNote {
					full, err := note.Load(path)
					if err != nil {
						continue
					}
					n = full
				}
				resultsChan <- SearchResult{
					Path:   path,
					UUID:   n.UUID,
					Title:  n.Title,
					Tags:   n.EffectiveGlobalTags(),
					Parent: n.Parent,
					note:   n,
				}
			}
		}
	}

	wg.Add(numWorkers)
	for range numWorkers {
		go processNote()
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var results []SearchResult
	for result := range resultsChan {
		results = append(results, result)

		// Early termination only when limit is set and no sorting requested.
		if opts.Limit > 0 && len(results) >= opts.Limit {
			break
		}
	}

	// Drain remaining results so goroutines can finish.
	if opts.Limit > 0 && len(results) >= opts.Limit {
		go func() {
			for range resultsChan {
			}
		}()
	}

	return results, nil
}

// sortResults sorts the results. Notes with unset Order sort last regardless of direction.
func sortResults(results []SearchResult, fields []SortField) {
	sort.Slice(results, func(i, j int) bool {
		for _, f := range fields {
			if f.Field == "order" {
				aSet, bSet := results[i].note.Order != nil, results[j].note.Order != nil
				if aSet != bSet {
					return aSet
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

func dispatchSearchResults(vlt *vault.Vault, results []SearchResult, flags *SearchFlags, jsonOutput bool, sortFields []SortField) error {
	if len(sortFields) > 0 {
		sortResults(results, sortFields)
		if flags.Limit > 0 && len(results) > flags.Limit {
			results = results[:flags.Limit]
		}
	}

	if len(results) == 0 {
		if jsonOutput {
			fmt.Println("[]")
		}
		return nil
	}

	fmMode := FrontmatterMode(flags.Frontmatter)
	if flags.Frontmatter != "" && flags.Frontmatter != "none" && flags.Frontmatter != "extra" && flags.Frontmatter != "full" {
		return fmt.Errorf("invalid frontmatter mode: %s (use: none, extra, full)", flags.Frontmatter)
	}

	if flags.Edit {
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
	if jsonOutput {
		return outputJSON(results, fmMode, flags.Content, flags.StripGlobalTags, flags.StripTitle)
	}
	return outputPaths(results, fmMode)
}
