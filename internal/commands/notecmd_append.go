package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

type noteAppendOutput struct {
	Path   string `json:"path"`
	UUID   string `json:"uuid"`
	Line   int    `json:"line"`
	Action string `json:"action"`
}

func newNoteAppendCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		line    int
		suffix  bool
		stdin   bool
		rawLine bool
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "append <note> [text]",
		Short: "Append or insert content into a note",
		Long: `Insert text into a note's content.

Without --line, appends text as a new line at the end.
With --line N, inserts before content line N (1-indexed, after frontmatter).
With --line N --suffix, appends text to the end of line N.
With --raw-line, line numbers count from the top of the file (including frontmatter).

Text comes from a positional argument or --stdin (mutually exclusive).`,
		Example: `  ruin note append "My Note" "New paragraph"
  ruin note append <uuid> --line 3 "Inserted line"
  ruin note append <uuid> --line 1 --suffix " (continued)"
  ruin note append <uuid> --raw-line --line 10 "After frontmatter line 10"
  echo "piped text" | ruin note append <uuid> --stdin`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if suffix && !cmd.Flags().Changed("line") {
				return fmt.Errorf("--suffix requires --line")
			}
			if rawLine && !cmd.Flags().Changed("line") {
				return fmt.Errorf("--raw-line requires --line")
			}

			hasArg := len(args) >= 2
			if hasArg && stdin {
				return fmt.Errorf("cannot use both positional text and --stdin")
			}
			if !hasArg && !stdin {
				return fmt.Errorf("text required as argument or via --stdin")
			}

			var text string
			if stdin {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				text = string(data)
				text = strings.TrimRight(text, "\n")
			} else {
				text = args[1]
			}

			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			n, err := ResolveNote(vlt, args[0])
			if err != nil {
				return err
			}

			if rawLine && cmd.Flags().Changed("line") {
				fmLines, err := frontmatterLineCount(n)
				if err != nil {
					return fmt.Errorf("failed to compute frontmatter size: %w", err)
				}
				line = line - fmLines
				if line < 1 {
					return fmt.Errorf("--line %d (raw) points into frontmatter (content starts at line %d)", line+fmLines, fmLines+1)
				}
			}

			oldGlobal := n.Tags
			oldInline := n.InlineTags

			lines := strings.Split(n.Content, "\n")
			totalLines := len(lines)

			var resultLine int
			var action string

			if !cmd.Flags().Changed("line") {
				if n.Content != "" && !strings.HasSuffix(n.Content, "\n") {
					n.Content += "\n"
				}
				n.Content += text + "\n"
				resultLine = totalLines + 1
				action = "appended"
			} else {
				if line < 1 || line > totalLines+1 {
					return fmt.Errorf("--line %d out of range (valid: 1-%d)", line, totalLines+1)
				}
				idx := line - 1
				if suffix {
					lines[idx] += text
					n.Content = strings.Join(lines, "\n")
					resultLine = line
					action = "appended"
				} else {
					newLines := make([]string, 0, totalLines+1)
					newLines = append(newLines, lines[:idx]...)
					newLines = append(newLines, text)
					newLines = append(newLines, lines[idx:]...)
					n.Content = strings.Join(newLines, "\n")
					resultLine = line
					action = "inserted"
				}
			}

			if err := saveWithIndexUpdate(n, vlt); err != nil {
				return err
			}

			vlt.SaveNote(n, oldGlobal, oldInline, fmt.Sprintf("ruin note append: Update %q", n.Title))

			if *jsonOutput {
				out := noteAppendOutput{Path: n.FilePath, UUID: n.UUID, Line: resultLine, Action: action}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Fprintf(os.Stderr, "Modified %s\n", n.FilePath)
			return nil
		},
	}

	cmd.Flags().IntVar(&line, "line", 0, "target content line (1-indexed)")
	cmd.Flags().BoolVar(&suffix, "suffix", false, "append to end of line (requires --line)")
	cmd.Flags().BoolVar(&rawLine, "raw-line", false, "line numbers count from top of file (including frontmatter)")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read text from stdin")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation")

	return cmd
}
