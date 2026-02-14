package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/d-kuro/gwq/internal/discovery"
	"github.com/d-kuro/gwq/internal/finder"
	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/registry"
	"github.com/d-kuro/gwq/internal/worktree"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/cobra"
)

var (
	removeForce       bool
	removeDryRun      bool
	removeGlobal      bool
	deleteBranch      bool
	forceDeleteBranch bool
)

// removeCmd represents the remove command.
var removeCmd = &cobra.Command{
	Use:     "remove [pattern]",
	Aliases: []string{"rm"},
	Short:   "Delete worktree",
	Long: `Delete a worktree from the repository.

If no pattern is provided, shows a fuzzy finder to select the worktree.
The pattern can match against branch name or path.

By default, only the worktree directory is removed and the branch is preserved.
Use -b flag to also delete the branch after removing the worktree.

When run inside a git repository, shows worktrees for the current repository.
When run outside a git repository, shows all worktrees from the configured base directory.
Use -g flag to always show all worktrees from the base directory.`,
	Example: `  # Select and delete using fuzzy finder
  gwq remove

  # Delete by pattern matching
  gwq remove feature/old

  # Force delete even if dirty
  gwq remove -f feature/broken

  # Delete worktree and branch
  gwq remove -b feature/completed

  # Force delete branch even if not merged
  gwq remove -b --force-delete-branch feature/abandoned

  # Show what would be deleted
  gwq remove --dry-run feature/old

  # Remove from all worktrees in base directory
  gwq remove -g myapp:feature/old`,
	RunE: runRemove,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if removeGlobal {
			return getGlobalWorktreeCompletions(cmd, args, toComplete)
		}
		return getRemoveCompletions(cmd, args, toComplete)
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)

	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Force delete even if dirty")
	removeCmd.Flags().BoolVarP(&removeDryRun, "dry-run", "d", false, "Show deletion targets only")
	removeCmd.Flags().BoolVarP(&removeGlobal, "global", "g", false, "Remove from any worktree in the configured base directory")
	removeCmd.Flags().BoolVarP(&deleteBranch, "delete-branch", "b", false, "Also delete the branch after removing worktree")
	removeCmd.Flags().BoolVar(&forceDeleteBranch, "force-delete-branch", false, "Force delete the branch even if not merged")
}

func runRemove(cmd *cobra.Command, args []string) error {
	return ExecuteWithArgs(false, func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
		// Try to get git context, but don't fail if we're not in a git repo
		gitCtx, gitErr := NewGitCommandContext()
		if gitErr == nil {
			ctx = gitCtx
		}

		return ctx.WithGlobalLocalSupport(
			removeGlobal,
			func(ctx *CommandContext) error {
				return removeLocalWorktree(ctx, args)
			},
			func(ctx *CommandContext) error {
				return removeGlobalWorktree(ctx, args)
			},
		)
	})(cmd, args)
}

func removeLocalWorktree(ctx *CommandContext, args []string) error {
	worktrees, err := ctx.WorktreeManager.List()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	nonMainWorktrees := filterNonMainWorktrees(worktrees)
	if len(nonMainWorktrees) == 0 {
		return fmt.Errorf("no removable worktrees found")
	}

	var toRemove []models.Worktree

	if len(args) > 0 {
		// Get all matching worktrees
		matches, err := ctx.WorktreeManager.GetMatchingWorktrees(args[0])
		if err != nil {
			return err
		}

		// Filter out main worktrees
		var nonMainMatches []models.Worktree
		for _, wt := range matches {
			if !wt.IsMain {
				nonMainMatches = append(nonMainMatches, wt)
			}
		}

		if len(nonMainMatches) == 0 {
			return fmt.Errorf("no worktree found matching pattern: %s", args[0])
		} else if len(nonMainMatches) == 1 {
			toRemove = nonMainMatches
		} else {
			// Multiple matches - use fuzzy finder
			selected, err := ctx.GetFinder().SelectMultipleWorktrees(nonMainMatches)
			if err != nil {
				return fmt.Errorf("worktree selection cancelled")
			}
			toRemove = selected
		}
	} else {
		selected, err := ctx.GetFinder().SelectMultipleWorktrees(nonMainWorktrees)
		if err != nil {
			return fmt.Errorf("worktree selection cancelled")
		}
		toRemove = selected
	}

	if removeDryRun {
		fmt.Println("Would remove the following worktrees:")
		for _, wt := range toRemove {
			fmt.Printf("  %s (%s)\n", wt.Branch, wt.Path)
			if deleteBranch {
				fmt.Printf("    - Would delete branch: %s\n", wt.Branch)
			}
		}
		return nil
	}

	for _, wt := range toRemove {
		if deleteBranch {
			if err := ctx.WorktreeManager.RemoveWithBranch(wt.Path, wt.Branch, removeForce, deleteBranch, forceDeleteBranch); err != nil {
				ctx.Printer.PrintError(fmt.Errorf("failed to remove %s: %v", wt.Branch, err))
				continue
			}
			ctx.Printer.PrintSuccess(fmt.Sprintf("Removed worktree: %s", wt.Branch))
			if wt.Branch != "" {
				ctx.Printer.PrintSuccess(fmt.Sprintf("Deleted branch: %s", wt.Branch))
			}
		} else {
			if err := ctx.WorktreeManager.Remove(wt.Path, removeForce); err != nil {
				ctx.Printer.PrintError(fmt.Errorf("failed to remove %s: %v", wt.Branch, err))
				continue
			}
			ctx.Printer.PrintSuccess(fmt.Sprintf("Removed worktree: %s", wt.Branch))
		}

		// Clean up registry entry after successful removal
		if reg, err := registry.New(); err == nil {
			_ = reg.Unregister(wt.Path)
		}
	}

	return nil
}

func filterNonMainWorktrees(worktrees []models.Worktree) []models.Worktree {
	var filtered []models.Worktree
	for _, wt := range worktrees {
		if !wt.IsMain {
			filtered = append(filtered, wt)
		}
	}
	return filtered
}

func removeGlobalWorktree(ctx *CommandContext, args []string) error {
	entries, err := discoverGlobalEntries(ctx.Config)
	if err != nil {
		return fmt.Errorf("failed to discover worktrees: %w", err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("no worktrees found")
	}

	// Filter out main worktrees
	var nonMainEntries []*discovery.GlobalWorktreeEntry
	for _, entry := range entries {
		if !entry.IsMain {
			nonMainEntries = append(nonMainEntries, entry)
		}
	}

	if len(nonMainEntries) == 0 {
		return fmt.Errorf("no removable worktrees found")
	}

	var toRemove []*discovery.GlobalWorktreeEntry

	if len(args) > 0 {
		// Pattern matching
		pattern := strings.ToLower(args[0])
		var matches []*discovery.GlobalWorktreeEntry

		for _, entry := range nonMainEntries {
			branchLower := strings.ToLower(entry.Branch)
			var repoName string
			if entry.RepositoryInfo != nil {
				repoName = strings.ToLower(entry.RepositoryInfo.Repository)
			}

			// Match against branch name, path, repo name, or repo:branch pattern
			if strings.Contains(branchLower, pattern) ||
				strings.Contains(strings.ToLower(entry.Path), pattern) ||
				strings.Contains(repoName, pattern) ||
				strings.Contains(repoName+":"+branchLower, pattern) {
				matches = append(matches, entry)
			}
		}

		if len(matches) == 0 {
			return fmt.Errorf("no worktree matches pattern: %s", args[0])
		} else if len(matches) == 1 {
			toRemove = matches
		} else {
			// Multiple matches - use fuzzy finder
			worktrees := discovery.ConvertToWorktreeModels(matches, true)

			// Create a temporary git instance for finder
			g, _ := git.NewFromCwd()
			if g == nil {
				g = &git.Git{}
			}

			f := finder.NewWithUI(g, &ctx.Config.Finder, &ctx.Config.UI, &ctx.Config.Naming)
			selected, err := f.SelectMultipleWorktrees(worktrees)
			if err != nil {
				return fmt.Errorf("worktree selection cancelled")
			}

			// Map selected worktrees back to entries
			selectedPaths := make(map[string]bool)
			for _, wt := range selected {
				selectedPaths[wt.Path] = true
			}

			for _, entry := range matches {
				if selectedPaths[entry.Path] {
					toRemove = append(toRemove, entry)
				}
			}
		}
	} else {
		// No pattern - show all in fuzzy finder
		worktrees := discovery.ConvertToWorktreeModels(nonMainEntries, true)

		// Use global finder for selection
		f := ctx.GetGlobalFinder()
		selected, err := f.SelectMultipleWorktrees(worktrees)
		if err != nil {
			return fmt.Errorf("worktree selection cancelled")
		}

		// Map selected worktrees back to entries
		selectedPaths := make(map[string]bool)
		for _, wt := range selected {
			selectedPaths[wt.Path] = true
		}

		for _, entry := range nonMainEntries {
			if selectedPaths[entry.Path] {
				toRemove = append(toRemove, entry)
			}
		}
	}

	if removeDryRun {
		fmt.Println("Would remove the following worktrees:")
		for _, entry := range toRemove {
			fmt.Printf("  %s:%s (%s)\n", entryRepoName(entry), entry.Branch, entry.Path)
			if deleteBranch {
				fmt.Printf("    - Would delete branch: %s\n", entry.Branch)
			}
		}
		return nil
	}

	// Remove each worktree by changing to its repository directory
	for _, entry := range toRemove {
		if err := removeOneGlobalWorktree(ctx, entry); err != nil {
			ctx.Printer.PrintError(err)
		}
	}

	return nil
}

// entryRepoName returns the repository name from an entry, or "unknown" if unavailable.
func entryRepoName(entry *discovery.GlobalWorktreeEntry) string {
	if entry.RepositoryInfo != nil {
		return entry.RepositoryInfo.Repository
	}
	return "unknown"
}

// removeOneGlobalWorktree removes a single global worktree entry.
func removeOneGlobalWorktree(ctx *CommandContext, entry *discovery.GlobalWorktreeEntry) error {
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	// Find the repository root directory
	repoPath := entry.RepositoryURL
	if entry.RepositoryInfo != nil {
		g := git.New(entry.Path)
		if repoRootPath, err := g.GetRepositoryPath(); err == nil {
			repoPath = repoRootPath
		}
	}

	if err := os.Chdir(repoPath); err != nil {
		return fmt.Errorf("failed to change to repository %s: %v", repoPath, err)
	}

	g := git.New(repoPath)
	wm := worktree.New(g, ctx.Config)

	displayName := entryRepoName(entry) + ":" + entry.Branch

	if deleteBranch {
		if err := wm.RemoveWithBranch(entry.Path, entry.Branch, removeForce, deleteBranch, forceDeleteBranch); err != nil {
			return fmt.Errorf("failed to remove %s: %v", displayName, err)
		}
	} else {
		if err := wm.Remove(entry.Path, removeForce); err != nil {
			return fmt.Errorf("failed to remove %s: %v", displayName, err)
		}
	}

	ctx.Printer.PrintSuccess(fmt.Sprintf("Removed worktree: %s", displayName))
	if deleteBranch && entry.Branch != "" {
		ctx.Printer.PrintSuccess(fmt.Sprintf("Deleted branch: %s", entry.Branch))
	}

	// Clean up registry entry after successful removal
	if reg, err := registry.New(); err == nil {
		_ = reg.Unregister(entry.Path)
	}

	return nil
}
