package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/registry"
	"github.com/spf13/cobra"
)

var (
	pruneExpired bool
	pruneDryRun  bool
	pruneForce   bool
)

// pruneCmd represents the prune command.
var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Clean up deleted worktree information",
	Long: `Clean up worktree information for directories that have been deleted.

This command removes administrative files from .git/worktrees for worktrees
whose working directories have been deleted from the filesystem.

With --expired flag, removes worktrees that have passed their expiration date.`,
	Example: `  # Clean up stale worktree information
  gwq prune

  # Preview expired worktrees
  gwq prune --expired --dry-run

  # Remove expired worktrees
  gwq prune --expired

  # Force remove even if dirty
  gwq prune --expired --force`,
	RunE: runPrune,
}

func init() {
	rootCmd.AddCommand(pruneCmd)

	pruneCmd.Flags().BoolVar(&pruneExpired, "expired", false, "Remove expired worktrees")
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "Show what would be removed")
	pruneCmd.Flags().BoolVar(&pruneForce, "force", false, "Remove even if uncommitted changes")
}

func runPrune(cmd *cobra.Command, args []string) error {
	if pruneExpired {
		return runPruneExpired(cmd, args)
	}

	return ExecuteWithContext(true, func(ctx *CommandContext) error {
		if err := ctx.WorktreeManager.Prune(); err != nil {
			return fmt.Errorf("failed to prune worktrees: %w", err)
		}

		ctx.Printer.PrintSuccess("Pruned stale worktree information")
		return nil
	})(cmd, args)
}

func runPruneExpired(cmd *cobra.Command, args []string) error {
	reg, err := registry.New()
	if err != nil {
		return fmt.Errorf("failed to open registry: %w", err)
	}

	expired := reg.ListExpired()
	if len(expired) == 0 {
		fmt.Println("No expired worktrees found")
		return nil
	}

	var removed int
	var skipped int

	for _, entry := range expired {
		// Never remove main worktrees
		if entry.IsMain {
			if pruneDryRun {
				fmt.Printf("Would skip (main worktree): %s\n", entry.Path)
			}
			skipped++
			continue
		}

		// Check for uncommitted changes unless --force
		if !pruneForce {
			dirty, err := isWorktreeDirty(entry.Path)
			if err != nil {
				fmt.Printf("Warning: could not check status for %s: %v\n", entry.Path, err)
				skipped++
				continue
			}
			if dirty {
				if pruneDryRun {
					fmt.Printf("Would skip (uncommitted changes): %s\n", entry.Path)
				} else {
					fmt.Printf("Skipping (uncommitted changes): %s (use --force to override)\n", entry.Path)
				}
				skipped++
				continue
			}
		}

		// Check if worktree directory still exists
		if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
			if pruneDryRun {
				fmt.Printf("Would unregister (already deleted): %s\n", entry.Path)
				removed++
				continue
			}
			// Directory already gone, just unregister
			if err := reg.Unregister(entry.Path); err != nil {
				fmt.Printf("Warning: failed to unregister %s: %v\n", entry.Path, err)
			}
			fmt.Printf("Unregistered (already deleted): %s\n", entry.Path)
			removed++
			continue
		}

		if pruneDryRun {
			fmt.Printf("Would remove: %s (branch: %s)\n", entry.Path, entry.Branch)
			removed++
			continue
		}

		// Remove the worktree using git
		g := git.New(entry.Path)
		if err := g.RemoveWorktree(entry.Path, pruneForce); err != nil {
			fmt.Printf("Failed to remove worktree %s: %v\n", entry.Path, err)
			skipped++
			continue
		}

		// Unregister from registry
		if err := reg.Unregister(entry.Path); err != nil {
			fmt.Printf("Warning: failed to unregister %s: %v\n", entry.Path, err)
		}

		fmt.Printf("Removed: %s (branch: %s)\n", entry.Path, entry.Branch)
		removed++
	}

	if pruneDryRun {
		fmt.Printf("\nDry run: would remove %d worktree(s), skip %d\n", removed, skipped)
	} else {
		fmt.Printf("\nRemoved %d expired worktree(s), skipped %d\n", removed, skipped)
	}

	return nil
}

// isWorktreeDirty checks if a worktree has uncommitted changes.
func isWorktreeDirty(path string) (bool, error) {
	cmd := exec.Command("git", "-C", path, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return strings.TrimSpace(string(output)) != "", nil
}
