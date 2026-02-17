package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"kvnd/ruin-note-cli/internal/dateparse"
	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"

	"github.com/spf13/cobra"
)

// NewNoteCmd creates the note command group with subcommands.
func NewNoteCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Programmatic note mutations",
		Long:  `Commands for modifying individual notes: set metadata/tags, append content, merge notes.`,
	}

	cmd.AddCommand(newNoteSetCmd(getVault, jsonOutput))
	cmd.AddCommand(newNoteAppendCmd(getVault, jsonOutput))
	cmd.AddCommand(newNoteMergeCmd(getVault, jsonOutput))

	return cmd
}

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
				nowChecked := !wasChecked

				// --sink: move checked item to bottom of contiguous checkbox block
				if sink && nowChecked {
					// Find end of contiguous checkbox block from this line
					blockEnd := idx
					for j := idx + 1; j < len(contentLines); j++ {
						if note.IsCheckboxLine(contentLines[j]) {
							blockEnd = j
						} else {
							break
						}
					}
					if blockEnd > idx {
						toggled := contentLines[idx]
						copy(contentLines[idx:blockEnd], contentLines[idx+1:blockEnd+1])
						contentLines[blockEnd] = toggled
					}
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
			if err := saveWithIndexUpdate(n, vlt, oldGlobal, oldInline); err != nil {
				return err
			}

			// Commit to version history
			vlt.Commit(fmt.Sprintf("ruin note set: Update %q", n.Title))

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
	cmd.Flags().BoolVar(&sink, "sink", false, "move checked item to bottom of checkbox block (requires --toggle-todo)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation")

	return cmd
}

// --- note append ---

type noteAppendOutput struct {
	Path   string `json:"path"`
	UUID   string `json:"uuid"`
	Line   int    `json:"line"`
	Action string `json:"action"`
}

func newNoteAppendCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var (
		line    int
		suffix  bool
		stdin   bool
		rawLine bool
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "append <note> [text]",
		Short: "Append or insert content into a note",
		Long: `Insert text into a note's content.

Without --line, appends text as a new line at the end.
With --line N, inserts before content line N (1-indexed, after frontmatter).
With --line N --suffix, appends text to the end of line N.
With --raw-line, line numbers count from the top of the file (including frontmatter).

Text comes from a positional argument or --stdin (mutually exclusive).`,
		Example: `  ruin note append "My Note" "New paragraph"
  ruin note append <uuid> --line 3 "Inserted line"
  ruin note append <uuid> --line 1 --suffix " (continued)"
  ruin note append <uuid> --raw-line --line 10 "After frontmatter line 10"
  echo "piped text" | ruin note append <uuid> --stdin`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if suffix && !cmd.Flags().Changed("line") {
				return fmt.Errorf("--suffix requires --line")
			}
			if rawLine && !cmd.Flags().Changed("line") {
				return fmt.Errorf("--raw-line requires --line")
			}

			// Determine text
			hasArg := len(args) >= 2
			if hasArg && stdin {
				return fmt.Errorf("cannot use both positional text and --stdin")
			}
			if !hasArg && !stdin {
				return fmt.Errorf("text required as argument or via --stdin")
			}

			var text string
			if stdin {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				text = string(data)
				// Remove a single trailing newline from piped input
				text = strings.TrimRight(text, "\n")
			} else {
				text = args[1]
			}

			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			n, err := ResolveNote(vlt, args[0])
			if err != nil {
				return err
			}

			// Translate --raw-line to content-relative line
			if rawLine && cmd.Flags().Changed("line") {
				fmLines, err := frontmatterLineCount(n)
				if err != nil {
					return fmt.Errorf("failed to compute frontmatter size: %w", err)
				}
				line = line - fmLines
				if line < 1 {
					return fmt.Errorf("--line %d (raw) points into frontmatter (content starts at line %d)", line+fmLines, fmLines+1)
				}
			}

			oldGlobal := n.Tags
			oldInline := n.InlineTags

			lines := strings.Split(n.Content, "\n")
			totalLines := len(lines)

			var resultLine int
			var action string

			if !cmd.Flags().Changed("line") {
				// Append at end
				// Ensure content ends with newline before appending
				if n.Content != "" && !strings.HasSuffix(n.Content, "\n") {
					n.Content += "\n"
				}
				n.Content += text + "\n"
				resultLine = totalLines + 1
				action = "appended"
			} else {
				if line < 1 || line > totalLines+1 {
					return fmt.Errorf("--line %d out of range (valid: 1-%d)", line, totalLines+1)
				}
				idx := line - 1 // 0-indexed
				if suffix {
					// Append to end of existing line
					lines[idx] += text
					n.Content = strings.Join(lines, "\n")
					resultLine = line
					action = "appended"
				} else {
					// Insert new line before idx
					newLines := make([]string, 0, totalLines+1)
					newLines = append(newLines, lines[:idx]...)
					newLines = append(newLines, text)
					newLines = append(newLines, lines[idx:]...)
					n.Content = strings.Join(newLines, "\n")
					resultLine = line
					action = "inserted"
				}
			}

			if err := saveWithIndexUpdate(n, vlt, oldGlobal, oldInline); err != nil {
				return err
			}

			// Commit to version history
			vlt.Commit(fmt.Sprintf("ruin note append: Update %q", n.Title))

			if *jsonOutput {
				out := noteAppendOutput{Path: n.FilePath, UUID: n.UUID, Line: resultLine, Action: action}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Fprintf(os.Stderr, "Modified %s\n", n.FilePath)
			return nil
		},
	}

	cmd.Flags().IntVar(&line, "line", 0, "target content line (1-indexed)")
	cmd.Flags().BoolVar(&suffix, "suffix", false, "append to end of line (requires --line)")
	cmd.Flags().BoolVar(&rawLine, "raw-line", false, "line numbers count from top of file (including frontmatter)")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read text from stdin")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation")

	return cmd
}

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
			if err := saveWithIndexUpdate(target, vlt, oldTargetGlobal, oldTargetInline); err != nil {
				return err
			}

			// 5. Delete source if requested
			sourceDeleted := false
			if deleteSource {
				oldSourceGlobal := source.Tags
				oldSourceInline := source.InlineTags

				if err := os.Remove(source.FilePath); err != nil {
					return fmt.Errorf("failed to delete source: %w", err)
				}
				vlt.DecrementTagsIndex(oldSourceGlobal, oldSourceInline)
				vlt.RemoveTitleEntry(source.UUID)
				sourceDeleted = true
			}

			// Commit to version history
			vlt.Commit(fmt.Sprintf("ruin note merge: Merge %q into %q", source.Title, target.Title))

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

// --- Shared helpers ---

// frontmatterLineCount returns the number of lines the frontmatter occupies
// in the serialized file (including the --- delimiters and trailing newline).
func frontmatterLineCount(n *note.Note) (int, error) {
	serialized, err := n.Serialize()
	if err != nil {
		return 0, err
	}
	// Content starts where frontmatter ends. Count lines in the frontmatter portion.
	fmLen := len(serialized) - len(n.Content)
	fm := serialized[:fmLen]
	// Count newlines in the frontmatter string
	return strings.Count(fm, "\n"), nil
}

// saveWithIndexUpdate performs the shared post-modification flow.
func saveWithIndexUpdate(n *note.Note, vlt *vault.Vault, oldGlobal, oldInline []string) error {
	n.RefreshTags()
	n.Content = note.ResolveDateTokens(n.Content)
	n.RefreshDates()

	titlesIndex, err := vlt.LoadTitles()
	if err == nil {
		RefreshLinkedCards(n, titlesIndex)
	}

	n.SetTimestamps()

	if err := n.Save(); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	vlt.DecrementTagsIndex(oldGlobal, oldInline)
	vlt.UpdateTagsIndex(n.Tags, n.InlineTags)
	vlt.UpdateTitleEntry(n.UUID, n.Title, n.FilePath, n.Parent)

	return nil
}

// ensureHashPrefix adds # prefix if missing.
func ensureHashPrefix(tag string) string {
	if !strings.HasPrefix(tag, "#") {
		return "#" + tag
	}
	return tag
}

// noteHasTag checks if a note already has a tag (case-insensitive).
func noteHasTag(n *note.Note, tag string) bool {
	normalized := note.NormalizeTag(tag)
	for _, t := range n.AllTags() {
		if note.NormalizeTag(t) == normalized {
			return true
		}
	}
	return false
}

// insertGlobalTag inserts a global tag into note content.
// If a tag-only line exists, appends to the first one using the same separator.
// Otherwise, inserts a new tag-only line after the title header (or at line 0).
func insertGlobalTag(content, tag string) string {
	lines := strings.Split(content, "\n")

	// Find the first tag-only line
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if note.IsTagOnlyLine(trimmed) {
			sep := detectSeparator(trimmed)
			lines[i] = trimmed + sep + tag
			return strings.Join(lines, "\n")
		}
	}

	// No tag-only line found — insert one after the title header
	insertIdx := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if note.IsHeaderLine(trimmed) {
			insertIdx = i + 1
			break
		}
	}

	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, tag)
	newLines = append(newLines, lines[insertIdx:]...)
	return strings.Join(newLines, "\n")
}

// detectSeparator returns the separator used between tags on a tag-only line.
// Returns ", " if commas are used, otherwise " ".
func detectSeparator(line string) string {
	if strings.Contains(line, ", ") {
		return ", "
	}
	if strings.Contains(line, ",") {
		return ", "
	}
	return " "
}

// removeTagClean removes a tag from content with proper cleanup.
// - Removes trailing/leading separators
// - Removes lines that become empty after tag removal
// - Trims trailing whitespace on lines where inline tag was removed
func removeTagClean(content, tag string) string {
	normalized := note.NormalizeTag(tag)
	lines := strings.Split(content, "\n")
	var result []string

	for _, line := range lines {
		newLine := removeTagFromLine(line, normalized)
		// If a tag-only line is now empty, skip it
		trimmed := strings.TrimSpace(newLine)
		if trimmed == "" && note.IsTagOnlyLine(strings.TrimSpace(line)) {
			continue
		}
		result = append(result, newLine)
	}

	return strings.Join(result, "\n")
}

// removeTagFromLine removes all occurrences of a tag from a single line.
func removeTagFromLine(line, normalizedTag string) string {
	matches := note.ExtractTagMatches(line)
	if len(matches) == 0 {
		return line
	}

	// Find which matches to remove (by normalized comparison)
	var toRemove []note.TagMatch
	for _, m := range matches {
		if note.NormalizeTag(m.Tag) == normalizedTag {
			toRemove = append(toRemove, m)
		}
	}

	if len(toRemove) == 0 {
		return line
	}

	// Remove matches from end to start to preserve positions
	result := line
	for i := len(toRemove) - 1; i >= 0; i-- {
		m := toRemove[i]
		before := result[:m.Start]
		after := result[m.End:]
		result = before + after
	}

	// Clean up separators: remove double commas, leading/trailing commas
	result = cleanSeparators(result)
	// Trim trailing whitespace
	result = strings.TrimRight(result, " \t")

	return result
}

// cleanSeparators cleans up leftover separator characters after tag removal.
func cleanSeparators(line string) string {
	// Collapse multiple spaces
	for strings.Contains(line, "  ") {
		line = strings.ReplaceAll(line, "  ", " ")
	}
	// Remove ", ," or ",," patterns
	for strings.Contains(line, ",,") {
		line = strings.ReplaceAll(line, ",,", ",")
	}
	for strings.Contains(line, ", ,") {
		line = strings.ReplaceAll(line, ", ,", ",")
	}
	// Remove leading/trailing commas (with optional whitespace)
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, ", ")
	line = strings.TrimRight(line, ", ")
	// If trimming removed meaningful whitespace, keep at least the trimmed form
	return line
}

// --- Line-targeted tag helpers ---

// insertInlineTag appends a tag to the end of a specific content line.
func insertInlineTag(content, tag string, lineNum int) (string, error) {
	lines := strings.Split(content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return "", fmt.Errorf("--line %d out of range (note has %d content lines)", lineNum, len(lines))
	}
	idx := lineNum - 1
	lines[idx] = strings.TrimRight(lines[idx], " \t") + " " + tag
	return strings.Join(lines, "\n"), nil
}

// removeTagFromLineNum removes a tag from a specific content line only.
func removeTagFromLineNum(content, tag string, lineNum int) (string, error) {
	lines := strings.Split(content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return "", fmt.Errorf("--line %d out of range (note has %d content lines)", lineNum, len(lines))
	}
	idx := lineNum - 1
	normalized := note.NormalizeTag(tag)
	newLine := removeTagFromLine(lines[idx], normalized)
	// If a tag-only line becomes empty, remove it
	trimmed := strings.TrimSpace(newLine)
	if trimmed == "" && note.IsTagOnlyLine(strings.TrimSpace(lines[idx])) {
		lines = append(lines[:idx], lines[idx+1:]...)
	} else {
		lines[idx] = newLine
	}
	return strings.Join(lines, "\n"), nil
}

// --- Date helpers ---

// resolveDateArg takes a user-provided date argument (with or without @),
// resolves natural language tokens, and returns "@YYYY-MM-DD".
func resolveDateArg(raw string) (string, error) {
	token := strings.TrimPrefix(raw, "@")
	resolved, ok := dateparse.ResolveDate(token)
	if !ok {
		return "", fmt.Errorf("unrecognized date: %q", raw)
	}
	return "@" + resolved.Format("2006-01-02"), nil
}

// resolvedDateRe matches @YYYY-MM-DD patterns for removal.
var resolvedDateRe = regexp.MustCompile(`\s*@\d{4}-\d{2}-\d{2}`)

// specificDateRe returns a regex matching a specific @YYYY-MM-DD date.
func specificDateRe(date string) *regexp.Regexp {
	// date is already "@YYYY-MM-DD"
	return regexp.MustCompile(`\s*` + regexp.QuoteMeta(date))
}

// insertDateInContent inserts a date reference into content.
// If lineNum==0: insert on tag-only line (like insertGlobalTag).
// Otherwise: append to end of specified line.
func insertDateInContent(content, date string, lineNum int) (string, error) {
	if lineNum == 0 {
		// Insert like a global tag — on the first tag-only line or after title
		return insertGlobalTag(content, date), nil
	}
	lines := strings.Split(content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return "", fmt.Errorf("--line %d out of range (note has %d content lines)", lineNum, len(lines))
	}
	idx := lineNum - 1
	lines[idx] = strings.TrimRight(lines[idx], " \t") + " " + date
	return strings.Join(lines, "\n"), nil
}

// removeDateFromContent removes date references from content.
// If date is empty, removes ALL @YYYY-MM-DD patterns.
// If date is set (e.g. "@2026-03-15"), removes only that specific date.
// If lineNum==0: operates on all lines. Otherwise on specific line only.
func removeDateFromContent(content, date string, lineNum int) string {
	lines := strings.Split(content, "\n")

	var re *regexp.Regexp
	if date == "" {
		re = resolvedDateRe
	} else {
		re = specificDateRe(date)
	}

	start, end := 0, len(lines)
	if lineNum > 0 && lineNum <= len(lines) {
		start = lineNum - 1
		end = lineNum
	}

	var result []string
	for i, l := range lines {
		if i >= start && i < end {
			newLine := re.ReplaceAllString(l, "")
			newLine = strings.TrimRight(newLine, " \t")
			// If a tag-only line becomes empty after date removal, skip it
			if strings.TrimSpace(newLine) == "" && note.IsTagOnlyLine(strings.TrimSpace(l)) {
				continue
			}
			result = append(result, newLine)
		} else {
			result = append(result, l)
		}
	}

	return strings.Join(result, "\n")
}
