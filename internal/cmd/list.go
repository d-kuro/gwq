package cmd

import (
	"fmt"

	"github.com/d-kuro/gwq/internal/discovery"
	"github.com/spf13/cobra"
)

var (
	listVerbose bool
	listJSON    bool
	listGlobal  bool
)

// listCmd represents the list command.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Display worktree list",
	Long: `Display a list of worktrees.

When run inside a git repository, shows worktrees for the current repository.
When run outside a git repository, shows all worktrees in the configured base directory.
Use -g flag to always show all worktrees from the base directory.
Use -v flag for detailed information including commit hashes and creation times.
Use --json flag to output in JSON format for scripting.`,
	Example: `  # Simple list
  gwq list

  # Detailed information
  gwq list -v

  # JSON format for scripting
  gwq list --json

  # Show all worktrees from base directory (from anywhere)
  gwq list -g`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolVarP(&listVerbose, "verbose", "v", false, "Show detailed information")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")
	listCmd.Flags().BoolVarP(&listGlobal, "global", "g", false, "Show all worktrees from the configured base directory")
}

func runList(cmd *cobra.Command, args []string) error {
	// Try git context first, fall back to non-git if needed
	ctx, err := NewGitCommandContext()
	if err != nil {
		// If git initialization fails, create non-git context for global mode
		ctx, err = NewCommandContext()
		if err != nil {
			return err
		}
	}

	return ctx.WithGlobalLocalSupport(
		listGlobal,
		func(ctx *CommandContext) error {
			// Local mode - show worktrees from current repository
			worktrees, err := ctx.WorktreeManager.List()
			if err != nil {
				return fmt.Errorf("failed to list worktrees: %w", err)
			}

			if listJSON {
				return ctx.Printer.PrintWorktreesJSON(worktrees)
			}

			ctx.Printer.PrintWorktrees(worktrees, listVerbose)
			return nil
		},
		func(ctx *CommandContext) error {
			// Global mode - show all worktrees from base directory
			return showGlobalWorktrees(ctx)
		},
	)
}

func showGlobalWorktrees(ctx *CommandContext) error {
	entries, err := discovery.DiscoverGlobalWorktrees(ctx.Config.Worktree.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to discover worktrees: %w", err)
	}

	if len(entries) == 0 {
		ctx.Printer.PrintInfo("No worktrees found in " + ctx.Config.Worktree.BaseDir)
		return nil
	}

	// Convert to worktree models with repository names for clarity
	worktrees := discovery.ConvertToWorktreeModels(entries, !listVerbose)

	if listJSON {
		return ctx.Printer.PrintWorktreesJSON(worktrees)
	}

	ctx.Printer.PrintWorktrees(worktrees, listVerbose)
	return nil
}
