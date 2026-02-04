// Package discovery provides filesystem-based global worktree discovery.
package discovery

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/url"
	"github.com/d-kuro/gwq/internal/utils"
	"github.com/d-kuro/gwq/pkg/models"
)

// GlobalWorktreeEntry represents a discovered worktree.
// Supports lazy loading: when created with lazy=true, only Path and IsMain are set.
// Call EnsureLoaded() to populate other fields on demand.
// Thread-safe: loaded field uses atomic.Bool for concurrent access.
type GlobalWorktreeEntry struct {
	RepositoryURL  string              // Full repository URL
	RepositoryInfo *url.RepositoryInfo // Parsed repository information
	Branch         string
	Path           string
	CommitHash     string
	IsMain         bool
	DisplayPath    string // Optional: ghq root relative path for display (matches ghq list format)

	// Lazy loading support (thread-safe)
	loaded   atomic.Bool
	loadOnce sync.Once
	loadErr  error
}

// EnsureLoaded loads worktree details if not already loaded.
// Safe to call multiple times; actual loading happens only once.
func (e *GlobalWorktreeEntry) EnsureLoaded() error {
	e.loadOnce.Do(func() {
		e.loadErr = e.loadDetails()
	})
	return e.loadErr
}

// IsLoaded returns true if details have been loaded.
// Thread-safe.
func (e *GlobalWorktreeEntry) IsLoaded() bool {
	return e.loaded.Load()
}

// loadDetails loads the worktree details from git files or commands.
func (e *GlobalWorktreeEntry) loadDetails() error {
	if e.Path == "" {
		return fmt.Errorf("path is required for loading")
	}

	// Try fast loading first
	if e.IsMain {
		return e.loadMainRepoDetails()
	}
	return e.loadWorktreeDetails()
}

// loadWorktreeDetails loads details for a worktree (non-main).
func (e *GlobalWorktreeEntry) loadWorktreeDetails() error {
	repoURL, repoInfo, branch, commitHash, err := readWorktreeDetailsFast(e.Path)
	if err != nil {
		return e.loadDetailsFromGit()
	}

	e.RepositoryURL = repoURL
	e.RepositoryInfo = repoInfo
	e.Branch = branch
	e.CommitHash = commitHash
	e.loaded.Store(true)
	return nil
}

// loadMainRepoDetails loads details for a main repository.
func (e *GlobalWorktreeEntry) loadMainRepoDetails() error {
	entry, err := extractMainRepoInfo(e.Path)
	if err != nil {
		return err
	}

	e.RepositoryURL = entry.RepositoryURL
	e.RepositoryInfo = entry.RepositoryInfo
	e.Branch = entry.Branch
	e.CommitHash = entry.CommitHash
	e.loaded.Store(true)
	return nil
}

// loadDetailsFromGit loads details using git commands (fallback).
func (e *GlobalWorktreeEntry) loadDetailsFromGit() error {
	entry, err := extractWorktreeInfo(e.Path)
	if err != nil {
		return err
	}

	e.RepositoryURL = entry.RepositoryURL
	e.RepositoryInfo = entry.RepositoryInfo
	e.Branch = entry.Branch
	e.CommitHash = entry.CommitHash
	e.loaded.Store(true)
	return nil
}

// expandBaseDir validates and expands a base directory path.
// Returns the expanded path, or an error if invalid.
// Returns ("", nil) if the directory does not exist (not an error, just empty result).
// Returns an error if the path exists but is not a directory.
func expandBaseDir(baseDir string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("base directory not configured")
	}

	expandedPath, err := utils.ExpandPath(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand base directory path: %w", err)
	}

	info, err := os.Stat(expandedPath)
	if os.IsNotExist(err) {
		return "", nil // Directory doesn't exist - not an error, just no results
	}
	if err != nil {
		return "", fmt.Errorf("failed to stat base directory: %w", err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		return "", fmt.Errorf("base directory path is not a directory: %s", expandedPath)
	}

	return expandedPath, nil
}

// isWorktreeDir checks if a path is a git worktree directory.
// Returns true if it's a worktree, false otherwise.
// Also returns whether to skip descending into this directory.
func isWorktreeDir(path string, d fs.DirEntry) (isWorktree, skipDir bool) {
	if !d.IsDir() {
		return false, false
	}

	if d.Name() == ".git" {
		return false, true
	}

	gitFile := filepath.Join(path, ".git")
	gitInfo, err := os.Lstat(gitFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[gwq] warning: failed to stat %s: %v\n", gitFile, err)
		}
		return false, false
	}

	// .git is a directory = main repository, skip descending
	if gitInfo.IsDir() {
		return false, true
	}

	// Handle symlinks: follow the symlink to check if it points to a directory
	if gitInfo.Mode()&os.ModeSymlink != 0 {
		// Follow the symlink
		targetInfo, err := os.Stat(gitFile)
		if err != nil {
			// Can't follow symlink, skip to be safe
			return false, true
		}
		// If symlink points to a directory, treat like main repo
		if targetInfo.IsDir() {
			return false, true
		}
		// Symlink points to a file, read its content
		gitContent, err := os.ReadFile(gitFile)
		if err != nil {
			return false, false
		}
		if strings.HasPrefix(strings.TrimSpace(string(gitContent)), "gitdir:") {
			return true, true
		}
		return false, false
	}

	// Not a regular file and not a symlink
	if !gitInfo.Mode().IsRegular() {
		return false, false
	}

	// Read .git file to verify it's a worktree
	gitContent, err := os.ReadFile(gitFile)
	if err != nil {
		return false, false
	}

	if !strings.HasPrefix(strings.TrimSpace(string(gitContent)), "gitdir:") {
		return false, false
	}

	return true, true // Is a worktree, skip descending into it
}

// DiscoverGlobalWorktrees finds all worktrees in the configured base directory.
// This function uses filepath.WalkDir for better performance (avoids redundant os.Stat calls).
func DiscoverGlobalWorktrees(baseDir string) ([]*GlobalWorktreeEntry, error) {
	expandedDir, err := expandBaseDir(baseDir)
	if err != nil {
		return nil, err
	}
	if expandedDir == "" {
		return []*GlobalWorktreeEntry{}, nil
	}

	var entries []*GlobalWorktreeEntry

	err = filepath.WalkDir(expandedDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "[gwq] warning: %v\n", err)
			}
			return nil
		}

		isWorktree, skipDir := isWorktreeDir(path, d)
		if !isWorktree {
			if skipDir {
				return filepath.SkipDir
			}
			return nil
		}

		entry, err := extractWorktreeInfoWithFallback(path)
		if err != nil {
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "[gwq] warning: failed to extract worktree info from %s: %v\n", path, err)
			}
			return filepath.SkipDir
		}

		// Set DisplayPath as basedir-relative path
		if rel, err := filepath.Rel(expandedDir, path); err == nil {
			entry.DisplayPath = rel
		}

		entries = append(entries, entry)
		return filepath.SkipDir
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return entries, nil
}

// extractWorktreeInfo extracts worktree information from a worktree directory.
func extractWorktreeInfo(worktreePath string) (*GlobalWorktreeEntry, error) {
	g := git.New(worktreePath)

	var repoURL string
	var repoInfo *url.RepositoryInfo
	if rawURL, err := g.GetRepositoryURL(); err == nil {
		repoURL = rawURL
		repoInfo, _ = url.ParseRepositoryURL(repoURL)
	}

	branch, err := getCurrentBranch(worktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	commitHash, err := getCurrentCommitHash(worktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit hash: %w", err)
	}

	entry := &GlobalWorktreeEntry{
		RepositoryURL:  repoURL,
		RepositoryInfo: repoInfo,
		Branch:         branch,
		Path:           worktreePath,
		CommitHash:     commitHash,
		IsMain:         false,
	}
	entry.loaded.Store(true)
	return entry, nil
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

// formatBranchDisplay formats the branch name for display.
// When showRepoName is true, formats as repo:branch (or DisplayPath:branch if set).
// If DisplayPath is set (ghq mode), uses that; otherwise uses Repository name only.
// Default format: "myapp:feature" or "myapp" for main worktree.
func formatBranchDisplay(entry *GlobalWorktreeEntry, showRepoName bool) string {
	if !showRepoName {
		return entry.Branch
	}

	// Use DisplayPath if available (already includes :dirname for worktrees)
	if entry.DisplayPath != "" {
		return entry.DisplayPath
	}

	// Default: use Repository name only (not FullPath)
	if entry.RepositoryInfo != nil {
		if entry.IsMain {
			return entry.RepositoryInfo.Repository
		}
		return entry.RepositoryInfo.Repository + ":" + entry.Branch
	}

	return entry.Branch
}

// ConvertToWorktreeModels converts GlobalWorktreeEntry to models.Worktree.
func ConvertToWorktreeModels(entries []*GlobalWorktreeEntry, showRepoName bool) []models.Worktree {
	worktrees := make([]models.Worktree, 0, len(entries))

	for _, entry := range entries {
		var repoInfo *models.RepositoryInfo
		if entry.RepositoryInfo != nil {
			repoInfo = &models.RepositoryInfo{
				Host:       entry.RepositoryInfo.Host,
				Owner:      entry.RepositoryInfo.Owner,
				Repository: entry.RepositoryInfo.Repository,
			}
		}

		worktrees = append(worktrees, models.Worktree{
			Branch:         formatBranchDisplay(entry, showRepoName),
			Path:           entry.Path,
			CommitHash:     entry.CommitHash,
			IsMain:         entry.IsMain,
			RepositoryInfo: repoInfo,
		})
	}

	return worktrees
}

// FilterGlobalWorktrees filters worktrees by pattern matching.
func FilterGlobalWorktrees(entries []*GlobalWorktreeEntry, pattern string) []*GlobalWorktreeEntry {
	pattern = strings.ToLower(pattern)
	var matches []*GlobalWorktreeEntry

	for _, entry := range entries {
		if matchesGlobalWorktree(entry, pattern) {
			matches = append(matches, entry)
		}
	}

	return matches
}

// matchesGlobalWorktree checks if a worktree entry matches the given pattern.
func matchesGlobalWorktree(entry *GlobalWorktreeEntry, pattern string) bool {
	branchLower := strings.ToLower(entry.Branch)
	pathLower := strings.ToLower(entry.Path)

	if strings.Contains(branchLower, pattern) || strings.Contains(pathLower, pattern) {
		return true
	}

	if entry.RepositoryInfo == nil {
		return false
	}

	repoName := strings.ToLower(entry.RepositoryInfo.Repository)
	ownerRepo := strings.ToLower(entry.RepositoryInfo.Owner) + "/" + repoName
	ownerRepoBranch := ownerRepo + ":" + branchLower

	return strings.Contains(repoName, pattern) ||
		strings.Contains(ownerRepo, pattern) ||
		strings.Contains(ownerRepoBranch, pattern)
}

// addUniqueEntries adds entries to the slice, skipping duplicates based on path.
func addUniqueEntries(allEntries *[]*GlobalWorktreeEntry, seen map[string]bool, entries []*GlobalWorktreeEntry) {
	for _, entry := range entries {
		if !seen[entry.Path] {
			*allEntries = append(*allEntries, entry)
			seen[entry.Path] = true
		}
	}
}

// DiscoverAllWorktrees discovers all worktrees from multiple sources:
// - ghq repositories and their .worktrees directories (if ghq mode enabled)
// - traditional basedir worktrees
// It returns both main repositories and their worktrees.
func DiscoverAllWorktrees(cfg *models.Config) ([]*GlobalWorktreeEntry, error) {
	var allEntries []*GlobalWorktreeEntry
	seen := make(map[string]bool)

	if cfg.Ghq.Enabled {
		ghqEntries, err := discoverGhqWorktrees(cfg.Ghq.WorktreesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gwq] warning: failed to discover ghq worktrees: %v\n", err)
		} else {
			addUniqueEntries(&allEntries, seen, ghqEntries)
		}
	}

	if cfg.Worktree.BaseDir != "" {
		basedirEntries, err := DiscoverGlobalWorktrees(cfg.Worktree.BaseDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gwq] warning: failed to discover basedir worktrees: %v\n", err)
		} else {
			addUniqueEntries(&allEntries, seen, basedirEntries)
		}
	}

	return allEntries, nil
}
