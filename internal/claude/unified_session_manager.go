package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/d-kuro/gwq/internal/tmux"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/d-kuro/gwq/pkg/utils"
)

// UnifiedSessionManager manages tmux sessions for all execution types
type UnifiedSessionManager struct {
	tmuxManager *tmux.SessionManager
	config      *SessionConfig
}

// SessionConfig holds session configuration
type SessionConfig struct {
	Enabled      bool
	TmuxCommand  string
	HistoryLimit int
	ConfigDir    string
}

// NewUnifiedSessionManager creates a new unified session manager
func NewUnifiedSessionManager(config *models.ClaudeConfig) (*UnifiedSessionManager, error) {
	sessionConfig := &SessionConfig{
		Enabled:      true,
		TmuxCommand:  "tmux",
		HistoryLimit: 50000,
		ConfigDir:    config.ConfigDir,
	}

	tmuxConfig := &tmux.SessionConfig{
		Enabled:      sessionConfig.Enabled,
		TmuxCommand:  sessionConfig.TmuxCommand,
		HistoryLimit: sessionConfig.HistoryLimit,
	}

	tmuxManager := tmux.NewSessionManager(tmuxConfig, sessionConfig.ConfigDir)

	return &UnifiedSessionManager{
		tmuxManager: tmuxManager,
		config:      sessionConfig,
	}, nil
}

// CreateSession creates a tmux session for unified execution
func (usm *UnifiedSessionManager) CreateSession(ctx context.Context, execution *UnifiedExecution) (*tmux.Session, error) {
	// Create metadata file for the execution
	if err := usm.createMetadataFile(execution); err != nil {
		// Log error but don't fail the execution
		fmt.Printf("Warning: Failed to create metadata file: %v\n", err)
	}

	// Build Claude command based on execution type
	command := usm.buildClaudeCommand(execution)

	// Create session with unified metadata
	sessionOpts := tmux.SessionOptions{
		Context:    fmt.Sprintf("claude-%s", execution.ExecutionType),
		Identifier: execution.ExecutionID,
		WorkingDir: execution.WorkingDir,
		Command:    command,
		Metadata: map[string]string{
			"execution_id":   execution.ExecutionID,
			"execution_type": string(execution.ExecutionType),
			"session_id":     execution.SessionID,
			"repository":     execution.Repository,
			"priority":       execution.Priority,
		},
	}

	// Add task-specific metadata if present
	if execution.TaskInfo != nil {
		sessionOpts.Metadata["task_id"] = execution.TaskInfo.TaskID
		sessionOpts.Metadata["task_name"] = execution.TaskInfo.TaskName
		sessionOpts.Metadata["worktree"] = execution.TaskInfo.Worktree
		sessionOpts.Metadata["worktree_path"] = execution.TaskInfo.WorktreePath
	}

	return usm.tmuxManager.CreateSession(ctx, sessionOpts)
}

// buildClaudeCommand builds the appropriate Claude command for task execution
func (usm *UnifiedSessionManager) buildClaudeCommand(execution *UnifiedExecution) string {
	return usm.buildTaskCommand(execution)
}

// buildTaskCommand builds Claude command for task execution
func (usm *UnifiedSessionManager) buildTaskCommand(execution *UnifiedExecution) string {
	// Escape the prompt for shell
	escapedPrompt := utils.EscapeForShell(execution.Prompt)

	// Generate log file path based on execution ID and timestamp
	// Note: ExecutionID already includes type prefix (e.g., "task-{id}"), so use it directly
	logDir := filepath.Join(usm.config.ConfigDir, "logs", "executions")
	timestamp := time.Now().Format("20060102-150405")
	logFile := filepath.Join(logDir, fmt.Sprintf("%s-%s.jsonl", timestamp, execution.ExecutionID))

	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// If we can't create the log directory, proceed without logging to file
		return fmt.Sprintf(`claude --verbose --dangerously-skip-permissions --output-format stream-json -p "%s"`, escapedPrompt)
	}

	// Build command with task-specific flags and log capture
	return fmt.Sprintf(`claude --verbose --dangerously-skip-permissions --output-format stream-json -p "%s" | tee "%s"`, escapedPrompt, logFile)
}

// createMetadataFile creates a metadata file for the execution
func (usm *UnifiedSessionManager) createMetadataFile(execution *UnifiedExecution) error {
	// Create metadata directory
	metadataDir := filepath.Join(usm.config.ConfigDir, "logs", "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Generate metadata file path
	// Note: ExecutionID already includes type prefix (e.g., "task-{id}"), so use it directly
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.json", timestamp, execution.ExecutionID)

	metadataFile := filepath.Join(metadataDir, filename)

	// Create metadata content
	metadata := map[string]interface{}{
		"execution_id":      execution.ExecutionID,
		"session_id":        execution.SessionID,
		"execution_type":    execution.ExecutionType,
		"start_time":        execution.StartTime.Format(time.RFC3339),
		"status":            "running",
		"repository":        execution.Repository,
		"working_directory": execution.WorkingDir,
		"tmux_session":      fmt.Sprintf("gwq-claude-%s-%s", execution.ExecutionID, timestamp),
		"prompt":            execution.Prompt,
		"tags":              execution.Tags,
		"priority":          execution.Priority,
	}

	// Add task-specific information if present
	if execution.TaskInfo != nil {
		metadata["task_info"] = map[string]interface{}{
			"task_id":       execution.TaskInfo.TaskID,
			"task_name":     execution.TaskInfo.TaskName,
			"worktree":      execution.TaskInfo.Worktree,
			"worktree_path": execution.TaskInfo.WorktreePath,
			"dependencies":  execution.TaskInfo.Dependencies,
			"task_priority": execution.TaskInfo.TaskPriority,
			"prompt":        execution.TaskInfo.Prompt,
		}
	}

	// Write metadata to file
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataFile, metadataJSON, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// HasSession checks if a session exists
func (usm *UnifiedSessionManager) HasSession(sessionName string) bool {
	return usm.tmuxManager.HasSession(sessionName)
}

// AttachSession attaches to an existing session
func (usm *UnifiedSessionManager) AttachSession(sessionName string) error {
	return usm.tmuxManager.AttachSession(sessionName)
}

// KillSession terminates a session
func (usm *UnifiedSessionManager) KillSession(sessionName string) error {
	return usm.tmuxManager.KillSession(sessionName)
}

// ListSessions lists all sessions
func (usm *UnifiedSessionManager) ListSessions() ([]*tmux.Session, error) {
	return usm.tmuxManager.ListSessions()
}
