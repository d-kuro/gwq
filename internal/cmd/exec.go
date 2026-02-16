package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/d-kuro/gwq/internal/config"
	"github.com/d-kuro/gwq/internal/discovery"
	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/worktree"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec [pattern] -- command [args...]",
	Short: "Execute command in worktree directory",
	Long: `Execute a command in a worktree directory without changing the current directory.

The command runs in a subshell with the working directory set to the selected worktree.
Use -- to separate gwq arguments from the command to execute.

If multiple worktrees match the pattern, an interactive fuzzy finder will be shown.
If no pattern is provided, all worktrees will be shown in the fuzzy finder.`,
	Example: `  # Run tests in a feature branch
  gwq exec feature -- npm test

  # Pull latest changes in main branch
  gwq exec main -- git pull

  # Run multiple commands
  gwq exec feature -- sh -c "git pull && npm install && npm test"

  # Stay in the worktree directory after command execution
  gwq exec --stay feature -- npm install

  # Execute in global worktree
  gwq exec -g project:feature -- make build

  # Execute in global worktree with ghq integration (includes main repos)
  gwq exec -g --ghq project:main -- git status`,
	Args: cobra.ArbitraryArgs,
	RunE: runExec,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Disable file completion after --
		if slices.Contains(args, "--") {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if len(args) == 0 || (len(args) == 1 && !strings.HasPrefix(args[0], "-")) {
			return getWorktreeCompletions(cmd, args, toComplete)
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	rootCmd.AddCommand(execCmd)

	execCmd.Flags().BoolP("global", "g", false, "Execute in global worktree")
	execCmd.Flags().BoolP("stay", "s", false, "Stay in worktree directory after command execution")
	execCmd.Flags().Bool("ghq", false, "Enable ghq integration mode (--ghq or --ghq=false)")
}

func runExec(cmd *cobra.Command, args []string) error {
	dashAt := cmd.ArgsLenAtDash()
	if dashAt == -1 {
		return fmt.Errorf("missing -- separator. Use: gwq exec [pattern] -- command [args...]")
	}

	commandArgs := args[dashAt:]
	if len(commandArgs) == 0 {
		return fmt.Errorf("no command specified after --")
	}

	// Extract pattern from positional args before --
	var pattern string
	if dashAt > 0 {
		pattern = args[0]
	}

	global, _ := cmd.Flags().GetBool("global")
	stay, _ := cmd.Flags().GetBool("stay")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Override ghq.enabled if --ghq flag was explicitly set
	if cmd.Flags().Changed("ghq") {
		ghqVal, _ := cmd.Flags().GetBool("ghq")
		cfg.Ghq.Enabled = ghqVal
	}

	var worktreePath string
	if global {
		worktreePath, err = getGlobalWorktreePathForExec(cfg, pattern)
	} else {
		worktreePath, err = getLocalWorktreePathForExec(cfg, pattern)
	}

	if err != nil {
		return err
	}

	// Execute the command in the worktree directory
	return executeInWorktree(worktreePath, commandArgs, stay)
}

func getLocalWorktreePathForExec(cfg *models.Config, pattern string) (string, error) {
	g, err := git.NewFromCwd()
	if err != nil {
		// Not in a git repo, try global
		return getGlobalWorktreePathForExec(cfg, pattern)
	}

	wm := worktree.New(g, cfg)

	// Get worktrees based on pattern
	var worktrees []models.Worktree
	if pattern != "" {
		worktrees, err = wm.GetMatchingWorktrees(pattern)
		if err != nil {
			return "", err
		}
		if len(worktrees) == 0 {
			return "", fmt.Errorf("no worktree found matching pattern: %s", pattern)
		}
	} else {
		worktrees, err = wm.List()
		if err != nil {
			return "", err
		}
		if len(worktrees) == 0 {
			return "", fmt.Errorf("no worktrees found")
		}
	}

	// Single match - return directly
	if len(worktrees) == 1 {
		return worktrees[0].Path, nil
	}

	// Multiple matches - use fuzzy finder
	f := CreateFinder(g, cfg)
	selected, err := f.SelectWorktree(worktrees)
	if err != nil {
		return "", fmt.Errorf("worktree selection cancelled")
	}
	return selected.Path, nil
}

func getGlobalWorktreePathForExec(cfg *models.Config, pattern string) (string, error) {
	entries, err := discoverGlobalEntries(cfg)
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no worktrees found across all repositories")
	}

	// Filter by pattern if provided
	candidates := entries
	if pattern != "" {
		candidates = discovery.FilterGlobalWorktrees(entries, pattern)
		if len(candidates) == 0 {
			return "", fmt.Errorf("no worktree matches pattern: %s", pattern)
		}
	}

	// Single match - return directly
	if len(candidates) == 1 {
		return candidates[0].Path, nil
	}

	// Multiple matches - use fuzzy finder
	return selectGlobalWorktreeWithFinder(cfg, candidates)
}

// selectGlobalWorktreeWithFinder shows a fuzzy finder to select from multiple worktrees.
func selectGlobalWorktreeWithFinder(cfg *models.Config, entries []*discovery.GlobalWorktreeEntry) (string, error) {
	worktrees := discovery.ConvertToWorktreeModels(entries, true)

	f := CreateGlobalFinder(cfg)
	selected, err := f.SelectWorktree(worktrees)
	if err != nil {
		return "", fmt.Errorf("worktree selection cancelled")
	}

	return selected.Path, nil
}

func executeInWorktree(worktreePath string, commandArgs []string, stay bool) error {
	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)

	cmd.Dir = worktreePath
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	if stay {
		// Launch a new shell in the worktree directory after command execution
		// Run the shell regardless of the original command's exit status
		_ = LaunchShell(worktreePath)
	}

	return err
}
