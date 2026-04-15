package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/urlresolve"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// LogOutput represents the JSON output for the log command.
type LogOutput struct {
	Path   string `json:"path"`
	UUID   string `json:"uuid"`
	Title  string `json:"title,omitempty"`
	Parent string `json:"parent,omitempty"`
}

// NewLogCmd creates the log command.
func NewLogCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		title     string
		useH1     bool
		useStdin  bool
		parentRef string
		orderVal  int
		noFetch   bool
	)

	cmd := &cobra.Command{
		Use:   "log [content]",
		Short: "Create a new note",
		Long: `Create a new note from the provided content.

Content can be provided as:
  - A positional argument
  - Via stdin (use --stdin or pipe content)
  - Via stdin when no argument is provided and stdin is not a TTY

The note will be saved with frontmatter containing UUID, timestamps, and tags.

See also:
  ruin search      Search for notes
  ruin today       Show notes created today`,
		Example: `  # From argument
  ruin log "Quick thought #idea"

  # With explicit title
  ruin log --title "Meeting Notes" "Discussion about X"

  # Extract title from first header
  ruin log --h1 "# Project Plan

Details here..."

  # From stdin
  echo "# My Note" | ruin log

  # JSON output for scripting
  echo "content" | ruin log --json`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			content, err := getContent(args, useStdin)
			if err != nil {
				return err
			}

			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("no content provided")
			}

			n, err := note.Parse(content)
			if err != nil {
				return fmt.Errorf("failed to parse content: %w", err)
			}

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

			// URL auto-detection: if content starts with a URL and no title is set
			urlResolved := false
			if title == "" && !useH1 && n.Title == "" && n.IsURLNote() && !noFetch {
				extractedURL := n.ExtractURL()
				if extractedURL != "" {
					resolver := urlresolve.NewHTMLResolver()
					meta, err := resolver.Resolve(context.Background(), extractedURL)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to resolve URL: %v\n", err)
					} else if meta.Title != "" {
						sanitized := sanitizeTitle(meta.Title)
						n.Content = "# " + sanitized + "\n\n" + n.Content
						n.Title = sanitized
						urlResolved = true
					}
				}
			}

			if err := createNote(n, vlt, title, useH1 || urlResolved); err != nil {
				return err
			}

			if *jsonOutput {
				output := LogOutput{
					Path:   n.FilePath,
					UUID:   n.UUID,
					Title:  n.Title,
					Parent: n.Parent,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Println(n.FilePath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "set filename explicitly")
	cmd.Flags().BoolVar(&useH1, "h1", false, "extract filename from first header in content")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "read content from stdin")
	cmd.Flags().StringVar(&parentRef, "parent", "", "set parent note (UUID, title, or path substring)")
	cmd.Flags().IntVar(&orderVal, "order", 0, "set manual sort order")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "skip URL title resolution for link notes")

	cmd.AddCommand(newLogExtractCmd(jsonOutput))

	return cmd
}

// getContent retrieves content from args or stdin.
func getContent(args []string, useStdin bool) (string, error) {
	// If content provided as argument
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}

	// Check if we should read from stdin
	if useStdin || !isTerminal(os.Stdin) {
		return readStdin()
	}

	return "", fmt.Errorf("no content provided; use argument or pipe content via stdin")
}

// isTerminal checks if the given file is a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// readStdin reads all content from stdin.
func readStdin() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	var builder strings.Builder

	for {
		line, err := reader.ReadString('\n')
		builder.WriteString(line)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read stdin: %w", err)
		}
	}

	return builder.String(), nil
}

// determineFilename determines the filename for the note.
// Priority: --title flag > --h1 flag > timestamp
func determineFilename(n *note.Note, titleFlag string, useH1 bool) string {
	// Priority 1: explicit title flag
	if titleFlag != "" {
		return note.SanitizeFilename(titleFlag)
	}

	// Priority 2: extract from header (only if --h1 flag is set)
	if useH1 && n.Title != "" {
		return note.SanitizeFilename(n.Title)
	}

	// Priority 3: timestamp (ignore title if --h1 not set)
	t := n.Created
	if t.IsZero() {
		t = time.Now()
	}
	return t.Format("2006-01-02T15-04-05")
}

// sanitizeTitle strips newlines and collapses whitespace in a resolved HTML title.
func sanitizeTitle(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
