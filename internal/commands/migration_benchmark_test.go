package commands

import (
	"testing"

	"github.com/donnellyk/ruin-note-cli/internal/note"
	"github.com/donnellyk/ruin-note-cli/internal/vault"
)

// runDoctorForBench builds the titles index and tags.yml so hot-path
// matchers see populated tag mirrors. Mirrors what `ruin init` would do
// against an existing notes folder.
func runDoctorForBench(b *testing.B, vlt *vault.Vault) {
	b.Helper()
	if _, err := RunDoctorFullScan(vlt, false); err != nil {
		b.Fatalf("doctor full scan: %v", err)
	}
}

// --- Pick tag-only benchmarks ---

func BenchmarkPick_TagOnly_1000(b *testing.B) {
	vlt := setupBenchmarkVault(b, 1000)
	runDoctorForBench(b, vlt)
	benchmarkPickTagOnly(b, vlt)
}

func BenchmarkPick_TagOnly_5000_Realistic(b *testing.B) {
	vlt := setupRealisticVault(b, 5000)
	runDoctorForBench(b, vlt)
	benchmarkPickTagOnly(b, vlt)
}

func BenchmarkPick_TagOnly_10000(b *testing.B) {
	vlt := setupBenchmarkVault(b, 10000)
	runDoctorForBench(b, vlt)
	benchmarkPickTagOnly(b, vlt)
}

// BenchmarkPick_TagOnly_50000 measures pick-style inline-tag filtering at the
// upper end of the plan §11 vault sizes. setupRealisticVault already runs
// doctor at the end, so titles are populated before the bench loop starts.
func BenchmarkPick_TagOnly_50000(b *testing.B) {
	vlt := setupRealisticVault(b, 50000)
	benchmarkPickTagOnly(b, vlt)
}

func benchmarkPickTagOnly(b *testing.B, vlt *vault.Vault) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runPickTagOnly(b, vlt)
	}
}

func runPickTagOnly(b *testing.B, vlt *vault.Vault) {
	b.Helper()
	// Mirrors NewPickCmd's main flow for `ruin pick "#daily"` (no --filter):
	// pre-filter via titles, then full Load + line extraction for each candidate.
	tagFilter := pickTagFilter{include: []string{"daily"}}
	candidates, err := pickCandidatePaths(vlt, tagFilter, nil, nil)
	if err != nil {
		b.Fatalf("pickCandidatePaths: %v", err)
	}
	matched := 0
	for _, path := range candidates {
		n, err := note.Load(path)
		if err != nil {
			continue
		}
		matches := pickLinesFromNote(n, tagFilter, nil, false, doneExclude, false)
		if len(matches) > 0 {
			matched++
		}
	}
	_ = matched
}

// --- Search tags:none benchmarks ---

func BenchmarkSearch_TagsNone_1000(b *testing.B) {
	vlt := setupBenchmarkVault(b, 1000)
	runDoctorForBench(b, vlt)
	benchmarkTagsNone(b, vlt)
}

func BenchmarkSearch_TagsNone_5000_Realistic(b *testing.B) {
	vlt := setupRealisticVault(b, 5000)
	runDoctorForBench(b, vlt)
	benchmarkTagsNone(b, vlt)
}

func BenchmarkSearch_TagsNone_10000(b *testing.B) {
	vlt := setupBenchmarkVault(b, 10000)
	runDoctorForBench(b, vlt)
	benchmarkTagsNone(b, vlt)
}

func BenchmarkSearch_TagsNone_50000(b *testing.B) {
	vlt := setupRealisticVault(b, 50000)
	benchmarkTagsNone(b, vlt)
}

func benchmarkTagsNone(b *testing.B, vlt *vault.Vault) {
	matcher, info, err := parseQuery("tags:none", TagScopeAll)
	if err != nil {
		b.Fatalf("parseQuery: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searchNotesWithOptions(vlt, matcher, info, SearchOptions{})
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
	}
}

// --- Doctor full scan benchmarks ---

func BenchmarkDoctor_FullScan_5000_Realistic(b *testing.B) {
	vlt := setupRealisticVault(b, 5000)
	benchmarkDoctorFullScan(b, vlt)
}

func BenchmarkDoctor_FullScan_10000(b *testing.B) {
	vlt := setupBenchmarkVault(b, 10000)
	benchmarkDoctorFullScan(b, vlt)
}

// BenchmarkDoctor_FullScan_50000 captures the worst-case wall-clock for the
// v0.4.0 tag-format migration on a large vault. Run separately via -run with
// `go test -bench=BenchmarkDoctor_FullScan_50000 -benchtime=1x` so the setup
// (50k file writes) doesn't dominate timing.
func BenchmarkDoctor_FullScan_50000(b *testing.B) {
	vlt := setupRealisticVault(b, 50000)
	benchmarkDoctorFullScan(b, vlt)
}

func benchmarkDoctorFullScan(b *testing.B, vlt *vault.Vault) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := RunDoctorFullScan(vlt, false); err != nil {
			b.Fatalf("doctor full scan: %v", err)
		}
	}
}
