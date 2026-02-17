package versioning

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsAvailable(t *testing.T) {
	// git should be available in test environment
	if !IsAvailable() {
		t.Skip("git not available")
	}
}

func TestIsRepo(t *testing.T) {
	dir := t.TempDir()
	g := New(dir)

	if g.IsRepo() {
		t.Fatal("expected IsRepo=false for fresh directory")
	}

	// Init a git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	if !g.IsRepo() {
		t.Fatal("expected IsRepo=true after git init")
	}
}

func TestInit(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()

	// Create a .ruin dir and a .md file to simulate vault
	os.MkdirAll(filepath.Join(dir, ".ruin"), 0755)
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("# Test"), 0644)

	g := New(dir)

	created, err := g.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if !created {
		t.Fatal("expected created=true for new repo")
	}
	if !g.IsRepo() {
		t.Fatal("expected IsRepo=true after Init")
	}

	// Verify .gitignore exists and contains .ruin/
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), ".ruin/") {
		t.Fatal("expected .gitignore to contain .ruin/")
	}

	// Verify initial commit exists
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if !strings.Contains(string(output), "ruin init") {
		t.Fatalf("expected initial commit message, got: %s", output)
	}

	// Init again should be idempotent
	created, err = g.Init()
	if err != nil {
		t.Fatalf("second Init failed: %v", err)
	}
	if created {
		t.Fatal("expected created=false for existing repo")
	}
}

func TestCommit(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	g := New(dir)

	// Init repo
	_, err := g.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a .md file
	os.WriteFile(filepath.Join(dir, "note.md"), []byte("# My Note\nContent here"), 0644)

	err = g.Commit("ruin log: Create \"My Note\"")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify commit message
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if !strings.Contains(string(output), "ruin log") {
		t.Fatalf("expected commit message to contain 'ruin log', got: %s", output)
	}
}

func TestCommitNoChanges(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	g := New(dir)

	_, err := g.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Commit with no changes should succeed (no-op)
	err = g.Commit("ruin: no changes")
	if err != nil {
		t.Fatalf("Commit with no changes should not error, got: %v", err)
	}
}

func TestCommitOnlyMdFiles(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	g := New(dir)

	_, err := g.Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create both .md and non-.md files
	os.WriteFile(filepath.Join(dir, "note.md"), []byte("# Note"), 0644)
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte("not a note"), 0644)

	err = g.Commit("ruin log: test")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify only .md file is tracked
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files failed: %v", err)
	}
	files := string(output)
	if !strings.Contains(files, "note.md") {
		t.Fatal("expected note.md to be tracked")
	}
	if strings.Contains(files, "data.txt") {
		t.Fatal("expected data.txt to NOT be tracked")
	}
}

func TestEnsureGitignoreIdempotent(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	g := New(dir)

	// Write a .gitignore with existing content
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.tmp\n.ruin/\n"), 0644)

	err := g.ensureGitignore()
	if err != nil {
		t.Fatalf("ensureGitignore failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Count(string(data), ".ruin/") != 1 {
		t.Fatalf("expected exactly one .ruin/ entry, got: %s", data)
	}
}
