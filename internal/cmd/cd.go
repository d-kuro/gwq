package cmd

import (
	"github.com/d-kuro/gwq/internal/config"
	"github.com/spf13/cobra"
)

var cdGlobal bool

var cdCmd = &cobra.Command{
	Use:   "cd [pattern]",
	Short: "Change to worktree directory in new shell",
	Long: `Change to a worktree directory by launching a new shell.

This command launches a new shell session in the selected worktree directory.
Type 'exit' to return to the original directory.

If multiple worktrees match the pattern, an interactive fuzzy finder will be shown.
If no pattern is provided, all worktrees will be shown in the fuzzy finder.

This is equivalent to: cd $(gwq get pattern)`,
	Example: `  # Change to a worktree matching 'feature'
  gwq cd feature

  # Show all worktrees and select with fuzzy finder
  gwq cd

  # Change to global worktree
  gwq cd -g project:feature`,
	RunE: runCd,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return getWorktreeCompletions(cmd, args, toComplete)
	},
}

func init() {
	rootCmd.AddCommand(cdCmd)
	cdCmd.Flags().BoolVarP(&cdGlobal, "global", "g", false, "Change to global worktree")
}

func runCd(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var pattern string
	if len(args) > 0 {
		pattern = args[0]
	}

	var worktreePath string
	if cdGlobal {
		worktreePath, err = getGlobalWorktreePathForExec(cfg, pattern)
	} else {
		worktreePath, err = getLocalWorktreePathForExec(cfg, pattern)
	}

	if err != nil {
		return err
	}

	return LaunchShell(worktreePath)
}
