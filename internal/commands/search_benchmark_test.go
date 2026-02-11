package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kvnd/ruin-note-cli/internal/vault"
)

// setupBenchmarkVault creates a vault with N notes for benchmarking.
// ~30% of notes have a parent reference to simulate real vault structure.
func setupBenchmarkVault(b *testing.B, numNotes int) *vault.Vault {
	b.Helper()

	tmpDir := b.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		b.Fatalf("failed to initialize vault: %v", err)
	}

	// Create notes with various tags
	tags := [][]string{
		{"#daily", "#work"},
		{"#daily", "#personal"},
		{"#project", "#work"},
		{"#idea", "#personal"},
		{"#meeting", "#work"},
	}

	// Create 5 hub notes as parent targets
	numHubs := 5
	for h := 0; h < numHubs; h++ {
		content := fmt.Sprintf(`---
uuid: hub-%d
created: 2025-01-01T10:00:00-05:00
updated: 2025-01-01T10:00:00-05:00
tags:
  - "#project"
---
# Hub %d

Project hub note for benchmarking.
`, h, h)
		path := filepath.Join(tmpDir, fmt.Sprintf("hub-%04d.md", h))
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create hub note: %v", err)
		}
	}

	for i := 0; i < numNotes; i++ {
		tagSet := tags[i%len(tags)]

		// ~30% of notes get a parent
		parentLine := ""
		if i%3 == 0 {
			parentLine = fmt.Sprintf("parent: hub-%d", i%numHubs)
		}

		var content string
		if parentLine != "" {
			content = fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-01T10:00:00-05:00
updated: 2025-01-01T10:00:00-05:00
tags:
  - "%s"
  - "%s"
%s
---
# Note %d
%s %s

This is the content of note %d. It contains some text for searching.
Lorem ipsum dolor sit amet, consectetur adipiscing elit.
`, i, tagSet[0], tagSet[1], parentLine, i, tagSet[0], tagSet[1], i)
		} else {
			content = fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-01T10:00:00-05:00
updated: 2025-01-01T10:00:00-05:00
tags:
  - "%s"
  - "%s"
---
# Note %d
%s %s

This is the content of note %d. It contains some text for searching.
Lorem ipsum dolor sit amet, consectetur adipiscing elit.
`, i, tagSet[0], tagSet[1], i, tagSet[0], tagSet[1], i)
		}

		path := filepath.Join(tmpDir, fmt.Sprintf("note-%04d.md", i))
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create test note: %v", err)
		}
	}

	return vlt
}

func BenchmarkSearch_TagOnly_100(b *testing.B) {
	vlt := setupBenchmarkVault(b, 100)
	benchmarkTagSearch(b, vlt)
}

func BenchmarkSearch_TagOnly_500(b *testing.B) {
	vlt := setupBenchmarkVault(b, 500)
	benchmarkTagSearch(b, vlt)
}

func BenchmarkSearch_TagOnly_1000(b *testing.B) {
	vlt := setupBenchmarkVault(b, 1000)
	benchmarkTagSearch(b, vlt)
}

func BenchmarkSearch_TextSearch_100(b *testing.B) {
	vlt := setupBenchmarkVault(b, 100)
	benchmarkTextSearch(b, vlt)
}

func BenchmarkSearch_TextSearch_500(b *testing.B) {
	vlt := setupBenchmarkVault(b, 500)
	benchmarkTextSearch(b, vlt)
}

func BenchmarkSearch_TextSearch_1000(b *testing.B) {
	vlt := setupBenchmarkVault(b, 1000)
	benchmarkTextSearch(b, vlt)
}

func BenchmarkSearch_TagOnly_10000(b *testing.B) {
	vlt := setupBenchmarkVault(b, 10000)
	benchmarkTagSearch(b, vlt)
}

func BenchmarkSearch_TextSearch_10000(b *testing.B) {
	vlt := setupBenchmarkVault(b, 10000)
	benchmarkTextSearch(b, vlt)
}

// setupLargeNoteVault creates notes with ~5KB of content each.
// ~30% of notes have a parent reference.
func setupLargeNoteVault(b *testing.B, numNotes int) *vault.Vault {
	b.Helper()

	tmpDir := b.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		b.Fatalf("failed to initialize vault: %v", err)
	}

	// Large content block (~4KB)
	largeContent := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 80)

	tags := [][]string{
		{"#daily", "#work"},
		{"#daily", "#personal"},
		{"#project", "#work"},
	}

	// Create 3 hub notes
	for h := 0; h < 3; h++ {
		content := fmt.Sprintf(`---
uuid: hub-%d
created: 2025-01-01T10:00:00-05:00
updated: 2025-01-01T10:00:00-05:00
tags:
  - "#project"
---
# Hub %d

Project hub note.
`, h, h)
		path := filepath.Join(tmpDir, fmt.Sprintf("hub-%04d.md", h))
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create hub note: %v", err)
		}
	}

	for i := 0; i < numNotes; i++ {
		tagSet := tags[i%len(tags)]
		parentLine := ""
		if i%3 == 0 {
			parentLine = fmt.Sprintf("\nparent: hub-%d", i%3)
		}

		content := fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-01T10:00:00-05:00
updated: 2025-01-01T10:00:00-05:00
tags:
  - "%s"
  - "%s"%s
---
# Note %d
%s %s

%s

## Section 1
%s

## Section 2
%s
`, i, tagSet[0], tagSet[1], parentLine, i, tagSet[0], tagSet[1], largeContent, largeContent, largeContent)

		path := filepath.Join(tmpDir, fmt.Sprintf("note-%04d.md", i))
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create test note: %v", err)
		}
	}

	return vlt
}

func BenchmarkSearch_TagOnly_1000_LargeNotes(b *testing.B) {
	vlt := setupLargeNoteVault(b, 1000)
	benchmarkTagSearch(b, vlt)
}

func BenchmarkSearch_TextSearch_1000_LargeNotes(b *testing.B) {
	vlt := setupLargeNoteVault(b, 1000)
	benchmarkTextSearch(b, vlt)
}

// setupRealisticVault creates notes with varied sizes like a real vault:
// - 40% tiny (quick thoughts): ~100 bytes
// - 30% small (daily notes): ~500 bytes
// - 20% medium (meeting notes): ~2KB
// - 10% large (documents): ~10KB
// ~30% of notes have a parent reference to a hub note.
func setupRealisticVault(b *testing.B, numNotes int) *vault.Vault {
	b.Helper()

	tmpDir := b.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		b.Fatalf("failed to initialize vault: %v", err)
	}

	// Content blocks of varying sizes
	paragraph := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. "

	tags := [][]string{
		{"#daily", "#work"},
		{"#daily", "#personal"},
		{"#project", "#work"},
		{"#idea"},
		{"#meeting", "#work", "#notes"},
		{"#draft", "#blog"},
		{"#reference"},
		{"#todo"},
	}

	// Create 5 hub notes
	numHubs := 5
	for h := 0; h < numHubs; h++ {
		content := fmt.Sprintf(`---
uuid: hub-%d
created: 2025-01-01T10:00:00-05:00
updated: 2025-01-01T10:00:00-05:00
tags:
  - "#project"
---
# Hub %d

Project hub note for benchmarking.
`, h, h)
		path := filepath.Join(tmpDir, fmt.Sprintf("hub-%04d.md", h))
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create hub note: %v", err)
		}
	}

	for i := 0; i < numNotes; i++ {
		tagSet := tags[i%len(tags)]
		tagYAML := ""
		tagContent := ""
		for _, t := range tagSet {
			tagYAML += fmt.Sprintf("\n  - \"%s\"", t)
			tagContent += t + " "
		}

		// ~30% get a parent
		parentLine := ""
		if i%3 == 0 {
			parentLine = fmt.Sprintf("\nparent: hub-%d", i%numHubs)
		}

		var content string
		pct := i % 100

		switch {
		case pct < 40: // 40% tiny (~100 bytes)
			content = fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-%02dT10:00:00-05:00
updated: 2025-01-%02dT10:00:00-05:00
tags:%s%s
---
Quick thought %s
`, i, (i%28)+1, (i%28)+1, tagYAML, parentLine, tagContent)

		case pct < 70: // 30% small (~500 bytes)
			content = fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-%02dT10:00:00-05:00
updated: 2025-01-%02dT10:00:00-05:00
tags:%s%s
---
# Daily Note %d
%s

%s

## Tasks
- Task 1
- Task 2
- Task 3
`, i, (i%28)+1, (i%28)+1, tagYAML, parentLine, i, tagContent, paragraph)

		case pct < 90: // 20% medium (~2KB)
			content = fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-%02dT10:00:00-05:00
updated: 2025-01-%02dT10:00:00-05:00
tags:%s%s
---
# Meeting Notes %d
%s

## Attendees
- Alice
- Bob
- Charlie

## Discussion
%s
%s
%s

## Action Items
- [ ] Follow up with team
- [ ] Send summary email
- [ ] Schedule next meeting

## Notes
%s
`, i, (i%28)+1, (i%28)+1, tagYAML, parentLine, i, tagContent,
				strings.Repeat(paragraph, 3),
				strings.Repeat(paragraph, 3),
				strings.Repeat(paragraph, 2),
				strings.Repeat(paragraph, 4))

		default: // 10% large (~10KB)
			content = fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-%02dT10:00:00-05:00
updated: 2025-01-%02dT10:00:00-05:00
tags:%s%s
---
# Document %d
%s

## Introduction
%s

## Background
%s

## Main Content
%s

## Analysis
%s

## Conclusion
%s

## References
%s
`, i, (i%28)+1, (i%28)+1, tagYAML, parentLine, i, tagContent,
				strings.Repeat(paragraph, 8),
				strings.Repeat(paragraph, 10),
				strings.Repeat(paragraph, 20),
				strings.Repeat(paragraph, 15),
				strings.Repeat(paragraph, 10),
				strings.Repeat(paragraph, 5))
		}

		path := filepath.Join(tmpDir, fmt.Sprintf("note-%04d.md", i))
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create test note: %v", err)
		}
	}

	return vlt
}

func BenchmarkSearch_TagOnly_1000_Realistic(b *testing.B) {
	vlt := setupRealisticVault(b, 1000)
	benchmarkTagSearch(b, vlt)
}

func BenchmarkSearch_TextSearch_1000_Realistic(b *testing.B) {
	vlt := setupRealisticVault(b, 1000)
	benchmarkTextSearch(b, vlt)
}

func BenchmarkSearch_TagOnly_5000_Realistic(b *testing.B) {
	vlt := setupRealisticVault(b, 5000)
	benchmarkTagSearch(b, vlt)
}

func BenchmarkSearch_TextSearch_5000_Realistic(b *testing.B) {
	vlt := setupRealisticVault(b, 5000)
	benchmarkTextSearch(b, vlt)
}

// --- Parent search benchmarks ---

func BenchmarkSearch_Parent_1000(b *testing.B) {
	vlt := setupBenchmarkVault(b, 1000)
	benchmarkParentSearch(b, vlt, "hub-0")
}

func BenchmarkSearch_Parent_5000_Realistic(b *testing.B) {
	vlt := setupRealisticVault(b, 5000)
	benchmarkParentSearch(b, vlt, "hub-0")
}

func BenchmarkSearch_ParentNone_1000(b *testing.B) {
	vlt := setupBenchmarkVault(b, 1000)
	benchmarkParentSearch(b, vlt, "none")
}

// --- Compose tree benchmarks ---

// setupComposeVault creates a vault with a titles index suitable for compose benchmarks.
// Structure: numHubs hub notes, each with numChildrenPerHub direct children.
func setupComposeVault(b *testing.B, numHubs, numChildrenPerHub int) (*vault.Vault, string) {
	b.Helper()

	tmpDir := b.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		b.Fatalf("failed to initialize vault: %v", err)
	}

	index := &vault.TitlesIndex{Titles: make(map[string]vault.TitleEntry)}

	// Create hub notes
	for h := 0; h < numHubs; h++ {
		uuid := fmt.Sprintf("hub-%d", h)
		title := fmt.Sprintf("Hub %d", h)
		filename := fmt.Sprintf("hub-%04d.md", h)
		path := filepath.Join(tmpDir, filename)
		content := fmt.Sprintf(`---
uuid: %s
created: 2025-01-01T10:00:00-05:00
updated: 2025-01-01T10:00:00-05:00
tags:
  - "#project"
---
# %s

Hub note content.
`, uuid, title)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create hub note: %v", err)
		}
		index.Titles[uuid] = vault.TitleEntry{Title: title, Path: path}
	}

	// Create children for each hub
	noteNum := 0
	for h := 0; h < numHubs; h++ {
		hubUUID := fmt.Sprintf("hub-%d", h)
		for c := 0; c < numChildrenPerHub; c++ {
			uuid := fmt.Sprintf("child-%d", noteNum)
			title := fmt.Sprintf("Child %d of Hub %d", c, h)
			filename := fmt.Sprintf("child-%05d.md", noteNum)
			path := filepath.Join(tmpDir, filename)
			content := fmt.Sprintf(`---
uuid: %s
created: 2025-01-01T10:00:00-05:00
updated: 2025-01-01T10:00:00-05:00
tags:
  - "#work"
parent: %s
---
# %s

Child note content with some text for realistic loading.
Lorem ipsum dolor sit amet, consectetur adipiscing elit.
`, uuid, hubUUID, title)
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				b.Fatalf("failed to create child note: %v", err)
			}
			index.Titles[uuid] = vault.TitleEntry{Title: title, Path: path, Parent: hubUUID}
			noteNum++
		}
	}

	// Write titles index
	if err := vlt.SaveTitles(index); err != nil {
		b.Fatalf("failed to save titles index: %v", err)
	}

	return vlt, "hub-0"
}

// setupDeepComposeVault creates a vault with a deep tree (chain of depth levels).
// One root -> fan children at each level.
func setupDeepComposeVault(b *testing.B, depth, fanOut int) (*vault.Vault, string) {
	b.Helper()

	tmpDir := b.TempDir()
	vlt := vault.New(tmpDir)
	if _, err := vlt.Initialize(false); err != nil {
		b.Fatalf("failed to initialize vault: %v", err)
	}

	index := &vault.TitlesIndex{Titles: make(map[string]vault.TitleEntry)}
	noteNum := 0

	createNote := func(uuid, title, parent string) {
		filename := fmt.Sprintf("note-%05d.md", noteNum)
		path := filepath.Join(tmpDir, filename)
		parentLine := ""
		if parent != "" {
			parentLine = fmt.Sprintf("\nparent: %s", parent)
		}
		content := fmt.Sprintf(`---
uuid: %s
created: 2025-01-01T10:00:00-05:00
updated: 2025-01-01T10:00:00-05:00
tags:
  - "#work"%s
---
# %s

Note content for tree traversal benchmarking.
`, uuid, parentLine, title)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create note: %v", err)
		}
		index.Titles[uuid] = vault.TitleEntry{Title: title, Path: path, Parent: parent}
		noteNum++
	}

	// Build tree breadth-first
	rootUUID := "root-0"
	createNote(rootUUID, "Root", "")

	currentLevel := []string{rootUUID}
	for d := 1; d <= depth; d++ {
		var nextLevel []string
		for _, parentUUID := range currentLevel {
			for f := 0; f < fanOut; f++ {
				uuid := fmt.Sprintf("n-%d-%d-%d", d, noteNum, f)
				title := fmt.Sprintf("Level %d Child %d", d, f)
				createNote(uuid, title, parentUUID)
				nextLevel = append(nextLevel, uuid)
			}
		}
		currentLevel = nextLevel
	}

	if err := vlt.SaveTitles(index); err != nil {
		b.Fatalf("failed to save titles index: %v", err)
	}

	return vlt, rootUUID
}

func BenchmarkCompose_Flat_20Children(b *testing.B) {
	vlt, rootUUID := setupComposeVault(b, 1, 20)
	benchmarkCompose(b, vlt, rootUUID, 0)
}

func BenchmarkCompose_Flat_100Children(b *testing.B) {
	vlt, rootUUID := setupComposeVault(b, 1, 100)
	benchmarkCompose(b, vlt, rootUUID, 0)
}

func BenchmarkCompose_Deep_5x3(b *testing.B) {
	// depth=5, fanOut=3 -> 1 + 3 + 9 + 27 + 81 + 243 = 364 notes
	vlt, rootUUID := setupDeepComposeVault(b, 5, 3)
	benchmarkCompose(b, vlt, rootUUID, 0)
}

func BenchmarkCompose_Deep_3x10(b *testing.B) {
	// depth=3, fanOut=10 -> 1 + 10 + 100 + 1000 = 1111 notes
	vlt, rootUUID := setupDeepComposeVault(b, 3, 10)
	benchmarkCompose(b, vlt, rootUUID, 0)
}

func BenchmarkCompose_Deep_3x10_Depth2(b *testing.B) {
	// depth=3, fanOut=10, but limited to maxDepth=2 -> 1 + 10 + 100 = 111 notes
	vlt, rootUUID := setupDeepComposeVault(b, 3, 10)
	benchmarkCompose(b, vlt, rootUUID, 2)
}

func BenchmarkCollectTreeNotes_Flat_100Children(b *testing.B) {
	vlt, rootUUID := setupComposeVault(b, 1, 100)
	benchmarkCollectTree(b, vlt, rootUUID, 0)
}

func BenchmarkCollectTreeNotes_Deep_5x3(b *testing.B) {
	vlt, rootUUID := setupDeepComposeVault(b, 5, 3)
	benchmarkCollectTree(b, vlt, rootUUID, 0)
}

func BenchmarkCollectTreeNotes_Deep_3x10(b *testing.B) {
	vlt, rootUUID := setupDeepComposeVault(b, 3, 10)
	benchmarkCollectTree(b, vlt, rootUUID, 0)
}

// --- Helper functions ---

func benchmarkTagSearch(b *testing.B, vlt *vault.Vault) {
	matcher, _ := parseQuery("#daily", TagScopeAll)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searchNotesWithOptions(vlt, matcher, SearchOptions{})
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
	}
}

func benchmarkTextSearch(b *testing.B, vlt *vault.Vault) {
	matcher, _ := parseQuery("lorem", TagScopeAll)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searchNotesWithOptions(vlt, matcher, SearchOptions{})
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
	}
}

func benchmarkParentSearch(b *testing.B, vlt *vault.Vault, parentValue string) {
	matcher, _ := parseQuery("parent:"+parentValue, TagScopeAll)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searchNotesWithOptions(vlt, matcher, SearchOptions{})
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
	}
}

func benchmarkCompose(b *testing.B, vlt *vault.Vault, rootUUID string, maxDepth int) {
	index, err := vlt.LoadTitles()
	if err != nil {
		b.Fatalf("failed to load titles: %v", err)
	}

	// Build parent->children map
	childrenMap := make(map[string][]string)
	for uuid, entry := range index.Titles {
		if entry.Parent != "" {
			childrenMap[entry.Parent] = append(childrenMap[entry.Parent], uuid)
		}
	}
	for parent := range childrenMap {
		sortChildUUIDs(vlt, index, childrenMap[parent], "title")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		composeText(vlt, index, childrenMap, rootUUID, make(map[string]bool), &sb, maxDepth, 0, false, false)
	}
}

func benchmarkCollectTree(b *testing.B, vlt *vault.Vault, rootUUID string, maxDepth int) {
	index, err := vlt.LoadTitles()
	if err != nil {
		b.Fatalf("failed to load titles: %v", err)
	}

	childrenMap := make(map[string][]string)
	for uuid, entry := range index.Titles {
		if entry.Parent != "" {
			childrenMap[entry.Parent] = append(childrenMap[entry.Parent], uuid)
		}
	}
	for parent := range childrenMap {
		sortChildUUIDs(vlt, index, childrenMap[parent], "title")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collectTreeNotes(vlt, index, childrenMap, rootUUID, make(map[string]bool), maxDepth, 0)
	}
}
