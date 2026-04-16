package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

type UpdateOutput struct {
	Modified []string `json:"modified,omitempty"`
	Deleted  []string `json:"deleted,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

func NewUpdateCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		originalPath string
		updatedPath  string
		force        bool
		dryRun       bool
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Apply changes from bulk edit",
		Long: `Apply changes from a bulk export edit session.

Compares original and updated bulk exports to identify:
  - Modified notes: content changed -> update file
  - Deleted notes: removed from updated -> delete file (requires --force or confirmation)

New UUIDs in the updated content are an error (use 'log' to create new notes).`,
		Example: `  # Standard workflow
  ruin search "#draft" --bulk > /tmp/original.txt
  cp /tmp/original.txt /tmp/edited.txt
  $EDITOR /tmp/edited.txt
  ruin update -o /tmp/original.txt -u /tmp/edited.txt

  # Preview changes
  ruin update -o orig.txt -u new.txt --dry-run

  # Non-interactive (script)
  ruin update -o orig.txt -u new.txt --force

  # Read updated from stdin
  cat edited.txt | ruin update -o orig.txt -u -`,
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			originalContent, err := readFileOrStdin(originalPath)
			if err != nil {
				return fmt.Errorf("failed to read original: %w", err)
			}

			updatedContent, err := readFileOrStdin(updatedPath)
			if err != nil {
				return fmt.Errorf("failed to read updated: %w", err)
			}

			originalMap := note.ParseBulk(originalContent)
			updatedMap := note.ParseBulk(updatedContent)

			uuidToPath, uuidToNote, err := buildUUIDMaps(vlt)
			if err != nil {
				return fmt.Errorf("failed to build UUID map: %w", err)
			}

			var toModify []string
			var toDelete []string
			var newUUIDs []string
			var errors []string

			for uuid, origContent := range originalMap {
				updContent, exists := updatedMap[uuid]

				if !exists {
					toDelete = append(toDelete, uuid)
				} else if updContent != origContent {
					toModify = append(toModify, uuid)
				}
			}

			for uuid := range updatedMap {
				if _, exists := originalMap[uuid]; !exists {
					newUUIDs = append(newUUIDs, uuid)
				}
			}

			for _, uuid := range newUUIDs {
				errors = append(errors, fmt.Sprintf("new UUID found: %s (use 'log' to create new notes)", uuid))
			}

			if len(toDelete) > 0 && !force && !dryRun {
				if !isTerminal(os.Stderr) {
					return fmt.Errorf("deletions require --force in non-interactive mode")
				}

				fmt.Fprintf(os.Stderr, "The following %d note(s) will be deleted:\n", len(toDelete))
				for _, uuid := range toDelete {
					path := uuidToPath[uuid]
					if path != "" {
						fmt.Fprintf(os.Stderr, "  - %s\n", path)
					} else {
						fmt.Fprintf(os.Stderr, "  - UUID: %s (path not found)\n", uuid)
					}
				}
				fmt.Fprint(os.Stderr, "Continue? [y/N]: ")

				var response string
				fmt.Scanln(&response)
				response = strings.ToLower(strings.TrimSpace(response))
				if response != "y" && response != "yes" {
					if *jsonOutput {
						out := UpdateOutput{Errors: []string{"user aborted"}}
						enc := json.NewEncoder(os.Stdout)
						enc.SetIndent("", "  ")
						enc.Encode(out)
					} else {
						fmt.Fprintln(os.Stderr, "Aborted.")
					}
					os.Exit(3)
				}
			}

			output := UpdateOutput{}
			prefix := ""
			if dryRun {
				prefix = "[dry-run] "
			}

			titlesIndex, titlesErr := vlt.LoadTitles()

			for _, uuid := range toModify {
				path := uuidToPath[uuid]
				if path == "" {
					errors = append(errors, fmt.Sprintf("UUID not found in vault: %s", uuid))
					continue
				}

				n := uuidToNote[uuid]
				if n == nil {
					errors = append(errors, fmt.Sprintf("failed to load note for UUID: %s", uuid))
					continue
				}

				oldGlobalTags := n.Tags
				oldInlineTags := n.InlineTags

				n.Content = updatedMap[uuid]
				n.RefreshTags()
				if n.EnsureLinkTag() {
					n.RefreshTags()
				}

				n.Content = note.ResolveDateTokens(n.Content)
				n.RefreshDates()

				if titlesErr == nil {
					RefreshLinkedCards(n, titlesIndex)
				}

				if _, err := RefreshInheritedTags(n, vlt); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to refresh inherited tags: %v\n", err)
				}

				n.SetTimestamps()

				if !dryRun {
					if err := n.Save(); err != nil {
						errors = append(errors, fmt.Sprintf("failed to save %s: %v", path, err))
						continue
					}

					vlt.SaveNote(n, oldGlobalTags, oldInlineTags, fmt.Sprintf("ruin update: Modify %q", n.Title))

					if !normalizedTagsEqual(oldGlobalTags, n.Tags) {
						if ti, err := vlt.LoadTitles(); err == nil {
							if err := CascadeInheritedTags(n.UUID, vlt, ti); err != nil {
								fmt.Fprintf(os.Stderr, "warning: failed to cascade inherited tags: %v\n", err)
							}
						}
					}
				}

				output.Modified = append(output.Modified, path)
			}

			for _, uuid := range toDelete {
				path := uuidToPath[uuid]
				if path == "" {
					errors = append(errors, fmt.Sprintf("UUID not found in vault: %s", uuid))
					continue
				}

				n := uuidToNote[uuid]

				if !dryRun {
					if n != nil {
						if err := vlt.DeleteNote(n, fmt.Sprintf("ruin update: Delete %q", n.Title)); err != nil {
							errors = append(errors, fmt.Sprintf("failed to delete %s: %v", path, err))
							continue
						}
					} else {
						if err := os.Remove(path); err != nil {
							errors = append(errors, fmt.Sprintf("failed to delete %s: %v", path, err))
							continue
						}
					}
				}

				output.Deleted = append(output.Deleted, path)
			}

			output.Errors = errors

			if *jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			if len(output.Modified) > 0 || len(output.Deleted) > 0 {
				fmt.Fprintf(os.Stderr, "%sModified: %d, Deleted: %d\n", prefix, len(output.Modified), len(output.Deleted))
			} else {
				fmt.Fprintf(os.Stderr, "%sNo changes\n", prefix)
			}

			if len(output.Errors) > 0 {
				fmt.Fprintln(os.Stderr, "Errors:")
				for _, e := range output.Errors {
					fmt.Fprintf(os.Stderr, "  - %s\n", e)
				}
				return fmt.Errorf("completed with %d error(s)", len(output.Errors))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&originalPath, "original", "o", "", "original bulk export file (required)")
	cmd.Flags().StringVarP(&updatedPath, "updated", "u", "", "updated bulk export file (required)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation for deletions")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would change without writing")

	cmd.MarkFlagRequired("original")
	cmd.MarkFlagRequired("updated")

	return cmd
}

// readFileOrStdin reads from path, or stdin if path is "-".
func readFileOrStdin(path string) (string, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func buildUUIDMaps(vlt *vault.Vault) (map[string]string, map[string]*note.Note, error) {
	paths, err := vlt.ListNotes()
	if err != nil {
		return nil, nil, err
	}

	uuidToPath := make(map[string]string)
	uuidToNote := make(map[string]*note.Note)

	for _, path := range paths {
		n, err := note.Load(path)
		if err != nil {
			continue
		}

		if n.UUID != "" {
			uuidToPath[n.UUID] = path
			uuidToNote[n.UUID] = n
		}
	}

	return uuidToPath, uuidToNote, nil
}
