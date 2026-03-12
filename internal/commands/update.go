package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
)

// UpdateOutput represents the JSON output for the update command.
type UpdateOutput struct {
	Modified []string `json:"modified,omitempty"`
	Deleted  []string `json:"deleted,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

// NewUpdateCmd creates the update command.
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

			// Read original content
			originalContent, err := readFileOrStdin(originalPath)
			if err != nil {
				return fmt.Errorf("failed to read original: %w", err)
			}

			// Read updated content
			updatedContent, err := readFileOrStdin(updatedPath)
			if err != nil {
				return fmt.Errorf("failed to read updated: %w", err)
			}

			// Parse bulk content
			originalMap := note.ParseBulk(originalContent)
			updatedMap := note.ParseBulk(updatedContent)

			// Build UUID -> note path map by loading all notes
			uuidToPath, uuidToNote, err := buildUUIDMaps(vlt)
			if err != nil {
				return fmt.Errorf("failed to build UUID map: %w", err)
			}

			// Identify changes
			var toModify []string // UUIDs to modify
			var toDelete []string // UUIDs to delete
			var newUUIDs []string // UUIDs only in updated (error)
			var errors []string

			// Check for modifications and deletions
			for uuid, origContent := range originalMap {
				updContent, exists := updatedMap[uuid]

				if !exists {
					// Deleted
					toDelete = append(toDelete, uuid)
				} else if updContent != origContent {
					// Modified
					toModify = append(toModify, uuid)
				}
			}

			// Check for new UUIDs
			for uuid := range updatedMap {
				if _, exists := originalMap[uuid]; !exists {
					newUUIDs = append(newUUIDs, uuid)
				}
			}

			// Report new UUIDs as errors
			for _, uuid := range newUUIDs {
				errors = append(errors, fmt.Sprintf("new UUID found: %s (use 'log' to create new notes)", uuid))
			}

			// Handle deletions - require confirmation or --force
			if len(toDelete) > 0 && !force && !dryRun {
				// Check if stderr is a TTY for interactive confirmation
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

			// Apply changes
			output := UpdateOutput{}
			prefix := ""
			if dryRun {
				prefix = "[dry-run] "
			}

			// Load titles index for linked-cards resolution
			titlesIndex, titlesErr := vlt.LoadTitles()

			// Process modifications
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

				// Capture old tags before modification for index update
				oldGlobalTags := n.Tags
				oldInlineTags := n.InlineTags

				// Update content
				n.Content = updatedMap[uuid]
				n.RefreshTags()

				// Resolve date tokens and extract dates
				n.Content = note.ResolveDateTokens(n.Content)
				n.RefreshDates()

				// Refresh linked-cards from wiki links
				if titlesErr == nil {
					RefreshLinkedCards(n, titlesIndex)
				}

				// Refresh inherited tags
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

					// Cascade if global tags changed
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

			// Process deletions
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

			// Output results
			if *jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			// Human-readable output
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

// readFileOrStdin reads content from a file path, or stdin if path is "-".
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

// buildUUIDMaps builds maps from UUID to file path and UUID to Note.
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
			continue // Skip unparseable notes
		}

		if n.UUID != "" {
			uuidToPath[n.UUID] = path
			uuidToNote[n.UUID] = n
		}
	}

	return uuidToPath, uuidToNote, nil
}
