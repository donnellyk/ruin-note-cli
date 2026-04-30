package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
	"github.com/spf13/cobra"
)

type DoctorOutput struct {
	Scanned               int      `json:"scanned"`
	UUIDGenerated         []string `json:"uuid_generated,omitempty"`
	TagsReindexed         []string `json:"tags_reindexed,omitempty"`
	LinkedCardsReindexed  []string `json:"linked_cards_reindexed,omitempty"`
	InheritedTagsUpdated  []string `json:"inherited_tags_updated,omitempty"`
	InheritedTagsStripped []string `json:"inherited_tags_stripped,omitempty"`
	TagFormatMigrated     []string `json:"tag_format_migrated,omitempty"`
	TagsYMLUpdated        bool     `json:"tags_yml_updated"`
	TitlesUpdated         bool     `json:"titles_updated"`
	OrphanedParents       []string `json:"orphaned_parents,omitempty"`
	OrphanedBookmarks     []string `json:"orphaned_bookmarks,omitempty"`
}

func NewDoctorCmd(getVault func() *vault.Vault, jsonOutput *bool) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "doctor [paths...]",
		Short: "Scan and repair vault metadata",
		Long: `Scan all notes in the vault and repair/update metadata as needed.

With no arguments, performs a full vault scan:
  - Generate UUID for notes missing one
  - Reindex tags and inline-tags from document content
  - Resolve [[wiki links]] and rebuild linked-cards
  - Rebuild .ruin/tags.yml from all notes
  - Rebuild .ruin/titles.json from all notes
  - Detect orphaned parent references

With file path arguments, reindexes only the specified files:
  - Same per-file operations (UUID, tags, linked-cards)
  - Incremental index updates (no full rebuild)
  - Useful after manual edits outside of ruin

Does NOT update created or updated timestamps.`,
		Example: `  # Full vault scan
  ruin doctor

  # Reindex specific files after manual edits
  ruin doctor notes/edited-file.md
  ruin doctor file1.md file2.md

  # Preview changes
  ruin doctor --dry-run`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vlt := getVault()
			if vlt == nil {
				return fmt.Errorf("vault not configured")
			}

			if !vlt.IsInitialized() {
				if !dryRun {
					if _, err := vlt.Initialize(false); err != nil {
						return fmt.Errorf("failed to initialize vault: %w", err)
					}
				}
			}

			if len(args) > 0 {
				return doctorFiles(vlt, args, dryRun, *jsonOutput)
			}
			output, err := RunDoctorFullScan(vlt, dryRun)
			if err != nil {
				return err
			}
			prefix := ""
			if dryRun {
				prefix = "[dry-run] "
			}
			return doctorPrintOutput(output, prefix, *jsonOutput)
		},
	}

	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would change without writing")

	return cmd
}

func doctorFiles(vlt *vault.Vault, paths []string, dryRun bool, jsonOutput bool) error {
	resolved := make([]string, 0, len(paths))
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(p) {
			abs = filepath.Join(vlt.Path, p)
		}
		if _, err := os.Stat(abs); err != nil {
			return fmt.Errorf("file not found: %s", p)
		}
		resolved = append(resolved, abs)
	}

	titlesIndex, err := vlt.LoadTitles()
	if err != nil {
		return fmt.Errorf("failed to load titles index: %w", err)
	}

	prefix := ""
	if dryRun {
		prefix = "[dry-run] "
	}

	output := DoctorOutput{
		Scanned: len(resolved),
	}

	for _, path := range resolved {
		rawBytes, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%swarning: failed to read %s: %v\n", prefix, path, err)
			continue
		}
		rawContent := string(rawBytes)
		rawFM, _, _ := note.ParseFrontmatter(rawContent)

		n, err := note.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%swarning: failed to parse %s: %v\n", prefix, path, err)
			continue
		}

		needsSave := false

		if n.UUID == "" {
			n.EnsureUUID()
			output.UUIDGenerated = append(output.UUIDGenerated, path)
			needsSave = true
		}

		if note.HasLegacyTagFrontmatter(rawContent) {
			output.TagFormatMigrated = append(output.TagFormatMigrated, path)
			needsSave = true
		}

		tagsChanged := !normalizedTagsEqual(rawFM.Tags, n.Tags) || !normalizedTagsEqual(rawFM.InlineTags, n.InlineTags)
		if tagsChanged {
			output.TagsReindexed = append(output.TagsReindexed, path)
			needsSave = true
		}

		resolvedContent := note.ResolveDateTokens(n.Content)
		if resolvedContent != n.Content {
			n.Content = resolvedContent
			needsSave = true
		}
		oldDates := n.Dates
		n.RefreshDates()
		if !stringSlicesEqual(oldDates, n.Dates) {
			needsSave = true
		}

		oldLinkedCards := make(map[string]bool)
		for _, lc := range n.LinkedCards {
			oldLinkedCards[lc] = true
		}
		RefreshLinkedCards(n, titlesIndex)
		newLinkedCards := make(map[string]bool)
		for _, lc := range n.LinkedCards {
			newLinkedCards[lc] = true
		}
		linkedCardsChanged := len(oldLinkedCards) != len(newLinkedCards)
		if !linkedCardsChanged {
			for lc := range oldLinkedCards {
				if !newLinkedCards[lc] {
					linkedCardsChanged = true
					break
				}
			}
		}
		if linkedCardsChanged {
			output.LinkedCardsReindexed = append(output.LinkedCardsReindexed, path)
			needsSave = true
		}

		var newInherited []string
		if vlt.TagInheritanceEnabled() && n.Parent != "" {
			// note.Load (full body classification) since v0.4.0 — fast-path
			// LoadFrontmatterOnly returns nil tag fields and would cause
			// inherited-tag computation to silently yield empty.
			loader := func(p string) (*note.Note, error) {
				return note.Load(p)
			}
			newInherited = ComputeInheritedTags(n.UUID, titlesIndex, loader)
		}
		if !normalizedTagsEqual(n.InheritedTags, newInherited) {
			n.InheritedTags = newInherited
			output.InheritedTagsUpdated = append(output.InheritedTagsUpdated, path)
			needsSave = true
		}

		if len(newInherited) > 0 {
			stripped := note.StripInheritedTagsFromContent(n.Content, newInherited)
			if stripped != n.Content {
				n.Content = stripped
				n.RefreshTags()
				output.InheritedTagsStripped = append(output.InheritedTagsStripped, path)
				needsSave = true
			}
		}

		if n.EnsureLinkTag() {
			n.RefreshTags()
			needsSave = true
		}

		if needsSave && !dryRun {
			if err := n.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "%swarning: failed to save %s: %v\n", prefix, path, err)
				continue
			}
		}

		if !dryRun {
			vlt.SaveNote(n, rawFM.Tags, rawFM.InlineTags, "")
			output.TagsYMLUpdated = true
			output.TitlesUpdated = true
		} else {
			output.TagsYMLUpdated = true
			output.TitlesUpdated = true
		}
	}

	if !dryRun {
		if migrated := len(output.TagFormatMigrated); migrated > 0 {
			vlt.Commit(fmt.Sprintf("ruin v0.4.0: tag-format migration (%d notes)", migrated))
		}
		repaired := len(output.UUIDGenerated) + len(output.TagsReindexed) + len(output.LinkedCardsReindexed) + len(output.InheritedTagsUpdated) + len(output.InheritedTagsStripped)
		if repaired > 0 {
			vlt.Commit(fmt.Sprintf("ruin doctor: Repair %d notes", repaired))
		}
	}

	return doctorPrintOutput(&output, prefix, jsonOutput)
}

// RunDoctorFullScan executes a full vault scan and returns the result. The
// caller is responsible for printing the output. Used by both `ruin doctor`
// (no args) and `ruin init` (when an existing notes folder is initialized).
func RunDoctorFullScan(vlt *vault.Vault, dryRun bool) (*DoctorOutput, error) {
	notePaths, err := vlt.ListNotes()
	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}

	output := DoctorOutput{
		Scanned: len(notePaths),
	}

	tagCounts := make(map[string]int)
	globalTagSet := make(map[string]bool)
	inlineTagSet := make(map[string]bool)

	titleEntries := make(map[string]vault.TitleEntry)

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
		rawContent := string(rawBytes)
		rawFM, _, _ := note.ParseFrontmatter(rawContent)

		n, err := note.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%swarning: failed to parse %s: %v\n", prefix, path, err)
			continue
		}

		needsSave := false

		if n.UUID == "" {
			n.EnsureUUID()
			output.UUIDGenerated = append(output.UUIDGenerated, path)
			needsSave = true
		}

		// Pre-v0.4.0 frontmatter: any `#` in tags:/inherited-tags: arrays or
		// any inline-tags: key. Force a rewrite via Serialize so the new
		// (stripped) form replaces the legacy one.
		if note.HasLegacyTagFrontmatter(rawContent) {
			output.TagFormatMigrated = append(output.TagFormatMigrated, path)
			needsSave = true
		}

		// Compare on-disk frontmatter tags against the content-derived
		// classification (already computed by Parse). This detects both
		// tag additions/removals AND misclassification between global/inline.
		if !normalizedTagsEqual(rawFM.Tags, n.Tags) || !normalizedTagsEqual(rawFM.InlineTags, n.InlineTags) {
			output.TagsReindexed = append(output.TagsReindexed, path)
			needsSave = true
		}

		resolvedContent := note.ResolveDateTokens(n.Content)
		if resolvedContent != n.Content {
			n.Content = resolvedContent
			needsSave = true
		}
		oldDates := n.Dates
		n.RefreshDates()
		if !stringSlicesEqual(oldDates, n.Dates) {
			needsSave = true
		}

		for _, t := range n.AllTags() {
			tagCounts[t]++
		}
		for _, t := range n.Tags {
			globalTagSet[t] = true
		}
		for _, t := range n.InlineTags {
			inlineTagSet[t] = true
		}

		titleEntries[n.UUID] = vault.MakeTitleEntry(n.Title, path, n.Parent, n.Tags, n.InlineTags, n.InheritedTags)

		if needsSave && !dryRun {
			if err := n.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "%swarning: failed to save %s: %v\n", prefix, path, err)
			}
		}

		loadedNotes = append(loadedNotes, n)
	}

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

	noteByUUID := make(map[string]*note.Note, len(loadedNotes))
	for _, n := range loadedNotes {
		noteByUUID[n.UUID] = n
	}

	loader := func(path string) (*note.Note, error) {
		for _, n := range loadedNotes {
			if n.FilePath == path {
				return n, nil
			}
		}
		return note.LoadFrontmatterOnly(path)
	}

	// BFS from roots ensures parents are processed before children.
	childrenMap := tempIndex.ChildrenMap()
	var roots []string
	for uuid, entry := range titleEntries {
		if entry.Parent == "" {
			roots = append(roots, uuid)
		}
	}

	queue := make([]string, len(roots))
	copy(queue, roots)
	for i := 0; i < len(queue); i++ {
		queue = append(queue, childrenMap[queue[i]]...)
	}

	for _, uuid := range queue {
		n, ok := noteByUUID[uuid]
		if !ok {
			continue
		}

		var newInherited []string
		entry := titleEntries[uuid]
		if vlt.TagInheritanceEnabled() && entry.Parent != "" {
			newInherited = ComputeInheritedTags(uuid, tempIndex, loader)
		}

		inheritedChanged := !normalizedTagsEqual(n.InheritedTags, newInherited)

		var contentChanged bool
		if len(newInherited) > 0 {
			stripped := note.StripInheritedTagsFromContent(n.Content, newInherited)
			if stripped != n.Content {
				for _, t := range n.AllTags() {
					tagCounts[t]--
				}

				n.Content = stripped
				n.RefreshTags()
				contentChanged = true

				for _, t := range n.AllTags() {
					tagCounts[t]++
				}
				for _, t := range n.Tags {
					globalTagSet[t] = true
				}
				for _, t := range n.InlineTags {
					inlineTagSet[t] = true
				}

				output.InheritedTagsStripped = append(output.InheritedTagsStripped, n.FilePath)
			}
		}

		if inheritedChanged {
			n.InheritedTags = newInherited
			output.InheritedTagsUpdated = append(output.InheritedTagsUpdated, n.FilePath)
		}

		if n.EnsureLinkTag() {
			n.RefreshTags()
			contentChanged = true
		}

		if (inheritedChanged || contentChanged) && !dryRun {
			if err := n.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "%swarning: failed to save inherited tags for %s: %v\n", prefix, n.FilePath, err)
			}
		}

		// Keep titleEntries in sync with any tag changes so the titles.json
		// rebuild below reflects the final post-cascade state.
		if inheritedChanged || contentChanged {
			titleEntries[n.UUID] = vault.MakeTitleEntry(n.Title, n.FilePath, n.Parent, n.Tags, n.InlineTags, n.InheritedTags)
		}
	}

	if !dryRun {
		if err := vlt.RebuildTagsIndex(tagCounts, globalTagSet, inlineTagSet); err != nil {
			fmt.Fprintf(os.Stderr, "%swarning: failed to rebuild tags.yml: %v\n", prefix, err)
		} else {
			output.TagsYMLUpdated = true
		}
	} else {
		output.TagsYMLUpdated = true
	}

	if !dryRun {
		if err := vlt.RebuildTitlesIndex(titleEntries); err != nil {
			fmt.Fprintf(os.Stderr, "%swarning: failed to rebuild titles.json: %v\n", prefix, err)
		} else {
			output.TitlesUpdated = true
		}
	} else {
		output.TitlesUpdated = true
	}

	for uuid, entry := range titleEntries {
		if entry.Parent != "" {
			if _, ok := titleEntries[entry.Parent]; !ok {
				output.OrphanedParents = append(output.OrphanedParents,
					fmt.Sprintf("%s (parent %s not found)", uuid, entry.Parent))
			}
		}
	}

	parentBookmarks, err := vlt.LoadParents()
	if err == nil {
		for _, p := range parentBookmarks.Parents {
			if _, ok := titleEntries[p.UUID]; !ok {
				output.OrphanedBookmarks = append(output.OrphanedBookmarks,
					fmt.Sprintf("%s (uuid %s not found)", p.Name, p.UUID))
			}
		}
	}

	if !dryRun {
		if migrated := len(output.TagFormatMigrated); migrated > 0 {
			vlt.Commit(fmt.Sprintf("ruin v0.4.0: tag-format migration (%d notes)", migrated))
		}
		repaired := len(output.UUIDGenerated) + len(output.TagsReindexed) + len(output.LinkedCardsReindexed) + len(output.InheritedTagsUpdated) + len(output.InheritedTagsStripped)
		if repaired > 0 {
			vlt.Commit(fmt.Sprintf("ruin doctor: Repair %d notes", repaired))
		}
	}

	return &output, nil
}

func doctorPrintOutput(output *DoctorOutput, prefix string, jsonOutput bool) error {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	fmt.Fprintf(os.Stderr, "%sScanned %d notes\n", prefix, output.Scanned)
	if len(output.TagFormatMigrated) > 0 {
		if strings.Contains(prefix, "dry-run") {
			fmt.Fprintf(os.Stderr, "  %d notes: %swill migrate from pre-v0.4.0 tag format\n", len(output.TagFormatMigrated), prefix)
			fmt.Fprintf(os.Stderr, "  Hint: This vault uses the pre-v0.4.0 tag format. Run without --dry-run to migrate.\n")
		} else {
			fmt.Fprintf(os.Stderr, "  %d notes: migrated from pre-v0.4.0 tag format\n", len(output.TagFormatMigrated))
		}
	}
	if len(output.UUIDGenerated) > 0 {
		fmt.Fprintf(os.Stderr, "  %d notes: %sgenerated missing UUID\n", len(output.UUIDGenerated), prefix)
	}
	if len(output.TagsReindexed) > 0 {
		fmt.Fprintf(os.Stderr, "  %d notes: %sreindexed tags\n", len(output.TagsReindexed), prefix)
	}
	if len(output.LinkedCardsReindexed) > 0 {
		fmt.Fprintf(os.Stderr, "  %d notes: %sreindexed linked-cards\n", len(output.LinkedCardsReindexed), prefix)
	}
	if len(output.InheritedTagsUpdated) > 0 {
		fmt.Fprintf(os.Stderr, "  %d notes: %supdated inherited-tags\n", len(output.InheritedTagsUpdated), prefix)
	}
	if len(output.InheritedTagsStripped) > 0 {
		fmt.Fprintf(os.Stderr, "  %d notes: %sstripped redundant inherited tags from content\n", len(output.InheritedTagsStripped), prefix)
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
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// normalizedTagsEqual compares two tag slices for equality after normalizing.
// Order matters (classification may reorder), and comparison is case-insensitive.
func normalizedTagsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if note.NormalizeStored(a[i]) != note.NormalizeStored(b[i]) {
			return false
		}
	}
	return true
}
