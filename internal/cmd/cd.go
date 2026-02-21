package cmd

import (
	"fmt"
	"os"

	"github.com/d-kuro/gwq/internal/config"
	"github.com/spf13/cobra"
)

var cdGlobal bool

var cdCmd = &cobra.Command{
	Use:   "cd [pattern]",
	Short: "Change to worktree directory",
	Long: `Change to a worktree directory by launching a new shell.

This command launches a new shell session in the selected worktree directory.
Type 'exit' to return to the original directory.

With shell integration (cd.launch_shell=false), this command changes the
current shell's directory instead of launching a new shell.

If multiple worktrees match the pattern, an interactive fuzzy finder will be shown.
If no pattern is provided, all worktrees will be shown in the fuzzy finder.`,
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

const envCdShim = "__GWQ_CD_SHIM"

func runCd(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// launch_shell=false without shell integration: show setup guidance
	if !cfg.Cd.LaunchShell && os.Getenv(envCdShim) != "1" {
		return fmt.Errorf(`'gwq cd' requires shell integration when cd.launch_shell is false.

To enable shell integration, add this to your shell configuration:

  # bash (~/.bashrc)
  source <(gwq completion bash)

  # zsh (~/.zshrc)
  source <(gwq completion zsh)

  # fish (~/.config/fish/config.fish)
  gwq completion fish | source

Then reload your shell:
  exec $SHELL

Or, to use the old behavior (launching a new shell), run:
  gwq config set cd.launch_shell true`)
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

	// Called from shell wrapper: print path to stdout
	if !cfg.Cd.LaunchShell && os.Getenv(envCdShim) == "1" {
		fmt.Println(worktreePath)
		return nil
	}

	// Default behavior: launch a new shell
	return LaunchShell(worktreePath)
}
