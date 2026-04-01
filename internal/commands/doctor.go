package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"kvnd/ruin-note-cli/internal/note"
	"kvnd/ruin-note-cli/internal/vault"
)

// DoctorOutput represents the JSON output for the doctor command.
type DoctorOutput struct {
	Scanned               int      `json:"scanned"`
	UUIDGenerated         []string `json:"uuid_generated,omitempty"`
	TagsReindexed         []string `json:"tags_reindexed,omitempty"`
	LinkedCardsReindexed  []string `json:"linked_cards_reindexed,omitempty"`
	InheritedTagsUpdated  []string `json:"inherited_tags_updated,omitempty"`
	InheritedTagsStripped []string `json:"inherited_tags_stripped,omitempty"`
	TagsYMLUpdated        bool     `json:"tags_yml_updated"`
	TitlesUpdated         bool     `json:"titles_updated"`
	OrphanedParents       []string `json:"orphaned_parents,omitempty"`
	OrphanedBookmarks     []string `json:"orphaned_bookmarks,omitempty"`
}

// NewDoctorCmd creates the doctor command.
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

			// Ensure vault is initialized
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
			return doctorFullScan(vlt, dryRun, *jsonOutput)
		},
	}

	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would change without writing")

	return cmd
}

// doctorFiles reindexes specific files with incremental index updates.
func doctorFiles(vlt *vault.Vault, paths []string, dryRun bool, jsonOutput bool) error {
	// Resolve paths relative to vault
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

	// Load titles index for linked-cards resolution
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
		// Read raw frontmatter to capture old on-disk tags
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

		// Compare on-disk frontmatter tags against content-derived classification
		tagsChanged := !normalizedTagsEqual(rawFM.Tags, n.Tags) || !normalizedTagsEqual(rawFM.InlineTags, n.InlineTags)
		if tagsChanged {
			output.TagsReindexed = append(output.TagsReindexed, path)
			needsSave = true
		}

		// Resolve date tokens and refresh dates
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

		// Resolve linked-cards
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

		// Compute inherited tags
		var newInherited []string
		if vlt.TagInheritanceEnabled() && n.Parent != "" {
			loader := func(p string) (*note.Note, error) {
				return note.LoadFrontmatterOnly(p)
			}
			newInherited = ComputeInheritedTags(n.UUID, titlesIndex, loader)
		}
		if !normalizedTagsEqual(n.InheritedTags, newInherited) {
			n.InheritedTags = newInherited
			output.InheritedTagsUpdated = append(output.InheritedTagsUpdated, path)
			needsSave = true
		}

		// Strip inherited tags from content
		if len(newInherited) > 0 {
			stripped := note.StripInheritedTagsFromContent(n.Content, newInherited)
			if stripped != n.Content {
				n.Content = stripped
				n.RefreshTags()
				output.InheritedTagsStripped = append(output.InheritedTagsStripped, path)
				needsSave = true
			}
		}

		// Ensure #link tag for URL notes
		if n.EnsureLinkTag() {
			n.RefreshTags()
			needsSave = true
		}

		// Save note if needed
		if needsSave && !dryRun {
			if err := n.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "%swarning: failed to save %s: %v\n", prefix, path, err)
				continue
			}
		}

		// Incremental index updates
		if !dryRun {
			vlt.SaveNote(n, rawFM.Tags, rawFM.InlineTags, "")
			output.TagsYMLUpdated = true
			output.TitlesUpdated = true
		} else {
			output.TagsYMLUpdated = true
			output.TitlesUpdated = true
		}
	}

	// Commit to version history
	if !dryRun {
		repaired := len(output.UUIDGenerated) + len(output.TagsReindexed) + len(output.LinkedCardsReindexed) + len(output.InheritedTagsUpdated) + len(output.InheritedTagsStripped)
		if repaired > 0 {
			vlt.Commit(fmt.Sprintf("ruin doctor: Repair %d notes", repaired))
		}
	}

	return doctorPrintOutput(&output, prefix, jsonOutput)
}

// doctorFullScan performs the original full-vault doctor scan.
func doctorFullScan(vlt *vault.Vault, dryRun bool, jsonOutput bool) error {
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
	globalTagSet := make(map[string]bool)
	inlineTagSet := make(map[string]bool)

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

		// Resolve date tokens and refresh dates
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

		// Count tags for rebuilding tags.yml (global + inline)
		for _, t := range n.AllTags() {
			tagCounts[t]++
		}
		for _, t := range n.Tags {
			globalTagSet[t] = true
		}
		for _, t := range n.InlineTags {
			inlineTagSet[t] = true
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

	// Post-loop: compute and fix inherited tags
	// Build note lookup and parent->children map
	noteByUUID := make(map[string]*note.Note, len(loadedNotes))
	for _, n := range loadedNotes {
		noteByUUID[n.UUID] = n
	}

	loader := func(path string) (*note.Note, error) {
		// Try to find in already-loaded notes first
		for _, n := range loadedNotes {
			if n.FilePath == path {
				return n, nil
			}
		}
		return note.LoadFrontmatterOnly(path)
	}

	// Process topologically: BFS from roots (notes without parents)
	childrenMap := tempIndex.ChildrenMap()
	var roots []string
	for uuid, entry := range titleEntries {
		if entry.Parent == "" {
			roots = append(roots, uuid)
		}
	}

	// BFS order ensures parents are processed before children
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

		// Strip inherited tags from content (tag-only lines)
		var contentChanged bool
		if len(newInherited) > 0 {
			stripped := note.StripInheritedTagsFromContent(n.Content, newInherited)
			if stripped != n.Content {
				// Subtract old tag counts before refreshing
				for _, t := range n.AllTags() {
					tagCounts[t]--
				}

				n.Content = stripped
				n.RefreshTags()
				contentChanged = true

				// Re-collect corrected tags for index
				for _, t := range n.AllTags() {
					tagCounts[t]++
				}
				// Refresh scope sets (idempotent for sets, but needed
				// in case a tag was entirely removed from this note)
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

		// Ensure #link tag for URL notes
		if n.EnsureLinkTag() {
			n.RefreshTags()
			contentChanged = true
		}

		if (inheritedChanged || contentChanged) && !dryRun {
			if err := n.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "%swarning: failed to save inherited tags for %s: %v\n", prefix, n.FilePath, err)
			}
		}
	}

	// Rebuild tags.yml
	if !dryRun {
		if err := vlt.RebuildTagsIndex(tagCounts, globalTagSet, inlineTagSet); err != nil {
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

	// Commit to version history
	if !dryRun {
		repaired := len(output.UUIDGenerated) + len(output.TagsReindexed) + len(output.LinkedCardsReindexed) + len(output.InheritedTagsUpdated) + len(output.InheritedTagsStripped)
		if repaired > 0 {
			vlt.Commit(fmt.Sprintf("ruin doctor: Repair %d notes", repaired))
		}
	}

	return doctorPrintOutput(&output, prefix, jsonOutput)
}

// doctorPrintOutput prints the doctor output in JSON or human-readable format.
func doctorPrintOutput(output *DoctorOutput, prefix string, jsonOutput bool) error {
	if jsonOutput {
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

// stringSlicesEqual compares two string slices for exact equality.
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
		if note.NormalizeTag(a[i]) != note.NormalizeTag(b[i]) {
			return false
		}
	}
	return true
}
