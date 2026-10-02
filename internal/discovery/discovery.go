// Package discovery provides filesystem-based global worktree discovery.
package discovery

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

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

	var candidates []worktreeCandidate
	// Bare repositories behind ".bare"-layout pointers; walking them finds nothing.
	bareDirs := map[string]bool{}

	err = filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors and continue walking
		}

		if !d.IsDir() {
			return nil
		}

		// Skip .git directories themselves and known bare repositories
		if d.Name() == ".git" || bareDirs[path] {
			return filepath.SkipDir
		}

		gitPath := filepath.Join(path, ".git")
		gitInfo, err := os.Stat(gitPath)
		if err != nil {
			return nil // No .git entry, continue
		}

		// A .git directory marks a repo boundary; never walk a checkout's contents.
		if gitInfo.IsDir() {
			candidates = append(candidates, worktreeCandidate{path: path, isMain: true})
			return filepath.SkipDir
		}

		// .git is a file: a linked worktree, a submodule, or a pointer to a bare repository
		gitContent, err := os.ReadFile(gitPath)
		if err != nil {
			return filepath.SkipDir
		}

		gitContentStr := strings.TrimSpace(string(gitContent))
		if !strings.HasPrefix(gitContentStr, "gitdir: ") {
			return filepath.SkipDir
		}

		// Skip submodules — their gitdir points to .git/modules/...
		gitDir := strings.TrimPrefix(gitContentStr, "gitdir: ")
		if !isSubmoduleGitDir(gitDir) {
			candidates = append(candidates, worktreeCandidate{path: path})
		}

		// The ".bare" layout points <root>/.git at a bare repository and keeps
		// its linked worktrees below <root>: walk <root>, skip the repository.
		if bareDir, ok := bareGitDir(path, gitDir); ok {
			bareDirs[bareDir] = true
			return nil
		}
		return filepath.SkipDir
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return extractAll(candidates), nil
}

// maxGitWorkers bounds concurrent git subprocesses during discovery.
const maxGitWorkers = 8

type worktreeCandidate struct {
	path   string
	isMain bool
}

// extractAll runs extractWorktreeInfo concurrently, preserving candidate
// order and dropping candidates whose info cannot be read.
func extractAll(candidates []worktreeCandidate) []*GlobalWorktreeEntry {
	results := make([]*GlobalWorktreeEntry, len(candidates))
	sem := make(chan struct{}, min(runtime.NumCPU(), maxGitWorkers))
	var wg sync.WaitGroup

	for i, c := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			entry, err := extractWorktreeInfo(c.path)
			if err != nil {
				return
			}
			entry.IsMain = c.isMain
			results[i] = entry
		}()
	}
	wg.Wait()

	entries := make([]*GlobalWorktreeEntry, 0, len(results))
	for _, e := range results {
		if e != nil {
			entries = append(entries, e)
		}
	}
	return entries
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

	return &GlobalWorktreeEntry{
		RepositoryURL:  repoURL,
		RepositoryInfo: repoInfo,
		Branch:         branch,
		Path:           worktreePath,
		CommitHash:     commitHash,
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

// isSubmoduleGitDir checks whether a gitdir path points to a submodule
// rather than a linked worktree. Submodule gitdirs always contain a
// "/modules/" segment — either under .git/modules/ (submodules in the main
// worktree) or under .git/worktrees/<name>/modules/ (submodules in a linked
// worktree). Linked worktree gitdirs point to .git/worktrees/<name> with no
// trailing /modules/ path.
func isSubmoduleGitDir(gitDir string) bool {
	normalized := filepath.ToSlash(gitDir)
	return strings.Contains(normalized, "/modules/")
}

// bareGitDir resolves gitDir, read from the .git file in dir, and reports
// whether it is a bare repository. Linked worktree gitdirs and stale pointers
// have no object store and are rejected without running git.
func bareGitDir(dir, gitDir string) (string, bool) {
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}
	gitDir = filepath.Clean(gitDir)
	if _, err := os.Stat(filepath.Join(gitDir, "objects")); err != nil {
		return "", false
	}
	output, err := git.New("").RunCommand("--git-dir="+gitDir, "rev-parse", "--is-bare-repository")
	if err != nil || strings.TrimSpace(output) != "true" {
		return "", false
	}
	return gitDir, true
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
