package tmux

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// TmuxInterface defines the contract for tmux operations
type TmuxInterface interface {
	NewSession(name, workDir string) error
	NewSessionContext(ctx context.Context, name, workDir string) error
	NewSessionWithCommandContext(ctx context.Context, name, workDir, command string) error
	SetOption(sessionName, option string, value any) error
	SetOptionContext(ctx context.Context, sessionName, option string, value any) error
	ListSessions() ([]string, error)
	ListSessionsDetailed() ([]*SessionInfo, error)
	KillSession(sessionName string) error
	AttachSession(sessionName string) error
	HasSession(sessionName string) bool
}

// SessionManagerInterface defines the contract for session management
type SessionManagerInterface interface {
	CreateSession(ctx context.Context, opts SessionOptions) (*Session, error)
	ListSessions() ([]*Session, error)
	GetSession(id string) (*Session, error)
	KillSession(id string) error
	KillSessionDirect(session *Session) error
	AttachSession(id string) error
	AttachSessionDirect(session *Session) error
	HasSession(sessionName string) bool
}

type TmuxCommand struct {
	command string
}

func NewTmuxCommand(command string) *TmuxCommand {
	if command == "" {
		command = "tmux"
	}
	return &TmuxCommand{command: command}
}

func (t *TmuxCommand) NewSession(name, workDir string) error {
	args := []string{"new-session", "-d", "-s", name}
	if workDir != "" {
		args = append(args, "-c", workDir)
	}
	return t.runCommand(args...)
}

func (t *TmuxCommand) NewSessionContext(ctx context.Context, name, workDir string) error {
	args := []string{"new-session", "-d", "-s", name}
	if workDir != "" {
		args = append(args, "-c", workDir)
	}
	return t.RunCommandContext(ctx, args...)
}

func (t *TmuxCommand) NewSessionWithCommandContext(ctx context.Context, name, workDir, command string) error {
	args := []string{"new-session", "-d", "-s", name}
	if workDir != "" {
		args = append(args, "-c", workDir)
	}
	if command != "" {
		args = append(args, command)
	}
	return t.RunCommandContext(ctx, args...)
}

func (t *TmuxCommand) SetOption(sessionName, option string, value any) error {
	args := []string{"set-option", "-t", sessionName, option, fmt.Sprintf("%v", value)}
	return t.runCommand(args...)
}

func (t *TmuxCommand) SetOptionContext(ctx context.Context, sessionName, option string, value any) error {
	args := []string{"set-option", "-t", sessionName, option, fmt.Sprintf("%v", value)}
	return t.RunCommandContext(ctx, args...)
}

func (t *TmuxCommand) ListSessions() ([]string, error) {
	args := []string{"list-sessions", "-F", "#{session_name}"}
	output, err := t.runCommandOutput(args...)
	if err != nil {
		if strings.Contains(err.Error(), "no server running") {
			return []string{}, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var sessions []string
	for _, line := range lines {
		if line != "" {
			sessions = append(sessions, line)
		}
	}
	return sessions, nil
}

func (t *TmuxCommand) ListSessionsDetailed() ([]*SessionInfo, error) {
	format := "#{session_name}:#{session_created}:#{session_activity}:#{session_attached}:#{pane_current_command}:#{pane_current_path}"
	args := []string{"list-sessions", "-F", format}
	output, err := t.runCommandOutput(args...)
	if err != nil {
		if strings.Contains(err.Error(), "no server running") {
			return []*SessionInfo{}, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var sessions []*SessionInfo
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) < 6 {
			continue
		}

		sessionInfo := &SessionInfo{
			Name:           parts[0],
			Created:        parts[1],
			Activity:       parts[2],
			Attached:       parts[3],
			CurrentCommand: parts[4],
			WorkingDir:     parts[5],
		}

		sessions = append(sessions, sessionInfo)
	}
	return sessions, nil
}

type SessionInfo struct {
	Name           string
	Created        string
	Activity       string
	Attached       string
	CurrentCommand string
	WorkingDir     string
}

func (t *TmuxCommand) KillSession(sessionName string) error {
	args := []string{"kill-session", "-t", sessionName}
	return t.runCommand(args...)
}

func (t *TmuxCommand) AttachSession(sessionName string) error {
	args := []string{"attach-session", "-t", sessionName}
	cmd := exec.Command(t.command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (t *TmuxCommand) HasSession(sessionName string) bool {
	args := []string{"has-session", "-t", sessionName}
	err := t.runCommand(args...)
	return err == nil
}

func (t *TmuxCommand) runCommand(args ...string) error {
	cmd := exec.Command(t.command, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("tmux command failed: %w, stderr: %s", err, stderr.String())
	}
	return nil
}

func (t *TmuxCommand) runCommandOutput(args ...string) (string, error) {
	cmd := exec.Command(t.command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("tmux command failed: %w, stderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func (t *TmuxCommand) RunCommandContext(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, t.command, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("tmux command failed: %w, stderr: %s", err, stderr.String())
	}
	return nil
}
