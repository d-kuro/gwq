package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/d-kuro/gwq/pkg/models"
)

// GetRepositoryName returns the name of the repository.
func (g *Git) GetRepositoryName() (string, error) {
	rootDir, err := g.getRootDir()
	if err != nil {
		return "", err
	}
	return filepath.Base(rootDir), nil
}

// GetRepositoryPath returns the root path of the git repository.
func (g *Git) GetRepositoryPath() (string, error) {
	return g.getRootDir()
}

// GetMainRepositoryPath returns the root path of the main repository.
// Unlike GetRepositoryPath, this always returns the main repo root even when
// called from inside a worktree.
func (g *Git) GetMainRepositoryPath() (string, error) {
	return g.getMainRepoRoot()
}

// GetRepositoryURL returns the remote origin URL of the repository.
func (g *Git) GetRepositoryURL() (string, error) {
	output, err := g.run("remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("failed to get repository URL: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// GetRecentCommits returns recent commits for a specific path.
func (g *Git) GetRecentCommits(path string, limit int) ([]models.CommitInfo, error) {
	oldWorkDir := g.workDir
	g.workDir = path
	defer func() { g.workDir = oldWorkDir }()

	args := []string{"log", fmt.Sprintf("-%d", limit), "--pretty=format:%H|%s|%an|%ai"}
	output, err := g.run(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent commits: %w", err)
	}

	var commits []models.CommitInfo
	lines := strings.SplitSeq(strings.TrimSpace(output), "\n")

	for line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		date, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[3])

		commits = append(commits, models.CommitInfo{
			Hash:    parts[0],
			Message: parts[1],
			Author:  parts[2],
			Date:    date,
		})
	}

	return commits, nil
}

// getMainRepoRoot returns the main repository root directory using git-common-dir.
// This works correctly from both the main repo and worktrees.
func (g *Git) getMainRepoRoot() (string, error) {
	output, err := g.run("rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("failed to get git common dir: %w", err)
	}
	commonDir := strings.TrimSpace(output)

	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(g.workDir, commonDir)
	}

	commonDir = filepath.Clean(commonDir)

	// A normal worktree's common directory is its `.git` directory, so its
	// repository root is the parent. A bare repository's common directory IS
	// the repository root; taking its parent collapses every bare repository
	// under the same directory and prevents per-repository gwq setup from
	// matching. Ask from inside the common directory: in a linked worktree,
	// --is-bare-repository describes the worktree, not the repository.
	bareOutput, err := New(commonDir).run("rev-parse", "--is-bare-repository")
	if err != nil {
		return "", fmt.Errorf("failed to determine whether repository is bare: %w", err)
	}

	repoRoot := filepath.Dir(commonDir)
	if strings.TrimSpace(bareOutput) == "true" && !isBareLayoutRoot(repoRoot, commonDir) {
		repoRoot = commonDir
	}

	// Resolve symlinks to ensure consistent path comparison
	// (e.g., macOS /var -> /private/var)
	if resolved, err := filepath.EvalSymlinks(repoRoot); err == nil {
		repoRoot = resolved
	}

	return repoRoot, nil
}

// isBareLayoutRoot checks whether dir/.git is a "gitdir:" file pointing at
// bareDir. In this ".bare" layout (<root>/.bare plus <root>/.git containing
// "gitdir: ./.bare"), <root> is the repository root.
func isBareLayoutRoot(dir, bareDir string) bool {
	content, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	gitDir, ok := strings.CutPrefix(strings.TrimSpace(string(content)), "gitdir: ")
	if !ok {
		return false
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}
	pointed, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	bare, err := os.Stat(bareDir)
	if err != nil {
		return false
	}
	return os.SameFile(pointed, bare)
}

// getRootDir returns the repository root directory.
func (g *Git) getRootDir() (string, error) {
	output, err := g.run("rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("failed to get repository root: %w", err)
	}
	return strings.TrimSpace(output), nil
}
