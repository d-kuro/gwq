package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/pkg/models"
)

// StatusCollectorOptions contains optional parameters for StatusCollector.
type StatusCollectorOptions struct {
	IncludeProcess bool
	FetchRemote    bool
	StaleThreshold time.Duration
	BaseDir        string
}

// StatusCollector collects status information for worktrees.
type StatusCollector struct {
	includeProcess bool
	fetchRemote    bool
	staleThreshold time.Duration
	basedir        string
}

// NewStatusCollector creates a new status collector instance.
func NewStatusCollector(includeProcess, fetchRemote bool) *StatusCollector {
	return &StatusCollector{
		includeProcess: includeProcess,
		fetchRemote:    fetchRemote,
		staleThreshold: 14 * 24 * time.Hour, // 14 days
	}
}

// NewStatusCollectorWithOptions creates a new status collector with custom options.
func NewStatusCollectorWithOptions(opts StatusCollectorOptions) *StatusCollector {
	// Default stale threshold to 14 days if not specified
	if opts.StaleThreshold == 0 {
		opts.StaleThreshold = 14 * 24 * time.Hour
	}

	return &StatusCollector{
		includeProcess: opts.IncludeProcess,
		fetchRemote:    opts.FetchRemote,
		staleThreshold: opts.StaleThreshold,
		basedir:        opts.BaseDir,
	}
}

// CollectAll collects status for all provided worktrees in parallel.
func (c *StatusCollector) CollectAll(ctx context.Context, worktrees []*models.Worktree) ([]*models.WorktreeStatus, error) {
	statuses := make([]*models.WorktreeStatus, len(worktrees))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	currentPath, _ := os.Getwd()

	for i, wt := range worktrees {
		wg.Add(1)
		go func(idx int, worktree *models.Worktree) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			status, err := c.collectOne(ctx, worktree)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			if strings.HasPrefix(currentPath, worktree.Path) {
				status.IsCurrent = true
			}

			statuses[idx] = status
		}(i, wt)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	var validStatuses []*models.WorktreeStatus
	for _, s := range statuses {
		if s != nil {
			validStatuses = append(validStatuses, s)
		}
	}

	return validStatuses, nil
}

func (c *StatusCollector) collectOne(ctx context.Context, worktree *models.Worktree) (*models.WorktreeStatus, error) {
	status := &models.WorktreeStatus{
		Path:       worktree.Path,
		Branch:     worktree.Branch,
		Repository: c.extractRepository(worktree.Path),
		Status:     models.WorktreeStatusClean,
	}

	g := git.New(worktree.Path)

	gitStatus, err := c.collectGitStatus(ctx, g)
	if err != nil {
		// Log error but continue with minimal status
		// fmt.Fprintf(os.Stderr, "Warning: Failed to collect git status for %s: %v\n", worktree.Path, err)
		status.GitStatus = models.GitStatus{}
		status.Status = models.WorktreeStatusUnknown
	} else {
		status.GitStatus = *gitStatus
		status.Status = c.determineWorktreeState(gitStatus)
	}

	lastActivity, err := c.getLastActivity(worktree.Path)
	if err == nil {
		status.LastActivity = lastActivity
		if time.Since(lastActivity) > c.staleThreshold {
			status.Status = models.WorktreeStatusStale
		}
	}

	if c.includeProcess {
		processes, err := c.collectProcesses(ctx, worktree.Path)
		if err == nil {
			status.ActiveProcess = processes
		}
	}

	return status, nil
}

func (c *StatusCollector) collectGitStatus(ctx context.Context, g *git.Git) (*models.GitStatus, error) {
	status := &models.GitStatus{}

	// Count modified, staged, and other file states
	if err := c.countFileStates(ctx, g, status); err != nil {
		return nil, err
	}

	// Count untracked files separately for more accurate count
	if err := c.countUntrackedFiles(ctx, g, status); err != nil {
		// Non-fatal: continue even if we can't count untracked files
		status.Untracked = 0
	}

	if c.fetchRemote {
		// Errors are ignored as remote might not be available
		_ = c.fetchRemoteStatus(ctx, g, status)
	}

	return status, nil
}

// countFileStates counts modified, staged, added, deleted, and conflicted files
func (c *StatusCollector) countFileStates(ctx context.Context, g *git.Git, status *models.GitStatus) error {
	gitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := g.RunWithContext(gitCtx, "status", "--porcelain=v1", "-uno")
	if err != nil {
		return err
	}

	for line := range strings.SplitSeq(output, "\n") {
		if len(line) < 3 {
			continue
		}

		c.processStatusLine(line, status)
	}

	return nil
}

// processStatusLine processes a single line from git status output
func (c *StatusCollector) processStatusLine(line string, status *models.GitStatus) {
	index := line[0]
	worktree := line[1]

	if index != ' ' && index != '?' {
		status.Staged++
	}

	switch worktree {
	case 'M':
		status.Modified++
	case 'A':
		status.Added++
	case 'D':
		status.Deleted++
	case '?':
		status.Untracked++
	case 'U':
		status.Conflicts++
	}
}

// countUntrackedFiles counts untracked files using ls-files
func (c *StatusCollector) countUntrackedFiles(ctx context.Context, g *git.Git, status *models.GitStatus) error {
	gitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	untrackedFiles, err := g.RunWithContext(gitCtx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return err
	}

	if untrackedFiles != "" {
		status.Untracked = len(strings.Split(strings.TrimSpace(untrackedFiles), "\n"))
	}

	return nil
}

func (c *StatusCollector) fetchRemoteStatus(ctx context.Context, g *git.Git, status *models.GitStatus) error {
	// Get current branch and upstream
	currentBranch, err := c.getCurrentBranch(ctx, g)
	if err != nil {
		return err
	}

	upstream, err := c.getUpstreamBranch(ctx, g, currentBranch)
	if err != nil || upstream == "" {
		return err
	}

	// Count ahead/behind commits
	c.countAheadBehind(ctx, g, upstream, status)

	return nil
}

// getCurrentBranch gets the current branch name
func (c *StatusCollector) getCurrentBranch(ctx context.Context, g *git.Git) (string, error) {
	gitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	currentBranch, err := g.RunWithContext(gitCtx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(currentBranch), nil
}

// getUpstreamBranch gets the upstream branch for the current branch
func (c *StatusCollector) getUpstreamBranch(ctx context.Context, g *git.Git, currentBranch string) (string, error) {
	gitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	upstream, err := g.RunWithContext(gitCtx, "rev-parse", "--abbrev-ref", currentBranch+"@{upstream}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(upstream), nil
}

// countAheadBehind counts commits ahead and behind upstream
func (c *StatusCollector) countAheadBehind(ctx context.Context, g *git.Git, upstream string, status *models.GitStatus) {
	// Count commits ahead
	status.Ahead = c.countRevList(ctx, g, upstream+"..HEAD")

	// Count commits behind
	status.Behind = c.countRevList(ctx, g, "HEAD.."+upstream)
}

// countRevList counts commits in a revision range
func (c *StatusCollector) countRevList(ctx context.Context, g *git.Git, revRange string) int {
	gitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := g.RunWithContext(gitCtx, "rev-list", "--count", revRange)
	if err != nil {
		return 0
	}

	var count int
	if _, err := fmt.Sscanf(strings.TrimSpace(output), "%d", &count); err != nil {
		return 0
	}
	return count
}

func (c *StatusCollector) determineWorktreeState(status *models.GitStatus) models.WorktreeState {
	if status.Conflicts > 0 {
		return models.WorktreeStatusConflict
	}
	if status.Staged > 0 {
		return models.WorktreeStatusStaged
	}
	if status.Modified > 0 || status.Added > 0 || status.Deleted > 0 || status.Untracked > 0 {
		return models.WorktreeStatusModified
	}
	return models.WorktreeStatusClean
}

func (c *StatusCollector) getLastActivity(path string) (time.Time, error) {
	// Use git ls-files to get tracked files efficiently
	// This approach respects .gitignore patterns automatically and is much faster
	// than walking the entire directory tree
	g := git.New(path)

	latestTime, err := c.getLastActivityFromTrackedFiles(g, path)
	if err != nil {
		// Fallback to directory walk if git command fails
		return c.getLastActivityFallback(path)
	}

	// Also check untracked files that are not ignored
	untrackedTime := c.getLastActivityFromUntrackedFiles(g, path)
	if untrackedTime.After(latestTime) {
		latestTime = untrackedTime
	}

	if latestTime.IsZero() {
		// If no files found, use the directory's own modification time
		info, err := os.Stat(path)
		if err == nil {
			latestTime = info.ModTime()
		}
	}

	return latestTime, nil
}

// getLastActivityFromTrackedFiles gets the latest modification time from tracked files
func (c *StatusCollector) getLastActivityFromTrackedFiles(g *git.Git, path string) (time.Time, error) {
	// Get list of tracked files
	// Using -z for null-terminated output to handle filenames with spaces
	output, err := g.Run("ls-files", "-z")
	if err != nil {
		return time.Time{}, err
	}

	var latestTime time.Time
	files := strings.SplitSeq(strings.TrimRight(output, "\x00"), "\x00")

	for file := range files {
		if file == "" {
			continue
		}

		fullPath := filepath.Join(path, file)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue // Skip files we can't stat
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
		}
	}

	return latestTime, nil
}

// getLastActivityFromUntrackedFiles gets the latest modification time from untracked files
func (c *StatusCollector) getLastActivityFromUntrackedFiles(g *git.Git, path string) time.Time {
	var latestTime time.Time

	untrackedOutput, err := g.Run("ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return latestTime
	}

	untrackedFiles := strings.SplitSeq(strings.TrimRight(untrackedOutput, "\x00"), "\x00")
	for file := range untrackedFiles {
		if file == "" {
			continue
		}

		fullPath := filepath.Join(path, file)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
		}
	}

	return latestTime
}

// getLastActivityFallback is the fallback method when git commands fail
func (c *StatusCollector) getLastActivityFallback(path string) (time.Time, error) {
	var latestTime time.Time

	// Common large directories to skip
	skipDirs := map[string]bool{
		".git":          true,
		"node_modules":  true,
		"vendor":        true,
		".next":         true,
		"dist":          true,
		"build":         true,
		"target":        true,
		".cache":        true,
		"coverage":      true,
		"__pycache__":   true,
		".pytest_cache": true,
		".venv":         true,
		"venv":          true,
		".idea":         true,
		".vscode":       true,
	}

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue even if we can't access a file
		}

		// Skip directories
		if info.IsDir() {
			dirName := filepath.Base(p)
			if skipDirs[dirName] {
				return filepath.SkipDir
			}
			// Also skip hidden directories (except the root)
			if dirName != "." && strings.HasPrefix(dirName, ".") && p != path {
				return filepath.SkipDir
			}
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
		}

		return nil
	})

	if err != nil {
		return time.Time{}, err
	}

	return latestTime, nil
}

func (c *StatusCollector) extractRepository(path string) string {
	// Return basename if basedir is not set
	if c.basedir == "" {
		return filepath.Base(path)
	}

	baseDir := filepath.Clean(c.basedir)
	cleanPath := filepath.Clean(path)

	// Check if the path is under the base directory
	if !strings.HasPrefix(cleanPath, baseDir) {
		// Path is not under base directory, return basename
		return filepath.Base(path)
	}

	rel, err := filepath.Rel(baseDir, cleanPath)
	if err != nil {
		// Failed to get relative path, fallback to basename
		return filepath.Base(path)
	}

	// Split the relative path into components
	parts := strings.Split(rel, string(filepath.Separator))

	// Expected structure: host/owner/repository/branch
	// Return the first 3 components if available
	if len(parts) >= 3 {
		return filepath.Join(parts[0], parts[1], parts[2])
	}

	// If we don't have enough parts, return what we have or the basename
	if len(parts) > 0 {
		return rel
	}

	return filepath.Base(path)
}

func (c *StatusCollector) collectProcesses(ctx context.Context, worktreePath string) ([]models.ProcessInfo, error) {
	// TODO: Implement process detection for AI agents and other tools
	// This would involve:
	// 1. Scanning for processes with working directory in worktreePath
	// 2. Identifying known AI agent processes (claude, copilot, cursor, etc.)
	// 3. Detecting common development tools (npm, cargo, python, etc.)
	// 4. Platform-specific process enumeration (ps on Unix, tasklist on Windows)
	//
	// For now, this is a stub that returns an empty slice.
	// The actual implementation is deferred as it requires platform-specific code
	// and careful consideration of performance implications.
	return []models.ProcessInfo{}, nil
}
