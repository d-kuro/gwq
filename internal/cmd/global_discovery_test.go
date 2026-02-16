package cmd

import (
	"errors"
	"testing"

	"github.com/d-kuro/gwq/internal/discovery"
	"github.com/d-kuro/gwq/pkg/models"
)

func TestDiscoverGlobalEntries_UsesGhqDiscoveryWhenEnabled(t *testing.T) {
	t.Helper()

	restoreAll := discoverAllWorktreesFn
	restoreGlobal := discoverGlobalWorktreesFn
	t.Cleanup(func() {
		discoverAllWorktreesFn = restoreAll
		discoverGlobalWorktreesFn = restoreGlobal
	})

	expected := []*discovery.GlobalWorktreeEntry{{Path: "/tmp/ghq/repo"}}
	ghqCalled := false
	globalCalled := false

	discoverAllWorktreesFn = func(cfg *models.Config) ([]*discovery.GlobalWorktreeEntry, error) {
		ghqCalled = true
		if !cfg.Ghq.Enabled {
			t.Fatalf("expected ghq.enabled=true, got false")
		}
		return expected, nil
	}
	discoverGlobalWorktreesFn = func(_ string) ([]*discovery.GlobalWorktreeEntry, error) {
		globalCalled = true
		return nil, nil
	}

	cfg := &models.Config{
		Ghq: models.GhqConfig{
			Enabled: true,
		},
		Worktree: models.WorktreeConfig{
			BaseDir: "/tmp/worktrees",
		},
	}

	entries, err := discoverGlobalEntries(cfg)
	if err != nil {
		t.Fatalf("discoverGlobalEntries() unexpected error: %v", err)
	}
	if !ghqCalled {
		t.Fatal("expected ghq discovery to be called")
	}
	if globalCalled {
		t.Fatal("expected basedir discovery not to be called when ghq is enabled")
	}
	if len(entries) != len(expected) || entries[0].Path != expected[0].Path {
		t.Fatalf("discoverGlobalEntries() returned unexpected entries: %+v", entries)
	}
}

func TestDiscoverGlobalEntries_UsesBasedirDiscoveryWhenDisabled(t *testing.T) {
	t.Helper()

	restoreAll := discoverAllWorktreesFn
	restoreGlobal := discoverGlobalWorktreesFn
	t.Cleanup(func() {
		discoverAllWorktreesFn = restoreAll
		discoverGlobalWorktreesFn = restoreGlobal
	})

	expected := []*discovery.GlobalWorktreeEntry{{Path: "/tmp/worktrees/repo/feature"}}
	ghqCalled := false
	globalCalled := false

	discoverAllWorktreesFn = func(_ *models.Config) ([]*discovery.GlobalWorktreeEntry, error) {
		ghqCalled = true
		return nil, nil
	}
	discoverGlobalWorktreesFn = func(baseDir string) ([]*discovery.GlobalWorktreeEntry, error) {
		globalCalled = true
		if baseDir != "/tmp/worktrees" {
			t.Fatalf("expected basedir /tmp/worktrees, got %s", baseDir)
		}
		return expected, nil
	}

	cfg := &models.Config{
		Ghq: models.GhqConfig{
			Enabled: false,
		},
		Worktree: models.WorktreeConfig{
			BaseDir: "/tmp/worktrees",
		},
	}

	entries, err := discoverGlobalEntries(cfg)
	if err != nil {
		t.Fatalf("discoverGlobalEntries() unexpected error: %v", err)
	}
	if ghqCalled {
		t.Fatal("expected ghq discovery not to be called when ghq is disabled")
	}
	if !globalCalled {
		t.Fatal("expected basedir discovery to be called")
	}
	if len(entries) != len(expected) || entries[0].Path != expected[0].Path {
		t.Fatalf("discoverGlobalEntries() returned unexpected entries: %+v", entries)
	}
}

func TestDiscoverGlobalEntries_PropagatesDiscoveryError(t *testing.T) {
	t.Helper()

	restoreAll := discoverAllWorktreesFn
	restoreGlobal := discoverGlobalWorktreesFn
	t.Cleanup(func() {
		discoverAllWorktreesFn = restoreAll
		discoverGlobalWorktreesFn = restoreGlobal
	})

	expectedErr := errors.New("discovery failed")
	discoverGlobalWorktreesFn = func(_ string) ([]*discovery.GlobalWorktreeEntry, error) {
		return nil, expectedErr
	}
	discoverAllWorktreesFn = func(_ *models.Config) ([]*discovery.GlobalWorktreeEntry, error) {
		return nil, nil
	}

	cfg := &models.Config{
		Ghq: models.GhqConfig{
			Enabled: false,
		},
		Worktree: models.WorktreeConfig{
			BaseDir: "/tmp/worktrees",
		},
	}

	_, err := discoverGlobalEntries(cfg)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("discoverGlobalEntries() error = %v, want %v", err, expectedErr)
	}
}
