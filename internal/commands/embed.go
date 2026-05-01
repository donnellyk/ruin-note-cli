package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/donnellyk/ruin-note-cli/internal/dateparse"
	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// embedEvalEnvelope is the discriminated JSON envelope produced by `ruin embed eval --json`.
// Results is type-specific: SearchResult slice for search/query, PickResult slice for pick,
// embedComposeResult for compose.
type embedEvalEnvelope struct {
	Type    string            `json:"type"`
	Query   string            `json:"query"`
	Options map[string]string `json:"options,omitempty"`
	Results any               `json:"results"`
}

type embedComposeResult struct {
	ExpandedMarkdown string        `json:"expanded_markdown"`
	SourceMap        []sourceEntry `json:"source_map,omitempty"`
}

func NewEmbedCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "embed",
		Short: "Work with dynamic embeds",
		Long: `Subcommands for working with dynamic embeds.

Subcommands:
  eval    Evaluate a single dynamic embed and emit its results`,
	}
	cmd.AddCommand(newEmbedEvalCmd(getVault, jsonOutput))
	return cmd
}

func newEmbedEvalCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eval <embed-string>",
		Short: "Evaluate a dynamic embed standalone",
		Long: `Evaluate a single dynamic embed and emit its results.

Accepts the canonical full-delimiter form ![[type: query | options]] and the
bare inner form (type: query | options). Any surrounding ![[ ]] is stripped.

Use - as the argument to read the embed string from stdin.

Default output is plain-text rendering matching what each embed produces in
compose-time output (search lists, pick groupings, compose expansion). With
--json, emits a typed envelope with discriminated results.

Embed types: search, pick, query, compose.

Embed options split into:
  - Query-shaping (limit=, sort=, tag-scope=, ...): honored in both modes.
  - Rendering (format=): plain-text only; ignored in JSON mode so callers
    receive a stable shape.`,
		Example: `  # Plain-text rendering of a search embed
  ruin embed eval "![[search: #daily | limit=5]]"

  # Bare inner form (delimiters auto-added)
  ruin embed eval "search: #daily | limit=5"

  # JSON envelope for programmatic consumers
  ruin embed eval "![[search: #daily]]" --json

  # Read embed from stdin
  echo "![[pick: #followup]]" | ruin embed eval -`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			input := args[0]
			if input == "-" {
				buf, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read embed from stdin: %w", err)
				}
				input = string(buf)
			}

			ref, err := parseEmbedEvalInput(input)
			if err != nil {
				return err
			}

			if *jsonOutput {
				env, err := evalEmbedJSON(vlt, ref)
				if err != nil {
					return err
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(env)
			}

			text, err := evalEmbedText(vlt, ref)
			if err != nil {
				return err
			}
			fmt.Print(text)
			if text != "" && !strings.HasSuffix(text, "\n") {
				fmt.Println()
			}
			return nil
		},
	}

	return cmd
}

// parseEmbedEvalInput accepts both ![[type: query | opts]] and bare type: query | opts.
// Bare input is wrapped in ![[ ]] before parsing through note.FindDynamicEmbeds so the
// same regex governs both forms; bare input containing stray ![[ or ]] would otherwise
// silently end up inside the captured query, so reject those up-front.
func parseEmbedEvalInput(s string) (note.DynamicEmbedRef, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return note.DynamicEmbedRef{}, fmt.Errorf("embed string is empty")
	}
	wrapped := s
	if !strings.HasPrefix(s, "![[") {
		if strings.Contains(s, "![[") || strings.Contains(s, "]]") {
			return note.DynamicEmbedRef{}, fmt.Errorf("invalid embed: %q contains stray ![[ or ]] (use the full form ![[type: query | options]] or bare type: query | options without delimiters)", s)
		}
		wrapped = "![[" + s + "]]"
	}
	refs := note.FindDynamicEmbeds(wrapped)
	if len(refs) == 0 {
		return note.DynamicEmbedRef{}, fmt.Errorf("invalid embed: %q (expected ![[type: query | options]] or bare type: query | options; types: search, pick, query, compose)", s)
	}
	return refs[0], nil
}

func evalEmbedJSON(vlt *vault.Vault, ref note.DynamicEmbedRef) (embedEvalEnvelope, error) {
	env := embedEvalEnvelope{
		Type:    ref.Type,
		Query:   ref.Query,
		Options: ref.Options,
	}

	switch ref.Type {
	case "search":
		results, err := evalEmbedSearch(vlt, ref)
		if err != nil {
			return env, err
		}
		env.Results = results
	case "query":
		results, err := evalEmbedQuery(vlt, ref)
		if err != nil {
			return env, err
		}
		env.Results = results
	case "pick":
		results, err := evalEmbedPick(vlt, ref)
		if err != nil {
			return env, err
		}
		env.Results = results
	case "compose":
		result, err := evalEmbedCompose(vlt, ref)
		if err != nil {
			return env, err
		}
		env.Results = result
	default:
		return env, fmt.Errorf("unknown embed type %q", ref.Type)
	}

	return env, nil
}

func evalEmbedSearch(vlt *vault.Vault, ref note.DynamicEmbedRef) ([]SearchResult, error) {
	tagScope := parseDynamicTagScope(ref.Options)
	matcher, info, err := parseQuery(ref.Query, tagScope)
	if err != nil {
		return nil, fmt.Errorf("search: invalid query: %w", err)
	}

	results, err := searchNotes(vlt, matcher, info)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if sortStr, ok := ref.Options["sort"]; ok {
		fields, err := parseSort(sortStr)
		if err == nil {
			sortResults(results, fields)
		}
	} else {
		sortResults(results, []SortField{{Field: "created", Ascending: false}})
	}

	if limitStr, ok := ref.Options["limit"]; ok {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(results) {
			results = results[:limit]
		}
	}

	if results == nil {
		results = []SearchResult{}
	}
	return results, nil
}

func evalEmbedQuery(vlt *vault.Vault, ref note.DynamicEmbedRef) ([]SearchResult, error) {
	queries, err := vlt.LoadQueries()
	if err != nil {
		return nil, fmt.Errorf("query: failed to load saved queries: %w", err)
	}
	var queryStr string
	for _, q := range queries.Queries {
		if q.Name == ref.Query {
			queryStr = q.Query
			break
		}
	}
	if queryStr == "" {
		return nil, fmt.Errorf("query: not found: %s", ref.Query)
	}
	searchRef := note.DynamicEmbedRef{
		Type:    "search",
		Query:   queryStr,
		Options: ref.Options,
		Line:    ref.Line,
	}
	return evalEmbedSearch(vlt, searchRef)
}

func evalEmbedPick(vlt *vault.Vault, ref note.DynamicEmbedRef) ([]PickResult, error) {
	resolved := normalizePickQueryCommas(note.ResolveDateTokensInQuery(ref.Query))
	args := strings.Fields(resolved)
	var filter pickTagFilter
	var dateRanges []dateparse.DateRange
	now := time.Now()
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "!#"):
			filter.exclude = append(filter.exclude, note.NormalizeStored(arg[1:]))
		case strings.HasPrefix(arg, "#"):
			filter.include = append(filter.include, note.NormalizeStored(arg))
		case strings.HasPrefix(arg, "@between:"):
			dr, err := parsePickBetween(arg, now)
			if err != nil {
				return nil, fmt.Errorf("pick: %w", err)
			}
			dateRanges = append(dateRanges, dr)
		case strings.HasPrefix(arg, "@"):
			token := arg[1:]
			dr, err := dateparse.ParseWithReference(token, now)
			if err != nil {
				return nil, fmt.Errorf("pick: unrecognized date %s", arg)
			}
			dateRanges = append(dateRanges, dr)
		default:
			filter.include = append(filter.include, note.NormalizeStored(arg))
		}
	}

	if len(filter.include) == 0 && len(dateRanges) == 0 {
		return nil, fmt.Errorf("pick: at least one positive tag or @date required")
	}

	anyMode := ref.Options["any"] == "true"
	todoMode := ref.Options["todo"] == "true"

	df := doneExclude
	if ref.Options["all"] == "true" {
		df = doneInclude
	} else if ref.Options["done"] == "true" {
		df = doneOnly
	}

	var filterMatcher QueryMatcher
	if filterStr, ok := ref.Options["filter"]; ok && filterStr != "" {
		m, _, err := parseQuery(filterStr, TagScopeAll)
		if err != nil {
			return nil, fmt.Errorf("pick filter: %w", err)
		}
		filterMatcher = m
	}

	candidates, err := pickCandidatePaths(vlt, filter, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("pick: %w", err)
	}

	var titlesIndex *vault.TitlesIndex
	if filterMatcher != nil {
		titlesIndex, err = vlt.LoadTitles()
		if err != nil {
			return nil, fmt.Errorf("pick: failed to load titles: %w", err)
		}
	}

	var results []PickResult
	for _, path := range candidates {
		if filterMatcher != nil {
			fast, err := note.LoadFrontmatterOnly(path)
			if err != nil {
				continue
			}
			hydrateNoteTagsFromIndex(fast, titlesIndex, path, false)
			if !filterMatcher(fast) {
				continue
			}
		}
		n, err := note.Load(path)
		if err != nil {
			continue
		}
		matches := pickLinesFromNote(n, filter, dateRanges, anyMode, df, todoMode)
		if len(matches) == 0 {
			continue
		}
		results = append(results, PickResult{
			UUID:    n.UUID,
			Title:   n.Title,
			File:    path,
			Matches: matches,
			created: n.Created,
			updated: n.Updated,
			order:   n.Order,
		})
	}

	var sortFields []SortField
	if sortStr, ok := ref.Options["sort"]; ok {
		if fields, err := parseSort(sortStr); err == nil {
			sortFields = fields
		}
	}
	if len(sortFields) == 0 {
		sortFields = []SortField{{Field: "created", Ascending: false}}
	}
	sortPickResults(results, sortFields)

	if limitStr, ok := ref.Options["limit"]; ok {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(results) {
			results = results[:limit]
		}
	}

	if results == nil {
		results = []PickResult{}
	}
	return results, nil
}

func evalEmbedCompose(vlt *vault.Vault, ref note.DynamicEmbedRef) (embedComposeResult, error) {
	resolved, err := ResolveNote(vlt, ref.Query)
	if err != nil {
		return embedComposeResult{}, fmt.Errorf("compose: %w", err)
	}

	index, err := vlt.LoadTitles()
	if err != nil {
		return embedComposeResult{}, fmt.Errorf("compose: failed to load titles: %w", err)
	}

	maxDepth := 0
	if depthStr, ok := ref.Options["depth"]; ok {
		if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
			maxDepth = d
		}
	}

	walker := &composeWalker{
		vault:         vlt,
		index:         index,
		childrenMap:   index.ChildrenMap(),
		visited:       make(map[string]bool),
		maxDepth:      maxDepth,
		expandEmbeds:  true,
		expandDynamic: true,
		rootUUID:      resolved.UUID,
	}
	tree := walker.Walk(resolved.UUID, 0)
	if tree == nil {
		return embedComposeResult{}, fmt.Errorf("compose: failed to build tree for %s", ref.Query)
	}

	text, sourceMap := renderText(tree)
	return embedComposeResult{
		ExpandedMarkdown: text,
		SourceMap:        sourceMap,
	}, nil
}

func evalEmbedText(vlt *vault.Vault, ref note.DynamicEmbedRef) (string, error) {
	// Compose has its own renderer (no compose-time wrapping) — bypass the
	// dynamic-expander rendering so output isn't double-wrapped.
	if ref.Type == "compose" {
		result, err := evalEmbedCompose(vlt, ref)
		if err != nil {
			return "", err
		}
		return result.ExpandedMarkdown, nil
	}

	// Pre-validate via the JSON evaluator so query parse / saved-query lookup /
	// pick-arg failures surface as hard errors instead of stderr warnings.
	if _, err := evalEmbedJSON(vlt, ref); err != nil {
		return "", err
	}

	walker, err := newEmbedEvalWalker(vlt)
	if err != nil {
		return "", err
	}

	result := walker.expandDynamicEmbed(ref, 0, "")
	if result == nil || len(result.segments) == 0 {
		return "", nil
	}

	tree := &composeTree{Segments: result.segments}
	text, _ := renderText(tree)
	return text, nil
}

func newEmbedEvalWalker(vlt *vault.Vault) (*composeWalker, error) {
	index, err := vlt.LoadTitles()
	if err != nil {
		return nil, fmt.Errorf("failed to load titles index: %w", err)
	}
	return &composeWalker{
		vault:         vlt,
		index:         index,
		childrenMap:   index.ChildrenMap(),
		visited:       make(map[string]bool),
		expandEmbeds:  true,
		expandDynamic: true,
	}, nil
}
