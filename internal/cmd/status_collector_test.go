package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/pkg/models"
)

func TestCollectAllCurrentWorktree(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	sibling := filepath.Join(root, "repo-other")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(root, "gitconfig"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	t.Setenv("GIT_CONFIG_VALUE_0", root)
	for _, path := range []string{repo, sibling} {
		cmd := exec.Command("git", "init", "-b", "main", path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
		cmd = exec.Command("git", "-C", path, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "fixture")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit: %v\n%s", err, out)
		}
	}
	worktrees, err := git.New(repo).ListWorktrees()
	if err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(repo, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{repo, subdir, sibling} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Chdir(path)
			statuses, err := NewStatusCollector(false, false).CollectAll(context.Background(), []*models.Worktree{&worktrees[0]})
			if err != nil {
				t.Fatal(err)
			}
			if len(statuses) != 1 {
				t.Fatalf("got %d statuses, want 1", len(statuses))
			}
			if want := path != sibling; statuses[0].IsCurrent != want {
				t.Errorf("IsCurrent from %s = %v, want %v", path, statuses[0].IsCurrent, want)
			}
		})
	}
}
