package cmd

import (
	"github.com/d-kuro/gwq/internal/discovery"
	"github.com/d-kuro/gwq/pkg/models"
)

var discoverAllWorktreesFn = discovery.DiscoverAllWorktrees
var discoverGlobalWorktreesFn = discovery.DiscoverGlobalWorktrees

// discoverGlobalEntries discovers global worktree entries based on config.
func discoverGlobalEntries(cfg *models.Config) ([]*discovery.GlobalWorktreeEntry, error) {
	if cfg.Ghq.Enabled {
		return discoverAllWorktreesFn(cfg)
	}
	return discoverGlobalWorktreesFn(cfg.Worktree.BaseDir)
}
