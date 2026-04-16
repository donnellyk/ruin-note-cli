package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

func NewGetCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var flags SearchFlags
	var pathFilter string
	var titleFilter string
	var uuidFilter string
	var edit bool
	var force bool

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a single note",
		Long: `Get a single note by path or title.

Requires one of --path or --title to identify the note.
Returns the first match if multiple notes match the filter.
Returns an error if no match is found.`,
		Example: `  # Get by title (substring match)
  ruin get --title "Meeting Notes"

  # Get by path (substring match)
  ruin get --path "2025-01-28"

  # Get with JSON output and content
  ruin get --title "Daily" --json --content

  # Get with content stripping
  ruin get --path "notes/idea.md" --json --content --strip-global-tags --strip-title

  # Edit a note
  ruin get --title "Meeting Notes" --edit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			filterCount := 0
			if pathFilter != "" {
				filterCount++
			}
			if titleFilter != "" {
				filterCount++
			}
			if uuidFilter != "" {
				filterCount++
			}
			if filterCount == 0 {
				return fmt.Errorf("one of --path, --title, or --uuid is required")
			}
			if filterCount > 1 {
				return fmt.Errorf("--path, --title, and --uuid are mutually exclusive")
			}

			if edit && *jsonOutput {
				return errMutuallyExclusive("--json", "--edit")
			}
			if err := ValidateSearchFlags(&flags, *jsonOutput); err != nil {
				return err
			}

			var result SearchResult

			if uuidFilter != "" {
				n, err := ResolveNote(vlt, uuidFilter)
				if err != nil {
					if *jsonOutput {
						fmt.Println("null")
					}
					return fmt.Errorf("no matching note found: %w", err)
				}
				result = SearchResult{
					Path:   n.FilePath,
					UUID:   n.UUID,
					Title:  n.Title,
					Tags:   n.Tags,
					Parent: n.Parent,
					note:   n,
				}
			} else {
				var matcher QueryMatcher
				if pathFilter != "" {
					matcher = pathMatcher(pathFilter)
				} else {
					matcher = titleMatcher(titleFilter)
				}

				results, err := searchNotesWithOptions(vlt, matcher, MatcherInfo{NeedsBody: false}, SearchOptions{NeedFullNote: true})
				if err != nil {
					return fmt.Errorf("search failed: %w", err)
				}

				if len(results) == 0 {
					if *jsonOutput {
						fmt.Println("null")
					}
					return fmt.Errorf("no matching note found")
				}

				result = results[0]
			}

			fmMode := FrontmatterMode(flags.Frontmatter)
			if flags.Frontmatter != "" && flags.Frontmatter != "none" && flags.Frontmatter != "extra" && flags.Frontmatter != "full" {
				return fmt.Errorf("invalid frontmatter mode: %s (use: none, extra, full)", flags.Frontmatter)
			}

			if edit {
				return handleEdit(vlt, []SearchResult{result}, force, fmMode)
			}

			if *jsonOutput {
				return outputSingleJSON(result, fmMode, flags.Content, flags.StripGlobalTags, flags.StripTitle)
			}

			return outputSingleNote(result, fmMode)
		},
	}

	cmd.Flags().StringVar(&pathFilter, "path", "", "match by file path (substring)")
	cmd.Flags().StringVar(&titleFilter, "title", "", "match by title (case-insensitive substring)")
	cmd.Flags().StringVar(&uuidFilter, "uuid", "", "match by UUID (exact or via resolve)")

	cmd.Flags().StringVar(&flags.Frontmatter, "frontmatter", "", "include frontmatter in output (modes: extra, full, none)")
	cmd.Flag("frontmatter").NoOptDefVal = "extra"
	cmd.Flags().BoolVar(&flags.Content, "content", false, "include note content in JSON output")
	cmd.Flags().BoolVar(&flags.StripGlobalTags, "strip-global-tags", false, "remove global tags from content (requires --content)")
	cmd.Flags().BoolVar(&flags.StripTitle, "strip-title", false, "remove H1 title from content (requires --content)")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "open note in $EDITOR")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation for deletions in edit mode")

	return cmd
}

func outputSingleJSON(r SearchResult, fmMode FrontmatterMode, includeContent, stripGlobalTags, stripTitle bool) error {
	type jsonResult struct {
		Path          string         `json:"path"`
		UUID          string         `json:"uuid"`
		Title         string         `json:"title,omitempty"`
		Tags          []string       `json:"tags,omitempty"`
		InheritedTags []string       `json:"inherited_tags,omitempty"`
		Parent        string         `json:"parent,omitempty"`
		Created       string         `json:"created,omitempty"`
		Updated       string         `json:"updated,omitempty"`
		Extra         map[string]any `json:"extra,omitempty"`
		Content       string         `json:"content,omitempty"`
	}

	jr := jsonResult{
		Path:          r.Path,
		UUID:          r.UUID,
		Title:         r.Title,
		Tags:          r.note.EffectiveGlobalTags(),
		InheritedTags: r.note.InheritedTags,
		Parent:        r.note.Parent,
		Created:       r.note.Created.Format(note.TimeFormat),
		Updated:       r.note.Updated.Format(note.TimeFormat),
	}

	if fmMode == FrontmatterExtra && len(r.note.Extra) > 0 {
		jr.Extra = r.note.Extra
	} else if fmMode == FrontmatterFull && len(r.note.Extra) > 0 {
		jr.Extra = r.note.Extra
	}

	if includeContent {
		content := r.note.Content

		if fmMode == FrontmatterExtra || fmMode == FrontmatterFull {
			if serialized, err := r.note.Serialize(); err == nil {
				content = serialized
			}
		}

		if stripTitle {
			content = note.StripTitle(content)
		}
		if stripGlobalTags {
			content = note.StripGlobalTags(content, r.note.InlineTags)
		}

		jr.Content = content
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}

func outputSingleNote(r SearchResult, fmMode FrontmatterMode) error {
	var output string
	if fmMode == FrontmatterFull {
		serialized, err := r.note.Serialize()
		if err == nil {
			output = serialized
		} else {
			output = r.note.Content
		}
	} else {
		output = r.note.Content
	}

	fmt.Print(output)
	if !strings.HasSuffix(output, "\n") {
		fmt.Println()
	}
	return nil
}
