package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/d-kuro/gwq/pkg/models"
)

// TestRepository creates a test git repository
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
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, output)
	}
	return nil
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
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = r.Path
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func TestNew(t *testing.T) {
	repo := NewTestRepository(t)
	g := New(repo.Path)

	if g.workDir != repo.Path {
		t.Errorf("New() workDir = %s, want %s", g.workDir, repo.Path)
	}
}

func TestNewFromCwd(t *testing.T) {
	repo := NewTestRepository(t)

	// Change to test repository directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := os.Chdir(repo.Path); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	g, err := NewFromCwd()
	if err != nil {
		t.Fatalf("NewFromCwd() error = %v", err)
	}

	// macOS may use /private/var symlinks, so resolve paths before comparing
	resolvedWorkDir, _ := filepath.EvalSymlinks(g.workDir)
	resolvedRepoPath, _ := filepath.EvalSymlinks(repo.Path)

	if resolvedWorkDir != resolvedRepoPath {
		t.Errorf("NewFromCwd() workDir = %s, want %s", resolvedWorkDir, resolvedRepoPath)
	}
}

func TestListWorktrees(t *testing.T) {
	repo := NewTestRepository(t)
	g := New(repo.Path)

	// Create test branches and worktrees
	repo.CreateBranch(t, "feature/test1")
	worktree1Path := filepath.Join(t.TempDir(), "worktree1")
	repo.CreateWorktree(t, worktree1Path, "feature/test1")

	// Switch back to main branch
	if err := repo.run("checkout", "main"); err != nil {
		t.Fatalf("Failed to checkout main: %v", err)
	}

	repo.CreateBranch(t, "feature/test2")
	worktree2Path := filepath.Join(t.TempDir(), "worktree2")
	repo.CreateWorktree(t, worktree2Path, "feature/test2")

	// List worktrees
	worktrees, err := g.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}

	// Should have 3 worktrees (main + 2 additional)
	if len(worktrees) != 3 {
		t.Errorf("ListWorktrees() returned %d worktrees, want 3", len(worktrees))
	}

	// Verify main worktree
	foundMain := false
	for _, wt := range worktrees {
		if wt.IsMain {
			foundMain = true
			// Compare resolved paths
			resolvedWtPath, _ := filepath.EvalSymlinks(wt.Path)
			resolvedRepoPath, _ := filepath.EvalSymlinks(repo.Path)
			if resolvedWtPath != resolvedRepoPath {
				t.Errorf("Main worktree path = %s, want %s", resolvedWtPath, resolvedRepoPath)
			}
		}
	}
	if !foundMain {
		t.Error("Main worktree not found")
	}

	// Verify additional worktrees
	if !containsWorktreeWithPath(worktrees, worktree1Path) {
		t.Errorf("Worktree 1 not found at path %s", worktree1Path)
	}
	if !containsWorktreeWithPath(worktrees, worktree2Path) {
		t.Errorf("Worktree 2 not found at path %s", worktree2Path)
	}
}

func TestAddWorktree(t *testing.T) {
	repo := NewTestRepository(t)
	g := New(repo.Path)

	t.Run("ExistingBranch", func(t *testing.T) {
		// Create a branch first
		repo.CreateBranch(t, "existing-branch")
		if err := repo.run("checkout", "main"); err != nil {
			t.Fatalf("Failed to checkout main: %v", err)
		}

		// Add worktree for existing branch
		worktreePath := filepath.Join(t.TempDir(), "existing-wt")
		err := g.AddWorktree(worktreePath, "existing-branch", false)
		if err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		// Verify worktree was created
		if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
			t.Error("Worktree directory was not created")
		}
	})

	t.Run("NewBranch", func(t *testing.T) {
		// Add worktree with new branch
		worktreePath := filepath.Join(t.TempDir(), "new-wt")
		err := g.AddWorktree(worktreePath, "new-branch", true)
		if err != nil {
			t.Fatalf("AddWorktree() with new branch error = %v", err)
		}

		// Verify worktree was created
		if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
			t.Error("Worktree directory was not created")
		}

		// Verify branch exists
		worktrees, err := g.ListWorktrees()
		if err != nil {
			t.Fatalf("ListWorktrees() error = %v", err)
		}

		found := false
		for _, wt := range worktrees {
			// Compare resolved paths
			resolvedWtPath, _ := filepath.EvalSymlinks(wt.Path)
			resolvedWorktreePath, _ := filepath.EvalSymlinks(worktreePath)

			if resolvedWtPath == resolvedWorktreePath {
				found = true
				if wt.Branch != "new-branch" {
					t.Errorf("Worktree branch = %s, want new-branch", wt.Branch)
				}
				break
			}
		}
		if !found {
			t.Error("New branch worktree not found")
		}
	})
}

func TestRemoveWorktree(t *testing.T) {
	repo := NewTestRepository(t)
	g := New(repo.Path)

	// Create a worktree to remove
	repo.CreateBranch(t, "to-remove")
	worktreePath := filepath.Join(t.TempDir(), "remove-wt")
	repo.CreateWorktree(t, worktreePath, "to-remove")

	// Remove the worktree
	err := g.RemoveWorktree(worktreePath, false)
	if err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}

	// Verify worktree is removed from list
	worktrees, _ := g.ListWorktrees()
	for _, wt := range worktrees {
		if wt.Path == worktreePath {
			t.Error("Worktree still exists in list after removal")
		}
	}
}

func TestPruneWorktrees(t *testing.T) {
	repo := NewTestRepository(t)
	g := New(repo.Path)

	// Create a worktree
	repo.CreateBranch(t, "to-prune")
	worktreePath := filepath.Join(t.TempDir(), "prune-wt")
	repo.CreateWorktree(t, worktreePath, "to-prune")

	// Manually remove the worktree directory
	if err := os.RemoveAll(worktreePath); err != nil {
		t.Fatalf("Failed to remove worktree directory: %v", err)
	}

	// Prune worktrees
	err := g.PruneWorktrees()
	if err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}

	// Verify worktree is pruned
	worktrees, _ := g.ListWorktrees()
	for _, wt := range worktrees {
		if wt.Path == worktreePath {
			t.Error("Deleted worktree still exists after prune")
		}
	}
}

func TestListBranches(t *testing.T) {
	repo := NewTestRepository(t)
	g := New(repo.Path)

	// Create test branches
	branches := []string{"feature/test", "bugfix/issue-123", "release/v1.0"}
	for _, branch := range branches {
		repo.CreateBranch(t, branch)

		// Add a commit to each branch
		testFile := filepath.Join(repo.Path, fmt.Sprintf("%s.txt", strings.ReplaceAll(branch, "/", "-")))
		if err := os.WriteFile(testFile, []byte(branch), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if err := repo.run("add", "."); err != nil {
			t.Fatalf("Failed to add files: %v", err)
		}
		if err := repo.run("commit", "-m", fmt.Sprintf("Commit for %s", branch)); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}
	}

	// Test without remote branches
	t.Run("LocalOnly", func(t *testing.T) {
		branchList, err := g.ListBranches(false)
		if err != nil {
			t.Fatalf("ListBranches(false) error = %v", err)
		}

		// Should have main + 3 created branches
		if len(branchList) < 4 {
			t.Errorf("ListBranches(false) returned %d branches, want at least 4", len(branchList))
		}

		// Verify branch properties
		foundCurrent := false
		for _, b := range branchList {
			if b.IsCurrent {
				foundCurrent = true
			}
			if b.IsRemote {
				t.Error("Found remote branch when includeRemote=false")
			}

			// Verify commit info
			if b.LastCommit.Hash == "" {
				t.Errorf("Branch %s has empty commit hash", b.Name)
			}
			if b.LastCommit.Message == "" {
				t.Errorf("Branch %s has empty commit message", b.Name)
			}
			if b.LastCommit.Author == "" {
				t.Errorf("Branch %s has empty commit author", b.Name)
			}
			if b.LastCommit.Date.IsZero() {
				t.Errorf("Branch %s has zero commit date", b.Name)
			}
		}

		if !foundCurrent {
			t.Error("No current branch found")
		}
	})
}

func TestGetRepositoryName(t *testing.T) {
	repo := NewTestRepository(t)
	g := New(repo.Path)

	name, err := g.GetRepositoryName()
	if err != nil {
		t.Fatalf("GetRepositoryName() error = %v", err)
	}

	// Repository name should be the base of the temp directory
	expectedName := filepath.Base(repo.Path)
	if name != expectedName {
		t.Errorf("GetRepositoryName() = %s, want %s", name, expectedName)
	}
}

func TestGetRecentCommits(t *testing.T) {
	repo := NewTestRepository(t)
	g := New(repo.Path)

	// Create multiple commits
	expectedMessages := []string{
		"Third commit",
		"Second commit",
		"First additional commit",
	}

	for i := len(expectedMessages) - 1; i >= 0; i-- {
		testFile := filepath.Join(repo.Path, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(testFile, fmt.Appendf(nil, "Content %d", i), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if err := repo.run("add", "."); err != nil {
			t.Fatalf("Failed to add files: %v", err)
		}
		if err := repo.run("commit", "-m", expectedMessages[i]); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Get recent commits
	commits, err := g.GetRecentCommits(repo.Path, 3)
	if err != nil {
		t.Fatalf("GetRecentCommits() error = %v", err)
	}

	if len(commits) != 3 {
		t.Errorf("GetRecentCommits() returned %d commits, want 3", len(commits))
	}

	// Verify commit messages (should be in reverse chronological order)
	for i, commit := range commits {
		if commit.Message != expectedMessages[i] {
			t.Errorf("Commit[%d].Message = %s, want %s", i, commit.Message, expectedMessages[i])
		}
		if commit.Hash == "" {
			t.Errorf("Commit[%d] has empty hash", i)
		}
		if commit.Author != "Test User" {
			t.Errorf("Commit[%d].Author = %s, want Test User", i, commit.Author)
		}
		if commit.Date.IsZero() {
			t.Errorf("Commit[%d] has zero date", i)
		}
	}
}

func TestGetCurrentBranch(t *testing.T) {
	repo := NewTestRepository(t)
	g := New(repo.Path)

	// Test on main branch
	branch := g.getCurrentBranch(repo.Path)
	if branch != "main" && branch != "master" {
		t.Errorf("getCurrentBranch() = %s, want main or master", branch)
	}

	// Create and checkout a new branch
	repo.CreateBranch(t, "test-branch")

	branch = g.getCurrentBranch(repo.Path)
	if branch != "test-branch" {
		t.Errorf("getCurrentBranch() after checkout = %s, want test-branch", branch)
	}
}

func TestGetRootDir(t *testing.T) {
	repo := NewTestRepository(t)

	// Create a subdirectory
	subDir := filepath.Join(repo.Path, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Test from subdirectory
	g := New(subDir)
	rootDir, err := g.getRootDir()
	if err != nil {
		t.Fatalf("getRootDir() error = %v", err)
	}

	// macOS may use /private/var symlinks, so resolve paths before comparing
	resolvedRootDir, _ := filepath.EvalSymlinks(rootDir)
	resolvedRepoPath, _ := filepath.EvalSymlinks(repo.Path)

	if resolvedRootDir != resolvedRepoPath {
		t.Errorf("getRootDir() = %s, want %s", resolvedRootDir, resolvedRepoPath)
	}
}

func TestRunCommand(t *testing.T) {
	repo := NewTestRepository(t)
	g := New(repo.Path)

	// Test successful command
	output, err := g.run("status", "--short")
	if err != nil {
		t.Fatalf("run('status --short') error = %v", err)
	}

	// Output should be empty for clean repository
	if strings.TrimSpace(output) != "" {
		t.Errorf("run('status --short') output = %s, want empty", output)
	}

	// Test failed command
	_, err = g.run("invalid-command")
	if err == nil {
		t.Error("run('invalid-command') should return error")
	}
}

// Helper function to compare worktrees with path resolution
func containsWorktreeWithPath(worktrees []models.Worktree, path string) bool {
	resolvedPath, _ := filepath.EvalSymlinks(path)
	for _, wt := range worktrees {
		resolvedWtPath, _ := filepath.EvalSymlinks(wt.Path)
		if resolvedWtPath == resolvedPath {
			return true
		}
	}
	return false
}
