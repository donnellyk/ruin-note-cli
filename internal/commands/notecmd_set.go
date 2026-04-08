package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

// --- note set ---

type noteSetChange struct {
	Field  string `json:"field"`
	Action string `json:"action"`
	Value  any    `json:"value"`
}

type noteSetOutput struct {
	Path    string          `json:"path"`
	UUID    string          `json:"uuid"`
	Title   string          `json:"title"`
	Changes []noteSetChange `json:"changes"`
}

func newNoteSetCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		addTags     []string
		removeTags  []string
		order       int
		noOrder     bool
		fields      []string
		parent      string
		noParent    bool
		force       bool
		line        int
		addDates    []string
		removeDates []string
		removeAllDt bool
		toggleTodo  bool
		sink        bool
	)

	cmd := &cobra.Command{
		Use:   "set <note> [flags]",
		Short: "Set metadata and tags on a note",
		Long: `Modify a note's tags, dates, order, parent, or extra frontmatter fields.

At least one mutation flag is required. Multiple flags can be combined
to batch changes in a single operation.

Use --line N to target a specific content line (1-indexed, after frontmatter).
Without --line, tags are added globally and removed from all lines.`,
		Example: `  ruin note set "My Note" --add-tag "#urgent"
  ruin note set <uuid> --remove-tag "#wip" --add-tag "#done"
  ruin note set <uuid> --add-tag "#inline" --line 3
  ruin note set <uuid> --add-date today
  ruin note set <uuid> --add-date tomorrow --line 3
  ruin note set <uuid> --remove-date @2026-03-15
  ruin note set <uuid> --remove-dates
  ruin note set <uuid> --order 1
  ruin note set <uuid> --no-order
  ruin note set <uuid> --field "status=active"
  ruin note set <uuid> --field "status="  # deletes the field
  ruin note set <uuid> --parent "Hub Note"
  ruin note set <uuid> --no-parent
  ruin note set <uuid> --toggle-todo --line 5
  ruin note set <uuid> --toggle-todo --line 5 --sink`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate mutually exclusive flags
			if noOrder && cmd.Flags().Changed("order") {
				return fmt.Errorf("--order and --no-order are mutually exclusive")
			}
			if noParent && parent != "" {
				return fmt.Errorf("--parent and --no-parent are mutually exclusive")
			}

			// --toggle-todo requires --line
			if toggleTodo && !cmd.Flags().Changed("line") {
				return fmt.Errorf("--toggle-todo requires --line")
			}
			// --sink requires --toggle-todo
			if sink && !toggleTodo {
				return fmt.Errorf("--sink requires --toggle-todo")
			}

			// --line requires a tag, date, or toggle-todo flag
			hasLineTarget := cmd.Flags().Changed("line")
			if hasLineTarget && len(addTags) == 0 && len(removeTags) == 0 &&
				len(addDates) == 0 && len(removeDates) == 0 && !removeAllDt && !toggleTodo {
				return fmt.Errorf("--line requires --add-tag, --remove-tag, --add-date, --remove-date, --remove-dates, or --toggle-todo")
			}

			// At least one mutation required
			hasMutation := len(addTags) > 0 || len(removeTags) > 0 ||
				cmd.Flags().Changed("order") || noOrder ||
				len(fields) > 0 || parent != "" || noParent ||
				len(addDates) > 0 || len(removeDates) > 0 || removeAllDt ||
				toggleTodo
			if !hasMutation {
				return fmt.Errorf("at least one mutation flag is required (--add-tag, --remove-tag, --add-date, --remove-date, --remove-dates, --order, --no-order, --field, --parent, --no-parent, --toggle-todo)")
			}

			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			n, err := ResolveNote(vlt, args[0])
			if err != nil {
				return err
			}

			// Validate --line range
			if hasLineTarget {
				if line < 1 {
					return fmt.Errorf("--line must be positive (got %d)", line)
				}
				contentLines := strings.Split(n.Content, "\n")
				if line > len(contentLines) {
					return fmt.Errorf("--line %d out of range (note has %d content lines)", line, len(contentLines))
				}
			}

			// Capture old tags for index update
			oldGlobal := n.Tags
			oldInline := n.InlineTags

			var changes []noteSetChange

			// --- add tags ---
			for _, raw := range addTags {
				tag := ensureHashPrefix(raw)
				if !hasLineTarget {
					// Global add (current behavior)
					if noteHasTag(n, tag) {
						continue
					}
					n.Content = insertGlobalTag(n.Content, tag)
				} else {
					// Inline add to specific line
					var err error
					n.Content, err = insertInlineTag(n.Content, tag, line)
					if err != nil {
						return err
					}
				}
				changes = append(changes, noteSetChange{Field: "tag", Action: "added", Value: tag})
			}

			// --- remove tags ---
			for _, raw := range removeTags {
				tag := ensureHashPrefix(raw)
				if !hasLineTarget {
					// Remove from all lines (current behavior)
					if !noteHasTag(n, tag) {
						continue
					}
					n.Content = removeTagClean(n.Content, tag)
				} else {
					// Remove from specific line only
					var err error
					n.Content, err = removeTagFromLineNum(n.Content, tag, line)
					if err != nil {
						return err
					}
				}
				changes = append(changes, noteSetChange{Field: "tag", Action: "removed", Value: tag})
			}

			// --- add dates ---
			for _, raw := range addDates {
				dateStr, err := resolveDateArg(raw)
				if err != nil {
					return err
				}
				var targetLine int
				if hasLineTarget {
					targetLine = line
				}
				n.Content, err = insertDateInContent(n.Content, dateStr, targetLine)
				if err != nil {
					return err
				}
				changes = append(changes, noteSetChange{Field: "date", Action: "added", Value: dateStr})
			}

			// --- remove dates ---
			if removeAllDt {
				var targetLine int
				if hasLineTarget {
					targetLine = line
				}
				n.Content = removeDateFromContent(n.Content, "", targetLine)
				changes = append(changes, noteSetChange{Field: "date", Action: "removed-all", Value: nil})
			}
			for _, raw := range removeDates {
				dateStr, err := resolveDateArg(raw)
				if err != nil {
					return err
				}
				var targetLine int
				if hasLineTarget {
					targetLine = line
				}
				n.Content = removeDateFromContent(n.Content, dateStr, targetLine)
				changes = append(changes, noteSetChange{Field: "date", Action: "removed", Value: dateStr})
			}

			// --- toggle-todo ---
			if toggleTodo {
				contentLines := strings.Split(n.Content, "\n")
				idx := line - 1
				if !note.IsCheckboxLine(contentLines[idx]) {
					return fmt.Errorf("line %d is not a checkbox", line)
				}
				wasChecked := note.IsCheckedLine(contentLines[idx])
				contentLines[idx] = note.ToggleCheckbox(contentLines[idx])
				toggledContent := strings.TrimSpace(contentLines[idx])

				// --sink: reposition within contiguous checkbox block
				//   Completing: move below all open todos, above completed ones
				//   Uncompleting: move to bottom of open todos
				if sink {
					// Find boundaries of contiguous checkbox block
					blockStart := idx
					for j := idx - 1; j >= 0; j-- {
						if note.IsCheckboxLine(contentLines[j]) {
							blockStart = j
						} else {
							break
						}
					}
					blockEnd := idx
					for j := idx + 1; j < len(contentLines); j++ {
						if note.IsCheckboxLine(contentLines[j]) {
							blockEnd = j
						} else {
							break
						}
					}

					// Remove the toggled line from its current position
					toggled := contentLines[idx]
					remaining := make([]string, 0, len(contentLines)-1)
					remaining = append(remaining, contentLines[:idx]...)
					remaining = append(remaining, contentLines[idx+1:]...)

					// Adjust block bounds for the shortened slice
					adjEnd := blockEnd - 1 // blockStart unchanged since idx >= blockStart

					// Find insertion point: after last open todo in the block
					insertAt := blockStart
					for j := blockStart; j <= adjEnd; j++ {
						if !note.IsCheckedLine(remaining[j]) {
							insertAt = j + 1
						}
					}

					// Re-insert at the boundary
					contentLines = make([]string, 0, len(remaining)+1)
					contentLines = append(contentLines, remaining[:insertAt]...)
					contentLines = append(contentLines, toggled)
					contentLines = append(contentLines, remaining[insertAt:]...)
				}

				n.Content = strings.Join(contentLines, "\n")
				action := "checked"
				if wasChecked {
					action = "unchecked"
				}
				changes = append(changes, noteSetChange{Field: "todo", Action: action, Value: toggledContent})
			}

			// --- order ---
			if cmd.Flags().Changed("order") {
				n.Order = &order
				changes = append(changes, noteSetChange{Field: "order", Action: "set", Value: order})
			}
			if noOrder {
				n.Order = nil
				changes = append(changes, noteSetChange{Field: "order", Action: "unset", Value: nil})
			}

			// --- extra fields ---
			if n.Extra == nil {
				n.Extra = make(map[string]any)
			}
			for _, kv := range fields {
				key, val, ok := strings.Cut(kv, "=")
				if !ok {
					return fmt.Errorf("--field requires key=value format, got %q", kv)
				}
				if key == "" {
					return fmt.Errorf("--field key cannot be empty")
				}
				if val == "" {
					delete(n.Extra, key)
					changes = append(changes, noteSetChange{Field: key, Action: "deleted", Value: nil})
				} else {
					n.Extra[key] = val
					changes = append(changes, noteSetChange{Field: key, Action: "set", Value: val})
				}
			}

			// --- parent ---
			if parent != "" {
				parentNote, err := ResolveNote(vlt, parent)
				if err != nil {
					return fmt.Errorf("parent: %w", err)
				}
				if parentNote.UUID == n.UUID {
					return fmt.Errorf("a note cannot be its own parent")
				}
				index, err := vlt.LoadTitles()
				if err != nil {
					return fmt.Errorf("failed to load titles index: %w", err)
				}
				if err := detectCycle(index, n.UUID, parentNote.UUID); err != nil {
					return err
				}
				if n.Parent != "" && n.Parent != parentNote.UUID && !force {
					existingTitle := n.Parent
					if entry, ok := index.Titles[n.Parent]; ok {
						existingTitle = entry.Title
					}
					return fmt.Errorf("note already has parent %q (use --force to overwrite)", existingTitle)
				}
				n.Parent = parentNote.UUID
				changes = append(changes, noteSetChange{Field: "parent", Action: "set", Value: parentNote.UUID})
			}
			if noParent {
				if n.Parent == "" {
					// Already no parent, but still count it
				} else {
					n.Parent = ""
					changes = append(changes, noteSetChange{Field: "parent", Action: "removed", Value: nil})
				}
			}

			if len(changes) == 0 {
				// All ops were no-ops
				if *jsonOutput {
					out := noteSetOutput{Path: n.FilePath, UUID: n.UUID, Title: n.Title, Changes: changes}
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out)
				}
				fmt.Fprintf(os.Stderr, "No changes to %s\n", n.FilePath)
				return nil
			}

			// Shared post-modification flow
			if err := saveWithIndexUpdate(n, vlt); err != nil {
				return err
			}

			vlt.SaveNote(n, oldGlobal, oldInline, fmt.Sprintf("ruin note set: Update %q", n.Title))

			if *jsonOutput {
				out := noteSetOutput{Path: n.FilePath, UUID: n.UUID, Title: n.Title, Changes: changes}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Fprintf(os.Stderr, "Modified %s\n", n.FilePath)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&addTags, "add-tag", nil, "add a tag (global by default, inline with --line)")
	cmd.Flags().StringArrayVar(&removeTags, "remove-tag", nil, "remove a tag (all lines by default, specific line with --line)")
	cmd.Flags().IntVar(&line, "line", 0, "target content line (1-indexed, after frontmatter)")
	cmd.Flags().StringArrayVar(&addDates, "add-date", nil, "add a @YYYY-MM-DD date reference (repeatable, accepts today/tomorrow/etc)")
	cmd.Flags().StringArrayVar(&removeDates, "remove-date", nil, "remove a specific @YYYY-MM-DD date (repeatable)")
	cmd.Flags().BoolVar(&removeAllDt, "remove-dates", false, "remove all @YYYY-MM-DD dates")
	cmd.Flags().IntVar(&order, "order", 0, "set order frontmatter field")
	cmd.Flags().BoolVar(&noOrder, "no-order", false, "unset order field")
	cmd.Flags().StringArrayVar(&fields, "field", nil, "set extra frontmatter field (key=value, empty value deletes)")
	cmd.Flags().StringVar(&parent, "parent", "", "set parent (UUID, title, path, or bookmark)")
	cmd.Flags().BoolVar(&noParent, "no-parent", false, "remove parent")
	cmd.Flags().BoolVar(&toggleTodo, "toggle-todo", false, "flip checkbox state (requires --line)")
	cmd.Flags().BoolVar(&sink, "sink", false, "reposition toggled item: completed below open, uncompleted to bottom of open (requires --toggle-todo)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation")

	return cmd
}
