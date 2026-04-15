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

// initRepoAt creates and initializes a git repository at the given directory
// with an initial commit and a remote.
func initRepoAt(t *testing.T, dir, remoteURL string) *TestRepository {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}
	repo := &TestRepository{Path: dir}
	if err := repo.run("init", "-b", "main"); err != nil {
		t.Fatalf("Failed to init: %v", err)
	}
	if err := repo.run("config", "user.name", "Test"); err != nil {
		t.Fatalf("Failed to set user.name: %v", err)
	}
	if err := repo.run("config", "user.email", "test@test.com"); err != nil {
		t.Fatalf("Failed to set user.email: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := repo.run("add", "."); err != nil {
		t.Fatalf("Failed to add: %v", err)
	}
	if err := repo.run("commit", "-m", "init"); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	repo.AddRemote(t, "origin", remoteURL)
	return repo
}

func TestDiscoverGlobalWorktrees_IncludesMainWorktree(t *testing.T) {
	baseDir := t.TempDir()

	repoDir := filepath.Join(baseDir, "github.com", "user", "repo", "main")
	initRepoAt(t, repoDir, "https://github.com/user/repo.git")

	entries, err := DiscoverGlobalWorktrees(baseDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if !entries[0].IsMain {
		t.Error("Expected entry to be marked as main worktree")
	}
	if entries[0].Branch != "main" {
		t.Errorf("Expected branch 'main', got '%s'", entries[0].Branch)
	}
}

func TestDiscoverGlobalWorktrees_MainAndLinkedWorktrees(t *testing.T) {
	baseDir := t.TempDir()

	repoDir := filepath.Join(baseDir, "github.com", "user", "repo", "main")
	repo := initRepoAt(t, repoDir, "https://github.com/user/repo.git")

	// Create a branch and linked worktree
	repo.CreateBranch(t, "feature")
	if err := repo.run("checkout", "main"); err != nil {
		t.Fatalf("Failed to checkout main: %v", err)
	}
	worktreeDir := filepath.Join(baseDir, "github.com", "user", "repo", "feature")
	repo.CreateWorktree(t, worktreeDir, "feature")

	entries, err := DiscoverGlobalWorktrees(baseDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	var mainCount, linkedCount int
	for _, e := range entries {
		if e.IsMain {
			mainCount++
		} else {
			linkedCount++
		}
	}
	if mainCount != 1 {
		t.Errorf("Expected 1 main worktree, got %d", mainCount)
	}
	if linkedCount != 1 {
		t.Errorf("Expected 1 linked worktree, got %d", linkedCount)
	}
}

func TestDiscoverGlobalWorktrees_DoesNotDescendIntoMainRepo(t *testing.T) {
	baseDir := t.TempDir()

	repoDir := filepath.Join(baseDir, "repo")
	initRepoAt(t, repoDir, "https://github.com/user/repo.git")

	// SkipDir on the main repo means nothing inside it (submodules, nested
	// repos, etc.) is ever visited. Verify only the main worktree is found.
	entries, err := DiscoverGlobalWorktrees(baseDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry (main only), got %d", len(entries))
	}
	if !entries[0].IsMain {
		t.Error("Expected entry to be marked as main worktree")
	}
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

	expected := "testrepo:feature"
	if worktrees[0].Branch != expected {
		t.Errorf("Expected branch '%s', got '%s'", expected, worktrees[0].Branch)
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

func TestIsSubmoduleGitDir(t *testing.T) {
	tests := []struct {
		name     string
		gitDir   string
		expected bool
	}{
		{
			name:     "worktree gitdir",
			gitDir:   "/path/to/repo/.git/worktrees/feature-branch",
			expected: false,
		},
		{
			name:     "relative worktree gitdir",
			gitDir:   "../../.git/worktrees/feature-branch",
			expected: false,
		},
		{
			name:     "submodule in main worktree",
			gitDir:   "/path/to/repo/.git/modules/my-submodule",
			expected: true,
		},
		{
			name:     "relative submodule in main worktree",
			gitDir:   "../../.git/modules/my-submodule",
			expected: true,
		},
		{
			name:     "submodule in linked worktree",
			gitDir:   "../../../repo/.git/worktrees/feature/modules/cm/lwip",
			expected: true,
		},
		{
			name:     "nested submodule in linked worktree",
			gitDir:   "../../../repo/.git/worktrees/feature/modules/third_party/xgrammar/xgrammar/modules/3rdparty/googletest",
			expected: true,
		},
		{
			name:     "nested submodule gitdir",
			gitDir:   "/path/to/repo/.git/modules/outer/modules/inner",
			expected: true,
		},
		{
			name:     "windows submodule gitdir",
			gitDir:   "C:\\repo\\.git\\modules\\my-submodule",
			expected: filepath.Separator == '\\', // only matches on Windows where ToSlash converts
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSubmoduleGitDir(tt.gitDir)
			if result != tt.expected {
				t.Errorf("isSubmoduleGitDir(%q) = %v, want %v", tt.gitDir, result, tt.expected)
			}
		})
	}
}

func TestDiscoverGlobalWorktrees_SkipsSubmodules(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory with a submodule-style .git file
	submoduleDir := filepath.Join(tmpDir, "my-submodule")
	if err := os.MkdirAll(submoduleDir, 0755); err != nil {
		t.Fatalf("Failed to create submodule directory: %v", err)
	}

	gitContent := "gitdir: /path/to/repo/.git/modules/my-submodule"
	if err := os.WriteFile(filepath.Join(submoduleDir, ".git"), []byte(gitContent), 0644); err != nil {
		t.Fatalf("Failed to create .git file: %v", err)
	}

	entries, err := DiscoverGlobalWorktrees(tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected no entries (submodule should be skipped), got %d", len(entries))
	}
}

// Benchmark tests
func BenchmarkDiscoverGlobalWorktrees(b *testing.B) {
	// Create a temporary directory with multiple worktrees
	baseDir := b.TempDir()

	// Create multiple repositories and worktrees
	for i := range 10 {
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
	for i := range 1000 {
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
