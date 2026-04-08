package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// NewNoteCmd creates the note command group with subcommands.
func NewNoteCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Programmatic note mutations",
		Long:  `Commands for modifying individual notes: set metadata/tags, append content, merge notes.`,
	}

	cmd.AddCommand(newNoteSetCmd(getVault, jsonOutput))
	cmd.AddCommand(newNoteAppendCmd(getVault, jsonOutput))
	cmd.AddCommand(newNoteMergeCmd(getVault, jsonOutput))
	cmd.AddCommand(newNoteDeleteCmd(getVault, jsonOutput))

	return cmd
}

// --- note delete ---

type noteDeleteOutput struct {
	Path  string `json:"path"`
	UUID  string `json:"uuid"`
	Title string `json:"title"`
}

func newNoteDeleteCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <note>",
		Short: "Delete a note",
		Long: `Delete a note from the vault.

The note is resolved by UUID, title, or path substring.
Requires confirmation unless --force is set.`,
		Example: `  ruin note delete "My Note"
  ruin note delete <uuid>
  ruin note delete <uuid> --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			n, err := ResolveNote(vlt, args[0])
			if err != nil {
				return err
			}

			if !force {
				if !isTerminal(os.Stderr) {
					return fmt.Errorf("delete requires --force in non-interactive mode")
				}
				fmt.Fprintf(os.Stderr, "Delete %q (%s)? [y/N] ", n.Title, n.FilePath)
				var response string
				fmt.Scanln(&response)
				response = strings.ToLower(strings.TrimSpace(response))
				if response != "y" && response != "yes" {
					return ErrUserAborted
				}
			}

			if err := vlt.DeleteNote(n, fmt.Sprintf("ruin note delete: Delete %q", n.Title)); err != nil {
				return err
			}

			if *jsonOutput {
				out := noteDeleteOutput{Path: n.FilePath, UUID: n.UUID, Title: n.Title}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Fprintf(os.Stderr, "Deleted %s\n", n.FilePath)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation")

	return cmd
}
