package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
)

// --- note merge ---

type noteMergeOutput struct {
	TargetPath    string   `json:"target_path"`
	TargetUUID    string   `json:"target_uuid"`
	SourcePath    string   `json:"source_path"`
	SourceUUID    string   `json:"source_uuid"`
	TagsMerged    []string `json:"tags_merged"`
	ChildrenMoved int      `json:"children_moved"`
	SourceDeleted bool     `json:"source_deleted"`
}

func newNoteMergeCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		deleteSource bool
		stripTitle   bool
		dryRun       bool
		force        bool
	)

	cmd := &cobra.Command{
		Use:   "merge <target> <source>",
		Short: "Merge source note into target",
		Long: `Combine two notes by merging source into target.

Merges frontmatter (target takes precedence for existing fields),
global tags (deduplicated), and appends source content to target.
Source's children are reparented to target.`,
		Example: `  ruin note merge "Target Note" "Source Note"
  ruin note merge <target-uuid> <source-uuid> --strip-title --delete-source
  ruin note merge target source --dry-run`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			target, err := ResolveNote(vlt, args[0])
			if err != nil {
				return fmt.Errorf("target: %w", err)
			}

			source, err := ResolveNote(vlt, args[1])
			if err != nil {
				return fmt.Errorf("source: %w", err)
			}

			if target.UUID == source.UUID {
				return fmt.Errorf("cannot merge a note into itself")
			}

			// Confirm unless --force or --dry-run
			if !force && !dryRun {
				if !isTerminal(os.Stderr) {
					return fmt.Errorf("merge requires --force in non-interactive mode")
				}
				fmt.Fprintf(os.Stderr, "Merge %q into %q?", source.Title, target.Title)
				if deleteSource {
					fmt.Fprintf(os.Stderr, " (source will be deleted)")
				}
				fmt.Fprint(os.Stderr, " [y/N] ")
				var response string
				fmt.Scanln(&response)
				response = strings.ToLower(strings.TrimSpace(response))
				if response != "y" && response != "yes" {
					return ErrUserAborted
				}
			}

			oldTargetGlobal := target.Tags
			oldTargetInline := target.InlineTags

			// 1. Merge source's Extra fields into target (target takes precedence)
			if target.Extra == nil {
				target.Extra = make(map[string]any)
			}
			for k, v := range source.Extra {
				if _, exists := target.Extra[k]; !exists {
					target.Extra[k] = v
				}
			}

			// 2. Merge source's global tags into target content
			var tagsMerged []string
			for _, tag := range source.Tags {
				if !noteHasTag(target, tag) {
					target.Content = insertGlobalTag(target.Content, tag)
					tagsMerged = append(tagsMerged, tag)
				}
			}

			// 3. Append source content
			sourceContent := source.Content
			if stripTitle {
				sourceContent = note.StripTitle(sourceContent)
			}
			if strings.TrimSpace(sourceContent) != "" {
				if target.Content != "" && !strings.HasSuffix(target.Content, "\n") {
					target.Content += "\n"
				}
				target.Content += "\n" + sourceContent
			}

			// 4. Reparent source's children to target
			titlesIndex, err := vlt.LoadTitles()
			if err != nil {
				return fmt.Errorf("failed to load titles index: %w", err)
			}

			var childrenMoved int
			for uuid, entry := range titlesIndex.Titles {
				if entry.Parent == source.UUID {
					childNote, loadErr := note.Load(entry.Path)
					if loadErr != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to load child %s: %v\n", entry.Path, loadErr)
						continue
					}
					if !dryRun {
						childNote.Parent = target.UUID
						childNote.SetTimestamps()
						if err := childNote.Save(); err != nil {
							fmt.Fprintf(os.Stderr, "warning: failed to reparent %s: %v\n", entry.Path, err)
							continue
						}
						if err := vlt.UpdateTitleEntry(uuid, childNote.Title, childNote.FilePath, target.UUID); err != nil {
							fmt.Fprintf(os.Stderr, "warning: failed to update title entry for %s: %v\n", uuid, err)
						}
					}
					childrenMoved++
				}
			}

			if dryRun {
				out := noteMergeOutput{
					TargetPath:    target.FilePath,
					TargetUUID:    target.UUID,
					SourcePath:    source.FilePath,
					SourceUUID:    source.UUID,
					TagsMerged:    tagsMerged,
					ChildrenMoved: childrenMoved,
					SourceDeleted: deleteSource,
				}
				if *jsonOutput {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out)
				}
				fmt.Fprintf(os.Stderr, "[dry-run] Would merge %q into %q (tags: %d, children: %d, delete source: %v)\n",
					source.Title, target.Title, len(tagsMerged), childrenMoved, deleteSource)
				return nil
			}

			// Save target with full pipeline
			if err := saveWithIndexUpdate(target, vlt); err != nil {
				return err
			}

			commitMsg := fmt.Sprintf("ruin note merge: Merge %q into %q", source.Title, target.Title)
			vlt.SaveNote(target, oldTargetGlobal, oldTargetInline, commitMsg)

			// 5. Delete source if requested
			sourceDeleted := false
			if deleteSource {
				if err := vlt.DeleteNote(source, commitMsg); err != nil {
					return fmt.Errorf("failed to delete source: %w", err)
				}
				sourceDeleted = true
			}

			out := noteMergeOutput{
				TargetPath:    target.FilePath,
				TargetUUID:    target.UUID,
				SourcePath:    source.FilePath,
				SourceUUID:    source.UUID,
				TagsMerged:    tagsMerged,
				ChildrenMoved: childrenMoved,
				SourceDeleted: sourceDeleted,
			}

			if *jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Fprintf(os.Stderr, "Merged %q into %q\n", source.Title, target.Title)
			return nil
		},
	}

	cmd.Flags().BoolVar(&deleteSource, "delete-source", false, "delete source note after merge")
	cmd.Flags().BoolVar(&stripTitle, "strip-title", false, "strip source's H1 title before appending")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "preview changes without writing")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation")

	return cmd
}
