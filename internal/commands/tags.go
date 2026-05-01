package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

func NewTagsCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Manage tags in the vault",
		Long:  `Commands for listing, renaming, and deleting tags across all notes.`,
	}

	cmd.AddCommand(newTagsListCmd(getVault, jsonOutput))
	cmd.AddCommand(newTagsRenameCmd(getVault, jsonOutput))
	cmd.AddCommand(newTagsDeleteCmd(getVault, jsonOutput))

	return cmd
}

func newTagsListCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		sortBy string
		minUse int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tags in the vault",
		Long: `List all tags with their usage counts.

See also:
  ruin search "#tag"    Search for notes with a specific tag
  ruin tags rename      Rename a tag across all notes
  ruin tags delete      Remove a tag from all notes`,
		Example: `  # List all tags sorted by count (most used first)
  ruin tags list

  # List tags sorted by name
  ruin tags list --sort name

  # Only show tags used at least 5 times
  ruin tags list --min 5

  # JSON output
  ruin tags list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			index, err := vlt.LoadTags()
			if err != nil {
				return fmt.Errorf("failed to load tags: %w", err)
			}

			var filtered []vault.TagEntry
			for _, t := range index.Tags {
				if t.Count >= minUse {
					filtered = append(filtered, t)
				}
			}

			switch {
			case strings.HasPrefix(sortBy, "name"):
				sort.Slice(filtered, func(i, j int) bool {
					if strings.HasSuffix(sortBy, ":desc") {
						return filtered[i].Name > filtered[j].Name
					}
					return filtered[i].Name < filtered[j].Name
				})
			case strings.HasPrefix(sortBy, "count"):
				sort.Slice(filtered, func(i, j int) bool {
					if strings.HasSuffix(sortBy, ":asc") {
						return filtered[i].Count < filtered[j].Count
					}
					return filtered[i].Count > filtered[j].Count
				})
			}

			if *jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(filtered)
			}

			if len(filtered) == 0 {
				fmt.Fprintln(os.Stderr, "No tags found")
				return nil
			}

			for _, t := range filtered {
				fmt.Printf("%s (%d) [%s]\n", t.Name, t.Count, strings.Join(t.Scope, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&sortBy, "sort", "s", "count:desc", "sort by: name, name:desc, count, count:asc, count:desc")
	cmd.Flags().IntVar(&minUse, "min", 0, "only show tags with at least N uses")

	return cmd
}

func newTagsRenameCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		force  bool
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "rename <old-tag> <new-tag>",
		Short: "Rename a tag across all notes",
		Long: `Rename a tag in all notes that contain it.

The old tag will be replaced with the new tag in all matching notes.
The # prefix is optional and will be added automatically if missing.

Note: When searching for tags with "ruin search", the # prefix is required.

See also:
  ruin tags list      List all tags in the vault
  ruin tags delete    Remove a tag from all notes`,
		Example: `  # Rename a tag (with or without # prefix)
  ruin tags rename "#wip" "#in-progress"
  ruin tags rename wip in-progress

  # Preview changes without applying
  ruin tags rename old new --dry-run

  # Skip confirmation
  ruin tags rename old new --force`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			oldStored := note.NormalizeStored(args[0])
			newStored := note.NormalizeStored(args[1])
			oldBody := note.BodyForm(oldStored)
			newBody := note.BodyForm(newStored)
			// Display values keep the # prefix for human readability.
			oldTag := oldBody
			newTag := newBody

			notePaths, err := vlt.ListNotes()
			if err != nil {
				return fmt.Errorf("failed to list notes: %w", err)
			}

			var toUpdate []string
			for _, path := range notePaths {
				n, err := note.Load(path)
				if err != nil {
					continue
				}
				for _, t := range n.AllTags() {
					if note.NormalizeStored(t) == oldStored {
						toUpdate = append(toUpdate, path)
						break
					}
				}
			}

			if len(toUpdate) == 0 {
				fmt.Fprintf(os.Stderr, "No notes found with tag %s\n", oldTag)
				return nil
			}

			if !force && !dryRun {
				if !isTerminal(os.Stderr) {
					return fmt.Errorf("rename requires --force in non-interactive mode")
				}

				fmt.Fprintf(os.Stderr, "Will rename %s to %s in %d note(s):\n", oldTag, newTag, len(toUpdate))
				for _, path := range toUpdate {
					fmt.Fprintf(os.Stderr, "  - %s\n", path)
				}
				fmt.Fprint(os.Stderr, "Continue? [y/N]: ")

				var response string
				fmt.Scanln(&response)
				response = strings.ToLower(strings.TrimSpace(response))
				if response != "y" && response != "yes" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			if dryRun {
				fmt.Fprintf(os.Stderr, "[dry-run] Would rename %s to %s in %d note(s):\n", oldTag, newTag, len(toUpdate))
				for _, path := range toUpdate {
					fmt.Fprintf(os.Stderr, "  - %s\n", path)
				}
				return nil
			}

			var updated int
			var errors []string
			var globalTagsChangedUUIDs []string
			for _, path := range toUpdate {
				n, err := note.Load(path)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Failed to load %s: %v", path, err))
					continue
				}

				oldGlobalTags := make([]string, len(n.Tags))
				copy(oldGlobalTags, n.Tags)

				n.Content = replaceTag(n.Content, oldBody, newBody)
				n.RefreshTags()
				n.SetTimestamps()

				if err := saveNoteForVault(n, vlt); err != nil {
					errors = append(errors, fmt.Sprintf("Failed to save %s: %v", path, err))
					continue
				}

				// Refresh the titles.json mirror so subsequent cascades and
				// hot-path matchers see the renamed tag immediately.
				if err := vlt.UpdateTitleEntryFull(n.UUID, n.Title, n.FilePath, n.Parent, n.Tags, n.InlineTags, n.InheritedTags); err != nil {
					errors = append(errors, fmt.Sprintf("Failed to update titles mirror for %s: %v", path, err))
				}

				if !normalizedTagsEqual(oldGlobalTags, n.Tags) {
					globalTagsChangedUUIDs = append(globalTagsChangedUUIDs, n.UUID)
				}
				updated++
			}

			if len(globalTagsChangedUUIDs) > 0 {
				if titlesIndex, err := vlt.LoadTitles(); err == nil {
					for _, uuid := range globalTagsChangedUUIDs {
						if err := CascadeInheritedTags(uuid, vlt, titlesIndex); err != nil {
							errors = append(errors, fmt.Sprintf("Failed to cascade inherited tags for %s: %v", uuid, err))
						}
					}
				}
			}

			if err := rebuildTagsIndex(vlt); err != nil {
				errors = append(errors, fmt.Sprintf("Failed to rebuild tags index: %v", err))
			}

			if updated > 0 {
				vlt.Commit(fmt.Sprintf("ruin tags rename: %s -> %s (%d notes)", oldTag, newTag, updated))
			}

			fmt.Fprintf(os.Stderr, "Renamed %s to %s in %d note(s)\n", oldTag, newTag, updated)
			if len(errors) > 0 {
				fmt.Fprintln(os.Stderr, "Errors:")
				for _, e := range errors {
					fmt.Fprintf(os.Stderr, "  - %s\n", e)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show changes without applying")

	return cmd
}

func newTagsDeleteCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		force  bool
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "delete <tag>",
		Short: "Delete a tag from all notes",
		Long: `Remove a tag from all notes that contain it.

The tag will be removed from the content of all matching notes.
The # prefix is optional and will be added automatically if missing.

Note: When searching for tags with "ruin search", the # prefix is required.

See also:
  ruin tags list      List all tags in the vault
  ruin tags rename    Rename a tag across all notes`,
		Example: `  # Delete a tag (with or without # prefix)
  ruin tags delete "#deprecated"
  ruin tags delete deprecated

  # Preview changes without applying
  ruin tags delete old --dry-run

  # Skip confirmation
  ruin tags delete old --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			stored := note.NormalizeStored(args[0])
			body := note.BodyForm(stored)
			tag := body

			notePaths, err := vlt.ListNotes()
			if err != nil {
				return fmt.Errorf("failed to list notes: %w", err)
			}

			var toUpdate []string
			for _, path := range notePaths {
				n, err := note.Load(path)
				if err != nil {
					continue
				}
				for _, t := range n.AllTags() {
					if note.NormalizeStored(t) == stored {
						toUpdate = append(toUpdate, path)
						break
					}
				}
			}

			if len(toUpdate) == 0 {
				fmt.Fprintf(os.Stderr, "No notes found with tag %s\n", tag)
				return nil
			}

			if !force && !dryRun {
				if !isTerminal(os.Stderr) {
					return fmt.Errorf("delete requires --force in non-interactive mode")
				}

				fmt.Fprintf(os.Stderr, "Will remove %s from %d note(s):\n", tag, len(toUpdate))
				for _, path := range toUpdate {
					fmt.Fprintf(os.Stderr, "  - %s\n", path)
				}
				fmt.Fprint(os.Stderr, "Continue? [y/N]: ")

				var response string
				fmt.Scanln(&response)
				response = strings.ToLower(strings.TrimSpace(response))
				if response != "y" && response != "yes" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			if dryRun {
				fmt.Fprintf(os.Stderr, "[dry-run] Would remove %s from %d note(s):\n", tag, len(toUpdate))
				for _, path := range toUpdate {
					fmt.Fprintf(os.Stderr, "  - %s\n", path)
				}
				return nil
			}

			var updated int
			var errors []string
			var globalTagsChangedUUIDs []string
			for _, path := range toUpdate {
				n, err := note.Load(path)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Failed to load %s: %v", path, err))
					continue
				}

				oldGlobalTags := make([]string, len(n.Tags))
				copy(oldGlobalTags, n.Tags)

				n.Content = removeTag(n.Content, body)
				n.RefreshTags()
				n.SetTimestamps()

				if err := saveNoteForVault(n, vlt); err != nil {
					errors = append(errors, fmt.Sprintf("Failed to save %s: %v", path, err))
					continue
				}

				if err := vlt.UpdateTitleEntryFull(n.UUID, n.Title, n.FilePath, n.Parent, n.Tags, n.InlineTags, n.InheritedTags); err != nil {
					errors = append(errors, fmt.Sprintf("Failed to update titles mirror for %s: %v", path, err))
				}

				if !normalizedTagsEqual(oldGlobalTags, n.Tags) {
					globalTagsChangedUUIDs = append(globalTagsChangedUUIDs, n.UUID)
				}
				updated++
			}

			if len(globalTagsChangedUUIDs) > 0 {
				if titlesIndex, err := vlt.LoadTitles(); err == nil {
					for _, uuid := range globalTagsChangedUUIDs {
						if err := CascadeInheritedTags(uuid, vlt, titlesIndex); err != nil {
							errors = append(errors, fmt.Sprintf("Failed to cascade inherited tags for %s: %v", uuid, err))
						}
					}
				}
			}

			if err := rebuildTagsIndex(vlt); err != nil {
				errors = append(errors, fmt.Sprintf("Failed to rebuild tags index: %v", err))
			}

			if updated > 0 {
				vlt.Commit(fmt.Sprintf("ruin tags delete: %s (%d notes)", tag, updated))
			}

			fmt.Fprintf(os.Stderr, "Removed %s from %d note(s)\n", tag, updated)
			if len(errors) > 0 {
				fmt.Fprintln(os.Stderr, "Errors:")
				for _, e := range errors {
					fmt.Fprintf(os.Stderr, "  - %s\n", e)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show changes without applying")

	return cmd
}

func replaceTag(content, oldTag, newTag string) string {
	return strings.ReplaceAll(content, oldTag, newTag)
}

func removeTag(content, tag string) string {
	result := strings.ReplaceAll(content, tag+" ", "")
	result = strings.ReplaceAll(result, " "+tag, "")
	result = strings.ReplaceAll(result, tag, "")
	return result
}

func rebuildTagsIndex(vlt *vault.Vault) error {
	notePaths, err := vlt.ListNotes()
	if err != nil {
		return err
	}

	totalCounts := make(map[string]int)
	globalTags := make(map[string]bool)
	inlineTags := make(map[string]bool)
	for _, path := range notePaths {
		n, err := note.Load(path)
		if err != nil {
			continue
		}
		for _, t := range n.AllTags() {
			totalCounts[t]++
		}
		for _, t := range n.Tags {
			globalTags[t] = true
		}
		for _, t := range n.InlineTags {
			inlineTags[t] = true
		}
	}

	return vlt.RebuildTagsIndex(totalCounts, globalTags, inlineTags)
}
