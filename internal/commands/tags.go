package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/kevin/ruin-note-cli/internal/note"
	"github.com/kevin/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// NewTagsCmd creates the tags command with subcommands.
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

// newTagsListCmd creates the "tags list" subcommand.
func newTagsListCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		sortBy string
		minUse int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tags in the vault",
		Long:  `List all tags with their usage counts.`,
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

			// Filter by minimum usage
			var filtered []vault.TagEntry
			for _, t := range index.Tags {
				if t.Count >= minUse {
					filtered = append(filtered, t)
				}
			}

			// Sort
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
					return filtered[i].Count > filtered[j].Count // desc by default
				})
			}

			// Output
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
				fmt.Printf("%s (%d)\n", t.Name, t.Count)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&sortBy, "sort", "s", "count:desc", "sort by: name, name:desc, count, count:asc, count:desc")
	cmd.Flags().IntVar(&minUse, "min", 0, "only show tags with at least N uses")

	return cmd
}

// newTagsRenameCmd creates the "tags rename" subcommand.
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
Tags should include the # prefix (e.g., #old-tag #new-tag).`,
		Example: `  # Rename a tag
  ruin tags rename "#wip" "#in-progress"

  # Preview changes without applying
  ruin tags rename "#old" "#new" --dry-run

  # Skip confirmation
  ruin tags rename "#old" "#new" --force`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			oldTag := args[0]
			newTag := args[1]

			// Validate tags start with #
			if !strings.HasPrefix(oldTag, "#") {
				oldTag = "#" + oldTag
			}
			if !strings.HasPrefix(newTag, "#") {
				newTag = "#" + newTag
			}

			// Find all notes with the old tag
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
				for _, t := range n.Tags {
					if strings.EqualFold(t, oldTag) {
						toUpdate = append(toUpdate, path)
						break
					}
				}
			}

			if len(toUpdate) == 0 {
				fmt.Fprintf(os.Stderr, "No notes found with tag %s\n", oldTag)
				return nil
			}

			// Confirm unless --force or --dry-run
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

			// Apply changes
			var updated int
			var errors []string
			for _, path := range toUpdate {
				n, err := note.Load(path)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Failed to load %s: %v", path, err))
					continue
				}

				// Replace tag in content
				n.Content = replaceTag(n.Content, oldTag, newTag)
				n.RefreshTags()
				n.SetTimestamps()

				if err := n.Save(); err != nil {
					errors = append(errors, fmt.Sprintf("Failed to save %s: %v", path, err))
					continue
				}

				updated++
			}

			// Update tags index
			if err := rebuildTagsIndex(vlt); err != nil {
				errors = append(errors, fmt.Sprintf("Failed to rebuild tags index: %v", err))
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

// newTagsDeleteCmd creates the "tags delete" subcommand.
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
Tags should include the # prefix (e.g., #tag-to-delete).`,
		Example: `  # Delete a tag
  ruin tags delete "#deprecated"

  # Preview changes without applying
  ruin tags delete "#old" --dry-run

  # Skip confirmation
  ruin tags delete "#old" --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			tag := args[0]

			// Validate tag starts with #
			if !strings.HasPrefix(tag, "#") {
				tag = "#" + tag
			}

			// Find all notes with the tag
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
				for _, t := range n.Tags {
					if strings.EqualFold(t, tag) {
						toUpdate = append(toUpdate, path)
						break
					}
				}
			}

			if len(toUpdate) == 0 {
				fmt.Fprintf(os.Stderr, "No notes found with tag %s\n", tag)
				return nil
			}

			// Confirm unless --force or --dry-run
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

			// Apply changes
			var updated int
			var errors []string
			for _, path := range toUpdate {
				n, err := note.Load(path)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Failed to load %s: %v", path, err))
					continue
				}

				// Remove tag from content
				n.Content = removeTag(n.Content, tag)
				n.RefreshTags()
				n.SetTimestamps()

				if err := n.Save(); err != nil {
					errors = append(errors, fmt.Sprintf("Failed to save %s: %v", path, err))
					continue
				}

				updated++
			}

			// Update tags index
			if err := rebuildTagsIndex(vlt); err != nil {
				errors = append(errors, fmt.Sprintf("Failed to rebuild tags index: %v", err))
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

// replaceTag replaces oldTag with newTag in content.
func replaceTag(content, oldTag, newTag string) string {
	// Handle both simple tags (#foo) and spaced tags (#foo bar#)
	// Simple replacement - this handles the basic case
	return strings.ReplaceAll(content, oldTag, newTag)
}

// removeTag removes a tag from content.
func removeTag(content, tag string) string {
	// Remove the tag and any trailing space
	result := strings.ReplaceAll(content, tag+" ", "")
	result = strings.ReplaceAll(result, " "+tag, "")
	result = strings.ReplaceAll(result, tag, "")
	return result
}

// rebuildTagsIndex rebuilds the tags index from all notes.
func rebuildTagsIndex(vlt *vault.Vault) error {
	notePaths, err := vlt.ListNotes()
	if err != nil {
		return err
	}

	tagCounts := make(map[string]int)
	for _, path := range notePaths {
		n, err := note.Load(path)
		if err != nil {
			continue
		}
		for _, t := range n.Tags {
			tagCounts[t]++
		}
	}

	return vlt.RebuildTagsIndex(tagCounts)
}
