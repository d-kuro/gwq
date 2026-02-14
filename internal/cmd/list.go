package cmd

import (
	"fmt"

	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/cobra"
)

var (
	listVerbose bool
	listJSON    bool
	listGlobal  bool
)

// listCmd represents the list command.
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "Display worktree list",
	Long: `Display a list of worktrees.

When run inside a git repository, shows worktrees for the current repository.
When run outside a git repository, shows all worktrees in the configured base directory.
Use -g flag to always show all worktrees from the base directory.
Use -v flag for detailed information including commit hashes and creation times.
Use --json flag to output in JSON format for scripting.`,
	Example: `  # Simple list
  gwq list

  # Using the ls alias
  gwq ls

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

	// Process --ghq flag to enable/disable ghq integration mode
	ctx.resolveGhqFlag(cmd)

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
	worktreePointers, err := ctx.DiscoverGlobalWorktrees()
	if err != nil {
		return fmt.Errorf("failed to discover worktrees: %w", err)
	}

	if len(worktreePointers) == 0 {
		ctx.Printer.PrintInfo("No worktrees found in " + ctx.Config.Worktree.BaseDir)
		return nil
	}

	worktrees := derefWorktrees(worktreePointers)

	if listJSON {
		return ctx.Printer.PrintWorktreesJSON(worktrees)
	}

	ctx.Printer.PrintWorktrees(worktrees, listVerbose)
	return nil
}

// derefWorktrees converts a slice of pointers to a slice of values.
func derefWorktrees(pointers []*models.Worktree) []models.Worktree {
	result := make([]models.Worktree, len(pointers))
	for i, p := range pointers {
		result[i] = *p
	}
	return result
}
