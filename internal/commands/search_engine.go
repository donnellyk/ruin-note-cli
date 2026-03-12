package commands

import (
	"runtime"
	"sort"
	"strings"
	"sync"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
)

// SearchOptions controls search behavior for performance optimizations.
type SearchOptions struct {
	// Limit is the maximum number of results to return (0 = unlimited).
	// When set and no sorting is requested, enables early termination.
	Limit int
	// NeedFullNote indicates that matched notes require full content loaded.
	// When false and MatcherInfo.NeedsBody is false, the fast path skips
	// full file reads for non-matching notes and defers full load for matches.
	NeedFullNote bool
}

// searchNotes finds all notes matching the query.
func searchNotes(vlt *vault.Vault, matcher QueryMatcher, info MatcherInfo) ([]SearchResult, error) {
	return searchNotesWithOptions(vlt, matcher, info, SearchOptions{})
}

// searchNotesWithOptions finds notes with performance optimizations.
// Uses concurrent file reading for improved performance.
// When info.NeedsBody is false, uses LoadFrontmatterOnly for the initial match,
// then defers to full Load only for matches that need content.
func searchNotesWithOptions(vlt *vault.Vault, matcher QueryMatcher, info MatcherInfo, opts SearchOptions) ([]SearchResult, error) {
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
				// If we matched on frontmatter-only but need full content for output
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
