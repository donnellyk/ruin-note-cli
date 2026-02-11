package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// DoctorOutput represents the JSON output for the doctor command.
type DoctorOutput struct {
	Scanned              int      `json:"scanned"`
	UUIDGenerated        []string `json:"uuid_generated,omitempty"`
	TagsReindexed        []string `json:"tags_reindexed,omitempty"`
	LinkedCardsReindexed []string `json:"linked_cards_reindexed,omitempty"`
	TagsYMLUpdated       bool     `json:"tags_yml_updated"`
	TitlesUpdated        bool     `json:"titles_updated"`
	OrphanedParents      []string `json:"orphaned_parents,omitempty"`
	OrphanedBookmarks    []string `json:"orphaned_bookmarks,omitempty"`
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
  - Resolve [[wiki links]] and rebuild linked-cards
  - Rebuild .ruin/tags.yml from all notes
  - Rebuild .ruin/titles.json from all notes
  - Detect orphaned parent references

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

			// Track all titles for rebuilding titles.json
			titleEntries := make(map[string]vault.TitleEntry)

			// Store loaded notes for linked-cards pass
			var loadedNotes []*note.Note

			prefix := ""
			if dryRun {
				prefix = "[dry-run] "
			}

			for _, path := range notePaths {
				// Read the raw frontmatter to compare against content-derived tags.
				// note.Load -> Parse re-derives Tags/InlineTags from body content,
				// discarding frontmatter values. We need the on-disk values to detect
				// when classification has drifted (e.g. a tag in both fields).
				rawBytes, err := os.ReadFile(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%swarning: failed to read %s: %v\n", prefix, path, err)
					continue
				}
				rawFM, _, _ := note.ParseFrontmatter(string(rawBytes))

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

				// Compare on-disk frontmatter tags against the content-derived
				// classification (already computed by Parse). This detects both
				// tag additions/removals AND misclassification between global/inline.
				if !normalizedTagsEqual(rawFM.Tags, n.Tags) || !normalizedTagsEqual(rawFM.InlineTags, n.InlineTags) {
					output.TagsReindexed = append(output.TagsReindexed, path)
					needsSave = true
				}

				// Count tags for rebuilding tags.yml (global + inline)
				for _, t := range n.AllTags() {
					tagCounts[t]++
				}

				// Collect title entry
				titleEntries[n.UUID] = vault.TitleEntry{
					Title:  n.Title,
					Path:   path,
					Parent: n.Parent,
				}

				// Save if needed
				if needsSave && !dryRun {
					if err := n.Save(); err != nil {
						fmt.Fprintf(os.Stderr, "%swarning: failed to save %s: %v\n", prefix, path, err)
					}
				}

				loadedNotes = append(loadedNotes, n)
			}

			// Post-loop: rebuild linked-cards using collected title entries
			tempIndex := &vault.TitlesIndex{Titles: titleEntries}
			for _, n := range loadedNotes {
				oldLinkedCards := make(map[string]bool)
				for _, lc := range n.LinkedCards {
					oldLinkedCards[lc] = true
				}

				RefreshLinkedCards(n, tempIndex)

				newLinkedCards := make(map[string]bool)
				for _, lc := range n.LinkedCards {
					newLinkedCards[lc] = true
				}

				changed := len(oldLinkedCards) != len(newLinkedCards)
				if !changed {
					for lc := range oldLinkedCards {
						if !newLinkedCards[lc] {
							changed = true
							break
						}
					}
				}

				if changed {
					output.LinkedCardsReindexed = append(output.LinkedCardsReindexed, n.FilePath)
					if !dryRun {
						if err := n.Save(); err != nil {
							fmt.Fprintf(os.Stderr, "%swarning: failed to save linked-cards for %s: %v\n", prefix, n.FilePath, err)
						}
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

			// Rebuild titles.json
			if !dryRun {
				if err := vlt.RebuildTitlesIndex(titleEntries); err != nil {
					fmt.Fprintf(os.Stderr, "%swarning: failed to rebuild titles.json: %v\n", prefix, err)
				} else {
					output.TitlesUpdated = true
				}
			} else {
				output.TitlesUpdated = true // Would update
			}

			// Detect orphaned parent references
			for uuid, entry := range titleEntries {
				if entry.Parent != "" {
					if _, ok := titleEntries[entry.Parent]; !ok {
						output.OrphanedParents = append(output.OrphanedParents,
							fmt.Sprintf("%s (parent %s not found)", uuid, entry.Parent))
					}
				}
			}

			// Detect orphaned bookmarks
			parentBookmarks, err := vlt.LoadParents()
			if err == nil {
				for _, p := range parentBookmarks.Parents {
					if _, ok := titleEntries[p.UUID]; !ok {
						output.OrphanedBookmarks = append(output.OrphanedBookmarks,
							fmt.Sprintf("%s (uuid %s not found)", p.Name, p.UUID))
					}
				}
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
			if len(output.LinkedCardsReindexed) > 0 {
				fmt.Fprintf(os.Stderr, "  %d notes: %sreindexed linked-cards\n", len(output.LinkedCardsReindexed), prefix)
			}
			if output.TagsYMLUpdated {
				fmt.Fprintf(os.Stderr, "%sUpdated .ruin/tags.yml\n", prefix)
			}
			if output.TitlesUpdated {
				fmt.Fprintf(os.Stderr, "%sUpdated .ruin/titles.json\n", prefix)
			}
			if len(output.OrphanedParents) > 0 {
				fmt.Fprintf(os.Stderr, "  %d orphaned parent reference(s):\n", len(output.OrphanedParents))
				for _, op := range output.OrphanedParents {
					fmt.Fprintf(os.Stderr, "    - %s\n", op)
				}
			}
			if len(output.OrphanedBookmarks) > 0 {
				fmt.Fprintf(os.Stderr, "  %d orphaned bookmark(s):\n", len(output.OrphanedBookmarks))
				for _, ob := range output.OrphanedBookmarks {
					fmt.Fprintf(os.Stderr, "    - %s\n", ob)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would change without writing")

	return cmd
}

// normalizedTagsEqual compares two tag slices for equality after normalizing.
// Order matters (classification may reorder), and comparison is case-insensitive.
func normalizedTagsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if note.NormalizeTag(a[i]) != note.NormalizeTag(b[i]) {
			return false
		}
	}
	return true
}
