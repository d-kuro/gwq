package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/d-kuro/gwq/internal/config"
	"github.com/d-kuro/gwq/internal/discovery"
	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/tmux"
	"github.com/d-kuro/gwq/internal/worktree"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/cobra"
)

var (
	tmuxRunWorktree    string
	tmuxRunIdentifier  string
	tmuxRunContext     string
	tmuxRunDetach      bool
	tmuxRunAutoCleanup bool
)

var tmuxRunCmd = &cobra.Command{
	Use:   "run [command]",
	Short: "Run command in new tmux session",
	Long: `Run command in a new tmux session with persistence and monitoring.

Creates a new tmux session and executes the specified command within it.
By default, the session persists after command completion (tmux native behavior).
The session can be detached, monitored, and attached to later.`,
	Example: `  # Run command (session persists after completion)
  gwq tmux run "npm run dev"

  # Run with automatic session cleanup on completion
  gwq tmux run --auto-cleanup "make test"

  # Run in specific worktree
  gwq tmux run -w feature/auth "make test"

  # Run with custom identifier
  gwq tmux run --id test-suite "go test -v ./..."

  # Run with custom context
  gwq tmux run --context build "npm run build"

  # Run and stay attached
  gwq tmux run --no-detach "npm start"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTmuxRun,
}

func init() {
	tmuxCmd.AddCommand(tmuxRunCmd)

	tmuxRunCmd.Flags().StringVarP(&tmuxRunWorktree, "worktree", "w", "", "Run in specific worktree")
	tmuxRunCmd.Flags().StringVar(&tmuxRunIdentifier, "id", "", "Custom identifier for the session")
	tmuxRunCmd.Flags().StringVar(&tmuxRunContext, "context", "", "Context for the session (default: 'run')")
	tmuxRunCmd.Flags().BoolVar(&tmuxRunDetach, "no-detach", false, "Stay attached to the session after creation")
	tmuxRunCmd.Flags().BoolVar(&tmuxRunAutoCleanup, "auto-cleanup", false, "Automatically kill session when command completes")
}

func runTmuxRun(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	command := strings.Join(args, " ")
	workingDir, err := determineWorkingDirectory(cfg)
	if err != nil {
		return fmt.Errorf("failed to determine working directory: %w", err)
	}

	// Set defaults
	context := tmuxRunContext
	if context == "" {
		context = "run"
	}

	identifier := tmuxRunIdentifier
	if identifier == "" {
		// Generate identifier from command or working directory
		identifier = generateIdentifierFromCommand(command, workingDir)
	}

	// Modify command for auto-cleanup if requested
	finalCommand := command
	if tmuxRunAutoCleanup {
		// Add a hook to kill the session when the command completes
		finalCommand = fmt.Sprintf("(%s); tmux kill-session -t $TMUX_PANE", command)
	}

	sessionManager := tmux.NewSessionManager(nil)

	opts := tmux.SessionOptions{
		Context:    context,
		Identifier: identifier,
		WorkingDir: workingDir,
		Command:    finalCommand,
		Metadata: map[string]string{
			"created_by":   "gwq tmux run",
			"auto_cleanup": fmt.Sprintf("%t", tmuxRunAutoCleanup),
			"orig_command": command,
		},
	}

	session, err := sessionManager.CreateSession(cmd.Context(), opts)
	if err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	fmt.Printf("Created tmux session: %s\n", session.SessionName)
	fmt.Printf("Session ID: %s\n", session.ID)
	fmt.Printf("Command: %s\n", command)
	fmt.Printf("Working Directory: %s\n", session.WorkingDir)

	if tmuxRunAutoCleanup {
		fmt.Printf("Auto-cleanup: Session will be deleted when command completes\n")
	} else {
		fmt.Printf("Persistence: Session will remain after command completes\n")
	}

	if !tmuxRunDetach {
		fmt.Printf("\nAttaching to session (use Ctrl+B, D to detach)...\n")
		return sessionManager.AttachSessionDirect(session)
	}

	fmt.Printf("\nSession created and running in background.\n")
	fmt.Printf("Use 'gwq tmux attach %s' to attach to this session.\n", identifier)
	fmt.Printf("Use 'gwq tmux list' to see all sessions.\n")

	return nil
}

func determineWorkingDirectory(cfg *models.Config) (string, error) {
	if tmuxRunWorktree != "" {
		// Worktree specified - find and validate it
		return resolveWorktreePath(tmuxRunWorktree, cfg)
	}

	// Use current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	return cwd, nil
}

func resolveWorktreePath(worktreePattern string, cfg *models.Config) (string, error) {
	// Try to resolve as exact path first
	if filepath.IsAbs(worktreePattern) {
		if _, err := os.Stat(worktreePattern); err == nil {
			return worktreePattern, nil
		}
	}

	// Check if we're in a git repository and can resolve locally
	g, err := git.NewFromCwd()
	if err == nil {
		// Try to find matching worktree in current repository
		wm := worktree.New(g, cfg)
		matches, err := wm.GetMatchingWorktrees(worktreePattern)
		if err == nil && len(matches) > 0 {
			if len(matches) == 1 {
				return matches[0].Path, nil
			}
			return "", fmt.Errorf("multiple worktrees match pattern '%s', please be more specific", worktreePattern)
		}
	}

	// Try global worktree discovery
	entries, err := discovery.DiscoverGlobalWorktrees(cfg.Worktree.BaseDir)
	if err != nil {
		return "", fmt.Errorf("failed to discover worktrees: %w", err)
	}

	matches := discovery.FilterGlobalWorktrees(entries, worktreePattern)
	if len(matches) == 0 {
		return "", fmt.Errorf("no worktree found matching pattern: %s", worktreePattern)
	}

	if len(matches) > 1 {
		return "", fmt.Errorf("multiple worktrees match pattern '%s', please be more specific", worktreePattern)
	}

	return matches[0].Path, nil
}

func generateIdentifierFromCommand(command, workingDir string) string {
	// Extract a meaningful identifier from the command or directory
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "session"
	}

	// Use the first command word as base
	baseCmd := filepath.Base(parts[0])

	// Add directory context if available
	dirName := filepath.Base(workingDir)
	if dirName != "." && dirName != "/" {
		return fmt.Sprintf("%s-%s", baseCmd, dirName)
	}

	return baseCmd
}
