package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevin/ruin-note-cli/internal/vault"
)

// setupBenchmarkVault creates a vault with N notes for benchmarking.
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

	for i := 0; i < numNotes; i++ {
		tagSet := tags[i%len(tags)]
		content := fmt.Sprintf(`---
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

// setupLargeNoteVault creates notes with ~5KB of content each
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

	for i := 0; i < numNotes; i++ {
		tagSet := tags[i%len(tags)]
		content := fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-01T10:00:00-05:00
updated: 2025-01-01T10:00:00-05:00
tags:
  - "%s"
  - "%s"
---
# Note %d
%s %s

%s

## Section 1
%s

## Section 2
%s
`, i, tagSet[0], tagSet[1], i, tagSet[0], tagSet[1], largeContent, largeContent, largeContent)

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

	for i := 0; i < numNotes; i++ {
		tagSet := tags[i%len(tags)]
		tagYAML := ""
		tagContent := ""
		for _, t := range tagSet {
			tagYAML += fmt.Sprintf("\n  - \"%s\"", t)
			tagContent += t + " "
		}

		var content string
		pct := i % 100

		switch {
		case pct < 40: // 40% tiny (~100 bytes)
			content = fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-%02dT10:00:00-05:00
updated: 2025-01-%02dT10:00:00-05:00
tags:%s
---
Quick thought %s
`, i, (i%28)+1, (i%28)+1, tagYAML, tagContent)

		case pct < 70: // 30% small (~500 bytes)
			content = fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-%02dT10:00:00-05:00
updated: 2025-01-%02dT10:00:00-05:00
tags:%s
---
# Daily Note %d
%s

%s

## Tasks
- Task 1
- Task 2
- Task 3
`, i, (i%28)+1, (i%28)+1, tagYAML, i, tagContent, paragraph)

		case pct < 90: // 20% medium (~2KB)
			content = fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-%02dT10:00:00-05:00
updated: 2025-01-%02dT10:00:00-05:00
tags:%s
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
`, i, (i%28)+1, (i%28)+1, tagYAML, i, tagContent,
				strings.Repeat(paragraph, 3),
				strings.Repeat(paragraph, 3),
				strings.Repeat(paragraph, 2),
				strings.Repeat(paragraph, 4))

		default: // 10% large (~10KB)
			content = fmt.Sprintf(`---
uuid: uuid-%d
created: 2025-01-%02dT10:00:00-05:00
updated: 2025-01-%02dT10:00:00-05:00
tags:%s
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
`, i, (i%28)+1, (i%28)+1, tagYAML, i, tagContent,
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

func benchmarkTagSearch(b *testing.B, vlt *vault.Vault) {
	matcher, _ := parseQuery("#daily")
	opts := SearchOptions{
		TagOnly:       true,
		NeedsFullNote: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searchNotesWithOptions(vlt, matcher, opts)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
	}
}

func benchmarkTextSearch(b *testing.B, vlt *vault.Vault) {
	matcher, _ := parseQuery("lorem")
	opts := SearchOptions{
		TagOnly:       false,
		NeedsFullNote: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searchNotesWithOptions(vlt, matcher, opts)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
	}
}
