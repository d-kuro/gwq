package worktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/d-kuro/gwq/pkg/models"
)

type testExecutor struct {
	calls   []string
	outputs []string
	errs    []error
}

func (e *testExecutor) ExecuteInDirWithOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	e.calls = append(e.calls, filepath.Join(dir, name+" "+strings.Join(args, " ")))
	if len(e.outputs) > 0 {
		out := e.outputs[0]
		e.outputs = e.outputs[1:]
		var err error
		if len(e.errs) > 0 {
			err = e.errs[0]
			e.errs = e.errs[1:]
		}
		return out, err
	}
	return "", nil
}

func TestManagerAdd_Integration(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()
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
	defer os.Chdir(oldwd)

	mockG := &mockGit{}
	m := New(mockG, cfg)

	err = m.Add("feature/test", filepath.Join(worktreeDir, "wt1"), false)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	// Check file was copied
	copied := filepath.Join(worktreeDir, "wt1", "copyme.txt")
	if _, err := os.Stat(copied); err != nil {
		t.Errorf("expected file to be copied: %v", err)
	}
	// (Optional) Check for setup command output/logs if needed
}
