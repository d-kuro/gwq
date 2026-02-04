package cmd

import (
	"fmt"

	"github.com/d-kuro/gwq/internal/config"
	"github.com/d-kuro/gwq/internal/discovery"
	"github.com/d-kuro/gwq/internal/finder"
	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/ui"
	"github.com/d-kuro/gwq/internal/worktree"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/cobra"
)

// CommandContext encapsulates common dependencies used across commands.
// This eliminates boilerplate code and provides consistent initialization.
type CommandContext struct {
	Config          *models.Config
	Git             *git.Git
	Printer         *ui.Printer
	WorktreeManager *worktree.Manager
	finder          *finder.Finder // Lazy-loaded
	IsGitRepo       bool
}

// NewCommandContext creates a new command context for commands that don't require git.
func NewCommandContext() (*CommandContext, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	printer := ui.New(&cfg.UI)

	return &CommandContext{
		Config:    cfg,
		Printer:   printer,
		IsGitRepo: false,
	}, nil
}

// NewGitCommandContext creates a new command context for commands that require git repository.
func NewGitCommandContext() (*CommandContext, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	g, err := git.NewFromCwd()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize git: %w", err)
	}

	printer := ui.New(&cfg.UI)
	wm := worktree.New(g, cfg)

	return &CommandContext{
		Config:          cfg,
		Git:             g,
		Printer:         printer,
		WorktreeManager: wm,
		IsGitRepo:       true,
	}, nil
}

// GetFinder returns a finder instance, creating it if needed.
// This provides lazy initialization to avoid creating finders for commands that don't need them.
func (ctx *CommandContext) GetFinder() *finder.Finder {
	if ctx.finder == nil && ctx.Git != nil {
		ctx.finder = finder.NewWithUI(ctx.Git, &ctx.Config.Finder, &ctx.Config.UI, &ctx.Config.Naming)
	}
	return ctx.finder
}

// GetGlobalFinder returns a finder instance for global operations that don't require a git repository.
// This creates a finder with an empty git instance, suitable for global worktree operations.
// Path display is disabled because the Branch field already contains the display path.
func (ctx *CommandContext) GetGlobalFinder() *finder.Finder {
	// For global operations, we use an empty git instance
	emptyGit := &git.Git{}
	f := finder.NewWithUI(emptyGit, &ctx.Config.Finder, &ctx.Config.UI, &ctx.Config.Naming)
	f.SetShowPath(false)
	return f
}

// Factory functions for commands that haven't been refactored to use CommandContext yet

// CreateFinder creates a finder instance for local operations with the given git instance.
func CreateFinder(g *git.Git, cfg *models.Config) *finder.Finder {
	return finder.NewWithUI(g, &cfg.Finder, &cfg.UI, &cfg.Naming)
}

// CreateGlobalFinder creates a finder instance for global operations.
// Path display is disabled because the Branch field already contains the display path.
func CreateGlobalFinder(cfg *models.Config) *finder.Finder {
	emptyGit := &git.Git{}
	f := finder.NewWithUI(emptyGit, &cfg.Finder, &cfg.UI, &cfg.Naming)
	f.SetShowPath(false)
	return f
}

// DiscoverGlobalWorktrees discovers global worktrees when -g flag is used.
// If ghq mode is enabled, it discovers worktrees from ghq repositories and their .worktrees directories.
// It also discovers worktrees from the traditional basedir.
func (ctx *CommandContext) DiscoverGlobalWorktrees() ([]*models.Worktree, error) {
	entries, err := discovery.DiscoverAllWorktrees(ctx.Config)
	if err != nil {
		return nil, err
	}

	// Convert GlobalWorktreeEntry to models.Worktree with proper display format
	// showRepoName=true to display in ghq list format (host/owner/repo:branch)
	worktreeSlice := discovery.ConvertToWorktreeModels(entries, true)

	return toWorktreePtrs(worktreeSlice), nil
}

// GetWorktrees returns worktrees with support for both global and local modes
func (ctx *CommandContext) GetWorktrees(forceGlobal bool) ([]*models.Worktree, error) {
	if forceGlobal || !ctx.IsGitRepo {
		return ctx.DiscoverGlobalWorktrees()
	}

	localWorktrees, err := ctx.WorktreeManager.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return toWorktreePtrs(localWorktrees), nil
}

// toWorktreePtrs converts a slice of worktrees to a slice of pointers.
func toWorktreePtrs(worktrees []models.Worktree) []*models.Worktree {
	result := make([]*models.Worktree, len(worktrees))
	for i := range worktrees {
		result[i] = &worktrees[i]
	}
	return result
}

// WithGlobalLocalSupport handles commands that support both global and local modes.
func (ctx *CommandContext) WithGlobalLocalSupport(
	forceGlobal bool,
	localFn func(*CommandContext) error,
	globalFn func(*CommandContext) error,
) error {
	if forceGlobal || !ctx.IsGitRepo {
		return globalFn(ctx)
	}
	return localFn(ctx)
}

// ExecuteWithContext creates a command context and executes the provided function.
// This is the main wrapper function that eliminates boilerplate in command implementations.
func ExecuteWithContext(requiresGit bool, fn func(*CommandContext) error) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, _ []string) error {
		ctx, err := createContext(requiresGit)
		if err != nil {
			return err
		}
		return fn(ctx)
	}
}

// ExecuteWithArgs is a variant that passes command arguments to the function.
func ExecuteWithArgs(requiresGit bool, fn func(*CommandContext, *cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx, err := createContext(requiresGit)
		if err != nil {
			return err
		}

		// Resolve ghq flag and override config if explicitly set
		ctx.resolveGhqFlag(cmd)

		return fn(ctx, cmd, args)
	}
}

// createContext creates a CommandContext based on whether git is required.
func createContext(requiresGit bool) (*CommandContext, error) {
	if requiresGit {
		return NewGitCommandContext()
	}
	return NewCommandContext()
}

// resolveGhqFlag checks if the --ghq flag was explicitly set and updates the config accordingly.
// Priority: command line flag > environment variable > config file (viper handles env/config)
func (ctx *CommandContext) resolveGhqFlag(cmd *cobra.Command) {
	if cmd.Flags().Changed("ghq") {
		if ghqVal, err := cmd.Flags().GetBool("ghq"); err == nil {
			ctx.Config.Ghq.Enabled = ghqVal
		}
	}
}

// IsGhqEnabled returns whether ghq integration mode is enabled.
// This considers the priority: command line flag > environment variable > config file.
// Call this method after resolveGhqFlag has been called (typically in ExecuteWithArgs).
func (ctx *CommandContext) IsGhqEnabled() bool {
	return ctx.Config.Ghq.Enabled
}
