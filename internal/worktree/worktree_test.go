package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/d-kuro/gwq/pkg/models"
)

// mockGit is a mock implementation of git operations for testing
type mockGit struct {
	worktrees         []models.Worktree
	repoName          string
	addError          error
	removeError       error
	listError         error
	pruneError        error
	deleteBranchError error
	recentCommits     []models.CommitInfo
}

func (m *mockGit) ListWorktrees() ([]models.Worktree, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	return m.worktrees, nil
}

func (m *mockGit) AddWorktree(path, branch string, createBranch bool) error {
	if m.addError != nil {
		return m.addError
	}
	m.worktrees = append(m.worktrees, models.Worktree{
		Path:   path,
		Branch: branch,
	})
	return nil
}

func (m *mockGit) RemoveWorktree(path string, force bool) error {
	if m.removeError != nil {
		return m.removeError
	}
	var updated []models.Worktree
	for _, wt := range m.worktrees {
		if wt.Path != path {
			updated = append(updated, wt)
		}
	}
	m.worktrees = updated
	return nil
}

func (m *mockGit) PruneWorktrees() error {
	return m.pruneError
}

func (m *mockGit) GetRepositoryName() (string, error) {
	if m.repoName == "" {
		return "test-repo", nil
	}
	return m.repoName, nil
}

func (m *mockGit) GetRecentCommits(path string, limit int) ([]models.CommitInfo, error) {
	return m.recentCommits, nil
}

func (m *mockGit) GetRepositoryURL() (string, error) {
	return "https://github.com/test-user/test-repo.git", nil
}

func (m *mockGit) DeleteBranch(branch string, force bool) error {
	if m.deleteBranchError != nil {
		return m.deleteBranchError
	}
	return nil
}

func (m *mockGit) AddWorktreeFromBase(path, branch, baseBranch string) error {
	if m.addError != nil {
		return m.addError
	}
	m.worktrees = append(m.worktrees, models.Worktree{
		Path:   path,
		Branch: branch,
	})
	return nil
}

func TestManagerAdd(t *testing.T) {
	tests := []struct {
		name         string
		branch       string
		customPath   string
		createBranch bool
		config       *models.Config
		wantErr      bool
		errContains  string
	}{
		{
			name:   "WithGeneratedPath",
			branch: "feature/test",
			config: &models.Config{
				Worktree: models.WorktreeConfig{
					BaseDir:   t.TempDir(),
					AutoMkdir: true,
				},
			},
			wantErr: false,
		},
		{
			name:       "WithCustomPath",
			branch:     "feature/test",
			customPath: filepath.Join(t.TempDir(), "custom-worktree"),
			config: &models.Config{
				Worktree: models.WorktreeConfig{
					BaseDir:   t.TempDir(),
					AutoMkdir: true,
				},
			},
			wantErr: false,
		},
		{
			name:         "CreateNewBranch",
			branch:       "feature/new",
			createBranch: true,
			config: &models.Config{
				Worktree: models.WorktreeConfig{
					BaseDir:   t.TempDir(),
					AutoMkdir: true,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockG := &mockGit{}
			m := New(mockG, tt.config)

			_, err := m.Add(tt.branch, tt.customPath, tt.createBranch)
			if (err != nil) != tt.wantErr {
				t.Errorf("Add() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Add() error = %v, want error containing %s", err, tt.errContains)
			}

			if !tt.wantErr {
				// Verify worktree was added
				if len(mockG.worktrees) != 1 {
					t.Errorf("Expected 1 worktree, got %d", len(mockG.worktrees))
				}
			}
		})
	}
}

func TestManagerRemove(t *testing.T) {
	mockG := &mockGit{
		worktrees: []models.Worktree{
			{Path: "/path/to/worktree1", Branch: "feature1"},
			{Path: "/path/to/worktree2", Branch: "feature2"},
		},
	}

	m := New(mockG, &models.Config{})

	// Remove worktree
	err := m.Remove("/path/to/worktree1", false)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Verify worktree was removed
	if len(mockG.worktrees) != 1 {
		t.Errorf("Expected 1 worktree after removal, got %d", len(mockG.worktrees))
	}

	if mockG.worktrees[0].Path != "/path/to/worktree2" {
		t.Errorf("Wrong worktree remained: %s", mockG.worktrees[0].Path)
	}
}

func TestManagerList(t *testing.T) {
	expectedWorktrees := []models.Worktree{
		{Path: "/path/1", Branch: "main", IsMain: true},
		{Path: "/path/2", Branch: "feature"},
	}

	mockG := &mockGit{
		worktrees: expectedWorktrees,
	}

	m := New(mockG, &models.Config{})

	worktrees, err := m.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(worktrees) != len(expectedWorktrees) {
		t.Errorf("List() returned %d worktrees, want %d", len(worktrees), len(expectedWorktrees))
	}
}

func TestManagerPrune(t *testing.T) {
	mockG := &mockGit{}
	m := New(mockG, &models.Config{})

	err := m.Prune()
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
}

func TestManagerGetWorktreePath(t *testing.T) {
	mockG := &mockGit{
		worktrees: []models.Worktree{
			{Path: "/path/to/feature-test", Branch: "feature/test"},
			{Path: "/path/to/main", Branch: "main"},
			{Path: "/path/to/bugfix", Branch: "bugfix/issue-123"},
		},
	}

	m := New(mockG, &models.Config{})

	tests := []struct {
		name     string
		pattern  string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "MatchBranch",
			pattern:  "feature",
			wantPath: "/path/to/feature-test",
		},
		{
			name:     "MatchPath",
			pattern:  "bugfix",
			wantPath: "/path/to/bugfix",
		},
		{
			name:    "NoMatch",
			pattern: "nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := m.GetWorktreePath(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetWorktreePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && path != tt.wantPath {
				t.Errorf("GetWorktreePath() = %s, want %s", path, tt.wantPath)
			}
		})
	}
}

func TestManagerGetMatchingWorktrees(t *testing.T) {
	mockG := &mockGit{
		worktrees: []models.Worktree{
			{Path: "/path/to/feature-test", Branch: "feature/test"},
			{Path: "/path/to/main", Branch: "main"},
			{Path: "/path/to/bugfix", Branch: "bugfix/issue-123"},
			{Path: "/path/to/feature-auth", Branch: "feature/auth"},
			{Path: "/path/to/feature-api", Branch: "feature/api"},
		},
	}

	m := New(mockG, &models.Config{})

	tests := []struct {
		name         string
		pattern      string
		wantCount    int
		wantBranches []string
	}{
		{
			name:         "MatchMultiple",
			pattern:      "feature",
			wantCount:    3,
			wantBranches: []string{"feature/test", "feature/auth", "feature/api"},
		},
		{
			name:         "MatchSingle",
			pattern:      "main",
			wantCount:    1,
			wantBranches: []string{"main"},
		},
		{
			name:         "MatchPath",
			pattern:      "bugfix",
			wantCount:    1,
			wantBranches: []string{"bugfix/issue-123"},
		},
		{
			name:         "NoMatch",
			pattern:      "nonexistent",
			wantCount:    0,
			wantBranches: []string{},
		},
		{
			name:         "CaseInsensitive",
			pattern:      "FEATURE",
			wantCount:    3,
			wantBranches: []string{"feature/test", "feature/auth", "feature/api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := m.GetMatchingWorktrees(tt.pattern)
			if err != nil {
				t.Errorf("GetMatchingWorktrees() unexpected error = %v", err)
				return
			}

			if len(matches) != tt.wantCount {
				t.Errorf("GetMatchingWorktrees() returned %d matches, want %d", len(matches), tt.wantCount)
			}

			// Check that all expected branches are found
			foundBranches := make(map[string]bool)
			for _, wt := range matches {
				foundBranches[wt.Branch] = true
			}

			for _, expectedBranch := range tt.wantBranches {
				if !foundBranches[expectedBranch] {
					t.Errorf("Expected branch %s not found in matches", expectedBranch)
				}
			}
		})
	}
}

func TestManagerValidateWorktreePath(t *testing.T) {
	tests := []struct {
		name      string
		setupPath func() string
		wantErr   bool
		errMsg    string
	}{
		{
			name: "NonExistentPath",
			setupPath: func() string {
				return filepath.Join(t.TempDir(), "nonexistent")
			},
			wantErr: false,
		},
		{
			name: "EmptyDirectory",
			setupPath: func() string {
				dir := filepath.Join(t.TempDir(), "empty")
				_ = os.MkdirAll(dir, 0755)
				return dir
			},
			wantErr: false,
		},
		{
			name: "NonEmptyDirectory",
			setupPath: func() string {
				dir := filepath.Join(t.TempDir(), "nonempty")
				_ = os.MkdirAll(dir, 0755)
				_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
				return dir
			},
			wantErr: true,
			errMsg:  "directory is not empty",
		},
		{
			name: "ExistingFile",
			setupPath: func() string {
				dir := t.TempDir()
				file := filepath.Join(dir, "file")
				_ = os.WriteFile(file, []byte("content"), 0644)
				return file
			},
			wantErr: true,
			errMsg:  "is not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(nil, &models.Config{})
			path := tt.setupPath()

			err := m.ValidateWorktreePath(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorktreePath() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateWorktreePath() error = %v, want error containing %s", err, tt.errMsg)
			}
		})
	}
}

func TestGenerateWorktreePath(t *testing.T) {
	tests := []struct {
		name       string
		branch     string
		repoName   string
		wantSuffix string
	}{
		{
			name:       "BasicTemplate",
			branch:     "feature/test",
			repoName:   "myrepo",
			wantSuffix: "github.com/test-user/test-repo/feature-test",
		},
		{
			name:       "BranchOnly",
			branch:     "main",
			repoName:   "myrepo",
			wantSuffix: "github.com/test-user/test-repo/main",
		},
		{
			name:       "ComplexSanitization",
			branch:     "feature/test:new",
			repoName:   "myrepo",
			wantSuffix: "github.com/test-user/test-repo/feature-test-new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockG := &mockGit{repoName: tt.repoName}

			config := &models.Config{
				Worktree: models.WorktreeConfig{
					BaseDir: "/base",
				},
			}

			m := New(mockG, config)

			path, err := m.generateWorktreePath(tt.branch)
			if err != nil {
				t.Fatalf("generateWorktreePath() error = %v", err)
			}

			expectedPath := filepath.Join("/base", tt.wantSuffix)
			if path != expectedPath {
				t.Errorf("generateWorktreePath() = %s, want %s", path, expectedPath)
			}
		})
	}
}

func TestManagerAdd_ConfigurableSetupIntegration(t *testing.T) {
	repoDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks: %v", err)
	}
	worktreeDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks: %v", err)
	}

	srcFile := filepath.Join(repoDir, "copyme.txt")
	if err := os.WriteFile(srcFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write src file: %v", err)
	}

	cfg := &models.Config{
		Worktree: models.WorktreeConfig{
			BaseDir:   worktreeDir,
			AutoMkdir: true,
		},
		RepositorySettings: []models.RepositorySetting{
			{
				Repository:    repoDir,
				CopyFiles:     []string{"copyme.txt"},
				SetupCommands: []string{"echo test"},
			},
		},
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current wd: %v", err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	mockG := &mockGit{}
	m := New(mockG, cfg)

	_, err = m.Add("feature/test", filepath.Join(worktreeDir, "wt1"), false)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	copied := filepath.Join(worktreeDir, "wt1", "copyme.txt")
	if _, err := os.Stat(copied); err != nil {
		t.Errorf("expected file to be copied: %v", err)
	}
}
