// Package discovery provides filesystem-based global worktree discovery.
package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/d-kuro/gwq/internal/ghq"
	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/url"
	"github.com/d-kuro/gwq/internal/worktree"
)

// discoverGhqWorktrees discovers worktrees from ghq-managed repositories.
// It includes both main repositories and worktrees under .worktrees directories.
func discoverGhqWorktrees(worktreesDir string) ([]*GlobalWorktreeEntry, error) {
	client := ghq.NewClient()

	if !client.IsInstalled() {
		return nil, fmt.Errorf("ghq is not installed")
	}

	repos, err := client.ListRepositories()
	if err != nil {
		return nil, err
	}

	// Get ghq roots to calculate DisplayPath (relative path matching ghq list format)
	ghqRoots, err := client.GetRoots()
	if err != nil {
		ghqRoots = nil // Continue without DisplayPath if roots unavailable
	}

	// Normalize worktreesDir once before the loop
	effectiveWorktreesDir := worktreesDir
	if effectiveWorktreesDir == "" {
		effectiveWorktreesDir = ".worktrees"
	}
	validWorktreesDir := worktree.ValidateWorktreesDir(effectiveWorktreesDir) == nil

	var entries []*GlobalWorktreeEntry

	for _, repoPath := range repos {
		mainEntry, err := extractMainRepoInfo(repoPath)
		if err != nil {
			// Error policy: log non-NotExist errors to stderr (consistent with parallel discovery)
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "[gwq] warning: failed to extract main repo info from %s: %v\n", repoPath, err)
			}
			continue
		}
		// Set DisplayPath for ghq list format compatibility
		mainEntry.DisplayPath = calculateGhqDisplayPath(mainEntry.Path, ghqRoots)
		entries = append(entries, mainEntry)

		if !validWorktreesDir {
			continue
		}

		worktreesDirPath := filepath.Join(repoPath, effectiveWorktreesDir)
		worktreeEntries, err := discoverWorktreesInDir(worktreesDirPath)
		if err != nil {
			// Error policy: log non-NotExist errors to stderr (consistent with parallel discovery)
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "[gwq] warning: failed to discover worktrees in %s: %v\n", worktreesDirPath, err)
			}
			continue
		}
		// Set DisplayPath for worktrees (main repo's DisplayPath + :dirname)
		for _, wtEntry := range worktreeEntries {
			wtEntry.DisplayPath = buildWorktreeDisplayPath(mainEntry, wtEntry.Path)
		}
		entries = append(entries, worktreeEntries...)
	}

	return entries, nil
}

// calculateGhqDisplayPath calculates the ghq list format path (relative to ghq root).
func calculateGhqDisplayPath(repoPath string, ghqRoots []string) string {
	if len(ghqRoots) == 0 {
		return ""
	}

	repoPath = filepath.Clean(repoPath)
	for _, root := range ghqRoots {
		root = filepath.Clean(root)
		rel, err := filepath.Rel(root, repoPath)
		if err != nil {
			continue
		}
		// If rel doesn't start with "..", it's under this root
		if !strings.HasPrefix(rel, "..") && rel != "." {
			return rel
		}
	}
	return ""
}

// extractMainRepoInfo extracts repository information from a main repository path.
func extractMainRepoInfo(repoPath string) (*GlobalWorktreeEntry, error) {
	gitDir := filepath.Join(repoPath, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		// Wrap the original error to preserve os.IsNotExist check capability
		return nil, fmt.Errorf("not a git repository %s: %w", repoPath, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("not a main repository (has .git file, not directory): %s", repoPath)
	}

	g := git.New(repoPath)

	var repoURL string
	var repoInfo *url.RepositoryInfo
	if rawURL, err := g.GetRepositoryURL(); err == nil {
		repoURL = rawURL
		repoInfo, _ = url.ParseRepositoryURL(repoURL)
	}

	branch, _ := getCurrentBranch(repoPath)
	if branch == "" {
		branch = "unknown"
	}

	commitHash, _ := getCurrentCommitHash(repoPath)

	entry := &GlobalWorktreeEntry{
		RepositoryURL:  repoURL,
		RepositoryInfo: repoInfo,
		Branch:         branch,
		Path:           repoPath,
		CommitHash:     commitHash,
		IsMain:         true,
	}
	entry.loaded.Store(true)
	return entry, nil
}

// isWorktreeDirectory checks if a path contains a .git file with gitdir: prefix.
func isWorktreeDirectory(path string) bool {
	gitFile := filepath.Join(path, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(string(content)), "gitdir:")
}

// discoverWorktreesInDir discovers all worktrees directly under a directory.
// This is used for .worktrees directories where each subdirectory is a worktree.
func discoverWorktreesInDir(dir string) ([]*GlobalWorktreeEntry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var worktrees []*GlobalWorktreeEntry

	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}

		worktreePath := filepath.Join(dir, dirEntry.Name())
		if !isWorktreeDirectory(worktreePath) {
			continue
		}

		wtEntry, err := extractWorktreeInfoWithFallback(worktreePath)
		if err != nil {
			// Error policy: log non-NotExist errors to stderr (consistent with parallel discovery)
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "[gwq] warning: failed to extract worktree info from %s: %v\n", worktreePath, err)
			}
			continue
		}
		worktrees = append(worktrees, wtEntry)
	}

	return worktrees, nil
}

// discoverGhqWorktreesParallel discovers worktrees from ghq-managed repositories using parallel processing.
func discoverGhqWorktreesParallel(worktreesDir string, maxWorkers int) ([]*GlobalWorktreeEntry, error) {
	client := ghq.NewClient()

	if !client.IsInstalled() {
		return nil, fmt.Errorf("ghq is not installed")
	}

	repos, err := client.ListRepositories()
	if err != nil {
		return nil, err
	}

	if len(repos) == 0 {
		return []*GlobalWorktreeEntry{}, nil
	}

	// Get ghq roots to calculate DisplayPath (relative path matching ghq list format)
	ghqRoots, err := client.GetRoots()
	if err != nil {
		ghqRoots = nil // Continue without DisplayPath if roots unavailable
	}

	if maxWorkers <= 0 {
		maxWorkers = min(runtime.NumCPU(), 4)
	}

	jobs := make(chan int, len(repos))
	results := make([][]*GlobalWorktreeEntry, len(repos))
	var wg sync.WaitGroup

	// Validate worktreesDir once
	if worktreesDir == "" {
		worktreesDir = ".worktrees"
	}
	validWorktreesDir := worktree.ValidateWorktreesDir(worktreesDir) == nil

	// Start fixed number of workers
	for range maxWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				results[idx] = extractRepoAndWorktrees(repos[idx], worktreesDir, validWorktreesDir, ghqRoots)
			}
		}()
	}

	// Submit jobs
	for i := range repos {
		jobs <- i
	}
	close(jobs)

	wg.Wait()

	// Flatten results
	var allEntries []*GlobalWorktreeEntry
	for _, entries := range results {
		allEntries = append(allEntries, entries...)
	}
	return allEntries, nil
}

// extractRepoAndWorktrees extracts entries for a main repo and its worktrees.
func extractRepoAndWorktrees(repoPath, worktreesDir string, validWorktreesDir bool, ghqRoots []string) []*GlobalWorktreeEntry {
	var entries []*GlobalWorktreeEntry

	// Main repo
	mainEntry, err := extractMainRepoInfo(repoPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[gwq] warning: failed to extract main repo info from %s: %v\n", repoPath, err)
		}
	} else {
		// Set DisplayPath for ghq list format compatibility
		mainEntry.DisplayPath = calculateGhqDisplayPath(mainEntry.Path, ghqRoots)
		entries = append(entries, mainEntry)
	}

	// Worktrees
	if validWorktreesDir && mainEntry != nil {
		wtDir := filepath.Join(repoPath, worktreesDir)
		wtEntries, err := discoverWorktreesInDir(wtDir)
		if err != nil {
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "[gwq] warning: failed to discover worktrees in %s: %v\n", wtDir, err)
			}
		} else {
			// Set DisplayPath for worktrees (main repo's DisplayPath + :dirname)
			for _, wtEntry := range wtEntries {
				wtEntry.DisplayPath = buildWorktreeDisplayPath(mainEntry, wtEntry.Path)
			}
			entries = append(entries, wtEntries...)
		}
	}

	return entries
}

func buildWorktreeDisplayPath(mainEntry *GlobalWorktreeEntry, worktreePath string) string {
	base := mainEntry.DisplayPath
	if base == "" {
		base = fallbackMainDisplayPath(mainEntry)
	}
	branchDir := filepath.Base(worktreePath)
	if base == "" {
		return branchDir
	}
	return base + ":" + branchDir
}

func fallbackMainDisplayPath(mainEntry *GlobalWorktreeEntry) string {
	if mainEntry == nil {
		return ""
	}
	if mainEntry.RepositoryInfo != nil && mainEntry.RepositoryInfo.FullPath != "" {
		return filepath.ToSlash(mainEntry.RepositoryInfo.FullPath)
	}
	if mainEntry.Path == "" {
		return ""
	}
	return filepath.Base(mainEntry.Path)
}
