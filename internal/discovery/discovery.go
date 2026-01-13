// Package discovery provides filesystem-based global worktree discovery.
package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/url"
	"github.com/d-kuro/gwq/internal/utils"
	"github.com/d-kuro/gwq/pkg/models"
)

// GlobalWorktreeEntry represents a discovered worktree.
type GlobalWorktreeEntry struct {
	RepositoryURL  string              // Full repository URL
	RepositoryInfo *url.RepositoryInfo // Parsed repository information
	Branch         string
	Path           string
	CommitHash     string
	IsMain         bool
}

// DiscoverGlobalWorktrees finds all worktrees in the configured base directory.
func DiscoverGlobalWorktrees(baseDir string) ([]*GlobalWorktreeEntry, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("base directory not configured")
	}

	// Expand path (handles ~, env vars, and relative paths)
	expandedPath, err := utils.ExpandPath(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand base directory path: %w", err)
	}
	baseDir = expandedPath

	// Check if base directory exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return []*GlobalWorktreeEntry{}, nil
	}

	var entries []*GlobalWorktreeEntry

	err = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors and continue walking
		}

		// Skip if not a directory
		if !info.IsDir() {
			return nil
		}

		// Check if this directory contains a .git file (worktree marker)
		gitFile := filepath.Join(path, ".git")
		if _, err := os.Stat(gitFile); err != nil {
			return nil // Not a git worktree, continue
		}

		// Try to determine if this is a worktree by reading .git file
		gitContent, err := os.ReadFile(gitFile)
		if err != nil {
			return nil
		}

		gitContentStr := strings.TrimSpace(string(gitContent))
		if !strings.HasPrefix(gitContentStr, "gitdir: ") {
			return nil // Not a worktree, it's a main repository
		}

		// This is a worktree, extract information
		entry, err := extractWorktreeInfo(path)
		if err != nil {
			// Log error but continue discovery
			return nil
		}

		entries = append(entries, entry)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return entries, nil
}

// extractWorktreeInfo extracts worktree information from a worktree directory.
func extractWorktreeInfo(worktreePath string) (*GlobalWorktreeEntry, error) {
	// Create a git instance for this worktree
	g := git.New(worktreePath)

	// Get repository URL
	repoURL, err := g.GetRepositoryURL()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository URL: %w", err)
	}

	// Parse repository URL
	repoInfo, err := url.ParseRepositoryURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository URL: %w", err)
	}

	// Get current branch
	branch, err := getCurrentBranch(worktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Get commit hash
	commitHash, err := getCurrentCommitHash(worktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit hash: %w", err)
	}

	// Check if this is the main worktree (unlikely since we filtered for worktrees)
	isMain := false

	return &GlobalWorktreeEntry{
		RepositoryURL:  repoURL,
		RepositoryInfo: repoInfo,
		Branch:         branch,
		Path:           worktreePath,
		CommitHash:     commitHash,
		IsMain:         isMain,
	}, nil
}

// getCurrentBranch gets the current branch name for a worktree.
func getCurrentBranch(worktreePath string) (string, error) {
	g := git.New(worktreePath)

	// Use git rev-parse to get the current branch
	output, err := g.RunCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}

	branch := strings.TrimSpace(output)
	if branch == "HEAD" {
		// Detached HEAD state, try to get a more meaningful name
		return "HEAD", nil
	}

	return branch, nil
}

// getCurrentCommitHash gets the current commit hash for a worktree.
func getCurrentCommitHash(worktreePath string) (string, error) {
	g := git.New(worktreePath)

	output, err := g.RunCommand("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}

// ConvertToWorktreeModels converts GlobalWorktreeEntry to models.Worktree.
func ConvertToWorktreeModels(entries []*GlobalWorktreeEntry, showRepoName bool) []models.Worktree {
	worktrees := make([]models.Worktree, 0, len(entries))

	for _, entry := range entries {
		branch := entry.Branch
		if showRepoName && entry.RepositoryInfo != nil {
			// Use repository name from parsed URL info
			branch = fmt.Sprintf("%s:%s", entry.RepositoryInfo.Repository, entry.Branch)
		}

		wt := models.Worktree{
			Branch:     branch,
			Path:       entry.Path,
			CommitHash: entry.CommitHash,
			IsMain:     entry.IsMain,
		}
		worktrees = append(worktrees, wt)
	}

	return worktrees
}

// FilterGlobalWorktrees filters worktrees by pattern matching.
func FilterGlobalWorktrees(entries []*GlobalWorktreeEntry, pattern string) []*GlobalWorktreeEntry {
	pattern = strings.ToLower(pattern)
	var matches []*GlobalWorktreeEntry

	for _, entry := range entries {
		branchLower := strings.ToLower(entry.Branch)
		var repoName string
		if entry.RepositoryInfo != nil {
			repoName = strings.ToLower(entry.RepositoryInfo.Repository)
		}

		// Match against branch name, path, repo name, or repo:branch pattern
		if strings.Contains(branchLower, pattern) ||
			strings.Contains(strings.ToLower(entry.Path), pattern) ||
			strings.Contains(repoName, pattern) ||
			strings.Contains(repoName+":"+branchLower, pattern) {
			matches = append(matches, entry)
		}
	}

	return matches
}
