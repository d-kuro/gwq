package cmd

import (
	"fmt"
	"strings"

	"github.com/d-kuro/gwq/internal/config"
	"github.com/d-kuro/gwq/internal/finder"
	"github.com/d-kuro/gwq/internal/tmux"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/cobra"
)

var (
	tmuxAttachInteractive bool
)

var tmuxAttachCmd = &cobra.Command{
	Use:   "attach [pattern]",
	Short: "Attach to tmux session",
	Long: `Attach to tmux session matching the given pattern.

If multiple sessions match the pattern, an interactive fuzzy finder will be shown.
If no pattern is provided, all sessions will be shown in the fuzzy finder.`,
	Example: `  # Attach to session matching 'auth'
  gwq tmux attach auth

  # Attach with exact identifier match
  gwq tmux attach auth-impl

  # Use fuzzy finder to select from all sessions
  gwq tmux attach

  # Explicit fuzzy finder usage
  gwq tmux attach -i`,
	RunE: runTmuxAttach,
}

func init() {
	tmuxCmd.AddCommand(tmuxAttachCmd)

	tmuxAttachCmd.Flags().BoolVarP(&tmuxAttachInteractive, "interactive", "i", false, "Always use fuzzy finder")
}

func runTmuxAttach(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("no tmux sessions found")
	}

	var sessionToAttach *tmux.Session

	if len(args) == 0 || tmuxAttachInteractive {
		// No pattern provided or interactive mode - use fuzzy finder
		sessionToAttach, err = selectSessionWithFinder(sessions, cfg)
		if err != nil {
			return fmt.Errorf("session selection cancelled: %w", err)
		}
	} else {
		// Pattern provided - find matching sessions
		pattern := args[0]
		matches := findMatchingSessions(sessions, pattern)

		if len(matches) == 0 {
			return fmt.Errorf("no session found matching pattern: %s", pattern)
		} else if len(matches) == 1 {
			sessionToAttach = matches[0]
		} else {
			// Multiple matches - use fuzzy finder
			sessionToAttach, err = selectSessionWithFinder(matches, cfg)
			if err != nil {
				return fmt.Errorf("session selection cancelled: %w", err)
			}
		}
	}

	if sessionToAttach == nil {
		return fmt.Errorf("no session selected")
	}

	return sessionManager.AttachSessionDirect(sessionToAttach)
}

func findMatchingSessions(sessions []*tmux.Session, pattern string) []*tmux.Session {
	pattern = strings.ToLower(pattern)
	var matches []*tmux.Session

	for _, session := range sessions {
		if strings.Contains(strings.ToLower(session.SessionName), pattern) ||
			strings.Contains(strings.ToLower(session.Identifier), pattern) ||
			strings.Contains(strings.ToLower(session.Context), pattern) {
			matches = append(matches, session)
		}
	}

	return matches
}

func createSessionFinder(cfg *models.Config) *finder.Finder {
	// Use centralized factory for global finder
	return CreateGlobalFinder(cfg)
}

func selectSessionWithFinder(sessions []*tmux.Session, cfg *models.Config) (*tmux.Session, error) {
	return createSessionFinder(cfg).SelectSession(sessions)
}
