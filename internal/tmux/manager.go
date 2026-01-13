package tmux

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/d-kuro/gwq/internal/utils"
)

type SessionManager struct {
	config  *SessionConfig
	tmuxCmd TmuxInterface
}

func NewSessionManager(config *SessionConfig) *SessionManager {
	if config == nil {
		config = DefaultSessionConfig()
	}

	return &SessionManager{
		config:  config,
		tmuxCmd: NewTmuxCommand(config.TmuxCommand),
	}
}

func (sm *SessionManager) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	sessionName := fmt.Sprintf("gwq-%s-%s-%s", opts.Context, opts.Identifier, time.Now().Format("20060102150405"))

	// Create session with or without command
	if opts.Command != "" {
		// Create session with command - when command finishes, session will automatically terminate
		if err := sm.tmuxCmd.NewSessionWithCommandContext(ctx, sessionName, opts.WorkingDir, opts.Command); err != nil {
			return nil, fmt.Errorf("failed to create tmux session with command: %w", err)
		}
	} else {
		// Create session without command (traditional behavior)
		if err := sm.tmuxCmd.NewSessionContext(ctx, sessionName, opts.WorkingDir); err != nil {
			return nil, fmt.Errorf("failed to create tmux session: %w", err)
		}
	}

	if err := sm.tmuxCmd.SetOptionContext(ctx, sessionName, "history-limit", sm.config.HistoryLimit); err != nil {
		_ = sm.tmuxCmd.KillSession(sessionName)
		return nil, fmt.Errorf("failed to set history limit: %w", err)
	}

	session := &Session{
		ID:          utils.GenerateID(),
		SessionName: sessionName,
		Context:     opts.Context,
		Identifier:  opts.Identifier,
		WorkingDir:  opts.WorkingDir,
		Command:     opts.Command,
		StartTime:   time.Now(),
		HistorySize: sm.config.HistoryLimit,
		Metadata:    opts.Metadata,
	}

	return session, nil
}

func (sm *SessionManager) ListSessions() ([]*Session, error) {
	tmuxSessions, err := sm.tmuxCmd.ListSessionsDetailed()
	if err != nil {
		return nil, err
	}

	var sessions []*Session
	for _, tmuxSession := range tmuxSessions {
		// Only show gwq-managed sessions
		if !strings.HasPrefix(tmuxSession.Name, "gwq-") {
			continue
		}

		session := sm.parseSessionFromTmux(tmuxSession)
		if session != nil {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

func (sm *SessionManager) parseSessionFromTmux(info *SessionInfo) *Session {
	// Parse session name format: gwq-{context}-{identifier}-{timestamp}
	re := regexp.MustCompile(`^gwq-([^-]+)-(.+)-(\d{14})$`)
	matches := re.FindStringSubmatch(info.Name)
	if len(matches) != 4 {
		return nil
	}

	context := matches[1]
	identifier := matches[2]
	timestamp := matches[3]

	startTime, err := time.Parse("20060102150405", timestamp)
	if err != nil {
		startTime = time.Now()
	}

	// Determine command from session name or current command
	command := info.CurrentCommand

	if command == "bash" || command == "zsh" || command == "sh" {
		// If shell is running, the original command likely finished but session is still active
		command = "Shell session (original command completed)"
	}

	return &Session{
		ID:          utils.GenerateShortID(),
		SessionName: info.Name,
		Context:     context,
		Identifier:  identifier,
		WorkingDir:  info.WorkingDir,
		Command:     command,
		StartTime:   startTime,
		HistorySize: sm.config.HistoryLimit,
		Metadata:    map[string]string{},
	}
}

func (sm *SessionManager) GetSession(id string) (*Session, error) {
	sessions, err := sm.ListSessions()
	if err != nil {
		return nil, err
	}

	for _, session := range sessions {
		if session.ID == id ||
			strings.Contains(session.SessionName, id) ||
			session.Identifier == id ||
			strings.Contains(session.Identifier, id) ||
			strings.Contains(session.Context, id) {
			return session, nil
		}
	}

	return nil, fmt.Errorf("session not found: %s", id)
}

func (sm *SessionManager) KillSession(id string) error {
	session, err := sm.GetSession(id)
	if err != nil {
		return err
	}

	return sm.KillSessionDirect(session)
}

func (sm *SessionManager) KillSessionDirect(session *Session) error {
	if sm.tmuxCmd.HasSession(session.SessionName) {
		if err := sm.tmuxCmd.KillSession(session.SessionName); err != nil {
			return fmt.Errorf("failed to kill tmux session: %w", err)
		}
	}

	return nil
}

func (sm *SessionManager) AttachSession(id string) error {
	session, err := sm.GetSession(id)
	if err != nil {
		return err
	}

	return sm.AttachSessionDirect(session)
}

func (sm *SessionManager) AttachSessionDirect(session *Session) error {
	if !sm.tmuxCmd.HasSession(session.SessionName) {
		return fmt.Errorf("tmux session %s no longer exists", session.SessionName)
	}

	return sm.tmuxCmd.AttachSession(session.SessionName)
}

// HasSession checks if a session exists
func (sm *SessionManager) HasSession(sessionName string) bool {
	return sm.tmuxCmd.HasSession(sessionName)
}
