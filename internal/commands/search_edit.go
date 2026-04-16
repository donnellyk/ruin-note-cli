package commands

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

func handleEdit(vlt *vault.Vault, results []SearchResult, force bool, fmMode FrontmatterMode) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	if len(results) == 1 {
		return handleEditSingle(vlt, results[0], force, fmMode, editor)
	}

	return handleEditBulk(vlt, results, force, fmMode, editor)
}

func handleEditSingle(vlt *vault.Vault, result SearchResult, force bool, fmMode FrontmatterMode, editor string) error {
	tmpFile, err := os.CreateTemp("", "ruin-edit-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	content := result.note.Content
	if fmMode == FrontmatterFull {
		serialized, err := result.note.Serialize()
		if err == nil {
			content = serialized
		}
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	originalContent := content

	cmd := exec.Command("sh", "-c", editor+" \"$1\"", "sh", tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	modifiedBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read modified file: %w", err)
	}
	modifiedContent := string(modifiedBytes)

	if modifiedContent == originalContent {
		fmt.Fprintln(os.Stderr, "No changes made")
		return nil
	}

	if strings.TrimSpace(modifiedContent) == "" {
		if !force {
			if !isTerminal(os.Stderr) {
				return fmt.Errorf("deletion requires --force in non-interactive mode")
			}

			fmt.Fprintf(os.Stderr, "The following 1 note(s) will be deleted:\n")
			fmt.Fprintf(os.Stderr, "  - %s\n", result.Path)
			fmt.Fprint(os.Stderr, "Continue? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}

		if err := vlt.DeleteNote(result.note, fmt.Sprintf("ruin search --edit: Delete %q", result.Title)); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Modified: 0, Deleted: 1\n")
		return nil
	}

	oldGlobalTags := result.note.Tags
	oldInlineTags := result.note.InlineTags

	if strings.HasPrefix(strings.TrimLeft(modifiedContent, "\n\r"), "---") {
		fm, body, err := note.ParseFrontmatter(modifiedContent)
		if err != nil {
			return fmt.Errorf("failed to parse frontmatter: %w", err)
		}

		if fm.UUID != "" && fm.UUID != result.note.UUID {
			return fmt.Errorf("cannot change UUID")
		}

		if len(fm.Extra) > 0 {
			if result.note.Extra == nil {
				result.note.Extra = make(map[string]any)
			}
			maps.Copy(result.note.Extra, fm.Extra)
		}

		if len(fm.Tags) > 0 {
			result.note.Tags = fm.Tags
		}

		result.note.Content = body
	} else {
		result.note.Content = modifiedContent
		result.note.RefreshTags()
	}

	if result.note.EnsureLinkTag() {
		result.note.RefreshTags()
	}

	result.note.Content = note.ResolveDateTokens(result.note.Content)
	result.note.RefreshDates()

	result.note.SetTimestamps()

	if titlesIndex, err := vlt.LoadTitles(); err == nil {
		RefreshLinkedCards(result.note, titlesIndex)
	}

	if _, err := RefreshInheritedTags(result.note, vlt); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to refresh inherited tags: %v\n", err)
	}

	if err := result.note.Save(); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	vlt.SaveNote(result.note, oldGlobalTags, oldInlineTags, fmt.Sprintf("ruin search --edit: Update %q", result.Title))

	if !normalizedTagsEqual(oldGlobalTags, result.note.Tags) {
		if titlesIndex, err := vlt.LoadTitles(); err == nil {
			if err := CascadeInheritedTags(result.note.UUID, vlt, titlesIndex); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to cascade inherited tags: %v\n", err)
			}
		}
	}

	fmt.Fprintf(os.Stderr, "Modified: 1, Deleted: 0\n")
	return nil
}

func handleEditBulk(vlt *vault.Vault, results []SearchResult, force bool, fmMode FrontmatterMode, editor string) error {
	tmpFile, err := os.CreateTemp("", "ruin-edit-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	entries := make([]note.BulkEntry, len(results))
	for i, r := range results {
		content := r.note.Content
		if fmMode == FrontmatterFull {
			serialized, err := r.note.Serialize()
			if err == nil {
				content = serialized
			}
		}
		entries[i] = note.BulkEntry{
			UUID:    r.UUID,
			Content: content,
		}
	}

	var original strings.Builder
	if err := note.FormatBulk(entries, &original); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to format bulk content: %w", err)
	}

	if _, err := tmpFile.WriteString(original.String()); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	originalContent := original.String()

	// Use shell so $EDITOR with arguments works (e.g., "code --wait").
	cmd := exec.Command("sh", "-c", editor+" \"$1\"", "sh", tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	modifiedBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read modified file: %w", err)
	}
	modifiedContent := string(modifiedBytes)

	if modifiedContent == originalContent {
		fmt.Fprintln(os.Stderr, "No changes made")
		return nil
	}

	return applyBulkChanges(vlt, originalContent, modifiedContent, results, force)
}

func applyBulkChanges(vlt *vault.Vault, original, modified string, results []SearchResult, force bool) error {
	originalMap := note.ParseBulk(original)
	modifiedMap := note.ParseBulk(modified)

	resultMap := make(map[string]SearchResult)
	for _, r := range results {
		resultMap[r.UUID] = r
	}

	var toModify []string
	var toDelete []string

	for uuid, origContent := range originalMap {
		modContent, exists := modifiedMap[uuid]

		if !exists {
			toDelete = append(toDelete, uuid)
		} else if modContent != origContent {
			toModify = append(toModify, uuid)
		}
	}

	var errors []string
	for uuid := range modifiedMap {
		if _, exists := originalMap[uuid]; !exists {
			errors = append(errors, fmt.Sprintf("New UUID found: %s (use 'log' to create new notes)", uuid))
		}
	}

	if len(toDelete) > 0 && !force {
		if !isTerminal(os.Stderr) {
			return fmt.Errorf("deletions require --force in non-interactive mode")
		}

		fmt.Fprintf(os.Stderr, "The following %d note(s) will be deleted:\n", len(toDelete))
		for _, uuid := range toDelete {
			result, ok := resultMap[uuid]
			if ok {
				fmt.Fprintf(os.Stderr, "  - %s\n", result.Path)
			} else {
				fmt.Fprintf(os.Stderr, "  - UUID: %s (path not found)\n", uuid)
			}
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

	titlesIndex, titlesErr := vlt.LoadTitles()

	var modifiedCount int
	for _, uuid := range toModify {
		result, ok := resultMap[uuid]
		if !ok {
			errors = append(errors, fmt.Sprintf("UUID not found: %s", uuid))
			continue
		}

		oldGlobalTags := result.note.Tags
		oldInlineTags := result.note.InlineTags

		modContent := modifiedMap[uuid]

		if strings.HasPrefix(strings.TrimLeft(modContent, "\n\r"), "---") {
			fm, body, err := note.ParseFrontmatter(modContent)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to parse frontmatter for %s: %v", result.Path, err))
				continue
			}

			if fm.UUID != "" && fm.UUID != result.note.UUID {
				errors = append(errors, fmt.Sprintf("Cannot change UUID for %s", result.Path))
				continue
			}

			if len(fm.Extra) > 0 {
				if result.note.Extra == nil {
					result.note.Extra = make(map[string]any)
				}
				maps.Copy(result.note.Extra, fm.Extra)
			}

			if len(fm.Tags) > 0 {
				result.note.Tags = fm.Tags
			}

			result.note.Content = body
		} else {
			result.note.Content = modContent
		}

		if !strings.HasPrefix(strings.TrimLeft(modContent, "\n\r"), "---") {
			result.note.RefreshTags()
		}

		if result.note.EnsureLinkTag() {
			result.note.RefreshTags()
		}

		result.note.Content = note.ResolveDateTokens(result.note.Content)
		result.note.RefreshDates()

		if titlesErr == nil {
			RefreshLinkedCards(result.note, titlesIndex)
		}

		if _, err := RefreshInheritedTags(result.note, vlt); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to refresh inherited tags: %v\n", err)
		}

		result.note.SetTimestamps()

		if err := result.note.Save(); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to save %s: %v", result.Path, err))
			continue
		}

		vlt.SaveNote(result.note, oldGlobalTags, oldInlineTags, fmt.Sprintf("ruin search --edit: Update %q", result.note.Title))

		if !normalizedTagsEqual(oldGlobalTags, result.note.Tags) {
			if ti, err := vlt.LoadTitles(); err == nil {
				if err := CascadeInheritedTags(result.note.UUID, vlt, ti); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to cascade inherited tags: %v\n", err)
				}
			}
		}

		modifiedCount++
	}

	var deletedCount int
	for _, uuid := range toDelete {
		result, ok := resultMap[uuid]
		if !ok {
			errors = append(errors, fmt.Sprintf("UUID not found in vault: %s", uuid))
			continue
		}

		if err := vlt.DeleteNote(result.note, fmt.Sprintf("ruin search --edit: Delete %q", result.Title)); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to delete %s: %v", result.Path, err))
			continue
		}

		deletedCount++
	}

	fmt.Fprintf(os.Stderr, "Modified: %d, Deleted: %d\n", modifiedCount, deletedCount)
	if len(errors) > 0 {
		fmt.Fprintf(os.Stderr, "Errors:\n")
		for _, e := range errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
	}

	return nil
}
