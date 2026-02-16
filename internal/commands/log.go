package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			// Get content from args or stdin
			content, err := getContent(args, useStdin)
			if err != nil {
				return err
			}

			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("no content provided")
			}

			// Parse the content as a note
			n, err := note.Parse(content)
			if err != nil {
				return fmt.Errorf("failed to parse content: %w", err)
			}

			// Ensure UUID and timestamps
			n.EnsureUUID()
			n.SetTimestamps()

			// Refresh tags from content
			n.RefreshTags()

			// Resolve date tokens and extract dates
			n.Content = note.ResolveDateTokens(n.Content)
			n.RefreshDates()

			// Refresh linked-cards from wiki links
			if titlesIndex, err := vlt.LoadTitles(); err == nil {
				RefreshLinkedCards(n, titlesIndex)
			} else {
				fmt.Fprintf(os.Stderr, "warning: failed to load titles index for linked-cards: %v\n", err)
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

			// Determine filename
			filename := determineFilename(n, title, useH1)

			// Set file path
			n.FilePath = filepath.Join(vlt.Path, filename+".md")

			// Check if file already exists
			if _, err := os.Stat(n.FilePath); err == nil {
				return fmt.Errorf("file already exists: %s", n.FilePath)
			}

			// Save the note
			if err := n.Save(); err != nil {
				return fmt.Errorf("failed to save note: %w", err)
			}

			// Update tags index (global + inline)
			if err := vlt.UpdateTagsIndex(n.Tags, n.InlineTags); err != nil {
				// Non-fatal: log warning but don't fail
				fmt.Fprintf(os.Stderr, "warning: failed to update tags index: %v\n", err)
			}

			// Update titles index
			if err := vlt.UpdateTitleEntry(n.UUID, n.Title, n.FilePath, n.Parent); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to update titles index: %v\n", err)
			}

			// Output result
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
