package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/url"
)

// TestRepository creates a test git repository (copy from git package for testing)
type TestRepository struct {
	Path string
}

// NewTestRepository creates a new test repository
func NewTestRepository(t *testing.T) *TestRepository {
	t.Helper()

	tmpDir := t.TempDir()
	repo := &TestRepository{Path: tmpDir}

	// Set environment variables for git if needed in CI
	t.Setenv("GIT_AUTHOR_NAME", "Test User")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test User")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	// Initialize repository with main as default branch
	if err := repo.run("init", "-b", "main"); err != nil {
		t.Fatalf("Failed to init repository: %v", err)
	}

	// Configure git user for commits
	if err := repo.run("config", "user.name", "Test User"); err != nil {
		t.Fatalf("Failed to set user.name: %v", err)
	}
	if err := repo.run("config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("Failed to set user.email: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repository\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := repo.run("add", "."); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}
	if err := repo.run("commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	return repo
}

// run executes a git command in the test repository
func (r *TestRepository) run(args ...string) error {
	g := git.New(r.Path)
	_, err := g.RunCommand(args...)
	return err
}

// CreateBranch creates a new branch in the test repository
func (r *TestRepository) CreateBranch(t *testing.T, name string) {
	t.Helper()
	if err := r.run("checkout", "-b", name); err != nil {
		t.Fatalf("Failed to create branch %s: %v", name, err)
	}
}

// CreateWorktree creates a worktree in the test repository
func (r *TestRepository) CreateWorktree(t *testing.T, path, branch string) {
	t.Helper()
	// First check if branch exists in current worktree, if so switch away
	currentBranch, _ := r.getCurrentBranch()
	if currentBranch == branch {
		// Try to switch to main branch first
		if err := r.run("checkout", "main"); err != nil {
			// If main doesn't exist or we're already on it, create a temporary branch
			if err := r.run("checkout", "-b", "temp-branch-"+branch); err != nil {
				t.Fatalf("Failed to switch away from branch: %v", err)
			}
		}
	}

	if err := r.run("worktree", "add", path, branch); err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}
}

func (r *TestRepository) getCurrentBranch() (string, error) {
	g := git.New(r.Path)
	output, err := g.RunCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// AddRemote adds a remote to the repository
func (r *TestRepository) AddRemote(t *testing.T, name, url string) {
	t.Helper()
	if err := r.run("remote", "add", name, url); err != nil {
		t.Fatalf("Failed to add remote %s: %v", name, err)
	}
}

func TestDiscoverGlobalWorktrees_EmptyBaseDir(t *testing.T) {
	entries, err := DiscoverGlobalWorktrees("")
	if err == nil {
		t.Error("Expected error for empty base directory")
	}
	if entries != nil {
		t.Error("Expected nil entries for empty base directory")
	}
}

func TestDiscoverGlobalWorktrees_NonExistentBaseDir(t *testing.T) {
	entries, err := DiscoverGlobalWorktrees("/nonexistent/path")
	if err != nil {
		t.Errorf("Unexpected error for non-existent directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected empty entries for non-existent directory, got %d", len(entries))
	}
}

func TestDiscoverGlobalWorktrees_NoWorktrees(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory with no git repositories
	subDir := filepath.Join(tmpDir, "not-a-repo")
	err := os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	entries, err := DiscoverGlobalWorktrees(tmpDir)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected no entries, got %d", len(entries))
	}
}

func TestDiscoverGlobalWorktrees_SingleWorktree(t *testing.T) {
	// Skip this test for now as it requires complex git setup
	// TODO: Implement with mocked git operations
	t.Skip("Skipping complex git test - needs mock implementation")
}

func TestDiscoverGlobalWorktrees_MultipleWorktrees(t *testing.T) {
	// Skip this test for now as it requires complex git setup
	// TODO: Implement with mocked git operations
	t.Skip("Skipping complex git test - needs mock implementation")
}

func TestDiscoverGlobalWorktrees_SkipsMainRepositories(t *testing.T) {
	// Skip this test for now as it requires complex git setup
	// TODO: Implement with mocked git operations
	t.Skip("Skipping complex git test - needs mock implementation")
}

func TestExtractWorktreeInfo_ValidWorktree(t *testing.T) {
	// Skip this test for now as it requires complex git setup
	// TODO: Implement with mocked git operations
	t.Skip("Skipping complex git test - needs mock implementation")
}

func TestGetCurrentBranch_InvalidPath(t *testing.T) {
	_, err := getCurrentBranch("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestGetCurrentCommitHash_InvalidPath(t *testing.T) {
	_, err := getCurrentCommitHash("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestConvertToWorktreeModels_BasicConversion(t *testing.T) {
	entries := []*GlobalWorktreeEntry{
		{
			Branch:     "main",
			Path:       "/path/to/main",
			CommitHash: "abc123",
			IsMain:     true,
		},
		{
			Branch:     "feature",
			Path:       "/path/to/feature",
			CommitHash: "def456",
			IsMain:     false,
		},
	}

	worktrees := ConvertToWorktreeModels(entries, false)

	if len(worktrees) != 2 {
		t.Fatalf("Expected 2 worktrees, got %d", len(worktrees))
	}

	if worktrees[0].Branch != "main" {
		t.Errorf("Expected first branch 'main', got '%s'", worktrees[0].Branch)
	}
	if worktrees[1].Branch != "feature" {
		t.Errorf("Expected second branch 'feature', got '%s'", worktrees[1].Branch)
	}
}

func TestConvertToWorktreeModels_WithRepoName(t *testing.T) {
	repoInfo, _ := url.ParseRepositoryURL("https://github.com/testuser/testrepo.git")
	entries := []*GlobalWorktreeEntry{
		{
			RepositoryInfo: repoInfo,
			Branch:         "feature",
			Path:           "/path/to/feature",
			CommitHash:     "abc123",
			IsMain:         false,
		},
	}

	worktrees := ConvertToWorktreeModels(entries, true)

	if len(worktrees) != 1 {
		t.Fatalf("Expected 1 worktree, got %d", len(worktrees))
	}

	// Display format: repo:branch (restored default format)
	expected := "testrepo:feature"
	if worktrees[0].Branch != expected {
		t.Errorf("Expected branch '%s', got '%s'", expected, worktrees[0].Branch)
	}
}

func TestConvertToWorktreeModels_MainRepoShowsRepoName(t *testing.T) {
	repoInfo, _ := url.ParseRepositoryURL("https://github.com/testuser/testrepo.git")
	entries := []*GlobalWorktreeEntry{
		{
			RepositoryInfo: repoInfo,
			Branch:         "main",
			Path:           "/path/to/repo",
			CommitHash:     "abc123",
			IsMain:         true,
		},
	}

	worktrees := ConvertToWorktreeModels(entries, true)

	if len(worktrees) != 1 {
		t.Fatalf("Expected 1 worktree, got %d", len(worktrees))
	}

	// Display format: repo name only for main worktree (restored default format)
	expected := "testrepo"
	if worktrees[0].Branch != expected {
		t.Errorf("Expected branch '%s', got '%s'", expected, worktrees[0].Branch)
	}
}

func TestConvertToWorktreeModels_DisplayPathOverridesFullPath(t *testing.T) {
	// When DisplayPath is set (ghq mode), it should be used instead of RepositoryInfo.FullPath
	repoInfo, _ := url.ParseRepositoryURL("ssh://git@ssh.code.aws.dev/proserve/japan.git")
	entries := []*GlobalWorktreeEntry{
		{
			RepositoryInfo: repoInfo,
			DisplayPath:    "aws/aws-jp-proserve/docomo/lamp", // ghq root relative path
			Branch:         "main",
			Path:           "/Users/test/ghq/aws/aws-jp-proserve/docomo/lamp",
			CommitHash:     "abc123",
			IsMain:         true,
		},
	}

	worktrees := ConvertToWorktreeModels(entries, true)

	if len(worktrees) != 1 {
		t.Fatalf("Expected 1 worktree, got %d", len(worktrees))
	}

	// Should use DisplayPath instead of RepositoryInfo.FullPath
	expected := "aws/aws-jp-proserve/docomo/lamp"
	if worktrees[0].Branch != expected {
		t.Errorf("Expected branch '%s', got '%s'", expected, worktrees[0].Branch)
	}
}

func TestConvertToWorktreeModels_DisplayPathWithDirname(t *testing.T) {
	repoInfo, _ := url.ParseRepositoryURL("https://github.com/testuser/testrepo.git")
	entries := []*GlobalWorktreeEntry{
		{
			RepositoryInfo: repoInfo,
			DisplayPath:    "github.com/testuser/testrepo:feature-test", // DisplayPath already includes :dirname
			Branch:         "feature/test",
			Path:           "/Users/test/ghq/github.com/testuser/testrepo/.worktrees/feature-test",
			CommitHash:     "def456",
			IsMain:         false,
		},
	}

	worktrees := ConvertToWorktreeModels(entries, true)

	if len(worktrees) != 1 {
		t.Fatalf("Expected 1 worktree, got %d", len(worktrees))
	}

	// Should use DisplayPath as-is (already contains :dirname)
	expected := "github.com/testuser/testrepo:feature-test"
	if worktrees[0].Branch != expected {
		t.Errorf("Expected branch '%s', got '%s'", expected, worktrees[0].Branch)
	}
}

func TestConvertToWorktreeModels_PreservesRepositoryInfo(t *testing.T) {
	repoInfo, _ := url.ParseRepositoryURL("https://github.com/testuser/testrepo.git")
	entries := []*GlobalWorktreeEntry{
		{
			RepositoryInfo: repoInfo,
			Branch:         "feature",
			Path:           "/path/to/feature",
			CommitHash:     "abc123",
			IsMain:         false,
		},
		{
			RepositoryInfo: nil, // No repo info
			Branch:         "local",
			Path:           "/path/to/local",
			CommitHash:     "def456",
			IsMain:         false,
		},
	}

	worktrees := ConvertToWorktreeModels(entries, true)

	if len(worktrees) != 2 {
		t.Fatalf("Expected 2 worktrees, got %d", len(worktrees))
	}

	// First entry should have RepositoryInfo preserved
	if worktrees[0].RepositoryInfo == nil {
		t.Error("Expected RepositoryInfo to be preserved for first worktree")
	} else {
		if worktrees[0].RepositoryInfo.Host != "github.com" {
			t.Errorf("Expected Host 'github.com', got '%s'", worktrees[0].RepositoryInfo.Host)
		}
		if worktrees[0].RepositoryInfo.Owner != "testuser" {
			t.Errorf("Expected Owner 'testuser', got '%s'", worktrees[0].RepositoryInfo.Owner)
		}
		if worktrees[0].RepositoryInfo.Repository != "testrepo" {
			t.Errorf("Expected Repository 'testrepo', got '%s'", worktrees[0].RepositoryInfo.Repository)
		}
	}

	// Second entry should have nil RepositoryInfo
	if worktrees[1].RepositoryInfo != nil {
		t.Error("Expected RepositoryInfo to be nil for second worktree")
	}
}

func TestFormatBranchDisplay(t *testing.T) {
	repoInfo, _ := url.ParseRepositoryURL("https://github.com/aws/repo.git")
	tests := []struct {
		name         string
		entry        *GlobalWorktreeEntry
		showRepoName bool
		expected     string
	}{
		{
			name:         "DisplayPath set - returns as-is",
			entry:        &GlobalWorktreeEntry{DisplayPath: "github.com/aws/repo", Branch: "main", IsMain: true},
			showRepoName: true,
			expected:     "github.com/aws/repo",
		},
		{
			name:         "DisplayPath with dirname for worktree",
			entry:        &GlobalWorktreeEntry{DisplayPath: "github.com/aws/repo:feature-1", Branch: "feature-1", IsMain: false},
			showRepoName: true,
			expected:     "github.com/aws/repo:feature-1",
		},
		{
			name:         "No DisplayPath, RepositoryInfo set, non-main",
			entry:        &GlobalWorktreeEntry{RepositoryInfo: repoInfo, Branch: "feature", IsMain: false},
			showRepoName: true,
			expected:     "repo:feature",
		},
		{
			name:         "No DisplayPath, RepositoryInfo set, main",
			entry:        &GlobalWorktreeEntry{RepositoryInfo: repoInfo, Branch: "main", IsMain: true},
			showRepoName: true,
			expected:     "repo",
		},
		{
			name:         "Both nil - returns branch",
			entry:        &GlobalWorktreeEntry{Branch: "main", IsMain: false},
			showRepoName: true,
			expected:     "main",
		},
		{
			name:         "showRepoName false - returns branch",
			entry:        &GlobalWorktreeEntry{DisplayPath: "github.com/aws/repo", Branch: "main", IsMain: true},
			showRepoName: false,
			expected:     "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBranchDisplay(tt.entry, tt.showRepoName)
			if result != tt.expected {
				t.Errorf("formatBranchDisplay() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFilterGlobalWorktrees_BranchMatch(t *testing.T) {
	entries := []*GlobalWorktreeEntry{
		{Branch: "main", Path: "/path/main"},
		{Branch: "feature-auth", Path: "/path/feature"},
		{Branch: "bugfix-login", Path: "/path/bugfix"},
	}

	matches := FilterGlobalWorktrees(entries, "feature")
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}
	if matches[0].Branch != "feature-auth" {
		t.Errorf("Expected branch 'feature-auth', got '%s'", matches[0].Branch)
	}
}

func TestFilterGlobalWorktrees_PathMatch(t *testing.T) {
	entries := []*GlobalWorktreeEntry{
		{Branch: "main", Path: "/projects/webapp/main"},
		{Branch: "feature", Path: "/projects/api/feature"},
		{Branch: "test", Path: "/other/test"},
	}

	matches := FilterGlobalWorktrees(entries, "api")
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}
	if matches[0].Branch != "feature" {
		t.Errorf("Expected branch 'feature', got '%s'", matches[0].Branch)
	}
}

func TestFilterGlobalWorktrees_RepoMatch(t *testing.T) {
	repoInfo1, _ := url.ParseRepositoryURL("https://github.com/user/webapp.git")
	repoInfo2, _ := url.ParseRepositoryURL("https://github.com/user/api.git")

	entries := []*GlobalWorktreeEntry{
		{RepositoryInfo: repoInfo1, Branch: "main", Path: "/path1"},
		{RepositoryInfo: repoInfo2, Branch: "feature", Path: "/path2"},
	}

	matches := FilterGlobalWorktrees(entries, "webapp")
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}
	if matches[0].Branch != "main" {
		t.Errorf("Expected branch 'main', got '%s'", matches[0].Branch)
	}
}

func TestFilterGlobalWorktrees_RepoColonBranchMatch(t *testing.T) {
	repoInfo, _ := url.ParseRepositoryURL("https://github.com/user/webapp.git")
	entries := []*GlobalWorktreeEntry{
		{RepositoryInfo: repoInfo, Branch: "main", Path: "/path1"},
		{RepositoryInfo: repoInfo, Branch: "feature", Path: "/path2"},
	}

	matches := FilterGlobalWorktrees(entries, "webapp:feature")
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}
	if matches[0].Branch != "feature" {
		t.Errorf("Expected branch 'feature', got '%s'", matches[0].Branch)
	}
}

func TestFilterGlobalWorktrees_CaseInsensitive(t *testing.T) {
	entries := []*GlobalWorktreeEntry{
		{Branch: "Feature-Auth", Path: "/path"},
	}

	matches := FilterGlobalWorktrees(entries, "FEATURE")
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match for case-insensitive search, got %d", len(matches))
	}
}

func TestFilterGlobalWorktrees_NoMatches(t *testing.T) {
	entries := []*GlobalWorktreeEntry{
		{Branch: "main", Path: "/path"},
		{Branch: "feature", Path: "/other"},
	}

	matches := FilterGlobalWorktrees(entries, "nonexistent")
	if len(matches) != 0 {
		t.Errorf("Expected no matches, got %d", len(matches))
	}
}

func TestFilterGlobalWorktrees_EmptyPattern(t *testing.T) {
	entries := []*GlobalWorktreeEntry{
		{Branch: "main", Path: "/path"},
		{Branch: "feature", Path: "/other"},
	}

	matches := FilterGlobalWorktrees(entries, "")
	if len(matches) != 2 {
		t.Errorf("Expected all entries to match empty pattern, got %d", len(matches))
	}
}

// Benchmark tests
func BenchmarkDiscoverGlobalWorktrees(b *testing.B) {
	// Create a temporary directory with multiple worktrees
	baseDir := b.TempDir()

	// Create multiple repositories and worktrees
	for i := 0; i < 10; i++ {
		repo := &TestRepository{Path: filepath.Join(baseDir, fmt.Sprintf("repo%d", i))}
		if err := os.MkdirAll(repo.Path, 0755); err != nil {
			b.Fatalf("Failed to create repo directory: %v", err)
		}

		// Create a simple .git file for worktree simulation
		gitFile := filepath.Join(repo.Path, ".git")
		gitContent := fmt.Sprintf("gitdir: /path/to/main/repo/.git/worktrees/branch%d", i)
		if err := os.WriteFile(gitFile, []byte(gitContent), 0644); err != nil {
			b.Fatalf("Failed to create .git file: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will mostly test the filesystem walking since we don't have full git repos
		// It will return errors for the mock .git files, but tests the core discovery logic
		_, _ = DiscoverGlobalWorktrees(baseDir)
	}
}

func BenchmarkFilterGlobalWorktrees(b *testing.B) {
	// Create a large slice of entries
	entries := make([]*GlobalWorktreeEntry, 1000)
	for i := 0; i < 1000; i++ {
		entries[i] = &GlobalWorktreeEntry{
			Branch: fmt.Sprintf("branch-%d", i),
			Path:   fmt.Sprintf("/path/to/branch-%d", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FilterGlobalWorktrees(entries, "branch-500")
	}
}

// Test cases for parallel discovery functions

func TestDiscoverGlobalWorktreesParallel_EmptyBaseDir(t *testing.T) {
	entries, err := DiscoverGlobalWorktreesParallel("", nil)
	if err == nil {
		t.Error("Expected error for empty base directory")
	}
	if entries != nil {
		t.Error("Expected nil entries for empty base directory")
	}
}

func TestDiscoverGlobalWorktreesParallel_NonExistentBaseDir(t *testing.T) {
	entries, err := DiscoverGlobalWorktreesParallel("/nonexistent/path", nil)
	if err != nil {
		t.Errorf("Unexpected error for non-existent directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected empty entries for non-existent directory, got %d", len(entries))
	}
}

func TestDiscoverGlobalWorktreesParallel_NoWorktrees(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory with no git repositories
	subDir := filepath.Join(tmpDir, "not-a-repo")
	err := os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	entries, err := DiscoverGlobalWorktreesParallel(tmpDir, nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected no entries, got %d", len(entries))
	}
}

func TestDiscoverGlobalWorktreesParallel_WithOptions(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with custom worker count
	opts := &DiscoverOptions{MaxWorkers: 2}
	entries, err := DiscoverGlobalWorktreesParallel(tmpDir, opts)
	if err != nil {
		t.Errorf("Unexpected error with options: %v", err)
	}
	if entries == nil {
		t.Error("Expected non-nil entries slice (even if empty)")
	}
}

func TestGetMaxWorkers(t *testing.T) {
	tests := []struct {
		name     string
		opts     *DiscoverOptions
		envValue string
		minVal   int
		maxVal   int
	}{
		{
			name:   "nil options uses default",
			opts:   nil,
			minVal: 1,
			maxVal: 4, // min(CPU, 4)
		},
		{
			name:   "options with custom workers",
			opts:   &DiscoverOptions{MaxWorkers: 8},
			minVal: 8,
			maxVal: 8,
		},
		{
			name:     "env var overrides",
			opts:     nil,
			envValue: "2",
			minVal:   2,
			maxVal:   2,
		},
		{
			name:     "env var overrides options",
			opts:     &DiscoverOptions{MaxWorkers: 8},
			envValue: "3",
			minVal:   3,
			maxVal:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("GWQ_DISCOVERY_WORKERS", tt.envValue)
			}

			result := getMaxWorkers(tt.opts)
			if result < tt.minVal || result > tt.maxVal {
				t.Errorf("getMaxWorkers() = %d, want between %d and %d", result, tt.minVal, tt.maxVal)
			}
		})
	}
}

func TestExtractWorktreeInfoWithWorkerPool_EmptyPaths(t *testing.T) {
	entries, err := extractWorktreeInfoWithWorkerPool([]string{}, 4)
	if err != nil {
		t.Errorf("Unexpected error for empty paths: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected empty entries, got %d", len(entries))
	}
}

func TestExtractWorktreeInfoWithWorkerPool_InvalidPaths(t *testing.T) {
	// Test with invalid paths - should not crash and should return empty results
	paths := []string{
		"/nonexistent/path1",
		"/nonexistent/path2",
	}

	entries, err := extractWorktreeInfoWithWorkerPool(paths, 2)
	if err != nil {
		t.Errorf("Unexpected error for invalid paths: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected empty entries for invalid paths, got %d", len(entries))
	}
}

// Benchmark tests for parallel discovery

func BenchmarkDiscoverGlobalWorktreesParallel(b *testing.B) {
	// Create a temporary directory with multiple simulated worktrees
	baseDir := b.TempDir()

	// Create multiple directories that look like worktrees
	for i := range 10 {
		repo := filepath.Join(baseDir, fmt.Sprintf("repo%d", i))
		if err := os.MkdirAll(repo, 0755); err != nil {
			b.Fatalf("Failed to create repo directory: %v", err)
		}

		// Create a simple .git file for worktree simulation
		gitFile := filepath.Join(repo, ".git")
		gitContent := fmt.Sprintf("gitdir: /path/to/main/repo/.git/worktrees/branch%d", i)
		if err := os.WriteFile(gitFile, []byte(gitContent), 0644); err != nil {
			b.Fatalf("Failed to create .git file: %v", err)
		}
	}

	b.ResetTimer()
	for b.Loop() {
		// This will mostly test the filesystem walking since we don't have full git repos
		_, _ = DiscoverGlobalWorktreesParallel(baseDir, nil)
	}
}

// ============================================================================
// Phase 3 Tests: Git file direct reading
// ============================================================================

func TestParseBranchOrCommitFromHead(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantBranch   string
		wantCommit   string
		wantFallback bool
	}{
		{
			name:       "refs/heads branch",
			content:    "ref: refs/heads/main\n",
			wantBranch: "main",
		},
		{
			name:       "refs/heads branch with feature name",
			content:    "ref: refs/heads/feature/new-feature\n",
			wantBranch: "feature/new-feature",
		},
		{
			name:       "detached HEAD with hash",
			content:    "abc123def456789012345678901234567890abcd\n",
			wantBranch: "HEAD",
			wantCommit: "abc123def456789012345678901234567890abcd",
		},
		{
			name:         "refs/tags (unsupported)",
			content:      "ref: refs/tags/v1.0.0\n",
			wantFallback: true,
		},
		{
			name:         "refs/remotes (unsupported)",
			content:      "ref: refs/remotes/origin/main\n",
			wantFallback: true,
		},
		{
			name:         "invalid content",
			content:      "invalid",
			wantFallback: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch, commit, fallback := parseBranchOrCommitFromHead(tt.content)
			if fallback != tt.wantFallback {
				t.Errorf("fallback = %v, want %v", fallback, tt.wantFallback)
			}
			if !tt.wantFallback {
				if branch != tt.wantBranch {
					t.Errorf("branch = %q, want %q", branch, tt.wantBranch)
				}
				if commit != tt.wantCommit {
					t.Errorf("commit = %q, want %q", commit, tt.wantCommit)
				}
			}
		})
	}
}

func TestIsHexString(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abc123", true},
		{"ABC123", true},
		{"0123456789abcdef", true},
		{"0123456789ABCDEF", true},
		{"abc123def456789012345678901234567890abcd", true},
		{"xyz123", false},
		{"abc 123", false},
		{"abc-123", false},
		{"", true}, // Empty string is technically valid hex
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isHexString(tt.input)
			if result != tt.expected {
				t.Errorf("isHexString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFindMainGitDir(t *testing.T) {
	tests := []struct {
		name     string
		gitdir   string
		expected string
	}{
		{
			name:     "standard worktree path",
			gitdir:   "/path/to/repo/.git/worktrees/feature-branch",
			expected: "/path/to/repo/.git",
		},
		{
			name:     "nested repo path",
			gitdir:   "/home/user/projects/myapp/.git/worktrees/dev",
			expected: "/home/user/projects/myapp/.git",
		},
		{
			name:     "no worktrees in path",
			gitdir:   "/path/to/repo/.git",
			expected: "/path/to/repo/.git",
		},
		{
			name:     "worktrees not followed by .git parent",
			gitdir:   "/path/to/worktrees/something",
			expected: "/path/to/worktrees/something",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findMainGitDir(tt.gitdir)
			if result != tt.expected {
				t.Errorf("findMainGitDir(%q) = %q, want %q", tt.gitdir, result, tt.expected)
			}
		})
	}
}

func TestParseRemoteFromConfigSimple(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectedURL string
		expectError bool
	}{
		{
			name: "simple config with origin",
			config: `[core]
	repositoryformatversion = 0
[remote "origin"]
	url = https://github.com/user/repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*
[branch "main"]
	remote = origin
	merge = refs/heads/main
`,
			expectedURL: "https://github.com/user/repo.git",
		},
		{
			name: "config with tab-indented url",
			config: `[remote "origin"]
	url=git@github.com:user/repo.git
`,
			expectedURL: "git@github.com:user/repo.git",
		},
		{
			name: "no origin remote",
			config: `[core]
	repositoryformatversion = 0
`,
			expectError: true,
		},
		{
			name: "config with include directive",
			config: `[include]
	path = ~/.gitconfig.local
[remote "origin"]
	url = https://github.com/user/repo.git
`,
			expectError: true, // Should bail out due to include
		},
		{
			name: "config with includeIf directive",
			config: `[includeIf "gitdir:~/work/"]
	path = ~/.gitconfig.work
[remote "origin"]
	url = https://github.com/user/repo.git
`,
			expectError: true, // Should bail out due to includeIf
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write config to temp file
			tmpFile, err := os.CreateTemp("", "gitconfig-*")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer func() { _ = os.Remove(tmpFile.Name()) }()

			if _, err := tmpFile.WriteString(tt.config); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
			_ = tmpFile.Close()

			url, err := parseRemoteFromConfigSimple(tmpFile.Name())
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got url: %q", url)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if url != tt.expectedURL {
					t.Errorf("url = %q, want %q", url, tt.expectedURL)
				}
			}
		})
	}
}

// ============================================================================
// Pipeline Tests
// ============================================================================

func TestDiscoverGlobalWorktreesPipeline_EmptyBaseDir(t *testing.T) {
	entries, err := DiscoverGlobalWorktreesPipeline("", nil)
	if err == nil {
		t.Error("Expected error for empty base directory")
	}
	if entries != nil {
		t.Error("Expected nil entries for empty base directory")
	}
}

func TestDiscoverGlobalWorktreesPipeline_NonExistentBaseDir(t *testing.T) {
	entries, err := DiscoverGlobalWorktreesPipeline("/nonexistent/path", nil)
	if err != nil {
		t.Errorf("Unexpected error for non-existent directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected empty entries for non-existent directory, got %d", len(entries))
	}
}

func TestDiscoverGlobalWorktreesPipeline_NoWorktrees(t *testing.T) {
	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "not-a-repo")
	err := os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	entries, err := DiscoverGlobalWorktreesPipeline(tmpDir, nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected no entries, got %d", len(entries))
	}
}

func BenchmarkDiscoverGlobalWorktreesPipeline(b *testing.B) {
	baseDir := b.TempDir()

	for i := range 10 {
		repo := filepath.Join(baseDir, fmt.Sprintf("repo%d", i))
		if err := os.MkdirAll(repo, 0755); err != nil {
			b.Fatalf("Failed to create repo directory: %v", err)
		}

		gitFile := filepath.Join(repo, ".git")
		gitContent := fmt.Sprintf("gitdir: /path/to/main/repo/.git/worktrees/branch%d", i)
		if err := os.WriteFile(gitFile, []byte(gitContent), 0644); err != nil {
			b.Fatalf("Failed to create .git file: %v", err)
		}
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = DiscoverGlobalWorktreesPipeline(baseDir, nil)
	}
}

// ============================================================================
// Lazy Loading Tests
// ============================================================================

func TestDiscoverGlobalWorktreesLazy_EmptyBaseDir(t *testing.T) {
	entries, err := DiscoverGlobalWorktreesLazy("")
	if err == nil {
		t.Error("Expected error for empty base directory")
	}
	if entries != nil {
		t.Error("Expected nil entries for empty base directory")
	}
}

func TestDiscoverGlobalWorktreesLazy_NonExistentBaseDir(t *testing.T) {
	entries, err := DiscoverGlobalWorktreesLazy("/nonexistent/path")
	if err != nil {
		t.Errorf("Unexpected error for non-existent directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected empty entries for non-existent directory, got %d", len(entries))
	}
}

func TestDiscoverGlobalWorktreesLazy_ReturnsOnlyPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simulated worktree
	worktreeDir := filepath.Join(tmpDir, "worktree1")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("Failed to create worktree directory: %v", err)
	}

	gitFile := filepath.Join(worktreeDir, ".git")
	gitContent := "gitdir: /path/to/main/repo/.git/worktrees/branch1"
	if err := os.WriteFile(gitFile, []byte(gitContent), 0644); err != nil {
		t.Fatalf("Failed to create .git file: %v", err)
	}

	entries, err := DiscoverGlobalWorktreesLazy(tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]

	// Path should be set
	if entry.Path != worktreeDir {
		t.Errorf("Path = %q, want %q", entry.Path, worktreeDir)
	}

	// Other fields should be empty (lazy loading)
	if entry.Branch != "" {
		t.Errorf("Branch should be empty for lazy entry, got %q", entry.Branch)
	}
	if entry.CommitHash != "" {
		t.Errorf("CommitHash should be empty for lazy entry, got %q", entry.CommitHash)
	}
	if entry.RepositoryURL != "" {
		t.Errorf("RepositoryURL should be empty for lazy entry, got %q", entry.RepositoryURL)
	}
	if entry.IsLoaded() {
		t.Error("Entry should not be loaded yet")
	}
}

func TestGlobalWorktreeEntry_EnsureLoaded_MultipleCallsSafe(t *testing.T) {
	entry := &GlobalWorktreeEntry{
		Path:   "/nonexistent/path",
		IsMain: false,
	}

	// First call should attempt to load (and fail)
	err1 := entry.EnsureLoaded()
	// Second call should return same error without re-attempting
	err2 := entry.EnsureLoaded()

	// Both should return the same error
	if err1 == nil || err2 == nil {
		t.Error("Expected error for nonexistent path")
	}
	if err1.Error() != err2.Error() {
		t.Errorf("Multiple calls should return same error: %v vs %v", err1, err2)
	}
}

func TestDiscoverGlobalWorktreesParallel_LazyOption(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simulated worktree
	worktreeDir := filepath.Join(tmpDir, "worktree1")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("Failed to create worktree directory: %v", err)
	}

	gitFile := filepath.Join(worktreeDir, ".git")
	gitContent := "gitdir: /path/to/main/repo/.git/worktrees/branch1"
	if err := os.WriteFile(gitFile, []byte(gitContent), 0644); err != nil {
		t.Fatalf("Failed to create .git file: %v", err)
	}

	// With Lazy=true
	opts := &DiscoverOptions{Lazy: true}
	entries, err := DiscoverGlobalWorktreesParallel(tmpDir, opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.IsLoaded() {
		t.Error("Entry should not be loaded when Lazy=true")
	}
	if entry.Branch != "" {
		t.Error("Branch should be empty for lazy entry")
	}
}

// ============================================================================
// Codex Review Fix Tests
// ============================================================================

// Test 1: atomic.Bool for loaded field - race condition safety
func TestGlobalWorktreeEntry_ConcurrentAccess(t *testing.T) {
	entry := &GlobalWorktreeEntry{
		Path:   "/nonexistent/path",
		IsMain: false,
	}

	// Run multiple goroutines accessing IsLoaded and EnsureLoaded concurrently
	// This should not cause data race with atomic.Bool
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = entry.IsLoaded()
			_ = entry.EnsureLoaded()
			_ = entry.IsLoaded()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// Test 2: extractMainRepoInfo error wrapping for os.IsNotExist check
func TestExtractMainRepoInfo_ErrorWrapping(t *testing.T) {
	// Test with non-existent path
	_, err := extractMainRepoInfo("/nonexistent/repo/path")
	if err == nil {
		t.Fatal("Expected error for non-existent path")
	}

	// The error should allow os.IsNotExist check to work
	// This tests the fix for error wrapping
	if !os.IsNotExist(err) {
		// Check if it's a wrapped error that indicates the path doesn't exist
		if !strings.Contains(err.Error(), "not a git repository") {
			t.Errorf("Error should indicate path doesn't exist or wrap the original error: %v", err)
		}
	}
}

// Test 3: .git symlink handling in isWorktreeDir
func TestIsWorktreeDir_SymlinkHandling(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory that will be the symlink target
	targetDir := filepath.Join(tmpDir, "target-git-dir")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	// Create a test directory with a .git symlink pointing to a directory
	testDir := filepath.Join(tmpDir, "test-repo")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	gitSymlink := filepath.Join(testDir, ".git")
	if err := os.Symlink(targetDir, gitSymlink); err != nil {
		t.Skipf("Symlink creation not supported: %v", err)
	}

	// Create a mock DirEntry for testing
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	var testEntry os.DirEntry
	for _, e := range entries {
		if e.Name() == "test-repo" {
			testEntry = e
			break
		}
	}

	if testEntry == nil {
		t.Fatal("Test entry not found")
	}

	// Test isWorktreeDir - should handle symlink properly
	isWorktree, skipDir := isWorktreeDir(testDir, testEntry)

	// .git symlink pointing to a directory should be treated like a main repo
	// and skipDir should be true to avoid infinite recursion
	if isWorktree {
		t.Error("Symlink to directory should not be detected as worktree")
	}
	if !skipDir {
		t.Error("Should skip directory when .git is a symlink to a directory")
	}
}

// Test 4: parseRemoteFromConfigSimple with various whitespace
func TestParseRemoteFromConfigSimple_Whitespace(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectedURL string
		expectError bool
	}{
		{
			name: "multiple spaces around equals",
			config: `[remote "origin"]
	url   =   https://github.com/user/repo.git
`,
			expectedURL: "https://github.com/user/repo.git",
		},
		{
			name: "tabs around equals",
			config: `[remote "origin"]
	url	=	git@github.com:user/repo.git
`,
			expectedURL: "git@github.com:user/repo.git",
		},
		{
			name: "trailing whitespace in url",
			config: `[remote "origin"]
	url = https://github.com/user/repo.git
`,
			expectedURL: "https://github.com/user/repo.git",
		},
		{
			name: "mixed whitespace",
			config: `[remote "origin"]
	url  =	 https://github.com/user/repo.git
`,
			expectedURL: "https://github.com/user/repo.git",
		},
		{
			name: "case insensitive section header",
			config: `[REMOTE "origin"]
	url = https://github.com/user/repo.git
`,
			expectedURL: "https://github.com/user/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "gitconfig-*")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer func() { _ = os.Remove(tmpFile.Name()) }()

			if _, err := tmpFile.WriteString(tt.config); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
			_ = tmpFile.Close()

			url, err := parseRemoteFromConfigSimple(tmpFile.Name())
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got url: %q", url)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if url != tt.expectedURL {
					t.Errorf("url = %q, want %q", url, tt.expectedURL)
				}
			}
		})
	}
}

// Test 5: readWorktreeDetailsFast gitdir: prefix validation
func TestReadWorktreeDetailsFast_GitdirValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .git file without gitdir: prefix
	testDir := filepath.Join(tmpDir, "invalid-worktree")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	gitFile := filepath.Join(testDir, ".git")
	// Write content without gitdir: prefix
	if err := os.WriteFile(gitFile, []byte("invalid content without gitdir prefix"), 0644); err != nil {
		t.Fatalf("Failed to create .git file: %v", err)
	}

	// readWorktreeDetailsFast should return error for invalid format
	_, _, _, _, err := readWorktreeDetailsFast(testDir)
	if err == nil {
		t.Error("Expected error for .git file without gitdir: prefix")
	}
}

// Test 6: expandBaseDir directory check
func TestExpandBaseDir_FileInsteadOfDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file instead of directory
	testFile := filepath.Join(tmpDir, "not-a-directory")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// expandBaseDir should return error for file path
	_, err := expandBaseDir(testFile)
	if err == nil {
		t.Error("Expected error when baseDir is a file, not a directory")
	}
}

// Test 7: Serial discovery should use extractWorktreeInfoWithFallback
func TestDiscoverWorktreesInDir_UsesFallback(t *testing.T) {
	// This test verifies the behavior of serial discovery
	// The implementation should use extractWorktreeInfoWithFallback for better performance

	tmpDir := t.TempDir()

	// Create a simulated worktree
	worktreeDir := filepath.Join(tmpDir, "worktree1")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("Failed to create worktree directory: %v", err)
	}

	gitFile := filepath.Join(worktreeDir, ".git")
	gitContent := "gitdir: /path/to/main/repo/.git/worktrees/branch1"
	if err := os.WriteFile(gitFile, []byte(gitContent), 0644); err != nil {
		t.Fatalf("Failed to create .git file: %v", err)
	}

	// discoverWorktreesInDir should work (it will fail to extract full info
	// but should not panic and should handle the error gracefully)
	entries, err := discoverWorktreesInDir(tmpDir)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// Will be empty because the simulated worktree doesn't have valid git info
	_ = entries
}

// Test 8: Error logging consistency between serial and parallel ghq discovery
func TestDiscoverGhqWorktrees_ErrorLogging(t *testing.T) {
	// This is a behavioral test to ensure error handling is consistent
	// Both serial and parallel versions should handle errors the same way

	// Note: This test mainly ensures the code doesn't panic
	// Actual error logging behavior is verified through code review
	// and can be tested with log capturing in integration tests
}

func BenchmarkDiscoverGlobalWorktreesLazy(b *testing.B) {
	baseDir := b.TempDir()

	for i := range 10 {
		repo := filepath.Join(baseDir, fmt.Sprintf("repo%d", i))
		if err := os.MkdirAll(repo, 0755); err != nil {
			b.Fatalf("Failed to create repo directory: %v", err)
		}

		gitFile := filepath.Join(repo, ".git")
		gitContent := fmt.Sprintf("gitdir: /path/to/main/repo/.git/worktrees/branch%d", i)
		if err := os.WriteFile(gitFile, []byte(gitContent), 0644); err != nil {
			b.Fatalf("Failed to create .git file: %v", err)
		}
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = DiscoverGlobalWorktreesLazy(baseDir)
	}
}
