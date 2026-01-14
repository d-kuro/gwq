package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	addBranch      bool
	addInteractive bool
	addForce       bool
	addStay        bool
)

// addCmd represents the add command.
var addCmd = &cobra.Command{
	Use:   "add [branch] [path]",
	Short: "Create a new worktree",
	Long: `Create a new worktree for the specified branch.

If no path is provided, it will be generated based on the configuration template.
Use -i flag to interactively select a branch using fuzzy finder.`,
	Example: `  # Create worktree from existing branch
  gwq add feature/new-ui

  # Create at specific path
  gwq add feature/new-ui ~/projects/myapp-feature

  # Create new branch and worktree
  gwq add -b feature/api-v2

  # Interactive branch selection
  gwq add -i

  # Create worktree and stay in the directory
  gwq add -s feature/new-ui`,
	RunE:              runAdd,
	ValidArgsFunction: getBranchCompletions,
}

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().BoolVarP(&addBranch, "branch", "b", false, "Create new branch")
	addCmd.Flags().BoolVarP(&addInteractive, "interactive", "i", false, "Select branch using fuzzy finder")
	addCmd.Flags().BoolVarP(&addForce, "force", "f", false, "Overwrite existing directory")
	addCmd.Flags().BoolVarP(&addStay, "stay", "s", false, "Stay in worktree directory after creation")
}

func runAdd(cmd *cobra.Command, args []string) error {
	return ExecuteWithArgs(true, func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
		var branch string
		var path string

		if addInteractive {
			if len(args) > 0 {
				return fmt.Errorf("cannot specify branch name with -i flag")
			}

			branches, err := ctx.Git.ListBranches(true)
			if err != nil {
				return fmt.Errorf("failed to list branches: %w", err)
			}

			selectedBranch, err := ctx.GetFinder().SelectBranch(branches)
			if err != nil {
				return fmt.Errorf("branch selection cancelled")
			}

			branch = selectedBranch.Name
			if selectedBranch.IsRemote {
				branch = selectedBranch.Name[len("origin/"):]
				addBranch = true
			}
		} else {
			if len(args) < 1 {
				return fmt.Errorf("branch name is required")
			}
			branch = args[0]
			if len(args) > 1 {
				path = args[1]
			}
		}

		if path != "" && !addForce {
			if err := ctx.WorktreeManager.ValidateWorktreePath(path); err != nil {
				return err
			}
		}

		worktreePath, err := ctx.WorktreeManager.Add(branch, path, addBranch)
		if err != nil {
			return err
		}

		ctx.Printer.PrintSuccess(fmt.Sprintf("Created worktree for branch '%s'", branch))

		if addStay {
			_ = LaunchShell(worktreePath)
		}

		return nil
	})(cmd, args)
}
