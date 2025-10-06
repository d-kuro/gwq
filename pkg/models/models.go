// Package models defines the core data structures used throughout the gwq application.
package models

import "time"

// Worktree represents a Git worktree with its associated metadata.
type Worktree struct {
	Path       string    `json:"path"`        // Absolute path to the worktree directory
	Branch     string    `json:"branch"`      // Branch name associated with this worktree
	CommitHash string    `json:"commit_hash"` // Current HEAD commit hash
	IsMain     bool      `json:"is_main"`     // Whether this is the main worktree
	CreatedAt  time.Time `json:"created_at"`  // Creation timestamp
}

// Branch represents a Git branch with its metadata.
type Branch struct {
	Name       string     `json:"name"`        // Branch name
	IsCurrent  bool       `json:"is_current"`  // Whether this is the current branch
	IsRemote   bool       `json:"is_remote"`   // Whether this is a remote branch
	LastCommit CommitInfo `json:"last_commit"` // Information about the last commit
}

// CommitInfo contains information about a Git commit.
type CommitInfo struct {
	Hash    string    `json:"hash"`    // Commit hash
	Message string    `json:"message"` // Commit message
	Author  string    `json:"author"`  // Commit author
	Date    time.Time `json:"date"`    // Commit date
}

// Config represents the application configuration.
type Config struct {
	Worktree           WorktreeConfig      `mapstructure:"worktree"`            // Worktree-related configuration
	Finder             FinderConfig        `mapstructure:"finder"`              // Fuzzy finder configuration
	UI                 UIConfig            `mapstructure:"ui"`                  // UI-related configuration
	Naming             NamingConfig        `mapstructure:"naming"`              // Naming and template configuration
	Claude             ClaudeConfig        `mapstructure:"claude"`              // Claude Code task queue configuration
	RepositorySettings []RepositorySetting `mapstructure:"repository_settings"` // Per-repository setup/copy overrides
}

// RepositorySetting defines per-repository setup commands and files to copy for worktree creation.
type RepositorySetting struct {
	Repository    string   `mapstructure:"repo"`           // Path or pattern for repository
	SetupCommands []string `mapstructure:"setup_commands"` // Commands to run in new worktree
	CopyFiles     []string `mapstructure:"copy_files"`     // Files/globs to copy into new worktree
}

// WorktreeConfig contains worktree-specific configuration options.
type WorktreeConfig struct {
	BaseDir   string `mapstructure:"basedir"`    // Base directory for creating worktrees
	AutoMkdir bool   `mapstructure:"auto_mkdir"` // Automatically create directories
}

// FinderConfig contains fuzzy finder configuration options.
type FinderConfig struct {
	Preview bool `mapstructure:"preview"` // Enable preview window
}

// UIConfig contains UI-related configuration options.
type UIConfig struct {
	Icons     bool `mapstructure:"icons"`      // Enable icon display
	TildeHome bool `mapstructure:"tilde_home"` // Display home directory as ~
}

// NamingConfig contains directory naming and template configuration options.
type NamingConfig struct {
	Template      string            `mapstructure:"template"`       // Directory name template
	SanitizeChars map[string]string `mapstructure:"sanitize_chars"` // Character replacement for branch names
}

// WorktreeStatus represents the current status of a worktree.
type WorktreeStatus struct {
	Path          string        `json:"path"`             // Absolute path to the worktree
	Branch        string        `json:"branch"`           // Branch name
	Repository    string        `json:"repository"`       // Repository identifier
	Status        WorktreeState `json:"status"`           // Current status (clean, modified, etc.)
	GitStatus     GitStatus     `json:"git_status"`       // Detailed git status
	LastActivity  time.Time     `json:"last_activity"`    // Last modification time
	ActiveProcess []ProcessInfo `json:"active_processes"` // Running processes
	IsCurrent     bool          `json:"is_current"`       // Whether this is the current worktree
}

// WorktreeState represents the overall state of a worktree.
type WorktreeState string

const (
	// WorktreeStatusClean indicates a worktree with no uncommitted changes.
	WorktreeStatusClean WorktreeState = "clean"
	// WorktreeStatusModified indicates a worktree with uncommitted modifications.
	WorktreeStatusModified WorktreeState = "modified"
	// WorktreeStatusStaged indicates a worktree with staged changes ready to commit.
	WorktreeStatusStaged WorktreeState = "staged"
	// WorktreeStatusConflict indicates a worktree with merge conflicts.
	WorktreeStatusConflict WorktreeState = "conflict"
	// WorktreeStatusStale indicates a worktree that is out of sync with the remote.
	WorktreeStatusStale WorktreeState = "stale"
	// WorktreeStatusUnknown indicates a worktree with an undetermined status.
	WorktreeStatusUnknown WorktreeState = "unknown"
)

// GitStatus contains detailed git status information.
type GitStatus struct {
	Modified  int `json:"modified"`  // Number of modified files
	Added     int `json:"added"`     // Number of added files
	Deleted   int `json:"deleted"`   // Number of deleted files
	Untracked int `json:"untracked"` // Number of untracked files
	Staged    int `json:"staged"`    // Number of staged files
	Ahead     int `json:"ahead"`     // Number of commits ahead of remote
	Behind    int `json:"behind"`    // Number of commits behind remote
	Conflicts int `json:"conflicts"` // Number of files with conflicts
}

// ProcessInfo represents information about a running process.
type ProcessInfo struct {
	PID     int    `json:"pid"`     // Process ID
	Command string `json:"command"` // Command name
	Type    string `json:"type"`    // Process type (e.g., "ai_agent")
}

// ClaudeConfig contains Claude Code task queue configuration.
type ClaudeConfig struct {
	// Claude Code executable and core options
	Executable string `mapstructure:"executable"` // Claude Code executable path
	ConfigDir  string `mapstructure:"config_dir"` // Configuration and state directory

	// Global parallelism control
	MaxParallel         int `mapstructure:"max_parallel"`          // Max parallel Claude instances
	MaxDevelopmentTasks int `mapstructure:"max_development_tasks"` // Max concurrent development tasks

	// Queue configuration
	Queue ClaudeQueueConfig `mapstructure:"queue"` // Queue management configuration

	// Worktree integration
	Worktree ClaudeWorktreeConfig `mapstructure:"worktree"` // Worktree integration options

	// Execution configuration
	Execution ClaudeExecutionConfig `mapstructure:"execution"` // Execution configuration
}

// ClaudeQueueConfig contains task queue management configuration.
type ClaudeQueueConfig struct {
	QueueDir string `mapstructure:"queue_dir"` // Queue storage directory
}

// ClaudeWorktreeConfig contains worktree integration configuration.
type ClaudeWorktreeConfig struct {
	AutoCreateWorktree      bool `mapstructure:"auto_create_worktree"`      // Auto create via gwq add
	RequireExistingWorktree bool `mapstructure:"require_existing_worktree"` // Only use existing worktrees
	ValidateBranchExists    bool `mapstructure:"validate_branch_exists"`    // Check branch exists
}

// ClaudeExecutionConfig contains execution configuration.
type ClaudeExecutionConfig struct {
	AutoCleanup bool `mapstructure:"auto_cleanup"` // Auto cleanup old logs
}

// ClaudeExecutionFormattingConfig contains log formatting configuration.
type ClaudeExecutionFormattingConfig struct {
	ShowToolDetails   bool `mapstructure:"show_tool_details"`   // Show detailed tool information
	ShowCostBreakdown bool `mapstructure:"show_cost_breakdown"` // Show cost breakdown
	ShowTimingInfo    bool `mapstructure:"show_timing_info"`    // Show timing information
	MaxContentLength  int  `mapstructure:"max_content_length"`  // Maximum content length for display
}

// Worktree type constants for display purposes.
const (
	// WorktreeTypeMain represents the main worktree (repository root).
	WorktreeTypeMain = "main"
	// WorktreeTypeWorktree represents an additional worktree.
	WorktreeTypeWorktree = "worktree"
)
