package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// NewGetCmd creates the get command.
func NewGetCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var flags SearchFlags
	var pathFilter string
	var titleFilter string

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
  ruin get --path "notes/idea.md" --json --content --strip-global-tags --strip-title`,
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			// Validate that exactly one of --path or --title is provided
			if pathFilter == "" && titleFilter == "" {
				return fmt.Errorf("one of --path or --title is required")
			}
			if pathFilter != "" && titleFilter != "" {
				return errMutuallyExclusive("--path", "--title")
			}

			// Validate other flags
			if err := ValidateSearchFlags(&flags, *jsonOutput); err != nil {
				return err
			}

			// Build matcher based on filter type
			var matcher QueryMatcher
			if pathFilter != "" {
				matcher = pathMatcher(pathFilter)
			} else {
				matcher = titleMatcher(titleFilter)
			}

			// Find matching notes
			results, err := searchNotes(vlt, matcher)
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			// Handle no results
			if len(results) == 0 {
				if *jsonOutput {
					fmt.Println("null")
				}
				return fmt.Errorf("no matching note found")
			}

			// Take first result
			result := results[0]

			// Parse frontmatter mode
			fmMode := FrontmatterMode(flags.Frontmatter)
			if flags.Frontmatter != "" && flags.Frontmatter != "none" && flags.Frontmatter != "extra" && flags.Frontmatter != "full" {
				return fmt.Errorf("invalid frontmatter mode: %s (use: none, extra, full)", flags.Frontmatter)
			}

			// Output based on mode
			if *jsonOutput {
				return outputSingleJSON(result, fmMode, flags.Content, flags.StripGlobalTags, flags.StripTitle)
			}

			// Default: output content
			return outputSingleNote(result, fmMode)
		},
	}

	// Add get-specific flags
	cmd.Flags().StringVar(&pathFilter, "path", "", "match by file path (substring)")
	cmd.Flags().StringVar(&titleFilter, "title", "", "match by title (case-insensitive substring)")

	// Add common search flags (but only certain ones are relevant)
	cmd.Flags().StringVar(&flags.Frontmatter, "frontmatter", "", "include frontmatter in output (modes: extra, full, none)")
	cmd.Flag("frontmatter").NoOptDefVal = "extra"
	cmd.Flags().BoolVar(&flags.Content, "content", false, "include note content in JSON output")
	cmd.Flags().BoolVar(&flags.StripGlobalTags, "strip-global-tags", false, "remove global tags from content (requires --content)")
	cmd.Flags().BoolVar(&flags.StripTitle, "strip-title", false, "remove H1 title from content (requires --content)")

	return cmd
}

// outputSingleJSON outputs a single result as JSON.
func outputSingleJSON(r SearchResult, fmMode FrontmatterMode, includeContent, stripGlobalTags, stripTitle bool) error {
	type jsonResult struct {
		Path    string                 `json:"path"`
		UUID    string                 `json:"uuid"`
		Title   string                 `json:"title,omitempty"`
		Tags    []string               `json:"tags,omitempty"`
		Created string                 `json:"created,omitempty"`
		Updated string                 `json:"updated,omitempty"`
		Extra   map[string]interface{} `json:"extra,omitempty"`
		Content string                 `json:"content,omitempty"`
	}

	jr := jsonResult{
		Path:  r.Path,
		UUID:  r.UUID,
		Title: r.Title,
		Tags:  r.Tags,
	}

	if fmMode == FrontmatterExtra && len(r.note.Extra) > 0 {
		jr.Extra = r.note.Extra
	} else if fmMode == FrontmatterFull {
		jr.Created = r.note.Created.Format(note.TimeFormat)
		jr.Updated = r.note.Updated.Format(note.TimeFormat)
		if len(r.note.Extra) > 0 {
			jr.Extra = r.note.Extra
		}
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

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}

// outputSingleNote outputs a single note's content.
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
