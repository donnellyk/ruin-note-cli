package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
)

// handleEdit opens results in $EDITOR and saves changes.
func handleEdit(vlt *vault.Vault, results []SearchResult, force bool, fmMode FrontmatterMode) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// Single note: use simple format without bulk separators
	if len(results) == 1 {
		return handleEditSingle(vlt, results[0], force, fmMode, editor)
	}

	// Multiple notes: use bulk format
	return handleEditBulk(vlt, results, force, fmMode, editor)
}

// handleEditSingle handles editing a single note without bulk separators.
func handleEditSingle(vlt *vault.Vault, result SearchResult, force bool, fmMode FrontmatterMode, editor string) error {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "ruin-edit-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Prepare content
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

	// Open editor
	cmd := exec.Command("sh", "-c", editor+" \"$1\"", "sh", tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	// Read modified content
	modifiedBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read modified file: %w", err)
	}
	modifiedContent := string(modifiedBytes)

	// If no changes, nothing to do
	if modifiedContent == originalContent {
		fmt.Fprintln(os.Stderr, "No changes made")
		return nil
	}

	// Check for deletion (empty content)
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

	// Capture old tags before modification for index update
	oldGlobalTags := result.note.Tags
	oldInlineTags := result.note.InlineTags

	// Apply changes
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
				result.note.Extra = make(map[string]interface{})
			}
			for k, v := range fm.Extra {
				result.note.Extra[k] = v
			}
		}

		if len(fm.Tags) > 0 {
			result.note.Tags = fm.Tags
		}

		result.note.Content = body
	} else {
		result.note.Content = modifiedContent
		result.note.RefreshTags()
	}

	// Resolve date tokens and extract dates
	result.note.Content = note.ResolveDateTokens(result.note.Content)
	result.note.RefreshDates()

	result.note.SetTimestamps()

	// Refresh linked-cards from wiki links
	if titlesIndex, err := vlt.LoadTitles(); err == nil {
		RefreshLinkedCards(result.note, titlesIndex)
	}

	// Refresh inherited tags
	if _, err := RefreshInheritedTags(result.note, vlt); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to refresh inherited tags: %v\n", err)
	}

	if err := result.note.Save(); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	vlt.SaveNote(result.note, oldGlobalTags, oldInlineTags, fmt.Sprintf("ruin search --edit: Update %q", result.Title))

	// Cascade if global tags changed
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

// handleEditBulk handles editing multiple notes with bulk format.
func handleEditBulk(vlt *vault.Vault, results []SearchResult, force bool, fmMode FrontmatterMode, editor string) error {
	// Create temp file with bulk format
	tmpFile, err := os.CreateTemp("", "ruin-edit-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write original content
	entries := make([]note.BulkEntry, len(results))
	for i, r := range results {
		content := r.note.Content
		if fmMode == FrontmatterFull {
			// Include full frontmatter in the content for editing
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

	// Save original for comparison
	originalContent := original.String()

	// Open editor - use shell to handle $EDITOR with arguments (e.g., "code --wait")
	cmd := exec.Command("sh", "-c", editor+" \"$1\"", "sh", tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	// Read modified content
	modifiedBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read modified file: %w", err)
	}
	modifiedContent := string(modifiedBytes)

	// If no changes, nothing to do
	if modifiedContent == originalContent {
		fmt.Fprintln(os.Stderr, "No changes made")
		return nil
	}

	// Parse and apply changes
	return applyBulkChanges(vlt, originalContent, modifiedContent, results, force)
}

// applyBulkChanges applies changes from bulk edit.
func applyBulkChanges(vlt *vault.Vault, original, modified string, results []SearchResult, force bool) error {
	// Parse original into uuid -> content map
	originalMap := note.ParseBulk(original)
	modifiedMap := note.ParseBulk(modified)

	// Build uuid -> result map
	resultMap := make(map[string]SearchResult)
	for _, r := range results {
		resultMap[r.UUID] = r
	}

	// First pass: collect modifications and deletions
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

	// Check for new UUIDs (error case)
	var errors []string
	for uuid := range modifiedMap {
		if _, exists := originalMap[uuid]; !exists {
			errors = append(errors, fmt.Sprintf("New UUID found: %s (use 'log' to create new notes)", uuid))
		}
	}

	// Handle deletions - require confirmation or --force
	if len(toDelete) > 0 && !force {
		// Check if stderr is a TTY for interactive confirmation
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

	// Load titles index for linked-cards resolution
	titlesIndex, titlesErr := vlt.LoadTitles()

	// Apply modifications
	var modifiedCount int
	for _, uuid := range toModify {
		result, ok := resultMap[uuid]
		if !ok {
			errors = append(errors, fmt.Sprintf("UUID not found: %s", uuid))
			continue
		}

		// Capture old tags before modification for index update
		oldGlobalTags := result.note.Tags
		oldInlineTags := result.note.InlineTags

		modContent := modifiedMap[uuid]

		// Check if modified content includes frontmatter
		if strings.HasPrefix(strings.TrimLeft(modContent, "\n\r"), "---") {
			// Parse frontmatter from modified content
			fm, body, err := note.ParseFrontmatter(modContent)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to parse frontmatter for %s: %v", result.Path, err))
				continue
			}

			// Protect immutable fields
			if fm.UUID != "" && fm.UUID != result.note.UUID {
				errors = append(errors, fmt.Sprintf("Cannot change UUID for %s", result.Path))
				continue
			}

			// Apply allowed frontmatter changes
			// Extra fields can be modified
			if len(fm.Extra) > 0 {
				if result.note.Extra == nil {
					result.note.Extra = make(map[string]interface{})
				}
				for k, v := range fm.Extra {
					result.note.Extra[k] = v
				}
			}

			// Tags from frontmatter override extracted tags if explicitly set
			if len(fm.Tags) > 0 {
				result.note.Tags = fm.Tags
			}

			// Set content (without frontmatter)
			result.note.Content = body
		} else {
			// No frontmatter - just update content
			result.note.Content = modContent
		}

		// Refresh tags from content (unless overridden by frontmatter)
		if !strings.HasPrefix(strings.TrimLeft(modContent, "\n\r"), "---") {
			result.note.RefreshTags()
		}

		// Resolve date tokens and extract dates
		result.note.Content = note.ResolveDateTokens(result.note.Content)
		result.note.RefreshDates()

		// Refresh linked-cards from wiki links
		if titlesErr == nil {
			RefreshLinkedCards(result.note, titlesIndex)
		}

		// Refresh inherited tags
		if _, err := RefreshInheritedTags(result.note, vlt); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to refresh inherited tags: %v\n", err)
		}

		result.note.SetTimestamps()

		if err := result.note.Save(); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to save %s: %v", result.Path, err))
			continue
		}

		vlt.SaveNote(result.note, oldGlobalTags, oldInlineTags, fmt.Sprintf("ruin search --edit: Update %q", result.note.Title))

		// Cascade if global tags changed
		if !normalizedTagsEqual(oldGlobalTags, result.note.Tags) {
			if ti, err := vlt.LoadTitles(); err == nil {
				if err := CascadeInheritedTags(result.note.UUID, vlt, ti); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to cascade inherited tags: %v\n", err)
				}
			}
		}

		modifiedCount++
	}

	// Apply deletions
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

	// Report results
	fmt.Fprintf(os.Stderr, "Modified: %d, Deleted: %d\n", modifiedCount, deletedCount)
	if len(errors) > 0 {
		fmt.Fprintf(os.Stderr, "Errors:\n")
		for _, e := range errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
	}

	return nil
}
