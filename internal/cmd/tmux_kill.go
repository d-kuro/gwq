package cmd

import (
	"fmt"
	"strings"

	"github.com/d-kuro/gwq/internal/config"
	"github.com/d-kuro/gwq/internal/tmux"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/cobra"
)

var (
	tmuxKillInteractive bool
	tmuxKillAll         bool
	tmuxKillForce       bool
)

var tmuxKillCmd = &cobra.Command{
	Use:   "kill [pattern]",
	Short: "Terminate tmux sessions",
	Long: `Terminate tmux sessions matching the given pattern.

If multiple sessions match the pattern, an interactive selection will be shown.
If no pattern is provided, an interactive session selector will be displayed.`,
	Example: `  # Terminate session matching 'auth'
  gwq tmux kill auth

  # Use interactive selection
  gwq tmux kill -i

  # Terminate all sessions (with confirmation)
  gwq tmux kill --all

  # Force kill without confirmation
  gwq tmux kill --all --force`,
	RunE: runTmuxKill,
}

func init() {
	tmuxCmd.AddCommand(tmuxKillCmd)

	tmuxKillCmd.Flags().BoolVarP(&tmuxKillInteractive, "interactive", "i", false, "Always use interactive selection")
	tmuxKillCmd.Flags().BoolVar(&tmuxKillAll, "all", false, "Terminate all sessions")
	tmuxKillCmd.Flags().BoolVar(&tmuxKillForce, "force", false, "Skip confirmation prompts")
}

func runTmuxKill(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	sessionManager := tmux.NewSessionManager(nil)

	sessions, err := sessionManager.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No tmux sessions found")
		return nil
	}

	var sessionsToKill []*tmux.Session

	switch {
	case tmuxKillAll:
		sessionsToKill = sessions
	case len(args) == 0 || tmuxKillInteractive:
		// Interactive selection using fuzzy finder
		selected, err := selectSessionsToKillWithFinder(sessions, cfg)
		if err != nil {
			return fmt.Errorf("session selection cancelled: %w", err)
		}
		sessionsToKill = selected
	default:
		// Pattern matching
		pattern := args[0]
		matches := findMatchingSessions(sessions, pattern)
		if len(matches) == 0 {
			return fmt.Errorf("no session found matching pattern: %s", pattern)
		} else if len(matches) == 1 {
			sessionsToKill = matches
		} else {
			// Multiple matches - use fuzzy finder
			selected, err := selectSessionsToKillWithFinder(matches, cfg)
			if err != nil {
				return fmt.Errorf("session selection cancelled: %w", err)
			}
			sessionsToKill = selected
		}
	}

	if len(sessionsToKill) == 0 {
		fmt.Println("No sessions selected for termination")
		return nil
	}

	// Confirmation unless force flag is used
	if !tmuxKillForce {
		if !confirmKillSessions(sessionsToKill) {
			fmt.Println("Operation cancelled")
			return nil
		}
	}

	// Kill selected sessions
	return killSessions(sessionManager, sessionsToKill)
}

func selectSessionsToKillWithFinder(sessions []*tmux.Session, cfg *models.Config) ([]*tmux.Session, error) {
	return createSessionFinder(cfg).SelectMultipleSessions(sessions)
}

func confirmKillSessions(sessions []*tmux.Session) bool {
	fmt.Printf("\nThis will terminate %d session(s):\n", len(sessions))
	for _, session := range sessions {
		fmt.Printf("  ● %s/%s\n", session.Context, session.Identifier)
	}

	fmt.Print("\nAre you sure? (y/N): ")
	var response string
	_, _ = fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func killSessions(sessionManager *tmux.SessionManager, sessions []*tmux.Session) error {
	var failedCount int

	for _, session := range sessions {
		fmt.Printf("Terminating session %s/%s...", session.Context, session.Identifier)

		if err := sessionManager.KillSessionDirect(session); err != nil {
			fmt.Printf(" FAILED: %v\n", err)
			failedCount++
		} else {
			fmt.Printf(" OK\n")
		}
	}

	successCount := len(sessions) - failedCount
	fmt.Printf("\nTerminated %d session(s)", successCount)

	if failedCount > 0 {
		fmt.Printf(" (%d failed)", failedCount)
		return fmt.Errorf("%d out of %d sessions failed to terminate", failedCount, len(sessions))
	}

	fmt.Println()
	return nil
}
