package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/d-kuro/gwq/internal/config"
	"github.com/d-kuro/gwq/internal/discovery"
	"github.com/d-kuro/gwq/internal/finder"
	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/worktree"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/cobra"
)

var (
	execGlobal bool
	execStay   bool
)

var execCmd = &cobra.Command{
	Use:                "exec [pattern] -- command [args...]",
	Short:              "Execute command in worktree directory",
	DisableFlagParsing: true,
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
  gwq exec -g project:feature -- make build`,
	Args: cobra.ArbitraryArgs,
	RunE: runExec,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Disable file completion after --
		for _, arg := range args {
			if arg == "--" {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		}

		if len(args) == 0 || (len(args) == 1 && !strings.HasPrefix(args[0], "-")) {
			return getWorktreeCompletions(cmd, args, toComplete)
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	rootCmd.AddCommand(execCmd)

	execCmd.Flags().BoolVarP(&execGlobal, "global", "g", false, "Execute in global worktree")
	execCmd.Flags().BoolVarP(&execStay, "stay", "s", false, "Stay in worktree directory after command execution")
}

func runExec(cmd *cobra.Command, args []string) error {
	// Since DisableFlagParsing is true, we need to manually parse flags
	var pattern string
	var commandArgs []string
	dashDashIndex := -1

	// Parse flags manually
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			dashDashIndex = i
			break
		}

		switch arg {
		case "-g", "--global":
			execGlobal = true
			i++
		case "-s", "--stay":
			execStay = true
			i++
		case "-h", "--help":
			return cmd.Help()
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown flag: %s", arg)
			}
			// This is the pattern
			if pattern == "" {
				pattern = arg
			}
			i++
		}
	}

	if dashDashIndex == -1 {
		return fmt.Errorf("missing -- separator. Use: gwq exec [pattern] -- command [args...]")
	}

	// Extract command and its arguments
	if dashDashIndex+1 >= len(args) {
		return fmt.Errorf("no command specified after --")
	}
	commandArgs = args[dashDashIndex+1:]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var worktreePath string

	if execGlobal {
		worktreePath, err = getGlobalWorktreePathForExec(cfg, pattern)
	} else {
		worktreePath, err = getLocalWorktreePathForExec(cfg, pattern)
	}

	if err != nil {
		return err
	}

	// Execute the command in the worktree directory
	return executeInWorktree(worktreePath, commandArgs, execStay)
}

func getLocalWorktreePathForExec(cfg *models.Config, pattern string) (string, error) {
	g, err := git.NewFromCwd()
	if err != nil {
		// Not in a git repo, try global
		return getGlobalWorktreePathForExec(cfg, pattern)
	}

	wm := worktree.New(g, cfg)

	if pattern != "" {
		// Get all matching worktrees
		matches, err := wm.GetMatchingWorktrees(pattern)
		if err != nil {
			return "", err
		}

		if len(matches) == 0 {
			return "", fmt.Errorf("no worktree found matching pattern: %s", pattern)
		} else if len(matches) == 1 {
			return matches[0].Path, nil
		} else {
			// Multiple matches - use fuzzy finder
			f := finder.NewWithUI(g, &cfg.Finder, &cfg.UI)
			selected, err := f.SelectWorktree(matches)
			if err != nil {
				return "", fmt.Errorf("worktree selection cancelled")
			}
			return selected.Path, nil
		}
	} else {
		// No pattern - show all worktrees
		worktrees, err := wm.List()
		if err != nil {
			return "", err
		}

		if len(worktrees) == 0 {
			return "", fmt.Errorf("no worktrees found")
		}

		if len(worktrees) == 1 {
			return worktrees[0].Path, nil
		}

		f := finder.NewWithUI(g, &cfg.Finder, &cfg.UI)
		selected, err := f.SelectWorktree(worktrees)
		if err != nil {
			return "", fmt.Errorf("worktree selection cancelled")
		}
		return selected.Path, nil
	}
}

func getGlobalWorktreePathForExec(cfg *models.Config, pattern string) (string, error) {
	entries, err := discovery.DiscoverGlobalWorktrees(cfg.Worktree.BaseDir)
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no worktrees found across all repositories")
	}

	var selected *discovery.GlobalWorktreeEntry

	if pattern != "" {
		// Pattern matching
		matches := discovery.FilterGlobalWorktrees(entries, pattern)

		if len(matches) == 0 {
			return "", fmt.Errorf("no worktree matches pattern: %s", pattern)
		} else if len(matches) == 1 {
			selected = matches[0]
		} else {
			// Multiple matches - use fuzzy finder
			worktrees := discovery.ConvertToWorktreeModels(matches, true)

			g := &git.Git{}
			f := finder.NewWithUI(g, &cfg.Finder, &cfg.UI)
			selectedWT, err := f.SelectWorktree(worktrees)
			if err != nil {
				return "", fmt.Errorf("worktree selection cancelled")
			}

			// Find the corresponding entry
			for _, entry := range matches {
				if entry.Path == selectedWT.Path {
					selected = entry
					break
				}
			}
		}
	} else {
		// No pattern - show all in fuzzy finder
		worktrees := discovery.ConvertToWorktreeModels(entries, true)

		g := &git.Git{}
		f := finder.NewWithUI(g, &cfg.Finder, &cfg.UI)
		selectedWT, err := f.SelectWorktree(worktrees)
		if err != nil {
			return "", fmt.Errorf("worktree selection cancelled")
		}

		// Find the corresponding entry
		for _, entry := range entries {
			if entry.Path == selectedWT.Path {
				selected = entry
				break
			}
		}
	}

	if selected == nil {
		return "", fmt.Errorf("no worktree selected")
	}

	return selected.Path, nil
}

func executeInWorktree(worktreePath string, commandArgs []string, stay bool) error {
	if stay {
		// Launch a new shell in the worktree directory
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}

		fmt.Printf("Launching shell in: %s\n", worktreePath)
		fmt.Println("Type 'exit' to return to the original directory")

		cmd := exec.Command(shell)
		cmd.Dir = worktreePath
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	// Execute the command in the worktree directory
	var cmd *exec.Cmd
	if len(commandArgs) == 1 {
		cmd = exec.Command(commandArgs[0])
	} else {
		cmd = exec.Command(commandArgs[0], commandArgs[1:]...)
	}

	cmd.Dir = worktreePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
