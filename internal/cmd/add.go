package cmd

import (
	"fmt"
	"time"

	"github.com/d-kuro/gwq/internal/duration"
	"github.com/d-kuro/gwq/internal/registry"
	"github.com/spf13/cobra"
)

var (
	addBranch      bool
	addInteractive bool
	addForce       bool
	addStay        bool
	addExpires     string
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
  gwq add -s feature/new-ui

  # Create worktree expiring in 7 days
  gwq add --expires 7d feature/experiment

  # Create worktree expiring in 1 hour
  gwq add --expires 1h hotfix/quick-test`,
	RunE:              runAdd,
	ValidArgsFunction: getBranchCompletions,
}

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().BoolVarP(&addBranch, "branch", "b", false, "Create new branch")
	addCmd.Flags().BoolVarP(&addInteractive, "interactive", "i", false, "Select branch using fuzzy finder")
	addCmd.Flags().BoolVarP(&addForce, "force", "f", false, "Overwrite existing directory")
	addCmd.Flags().BoolVarP(&addStay, "stay", "s", false, "Stay in worktree directory after creation")
	addCmd.Flags().StringVar(&addExpires, "expires", "", "Set expiration (e.g., 1d, 7d, 1h)")
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

		// Register worktree with expiration if --expires is specified
		if addExpires != "" {
			d, err := duration.Parse(addExpires)
			if err != nil {
				return fmt.Errorf("invalid expiration duration: %w", err)
			}

			expiresAt := time.Now().Add(d)

			reg, err := registry.New()
			if err != nil {
				return fmt.Errorf("failed to open registry: %w", err)
			}

			// Get repository URL for the entry
			repoURL, _ := ctx.Git.GetRepositoryURL()

			entry := &registry.WorktreeEntry{
				Repository: repoURL,
				Branch:     branch,
				Path:       worktreePath,
				IsMain:     false,
				ExpiresAt:  &expiresAt,
			}

			if err := reg.Register(entry); err != nil {
				return fmt.Errorf("failed to register worktree: %w", err)
			}

			ctx.Printer.PrintSuccess(fmt.Sprintf("Worktree expires at %s", expiresAt.Format(time.RFC3339)))
		}

		if addStay {
			_ = LaunchShell(worktreePath)
		}

		return nil
	})(cmd, args)
}
