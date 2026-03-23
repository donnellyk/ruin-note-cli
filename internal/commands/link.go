package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/urlresolve"
	"kvnd/ruin-note-cli/internal/vault"
)

// LinkNewOutput represents the JSON output for the link new command.
type LinkNewOutput struct {
	Path            string `json:"path"`
	UUID            string `json:"uuid"`
	Title           string `json:"title,omitempty"`
	URL             string `json:"url"`
	ResolvedTitle   string `json:"resolved_title,omitempty"`
	ResolvedSummary string `json:"resolved_summary,omitempty"`
}

// NewLinkCmd creates the link command with subcommands.
func NewLinkCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Manage link notes",
		Long:  "Create, resolve, and list notes whose primary content is a web URL.",
	}

	cmd.AddCommand(newLinkNewCmd(getVault, jsonOutput))
	cmd.AddCommand(newLinkResolveCmd(jsonOutput))
	cmd.AddCommand(newLinkListCmd(getVault, jsonOutput))

	return cmd
}

func newLinkNewCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		title     string
		tags      string
		parentRef string
		orderVal  int
		noFetch   bool
		comment   string
	)

	cmd := &cobra.Command{
		Use:   "new <url>",
		Short: "Create a link note from a URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			rawURL := args[0]
			if !note.IsValidURL(rawURL) {
				return fmt.Errorf("invalid URL: must be http:// or https://")
			}

			var resolvedTitle, resolvedSummary string

			if !noFetch {
				resolver := urlresolve.NewHTMLResolver()
				meta, err := resolver.Resolve(context.Background(), rawURL)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to resolve URL: %v\n", err)
				} else {
					resolvedTitle = sanitizeTitle(meta.Title)
					resolvedSummary = meta.Summary
				}
			}

			// Determine the note title
			noteTitle := title
			if noteTitle == "" && resolvedTitle != "" {
				noteTitle = resolvedTitle
			}

			// Build note content
			var content strings.Builder
			if noteTitle != "" {
				fmt.Fprintf(&content, "# %s\n\n", noteTitle)
			}

			content.WriteString(rawURL + "\n")

			if comment != "" {
				content.WriteString("\n" + comment + "\n")
			}

			if resolvedSummary != "" {
				content.WriteString("\n> " + resolvedSummary + "\n")
			}

			// Build tag line
			var tagLine strings.Builder
			tagLine.WriteString("#link")
			if tags != "" {
				for t := range strings.SplitSeq(tags, ",") {
					t = strings.TrimSpace(t)
					if t == "" {
						continue
					}
					if !strings.HasPrefix(t, "#") {
						t = "#" + t
					}
					tagLine.WriteString(" " + t)
				}
			}
			content.WriteString("\n" + tagLine.String() + "\n")

			n, err := note.Parse(content.String())
			if err != nil {
				return fmt.Errorf("failed to parse note content: %w", err)
			}
			n.URL = rawURL

			// Set order if specified
			if cmd.Flags().Changed("order") {
				n.Order = &orderVal
			}

			// Resolve parent if specified
			if parentRef != "" {
				parent, err := ResolveNote(vlt, parentRef)
				if err != nil {
					return fmt.Errorf("parent: %w", err)
				}
				n.Parent = parent.UUID
			}

			if err := createNote(n, vlt, "", true); err != nil {
				return err
			}

			// Warn on duplicate URLs
			warnDuplicateURL(vlt, rawURL, n.UUID)

			if *jsonOutput {
				output := LinkNewOutput{
					Path:            n.FilePath,
					UUID:            n.UUID,
					Title:           n.Title,
					URL:             rawURL,
					ResolvedTitle:   resolvedTitle,
					ResolvedSummary: resolvedSummary,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Println(n.FilePath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "override resolved page title")
	cmd.Flags().StringVar(&tags, "tags", "", "additional tags (comma-separated, # auto-added)")
	cmd.Flags().StringVar(&parentRef, "parent", "", "set parent note")
	cmd.Flags().IntVar(&orderVal, "order", 0, "set manual sort order")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "skip URL title/description resolution")
	cmd.Flags().StringVarP(&comment, "comment", "c", "", "add personal commentary below the URL")

	return cmd
}

func newLinkResolveCmd(jsonOutput *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "resolve <url>",
		Short: "Fetch and display URL metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawURL := args[0]
			if !note.IsValidURL(rawURL) {
				return fmt.Errorf("invalid URL: must be http:// or https://")
			}

			resolver := urlresolve.NewHTMLResolver()
			meta, err := resolver.Resolve(context.Background(), rawURL)
			if err != nil {
				return fmt.Errorf("failed to resolve URL: %w", err)
			}

			if *jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(meta)
			}

			fmt.Printf("Title: %s\n", meta.Title)
			fmt.Printf("URL: %s\n", meta.URL)
			if meta.Summary != "" {
				fmt.Printf("Summary: %s\n", meta.Summary)
			}
			return nil
		},
	}
}

func newLinkListCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var flags SearchFlags

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List link notes",
		Long:  "List all link notes (notes with a URL). Equivalent to: ruin search --link",
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			if err := ValidateSearchFlags(&flags, *jsonOutput); err != nil {
				return err
			}

			return executeSearch(vlt, linkNoteMatcher(), MatcherInfo{NeedsBody: true}, &flags, *jsonOutput, nil)
		},
	}

	AddSearchFlags(cmd, &flags, "created:desc")

	return cmd
}

func warnDuplicateURL(vlt *vault.Vault, url string, excludeUUID string) {
	paths, err := vlt.ListNotes()
	if err != nil {
		return
	}
	for _, p := range paths {
		n, err := note.LoadFrontmatterOnly(p)
		if err != nil {
			continue
		}
		if n.URL == url && n.UUID != excludeUUID {
			fmt.Fprintf(os.Stderr, "warning: duplicate URL found in %s\n", p)
		}
	}
}
