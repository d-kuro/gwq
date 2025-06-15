// Package worktree provides high-level worktree management functionality.
package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/d-kuro/gwq/internal/url"
	"github.com/d-kuro/gwq/pkg/models"
)

// GitInterface defines the git operations used by Manager.
type GitInterface interface {
	ListWorktrees() ([]models.Worktree, error)
	AddWorktree(path, branch string, createBranch bool) error
	AddWorktreeFromBase(path, branch, baseBranch string) error
	RemoveWorktree(path string, force bool) error
	DeleteBranch(branch string, force bool) error
	PruneWorktrees() error
	GetRepositoryName() (string, error)
	GetRecentCommits(path string, limit int) ([]models.CommitInfo, error)
	GetRepositoryURL() (string, error)
}

// Manager handles worktree operations.
type Manager struct {
	git    GitInterface
	config *models.Config
}

// New creates a new worktree Manager.
func New(g GitInterface, config *models.Config) *Manager {
	return &Manager{
		git:    g,
		config: config,
	}
}

// Add creates a new worktree.
func (m *Manager) Add(branch string, customPath string, createBranch bool) error {
	path := customPath
	if path == "" {
		generatedPath, err := m.generateWorktreePath(branch)
		if err != nil {
			return fmt.Errorf("failed to generate worktree path: %w", err)
		}
		path = generatedPath
	}

	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		path = absPath
	}

	if m.config.Worktree.AutoMkdir {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	if err := m.git.AddWorktree(path, branch, createBranch); err != nil {
		return err
	}

	return nil
}

// AddFromBase creates a new worktree with a branch from a specific base branch.
func (m *Manager) AddFromBase(branch string, baseBranch string, customPath string) error {
	path := customPath
	if path == "" {
		generatedPath, err := m.generateWorktreePath(branch)
		if err != nil {
			return fmt.Errorf("failed to generate worktree path: %w", err)
		}
		path = generatedPath
	}

	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		path = absPath
	}

	if m.config.Worktree.AutoMkdir {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	if err := m.git.AddWorktreeFromBase(path, branch, baseBranch); err != nil {
		return err
	}

	return nil
}

// Remove deletes a worktree.
func (m *Manager) Remove(path string, force bool) error {
	return m.git.RemoveWorktree(path, force)
}

// RemoveWithBranch deletes a worktree and optionally its branch.
func (m *Manager) RemoveWithBranch(path string, branch string, forceWorktree bool, deleteBranch bool, forceBranch bool) error {
	// First remove the worktree
	if err := m.git.RemoveWorktree(path, forceWorktree); err != nil {
		return err
	}

	// Then delete the branch if requested
	if deleteBranch && branch != "" {
		if err := m.git.DeleteBranch(branch, forceBranch); err != nil {
			// Return error but worktree is already removed
			return fmt.Errorf("worktree removed but failed to delete branch: %w", err)
		}
	}

	return nil
}

// List returns all worktrees.
func (m *Manager) List() ([]models.Worktree, error) {
	return m.git.ListWorktrees()
}

// Prune removes worktree information for deleted directories.
func (m *Manager) Prune() error {
	return m.git.PruneWorktrees()
}

// GetWorktreePath returns the path for a worktree by pattern matching.
func (m *Manager) GetWorktreePath(pattern string) (string, error) {
	worktrees, err := m.List()
	if err != nil {
		return "", err
	}

	pattern = strings.ToLower(pattern)
	for _, wt := range worktrees {
		if strings.Contains(strings.ToLower(wt.Branch), pattern) ||
			strings.Contains(strings.ToLower(wt.Path), pattern) {
			return wt.Path, nil
		}
	}

	return "", fmt.Errorf("no worktree found matching pattern: %s", pattern)
}

// GetMatchingWorktrees returns all worktrees matching the given pattern.
func (m *Manager) GetMatchingWorktrees(pattern string) ([]models.Worktree, error) {
	worktrees, err := m.List()
	if err != nil {
		return nil, err
	}

	var matches []models.Worktree
	pattern = strings.ToLower(pattern)
	for _, wt := range worktrees {
		if strings.Contains(strings.ToLower(wt.Branch), pattern) ||
			strings.Contains(strings.ToLower(wt.Path), pattern) {
			matches = append(matches, wt)
		}
	}

	return matches, nil
}

// ValidateWorktreePath checks if a path can be used for a new worktree.
func (m *Manager) ValidateWorktreePath(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return fmt.Errorf("failed to read directory: %w", err)
			}
			if len(entries) > 0 {
				return fmt.Errorf("directory is not empty: %s", path)
			}
		} else {
			return fmt.Errorf("path exists and is not a directory: %s", path)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check path: %w", err)
	}

	return nil
}

// generateWorktreePath generates a path for a new worktree using URL-based hierarchy.
func (m *Manager) generateWorktreePath(branch string) (string, error) {
	// Get repository URL
	repoURL, err := m.git.GetRepositoryURL()
	if err != nil {
		return "", fmt.Errorf("failed to get repository URL: %w", err)
	}

	// Parse repository URL to extract hierarchy
	repoInfo, err := url.ParseRepositoryURL(repoURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse repository URL: %w", err)
	}

	// Generate path using URL hierarchy
	path := url.GenerateWorktreePath(m.config.Worktree.BaseDir, repoInfo, branch)

	return path, nil
}
