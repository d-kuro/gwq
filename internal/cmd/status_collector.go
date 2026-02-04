package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

			// Check if current working directory is within this worktree path.
			// Using filepath.Rel avoids prefix collision (e.g., /repo vs /repo2).
			relPath, relErr := filepath.Rel(worktree.Path, currentPath)
			if relErr == nil && !strings.HasPrefix(relPath, "..") {
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

	count, _ := strconv.Atoi(strings.TrimSpace(output))
	return count
}

func (c *StatusCollector) determineWorktreeState(status *models.GitStatus) models.WorktreeState {
	switch {
	case status.Conflicts > 0:
		return models.WorktreeStatusConflict
	case status.Staged > 0:
		return models.WorktreeStatusStaged
	case status.Modified > 0 || status.Added > 0 || status.Deleted > 0 || status.Untracked > 0:
		return models.WorktreeStatusModified
	default:
		return models.WorktreeStatusClean
	}
}

func (c *StatusCollector) getLastActivity(path string) (time.Time, error) {
	g := git.New(path)

	// Step 1: Check dirty files (staged + unstaged + untracked) via git status
	// This is fast and reflects recent activity more accurately
	latestTime, err := c.getLastActivityFromDirtyFiles(g, path)
	if err == nil && !latestTime.IsZero() {
		return latestTime, nil
	}

	// Step 2: If no dirty files, use last commit timestamp
	commitTime, err := c.getLastCommitTime(g)
	if err == nil && !commitTime.IsZero() {
		return commitTime, nil
	}

	// Step 3: Fallback to sampling tracked files
	return c.getLastActivityFromTrackedFilesSampled(g, path)
}

// getLastActivityFromDirtyFiles gets the latest modification time from dirty files.
// This includes staged changes, unstaged changes, and untracked files.
func (c *StatusCollector) getLastActivityFromDirtyFiles(g *git.Git, path string) (time.Time, error) {
	// git status --porcelain -z returns all dirty files
	// Note: We don't use -uall to avoid performance issues with large numbers of untracked files
	output, err := g.Run("status", "--porcelain", "-z")
	if err != nil {
		return time.Time{}, err
	}

	if output == "" {
		return time.Time{}, nil // No dirty files
	}

	var latestTime time.Time
	files := parseGitStatusFiles(output)

	for _, file := range files {
		fullPath := filepath.Join(path, file)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
		}
	}

	return latestTime, nil
}

// parseGitStatusFiles parses git status --porcelain -z output and returns file paths.
//
// Format: "XY filename\x00" or "XY oldname\x00newname\x00" for renames/copies
// - X: status in index
// - Y: status in work tree
// - Rename/Copy (R/C): has two NUL-separated names (old and new)
//
// Notes:
// - Do not use TrimSpace (it would corrupt filenames with leading/trailing spaces)
// - Skip first 3 characters (XY + space) to get filename
func parseGitStatusFiles(output string) []string {
	var files []string
	parts := strings.Split(strings.TrimRight(output, "\x00"), "\x00")

	i := 0
	for i < len(parts) {
		part := parts[i]
		if len(part) < 3 {
			i++
			continue
		}

		statusCode := part[0:2]
		// Skip first 3 characters (XY + space)
		filename := part[3:]

		// Check for Rename (R) or Copy (C) in either X or Y position
		isRenameOrCopy := statusCode[0] == 'R' || statusCode[0] == 'C' ||
			statusCode[1] == 'R' || statusCode[1] == 'C'
		if isRenameOrCopy && i+1 < len(parts) {
			// For rename/copy: oldname is part[3:], newname is next part
			// Use newname (the current file)
			i++
			newname := parts[i]
			if newname != "" {
				files = append(files, newname)
			}
		} else {
			if filename != "" {
				files = append(files, filename)
			}
		}
		i++
	}
	return files
}

// getLastCommitTime gets the timestamp of the last commit.
func (c *StatusCollector) getLastCommitTime(g *git.Git) (time.Time, error) {
	output, err := g.Run("log", "-1", "--format=%ct")
	if err != nil {
		return time.Time{}, err
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return time.Time{}, fmt.Errorf("no commits found")
	}

	timestamp, err := strconv.ParseInt(output, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(timestamp, 0), nil
}

// getLastActivityFromTrackedFilesSampled gets the latest modification time from tracked files
// using sampling (first N files) for performance.
func (c *StatusCollector) getLastActivityFromTrackedFilesSampled(g *git.Git, path string) (time.Time, error) {
	const sampleSize = 100

	output, err := g.Run("ls-files", "-z")
	if err != nil {
		// Fallback to directory walk
		return c.getLastActivityFallback(path)
	}

	files := strings.Split(strings.TrimRight(output, "\x00"), "\x00")
	if len(files) > sampleSize {
		files = files[:sampleSize]
	}

	var latestTime time.Time
	for _, file := range files {
		if file == "" {
			continue
		}

		fullPath := filepath.Join(path, file)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
		}
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

// Directories to skip during fallback file walking.
var skipDirs = map[string]bool{
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

// getLastActivityFallback is the fallback method when git commands fail
func (c *StatusCollector) getLastActivityFallback(path string) (time.Time, error) {
	var latestTime time.Time

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue even if we can't access a file
		}

		if info.IsDir() {
			if shouldSkipDir(p, path) {
				return filepath.SkipDir
			}
			return nil
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

// shouldSkipDir determines if a directory should be skipped during walking.
func shouldSkipDir(dirPath, rootPath string) bool {
	dirName := filepath.Base(dirPath)
	if skipDirs[dirName] {
		return true
	}
	// Skip hidden directories except the root
	return dirName != "." && strings.HasPrefix(dirName, ".") && dirPath != rootPath
}

func (c *StatusCollector) extractRepository(path string) string {
	cleanPath := filepath.Clean(path)

	// Check for ghq-style .worktrees directory in path
	// Example: /home/user/ghq/github.com/owner/repo/.worktrees/branch
	worktreesPattern := string(filepath.Separator) + ".worktrees" + string(filepath.Separator)
	if repoPath, _, found := strings.Cut(cleanPath, worktreesPattern); found {
		return c.extractRepoNameFromPath(repoPath)
	}

	// Also handle main repository in ghq (no .worktrees)
	// Check if path contains typical ghq structure (github.com/owner/repo)
	if repoName := c.extractGhqStyleRepo(cleanPath); repoName != "" {
		return repoName
	}

	// Return basename if basedir is not set
	if c.basedir == "" {
		return filepath.Base(path)
	}

	baseDir := filepath.Clean(c.basedir)

	// Check if the path is under the base directory using filepath.Rel
	// This avoids prefix collision (e.g., /base vs /base2)
	rel, err := filepath.Rel(baseDir, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		// Path is not under base directory, return basename
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

// extractRepoNameFromPath extracts the repository identifier from a path.
// It looks for patterns like github.com/owner/repo.
func (c *StatusCollector) extractRepoNameFromPath(path string) string {
	parts := strings.Split(path, string(filepath.Separator))

	// Find a part that looks like a host (contains a dot)
	for i, part := range parts {
		if strings.Contains(part, ".") && i+2 < len(parts) {
			// Found host, try to construct owner/repo
			return filepath.Join(part, parts[i+1], parts[i+2])
		}
	}

	return filepath.Base(path)
}

// extractGhqStyleRepo extracts repository identifier from a ghq-style path.
// Returns empty string if not a ghq-style path.
func (c *StatusCollector) extractGhqStyleRepo(path string) string {
	parts := strings.Split(path, string(filepath.Separator))

	// Look for a pattern like host/owner/repo (e.g., github.com/user/project)
	for i := 0; i < len(parts)-2; i++ {
		if strings.Contains(parts[i], ".") {
			// Found potential host (github.com, gitlab.com, etc.)
			// Check if next two parts could be owner/repo
			if parts[i+1] != "" && parts[i+2] != "" && parts[i+2] != ".worktrees" {
				return filepath.Join(parts[i], parts[i+1], parts[i+2])
			}
		}
	}

	return ""
}

func (c *StatusCollector) collectProcesses(_ context.Context, _ string) ([]models.ProcessInfo, error) {
	// TODO: Implement process detection for AI agents and other tools
	return nil, nil
}
