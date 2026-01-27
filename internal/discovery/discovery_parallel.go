// Package discovery provides filesystem-based global worktree discovery.
package discovery

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/d-kuro/gwq/pkg/models"
)

// DiscoverOptions contains options for parallel worktree discovery.
type DiscoverOptions struct {
	MaxWorkers int  // Default: min(runtime.NumCPU(), 4)
	Lazy       bool // If true, return entries with only Path set; call EnsureLoaded() for details
}

// getMaxWorkers returns the number of workers to use for parallel operations.
// Priority: opts.MaxWorkers > GWQ_DISCOVERY_WORKERS env var > default (min(CPU, 4))
func getMaxWorkers(opts *DiscoverOptions) int {
	maxWorkers := min(runtime.NumCPU(), 4) // I/O bound, so keep it conservative

	if opts != nil && opts.MaxWorkers > 0 {
		maxWorkers = opts.MaxWorkers
	}

	// Environment variable override
	if envWorkers := os.Getenv("GWQ_DISCOVERY_WORKERS"); envWorkers != "" {
		if n, err := strconv.Atoi(envWorkers); err == nil && n > 0 {
			maxWorkers = n
		}
	}

	return maxWorkers
}

// DiscoverGlobalWorktreesLazy finds all worktrees with minimal overhead.
// Returns entries with only Path and IsMain set. Call EnsureLoaded() on each entry
// to load details (Branch, CommitHash, RepositoryURL, etc.) on demand.
// This is the fastest discovery method when you don't need all details immediately.
func DiscoverGlobalWorktreesLazy(baseDir string) ([]*GlobalWorktreeEntry, error) {
	expandedDir, err := expandBaseDir(baseDir)
	if err != nil {
		return nil, err
	}
	if expandedDir == "" {
		return []*GlobalWorktreeEntry{}, nil
	}

	paths, err := collectWorktreePaths(expandedDir)
	if err != nil {
		return nil, err
	}

	// Convert paths to lazy entries
	entries := make([]*GlobalWorktreeEntry, len(paths))
	for i, path := range paths {
		entries[i] = &GlobalWorktreeEntry{
			Path:   path,
			IsMain: false,
		}
	}

	return entries, nil
}

// DiscoverGlobalWorktreesParallel finds all worktrees in the configured base directory
// using parallel processing for extracting worktree information.
// If opts.Lazy is true, returns lazy entries (call EnsureLoaded() for details).
func DiscoverGlobalWorktreesParallel(baseDir string, opts *DiscoverOptions) ([]*GlobalWorktreeEntry, error) {
	// Fast path: lazy loading mode returns immediately after path collection
	if opts != nil && opts.Lazy {
		return DiscoverGlobalWorktreesLazy(baseDir)
	}

	expandedDir, err := expandBaseDir(baseDir)
	if err != nil {
		return nil, err
	}
	if expandedDir == "" {
		return []*GlobalWorktreeEntry{}, nil
	}

	// Phase 1: Collect worktree paths (lightweight)
	worktreePaths, err := collectWorktreePaths(expandedDir)
	if err != nil {
		return nil, err
	}

	if len(worktreePaths) == 0 {
		return []*GlobalWorktreeEntry{}, nil
	}

	// Phase 2: Extract worktree info with worker pool
	maxWorkers := getMaxWorkers(opts)
	return extractWorktreeInfoWithWorkerPool(worktreePaths, maxWorkers)
}

// collectWorktreePaths walks a directory and collects all worktree paths.
func collectWorktreePaths(baseDir string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
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

		paths = append(paths, path)
		return filepath.SkipDir
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return paths, nil
}

// extractWorktreeInfoWithWorkerPool extracts worktree information using a fixed-size worker pool.
// Uses fast file-based extraction with fallback to git commands.
func extractWorktreeInfoWithWorkerPool(paths []string, maxWorkers int) ([]*GlobalWorktreeEntry, error) {
	jobs := make(chan int, len(paths))
	results := make([]*GlobalWorktreeEntry, len(paths))
	var wg sync.WaitGroup

	// Start fixed number of workers
	for range maxWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				// Use fast extraction with fallback
				entry, err := extractWorktreeInfoWithFallback(paths[idx])
				if err != nil {
					// Error policy: log non-NotExist errors to stderr
					if !os.IsNotExist(err) {
						fmt.Fprintf(os.Stderr, "[gwq] warning: failed to extract worktree info from %s: %v\n", paths[idx], err)
					}
					continue
				}
				results[idx] = entry
			}
		}()
	}

	// Submit jobs
	for i := range paths {
		jobs <- i
	}
	close(jobs)

	wg.Wait()

	return filterNilEntries(results), nil
}

// filterNilEntries removes nil entries from a slice.
func filterNilEntries(entries []*GlobalWorktreeEntry) []*GlobalWorktreeEntry {
	result := make([]*GlobalWorktreeEntry, 0, len(entries))
	for _, e := range entries {
		if e != nil {
			result = append(result, e)
		}
	}
	return result
}

// DiscoverAllWorktreesParallel discovers all worktrees from multiple sources using parallel processing.
// This is a parallel version of DiscoverAllWorktrees.
func DiscoverAllWorktreesParallel(cfg *models.Config, opts *DiscoverOptions) ([]*GlobalWorktreeEntry, error) {
	var allEntries []*GlobalWorktreeEntry
	seen := make(map[string]bool)

	if cfg.Ghq.Enabled {
		maxWorkers := getMaxWorkers(opts)
		ghqEntries, err := discoverGhqWorktreesParallel(cfg.Ghq.WorktreesDir, maxWorkers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gwq] warning: failed to discover ghq worktrees: %v\n", err)
		} else {
			addUniqueEntries(&allEntries, seen, ghqEntries)
		}
	}

	if cfg.Worktree.BaseDir != "" {
		basedirEntries, err := DiscoverGlobalWorktreesParallel(cfg.Worktree.BaseDir, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gwq] warning: failed to discover basedir worktrees: %v\n", err)
		} else {
			addUniqueEntries(&allEntries, seen, basedirEntries)
		}
	}

	return allEntries, nil
}

// DiscoverGlobalWorktreesPipeline discovers worktrees using a pipeline approach.
// Exploration and extraction run in parallel, reducing overall latency.
func DiscoverGlobalWorktreesPipeline(baseDir string, opts *DiscoverOptions) ([]*GlobalWorktreeEntry, error) {
	expandedDir, err := expandBaseDir(baseDir)
	if err != nil {
		return nil, err
	}
	if expandedDir == "" {
		return []*GlobalWorktreeEntry{}, nil
	}

	maxWorkers := getMaxWorkers(opts)

	// Channels for pipeline
	pathChan := make(chan string, maxWorkers*2) // Buffered to prevent blocking
	resultChan := make(chan *GlobalWorktreeEntry, maxWorkers*2)
	doneChan := make(chan struct{})

	var wg sync.WaitGroup

	// Start extraction workers (consumers)
	for range maxWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathChan {
				entry, err := extractWorktreeInfoWithFallback(path)
				if err != nil {
					if !os.IsNotExist(err) {
						fmt.Fprintf(os.Stderr, "[gwq] warning: failed to extract worktree info from %s: %v\n", path, err)
					}
					continue
				}
				resultChan <- entry
			}
		}()
	}

	// Collector goroutine
	var entries []*GlobalWorktreeEntry
	go func() {
		for entry := range resultChan {
			entries = append(entries, entry)
		}
		close(doneChan)
	}()

	// Explorer (producer) - walks and sends paths immediately
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

		// Send path to workers immediately (non-blocking due to buffer)
		pathChan <- path
		return filepath.SkipDir
	})

	// Close path channel to signal workers to finish
	close(pathChan)

	// Wait for all workers to finish
	wg.Wait()

	// Close result channel and wait for collector
	close(resultChan)
	<-doneChan

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return entries, nil
}
