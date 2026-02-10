package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/d-kuro/gwq/internal/config"
	"github.com/d-kuro/gwq/internal/discovery"
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
  gwq exec -g project:feature -- make build

  # Execute in global worktree with ghq integration (includes main repos)
  gwq exec -g --ghq project:main -- git status`,
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

// execArgs holds parsed execution arguments
type execArgs struct {
	pattern     string
	commandArgs []string
	global      bool
	stay        bool
	ghq         *bool // nil = not specified, true/false = explicitly set
}

// ptrTo returns a pointer to the given value.
func ptrTo[T any](v T) *T {
	return &v
}

// parseExecArgs manually parses command arguments since DisableFlagParsing is true
func parseExecArgs(cmd *cobra.Command, args []string) (*execArgs, error) {
	result := &execArgs{}
	separatorIndex := -1

	// Parse flags manually until we hit the "--" separator
	for i, arg := range args {
		if arg == "--" {
			separatorIndex = i
			break
		}

		switch arg {
		case "-g", "--global":
			result.global = true
		case "-s", "--stay":
			result.stay = true
		case "--ghq", "--ghq=true":
			result.ghq = ptrTo(true)
		case "--ghq=false":
			result.ghq = ptrTo(false)
		case "-h", "--help":
			return nil, cmd.Help()
		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("unknown flag: %s", arg)
			}
			// First non-flag argument is the pattern
			if result.pattern == "" {
				result.pattern = arg
			}
		}
	}

	if separatorIndex == -1 {
		return nil, fmt.Errorf("missing -- separator. Use: gwq exec [pattern] -- command [args...]")
	}

	// Extract command and its arguments after the separator
	if separatorIndex+1 >= len(args) {
		return nil, fmt.Errorf("no command specified after --")
	}
	result.commandArgs = args[separatorIndex+1:]

	return result, nil
}

func runExec(cmd *cobra.Command, args []string) error {
	parsedArgs, err := parseExecArgs(cmd, args)
	if err != nil {
		return err
	}

	// Check if parsedArgs is nil (e.g., when --help is used)
	if parsedArgs == nil {
		return nil
	}

	// Set global variables for backward compatibility
	execGlobal = parsedArgs.global
	execStay = parsedArgs.stay

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Override ghq.enabled if --ghq flag was explicitly set
	if parsedArgs.ghq != nil {
		cfg.Ghq.Enabled = *parsedArgs.ghq
	}

	var worktreePath string
	if parsedArgs.global {
		worktreePath, err = getGlobalWorktreePathForExec(cfg, parsedArgs.pattern)
	} else {
		worktreePath, err = getLocalWorktreePathForExec(cfg, parsedArgs.pattern)
	}

	if err != nil {
		return err
	}

	// Execute the command in the worktree directory
	return executeInWorktree(worktreePath, parsedArgs.commandArgs, parsedArgs.stay)
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
