package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/ui"
	"github.com/d-kuro/gwq/internal/worktree"
	"github.com/d-kuro/gwq/pkg/models"
)

func removeFixture(t *testing.T) (*CommandContext, string) {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "main")
	wt := filepath.Join(root, "trees", "feature")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	t.Setenv("GIT_CONFIG_VALUE_0", root)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main", repo)
	run("-C", repo, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "fixture")
	run("-C", repo, "remote", "add", "origin", "https://fixture.invalid/owner/repo.git")
	run("-C", repo, "worktree", "add", "-b", "feature", wt)
	t.Chdir(repo)
	cfg := &models.Config{}
	cfg.Worktree.BaseDir = filepath.Dir(wt)
	g := git.New(repo)
	ctx := &CommandContext{Config: cfg, Git: g, Printer: ui.New(&cfg.UI), WorktreeManager: worktree.New(g, cfg), IsGitRepo: true}
	return ctx, wt
}

func TestRemoveDirtyReturnsError(t *testing.T) {
	for _, global := range []bool{false, true} {
		name := "local"
		if global {
			name = "global"
		}
		t.Run(name, func(t *testing.T) {
			ctx, wt := removeFixture(t)
			if err := os.WriteFile(filepath.Join(wt, "untracked"), []byte("keep"), 0644); err != nil {
				t.Fatal(err)
			}
			remove := removeLocalWorktree
			if global {
				remove = removeGlobalWorktree
			}
			if err := remove(ctx, []string{"feature"}); err == nil {
				t.Error("removing a dirty worktree returned success")
			}
			if _, err := os.Stat(filepath.Join(wt, "untracked")); err != nil {
				t.Fatalf("dirty worktree not preserved: %v", err)
			}
		})
	}
}

func TestRemoveGlobalWorktree(t *testing.T) {
	ctx, wt := removeFixture(t)
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := removeGlobalWorktree(ctx, []string{"repo:feature"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree directory remains: %v", err)
	}
	worktrees, err := ctx.Git.ListWorktrees()
	if err != nil {
		t.Fatal(err)
	}
	if len(worktrees) != 1 || !worktrees[0].IsMain {
		t.Errorf("remaining worktrees = %v, want main only", worktrees)
	}
	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(before, after) {
		t.Errorf("working directory changed: %s -> %s", before, after)
	}
}
