package versioning

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitVersioning provides git-based versioning for a vault.
type GitVersioning struct {
	vaultPath string
}

// New creates a new GitVersioning instance for the given vault path.
func New(vaultPath string) *GitVersioning {
	return &GitVersioning{vaultPath: vaultPath}
}

// IsAvailable checks if the git binary is on PATH.
func IsAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// IsRepo checks if the vault path contains a .git directory.
func (g *GitVersioning) IsRepo() bool {
	info, err := os.Stat(filepath.Join(g.vaultPath, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Init initializes a git repo in the vault, creates .gitignore, and makes an initial commit.
// Returns true if a new repo was created, false if one already existed.
func (g *GitVersioning) Init() (bool, error) {
	if g.IsRepo() {
		// Ensure .gitignore has .ruin/ entry even if repo already exists
		if err := g.ensureGitignore(); err != nil {
			return false, err
		}
		return false, nil
	}

	if err := g.run("init"); err != nil {
		return false, fmt.Errorf("git init: %w", err)
	}

	if err := g.ensureGitignore(); err != nil {
		return false, err
	}

	// Stage everything and make initial commit
	if err := g.run("add", "-A"); err != nil {
		return false, fmt.Errorf("git add: %w", err)
	}

	if err := g.run("commit", "-m", "ruin init: Initialize vault", "--allow-empty"); err != nil {
		return false, fmt.Errorf("git commit: %w", err)
	}

	return true, nil
}

// Commit stages all .md files and deletions, then commits with the given message.
// Returns nil on success. Does not error on empty commits (nothing to commit).
func (g *GitVersioning) Commit(msg string) error {
	// Stage all .md files. Use git add with pathspec; ignore errors from no matches.
	// We run two adds: one for .md files and one for .gitignore.
	_ = g.run("add", "-A", "--", "*.md")
	_ = g.run("add", "--", ".gitignore")

	// Check if there's anything to commit
	err := g.run("diff", "--cached", "--quiet")
	if err == nil {
		// Exit code 0 means no staged changes
		return nil
	}

	// There are staged changes, commit them
	if err := g.run("commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

// ensureGitignore creates or updates .gitignore to include .ruin/.
func (g *GitVersioning) ensureGitignore() error {
	gitignorePath := filepath.Join(g.vaultPath, ".gitignore")

	data, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read .gitignore: %w", err)
	}

	content := string(data)
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		if strings.TrimSpace(line) == ".ruin/" {
			return nil // Already has .ruin/ entry
		}
	}

	// Append .ruin/ to .gitignore
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += ".ruin/\n"

	if err := os.WriteFile(gitignorePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}

	return nil
}

// run executes a git command in the vault directory.
func (g *GitVersioning) run(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.vaultPath
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=ruin",
		"GIT_AUTHOR_EMAIL=ruin@localhost",
		"GIT_COMMITTER_NAME=ruin",
		"GIT_COMMITTER_EMAIL=ruin@localhost",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
