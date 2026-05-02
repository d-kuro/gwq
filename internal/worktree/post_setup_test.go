package worktree

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/d-kuro/gwq/pkg/models"
)

// recordingExecutor records every Execute call so we can assert the exact
// rendered command string passed to `sh -c`.
type recordingExecutor struct {
	fakeExecutor
}

func newRecordingExecutor() *recordingExecutor {
	return &recordingExecutor{}
}

func (r *recordingExecutor) rendered() []string {
	out := make([]string, 0, len(r.calls))
	for _, c := range r.calls {
		// args layout is always ["-c", "<rendered cmd>"]
		if len(c.args) == 2 && c.args[0] == "-c" {
			out = append(out, c.args[1])
		}
	}
	return out
}

func buildManagerWithRepoSetting(g *mockGit, setting models.RepositorySetting) *Manager {
	cfg := &models.Config{
		RepositorySettings: []models.RepositorySetting{setting},
	}
	return &Manager{git: g, config: cfg}
}

func TestRunPostWorktreeSetup_RendersTemplateVariables(t *testing.T) {
	git := &mockGit{
		repoPath: "/mock/repo/path",
		repoURL:  "https://github.com/test-user/test-repo.git",
	}
	setting := models.RepositorySetting{
		Repository: "/mock/repo/path",
		SetupCommands: []string{
			"echo branch={{.Branch}} path={{.Path}}",
			"echo host={{.Host}} owner={{.Owner}} repo={{.Repository}}",
		},
	}
	m := buildManagerWithRepoSetting(git, setting)

	exec := newRecordingExecutor()
	results := m.runPostWorktreeSetupWithExecutor(context.Background(), exec, "feature/new-ui", "/tmp/worktrees/gwq/feature-new-ui")

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	got := exec.rendered()
	wantContains := []string{
		"branch=feature/new-ui path=/tmp/worktrees/gwq/feature-new-ui",
		"host=github.com owner=test-user repo=test-repo",
	}
	for i, w := range wantContains {
		if i >= len(got) {
			t.Fatalf("missing rendered command at index %d", i)
		}
		if !strings.Contains(got[i], w) {
			t.Errorf("rendered[%d] = %q; want it to contain %q", i, got[i], w)
		}
	}
}

func TestRunPostWorktreeSetup_FallbackWhenRemoteUnavailable(t *testing.T) {
	git := &mockGit{
		repoPath:     "/mock/repo/path",
		repoURLError: errors.New("no origin remote"),
	}
	setting := models.RepositorySetting{
		Repository: "/mock/repo/path",
		SetupCommands: []string{
			"echo branch={{.Branch}} path={{.Path}} host=[{{.Host}}]",
		},
	}
	m := buildManagerWithRepoSetting(git, setting)

	exec := newRecordingExecutor()
	results := m.runPostWorktreeSetupWithExecutor(context.Background(), exec, "topic", "/wt/topic")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := exec.rendered()
	if len(got) != 1 {
		t.Fatalf("expected 1 rendered call, got %d", len(got))
	}
	if !strings.Contains(got[0], "branch=topic") {
		t.Errorf("rendered = %q; missing branch=topic", got[0])
	}
	if !strings.Contains(got[0], "path=/wt/topic") {
		t.Errorf("rendered = %q; missing path=/wt/topic", got[0])
	}
	if !strings.Contains(got[0], "host=[]") {
		t.Errorf("rendered = %q; Host should be empty when remote is unavailable", got[0])
	}
}

func TestRunPostWorktreeSetup_TemplateErrorSkipsOnlyFailing(t *testing.T) {
	git := &mockGit{
		repoPath: "/mock/repo/path",
		repoURL:  "https://github.com/test-user/test-repo.git",
	}
	setting := models.RepositorySetting{
		Repository: "/mock/repo/path",
		SetupCommands: []string{
			"echo ok {{.Branch}}",
			"echo bad {{.NoSuchVar}}",
			"echo also ok {{.Path}}",
		},
	}
	m := buildManagerWithRepoSetting(git, setting)

	exec := newRecordingExecutor()
	results := m.runPostWorktreeSetupWithExecutor(context.Background(), exec, "br", "/wt/br")

	if len(results) != 2 {
		t.Fatalf("expected 2 results (bad skipped), got %d", len(results))
	}
	got := exec.rendered()
	if len(got) != 2 {
		t.Fatalf("expected 2 executor calls, got %d", len(got))
	}
	if !strings.Contains(got[0], "ok br") {
		t.Errorf("rendered[0] = %q; want contains \"ok br\"", got[0])
	}
	if !strings.Contains(got[1], "also ok /wt/br") {
		t.Errorf("rendered[1] = %q; want contains \"also ok /wt/br\"", got[1])
	}
}

func TestRunPostWorktreeSetup_NoMatchingRepoSetting(t *testing.T) {
	git := &mockGit{repoPath: "/mock/repo/path"}
	setting := models.RepositorySetting{
		Repository:    "/different/repo",
		SetupCommands: []string{"echo should-not-run"},
	}
	m := buildManagerWithRepoSetting(git, setting)

	exec := newRecordingExecutor()
	results := m.runPostWorktreeSetupWithExecutor(context.Background(), exec, "br", "/wt/br")

	if len(results) != 0 {
		t.Errorf("expected no results when repo does not match, got %d", len(results))
	}
	if len(exec.calls) != 0 {
		t.Errorf("expected no executor calls, got %d", len(exec.calls))
	}
}
