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

func New(vaultPath string) *GitVersioning {
	return &GitVersioning{vaultPath: vaultPath}
}

func IsAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func (g *GitVersioning) IsRepo() bool {
	info, err := os.Stat(filepath.Join(g.vaultPath, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Init initializes a git repo in the vault, creates .gitignore, and makes an
// initial commit. Returns true if a new repo was created.
func (g *GitVersioning) Init() (bool, error) {
	if g.IsRepo() {
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

	if err := g.run("add", "-A"); err != nil {
		return false, fmt.Errorf("git add: %w", err)
	}

	if err := g.run("commit", "-m", "ruin init: Initialize vault", "--allow-empty"); err != nil {
		return false, fmt.Errorf("git commit: %w", err)
	}

	return true, nil
}

// Commit stages all .md files and .gitignore, then commits. No-op if nothing
// is staged.
func (g *GitVersioning) Commit(msg string) error {
	_ = g.run("add", "-A", "--", "*.md")
	_ = g.run("add", "--", ".gitignore")

	err := g.run("diff", "--cached", "--quiet")
	if err == nil {
		return nil
	}

	if err := g.run("commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

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
			return nil
		}
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += ".ruin/\n"

	if err := os.WriteFile(gitignorePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}

	return nil
}

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
