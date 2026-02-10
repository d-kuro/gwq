package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/d-kuro/gwq/internal/ui"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/cobra"
)

func TestRunExec_GlobalPersistentGhqFlagEnablesGhqDiscovery(t *testing.T) {
	repoPath := setupGhqTestRepository(t)
	setupFakeGhqCommand(t, filepath.Dir(filepath.Dir(filepath.Dir(repoPath))), repoPath)

	execCommand := &cobra.Command{
		Use:                "exec [pattern] -- command [args...]",
		DisableFlagParsing: true,
		RunE:               runExec,
	}
	root := &cobra.Command{Use: "gwq"}
	root.PersistentFlags().Bool("ghq", false, "Enable ghq mode")
	root.AddCommand(execCommand)

	root.SetArgs([]string{"--ghq", "exec", "-g", "testrepo:feature-test", "--", "true"})
	if err := root.Execute(); err != nil {
		t.Fatalf("expected exec to succeed with global --ghq flag, got error: %v", err)
	}
}

func TestRemoveGlobalWorktree_UsesGhqDiscoveryWhenEnabled(t *testing.T) {
	repoPath := setupGhqTestRepository(t)
	setupFakeGhqCommand(t, filepath.Dir(filepath.Dir(filepath.Dir(repoPath))), repoPath)

	restoreRemoveDryRun := removeDryRun
	restoreDeleteBranch := deleteBranch
	restoreRemoveForce := removeForce
	restoreForceDeleteBranch := forceDeleteBranch
	removeDryRun = true
	deleteBranch = false
	removeForce = false
	forceDeleteBranch = false
	t.Cleanup(func() {
		removeDryRun = restoreRemoveDryRun
		deleteBranch = restoreDeleteBranch
		removeForce = restoreRemoveForce
		forceDeleteBranch = restoreForceDeleteBranch
	})

	ctx := &CommandContext{
		Config: &models.Config{
			Worktree: models.WorktreeConfig{
				BaseDir: "",
			},
			Ghq: models.GhqConfig{
				Enabled:      true,
				WorktreesDir: ".worktrees",
			},
			Finder: models.FinderConfig{
				Preview: false,
			},
			UI: models.UIConfig{
				Icons: false,
			},
		},
		Printer: ui.New(&models.UIConfig{}),
	}

	if err := removeGlobalWorktree(ctx, []string{"testrepo:feature-test"}); err != nil {
		t.Fatalf("expected remove -g to discover ghq worktree when ghq is enabled, got error: %v", err)
	}
}

func setupGhqTestRepository(t *testing.T) string {
	t.Helper()

	t.Setenv("GIT_AUTHOR_NAME", "Test User")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test User")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	tmpDir := t.TempDir()
	ghqRoot := filepath.Join(tmpDir, "ghq")
	repoPath := filepath.Join(ghqRoot, "github.com", "testuser", "testrepo")
	worktreePath := filepath.Join(repoPath, ".worktrees", "feature-test")

	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create repo directory: %v", err)
	}

	runGit(t, repoPath, "init", "-b", "main")
	runGit(t, repoPath, "config", "user.name", "Test User")
	runGit(t, repoPath, "config", "user.email", "test@example.com")

	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# testrepo\n"), 0644); err != nil {
		t.Fatalf("failed to write README.md: %v", err)
	}
	runGit(t, repoPath, "add", "README.md")
	runGit(t, repoPath, "commit", "-m", "initial commit")
	runGit(t, repoPath, "remote", "add", "origin", "https://github.com/testuser/testrepo.git")
	runGit(t, repoPath, "worktree", "add", worktreePath, "-b", "feature-test")

	return repoPath
}

func setupFakeGhqCommand(t *testing.T, ghqRoot, repoPath string) {
	t.Helper()

	binDir := t.TempDir()
	ghqPath := filepath.Join(binDir, "ghq")
	script := `#!/bin/sh
if [ "$1" = "root" ] && [ "$2" = "--all" ]; then
  echo "$GWQ_TEST_GHQ_ROOT"
  exit 0
fi
if [ "$1" = "root" ]; then
  echo "$GWQ_TEST_GHQ_ROOT"
  exit 0
fi
if [ "$1" = "list" ] && [ "$2" = "-p" ]; then
  echo "$GWQ_TEST_GHQ_REPO"
  exit 0
fi
exit 1
`
	if err := os.WriteFile(ghqPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake ghq command: %v", err)
	}

	t.Setenv("GWQ_TEST_GHQ_ROOT", ghqRoot)
	t.Setenv("GWQ_TEST_GHQ_REPO", repoPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}
