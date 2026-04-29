package cmd

import (
	"fmt"
	"io"
	"os"
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

		// Validate --expires duration before creating the worktree so an
		// invalid value does not leave a stray worktree behind. The actual
		// ExpiresAt is computed after creation so the effective lifetime
		// isn't shortened by setup time (e.g. repository_settings hooks).
		var expiresDuration time.Duration
		if addExpires != "" {
			d, err := duration.Parse(addExpires)
			if err != nil {
				return fmt.Errorf("invalid --expires duration %q: %w", addExpires, err)
			}
			expiresDuration = d
		}

		worktreePath, err := ctx.WorktreeManager.Add(branch, path, addBranch)
		if err != nil {
			return err
		}

		var expiresAt *time.Time
		if addExpires != "" {
			reg, err := registry.New()
			if err != nil {
				return fmt.Errorf("failed to open registry: %w", err)
			}

			repoURL, _ := ctx.Git.GetRepositoryURL()

			t := time.Now().Add(expiresDuration)
			expiresAt = &t

			entry := &registry.WorktreeEntry{
				Repository: repoURL,
				Branch:     branch,
				Path:       worktreePath,
				IsMain:     false,
				ExpiresAt:  expiresAt,
			}

			if err := reg.Register(entry); err != nil {
				return fmt.Errorf("failed to register worktree: %w", err)
			}
		}

		handleAddPostCreate(
			os.Stdout, os.Stderr,
			isCdShimActive(),
			ctx.Config.Cd.AutoCdOnAdd,
			addResult{
				Branch:    branch,
				Path:      worktreePath,
				Stay:      addStay,
				ExpiresAt: expiresAt,
			},
			LaunchShell,
		)
		return nil
	})(cmd, args)
}

// addResult carries the outcome of a successful `gwq add` into the
// post-create output routing.
type addResult struct {
	Branch    string
	Path      string
	Stay      bool
	ExpiresAt *time.Time
}

// handleAddPostCreate routes success messages and the worktree path to the
// appropriate destinations after a successful `gwq add`.
//
// Under shell integration (inShim), success messages go to stderr and stdout
// carries only the worktree path when a cd is wanted. When no cd is wanted,
// stdout is left empty — the shell wrapper's "-n" guard then refuses to cd
// into a success message.
func handleAddPostCreate(
	stdout, stderr io.Writer,
	inShim, autoCdOnAdd bool,
	r addResult,
	launchShell func(string) error,
) {
	wantCd := r.Stay || autoCdOnAdd

	msgDst := stdout
	if inShim {
		msgDst = stderr
	}
	_, _ = fmt.Fprintf(msgDst, "Created worktree for branch '%s'\n", r.Branch)
	if r.ExpiresAt != nil {
		_, _ = fmt.Fprintf(msgDst, "Worktree expires at %s\n", r.ExpiresAt.Format(time.RFC3339))
	}

	switch {
	case inShim && wantCd:
		_, _ = fmt.Fprintln(stdout, r.Path)
	case inShim:
		// stdout intentionally empty: shell wrapper's `if [[ -n "$__gwq_result" ]]`
		// guard prevents an incorrect cd.
	case r.Stay:
		_ = launchShell(r.Path)
	}
}
