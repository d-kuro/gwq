package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/d-kuro/gwq/pkg/models"
)

func TestManagerAdd_Integration(t *testing.T) {
	repoDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks: %v", err)
	}
	worktreeDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to eval symlinks: %v", err)
	}
	// Create a file to be copied
	srcFile := filepath.Join(repoDir, "copyme.txt")
	if err := os.WriteFile(srcFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write src file: %v", err)
	}

	// Set up config with repository_settings for repoDir
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

	// Change working directory to repoDir for the test
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
