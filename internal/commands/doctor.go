package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kevin/ruin-note-cli/internal/note"
	"github.com/kevin/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// DoctorOutput represents the JSON output for the doctor command.
type DoctorOutput struct {
	Scanned        int      `json:"scanned"`
	UUIDGenerated  []string `json:"uuid_generated,omitempty"`
	TagsReindexed  []string `json:"tags_reindexed,omitempty"`
	TagsYMLUpdated bool     `json:"tags_yml_updated"`
}

// NewDoctorCmd creates the doctor command.
func NewDoctorCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Scan and repair vault metadata",
		Long: `Scan all notes in the vault and repair/update metadata as needed.

Operations performed:
  - Generate UUID for notes missing one
  - Reindex tags and inline-tags from document content
  - Rebuild .ruin/tags.yml from all notes

Does NOT update created or updated timestamps.`,
		Example: `  # Run doctor
  ruin doctor

  # Preview changes without writing
  ruin doctor --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			// Ensure vault is initialized
			if !vlt.IsInitialized() {
				if !dryRun {
					if _, err := vlt.Initialize(false); err != nil {
						return fmt.Errorf("failed to initialize vault: %w", err)
					}
				}
			}

			// Get all notes
			notePaths, err := vlt.ListNotes()
			if err != nil {
				return fmt.Errorf("failed to list notes: %w", err)
			}

			output := DoctorOutput{
				Scanned: len(notePaths),
			}

			// Track all tags across the vault for rebuilding tags.yml
			tagCounts := make(map[string]int)

			prefix := ""
			if dryRun {
				prefix = "[dry-run] "
			}

			for _, path := range notePaths {
				n, err := note.Load(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%swarning: failed to parse %s: %v\n", prefix, path, err)
					continue
				}

				needsSave := false

				// Check for missing UUID
				if n.UUID == "" {
					n.EnsureUUID()
					output.UUIDGenerated = append(output.UUIDGenerated, path)
					needsSave = true
				}

				// Reindex tags from content
				oldTags := make(map[string]bool)
				for _, t := range n.Tags {
					oldTags[note.NormalizeTag(t)] = true
				}

				n.RefreshTags()

				// Check if tags changed
				newTags := make(map[string]bool)
				for _, t := range n.Tags {
					newTags[note.NormalizeTag(t)] = true
				}

				tagsChanged := len(oldTags) != len(newTags)
				if !tagsChanged {
					for t := range oldTags {
						if !newTags[t] {
							tagsChanged = true
							break
						}
					}
				}

				if tagsChanged {
					output.TagsReindexed = append(output.TagsReindexed, path)
					needsSave = true
				}

				// Count tags for rebuilding tags.yml
				for _, t := range n.Tags {
					tagCounts[t]++
				}

				// Save if needed
				if needsSave && !dryRun {
					if err := n.Save(); err != nil {
						fmt.Fprintf(os.Stderr, "%swarning: failed to save %s: %v\n", prefix, path, err)
					}
				}
			}

			// Rebuild tags.yml
			if !dryRun {
				if err := vlt.RebuildTagsIndex(tagCounts); err != nil {
					fmt.Fprintf(os.Stderr, "%swarning: failed to rebuild tags.yml: %v\n", prefix, err)
				} else {
					output.TagsYMLUpdated = true
				}
			} else {
				output.TagsYMLUpdated = true // Would update
			}

			// Output results
			if *jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			// Human-readable output
			fmt.Fprintf(os.Stderr, "%sScanned %d notes\n", prefix, output.Scanned)
			if len(output.UUIDGenerated) > 0 {
				fmt.Fprintf(os.Stderr, "  %d notes: %sgenerated missing UUID\n", len(output.UUIDGenerated), prefix)
			}
			if len(output.TagsReindexed) > 0 {
				fmt.Fprintf(os.Stderr, "  %d notes: %sreindexed tags\n", len(output.TagsReindexed), prefix)
			}
			if output.TagsYMLUpdated {
				fmt.Fprintf(os.Stderr, "%sUpdated .ruin/tags.yml\n", prefix)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would change without writing")

	return cmd
}
