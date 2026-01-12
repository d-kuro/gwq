# tmux Session Management Design

## Overview

Design for tmux integration to provide session management capabilities for gwq. This enables process persistence, monitoring, and history management for any long-running commands or processes.

## Core Concepts

### Session Management Goals

1. **Process Persistence**: Make command execution independent of terminal connections
2. **History Preservation**: Automatically preserve command output in tmux history
3. **Monitoring**: Monitor the status of running processes
4. **Detach/Attach**: Continue processing even when terminal is closed

### Session Naming Convention

```
gwq-{context}-{identifier}-{timestamp}
```

Examples:
- `gwq-task-abc123-20240115103045`
- `gwq-agent-def456-20240115110230`
- `gwq-test-feature-auth-20240115120000`

## Architecture

### tmux Session Manager

```go
type SessionManager struct {
    config    *SessionConfig
    tmuxCmd   *TmuxCommand
}

type Session struct {
    ID           string            `json:"id"`
    SessionName  string            `json:"session_name"`
    Context      string            `json:"context"`     // e.g., "task", "agent", "test"
    Identifier   string            `json:"identifier"`  // e.g., task ID, branch name
    WorkingDir   string            `json:"working_dir"`
    Command      string            `json:"command"`
    PID          int               `json:"pid"`
    StartTime    time.Time         `json:"start_time"`
    Status       Status            `json:"status"`
    HistorySize  int               `json:"history_size"`
    Metadata     map[string]string `json:"metadata,omitempty"`
}

type SessionOptions struct {
    Context    string
    Identifier string  
    WorkingDir string
    Command    string
    Metadata   map[string]string
}

type Status string

const (
    StatusRunning   Status = "running"
    StatusCompleted Status = "completed"
    StatusFailed    Status = "failed"
    StatusDetached  Status = "detached"
)
```

## Core Features

### Session Creation

Create tmux sessions for any long-running commands:

```go
func (s *SessionManager) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
    sessionName := fmt.Sprintf("gwq-%s-%s-%s", opts.Context, opts.Identifier, time.Now().Format("20060102150405"))
    
    // Create tmux session with increased history limit
    tmuxCmd := fmt.Sprintf("tmux new-session -d -s %s -c %s", sessionName, opts.WorkingDir)
    if err := exec.Command("sh", "-c", tmuxCmd).Run(); err != nil {
        return nil, err
    }
    
    // Set history limit for this session
    historyCmd := fmt.Sprintf("tmux set-option -t %s history-limit %d", sessionName, s.config.HistoryLimit)
    exec.Command("sh", "-c", historyCmd).Run()
    
    // Execute command in the session
    cmdExec := fmt.Sprintf("tmux send-keys -t %s '%s' Enter", sessionName, opts.Command)
    exec.Command("sh", "-c", cmdExec).Run()
    
    session := &Session{
        ID:           generateID(),
        SessionName:  sessionName,
        Context:      opts.Context,
        Identifier:   opts.Identifier,
        WorkingDir:   opts.WorkingDir,
        Command:      opts.Command,
        StartTime:    time.Now(),
        Status:       StatusRunning,
        HistorySize:  s.config.HistoryLimit,
        Metadata:     opts.Metadata,
    }
    
    return session, nil
}
```

### tmux History Management

Uses tmux's built-in history and buffer functionality instead of custom logging:

```go
type SessionManager struct {
    config    *SessionConfig
    tmuxCmd   *TmuxCommand
}

func (s *SessionManager) CaptureHistory(sessionName string, lines int) ([]string, error) {
    // Use tmux capture-pane to get session history
    cmd := fmt.Sprintf("tmux capture-pane -t %s -p -S -%d", sessionName, lines)
    output, err := exec.Command("sh", "-c", cmd).Output()
    if err != nil {
        return nil, err
    }
    return strings.Split(string(output), "\n"), nil
}

func (s *SessionManager) SaveHistory(sessionName, filename string) error {
    // Save current buffer to file using tmux save-buffer
    cmd := fmt.Sprintf("tmux capture-pane -t %s && tmux save-buffer %s", sessionName, filename)
    return exec.Command("sh", "-c", cmd).Run()
}
```

## Command Design

### gwq tmux subcommands

#### `gwq tmux list`

List running tmux sessions (following existing status command patterns):

```bash
# Session list (simple table format)
gwq tmux list

# Output:
# CONTEXT     IDENTIFIER      STATUS     DURATION   COMMAND
# ● agent     auth-impl       running    1h 25m     agent --task "impl auth"
#   task      api-dev         running    45m        make test
#   test      feature-auth    completed  2h 15m     go test -v ./...

# Detailed information
gwq tmux list --verbose

# JSON output
gwq tmux list --json

# CSV output
gwq tmux list --csv

# Real-time monitoring
gwq tmux list --watch

# Status filter
gwq tmux list --filter running
gwq tmux list --filter completed

# Sort
gwq tmux list --sort duration
gwq tmux list --sort task
```

#### `gwq tmux attach`

Attach to sessions (following existing get/exec patterns):

```bash
# Attach with pattern matching
gwq tmux attach auth

# Attach with exact match
gwq tmux attach auth-impl

# Auto fuzzy finder when multiple matches
gwq tmux attach feature  # when feature/* matches

# Fuzzy finder selection for all sessions without arguments
gwq tmux attach

# Explicit fuzzy finder usage
gwq tmux attach -i
```


#### `gwq tmux run`

Run commands in tmux sessions:

```bash
# Run command in current directory
gwq tmux run "npm run dev"

# Run in specific worktree
gwq tmux run -w feature/auth "make test"

# Run with custom identifier
gwq tmux run --id test-suite "go test -v ./..."

# Run with context
gwq tmux run --context test "pytest -v"
```

#### `gwq tmux kill`

Terminate sessions (following existing remove patterns):

```bash
# Terminate with pattern matching
gwq tmux kill auth

# Auto fuzzy finder when multiple matches
gwq tmux kill feature

# Fuzzy finder selection without arguments
gwq tmux kill

# Explicit fuzzy finder usage
gwq tmux kill -i

# Terminate all sessions (with confirmation)
gwq tmux kill --all

# Cleanup only completed sessions
gwq tmux kill --completed
```

## Integration Examples

### Using tmux Sessions from Other Features

```go
// Example: Task execution with tmux
func (w *Worker) executeTaskWithSession(task *Task) error {
    opts := SessionOptions{
        Context:    "task",
        Identifier: task.ID,
        WorkingDir: task.WorktreePath,
        Command:    task.Command,
        Metadata: map[string]string{
            "task_name": task.Name,
            "priority":  task.Priority.String(),
        },
    }
    
    session, err := w.sessionManager.CreateSession(ctx, opts)
    if err != nil {
        return err
    }
    
    // Record session information
    task.SessionID = session.ID
    task.SessionName = session.SessionName
    
    return nil
}

// Example: Test execution with tmux
func runTestsInTmux(branch string, testCommand string) error {
    opts := SessionOptions{
        Context:    "test",
        Identifier: branch,
        WorkingDir: getWorktreePath(branch),
        Command:    testCommand,
    }
    
    _, err := sessionManager.CreateSession(ctx, opts)
    return err
}
```

### Integration with gwq Status

Extend existing status command to show active tmux sessions:

```bash
# Add session information to existing status command
gwq status --verbose

# Output:
# BRANCH          STATUS       CHANGES           ACTIVITY     SESSIONS
# ● main          up to date   -                2 hours ago  -
#   feature/auth  changed      5 added, 3 mod   1 hour ago   2 active
#   feature/api   changed      12 added, 8 mod  30 min ago   1 active
#   bugfix/login  clean        -                3 hours ago  -

# Filter by session activity
gwq status --filter "has-session"
gwq status --filter "no-session"
```

## tmux History Features

### History Configuration

tmux provides built-in history management that is more efficient than custom logging:

```bash
# Set global history limit in tmux.conf
set-option -g history-limit 50000

# Per-session history limit (set by gwq)
tmux set-option -t session-name history-limit 50000

# Capture session history
tmux capture-pane -t session-name -p -S -1000

# Save captured content to file
tmux save-buffer ~/session-history.txt
```

### History Search

Use tmux's built-in search functionality:

```bash
# Enter copy mode and search (within tmux session)
Ctrl+B [         # Enter copy mode
Ctrl+S           # Search forward
Ctrl+R           # Search backward

# Capture and search outside tmux
tmux capture-pane -t auth-session -p -S -1000 | grep "error"
tmux capture-pane -t auth-session && tmux save-buffer /tmp/history.txt
grep "authentication" /tmp/history.txt
```

## Configuration

### tmux Session Configuration

```toml
[tmux]
# Enable tmux integration
enabled = true

# Auto session creation
auto_create_session = true

# Behavior on session creation
detach_on_create = true
auto_cleanup_completed = true

# tmux configuration
tmux_command = "tmux"
default_shell = "/bin/bash"

# Session configuration
session_timeout = "24h"
keep_alive = true

# tmux history configuration
history_limit = 50000
history_auto_save = true
history_save_dir = "~/.gwq/history"
```

## Usage Examples

### Basic Usage Flow

```bash
# Start a long-running process in tmux
gwq tmux run -w feature/auth "npm run test:watch"

# Check session status
gwq tmux list
gwq status --verbose  # includes session count

# Attach to session to check progress
gwq tmux attach auth

# Detach from session (Ctrl+B, D)
# → Process continues running

# Later, check completed sessions
gwq tmux list --filter completed
# Use tmux history to check output
tmux capture-pane -t gwq-test-auth-* -p -S -100
```

### History Analysis Examples

```bash
# Capture and search session history for errors
tmux capture-pane -t session-name -p -S -1000 | grep "error\|failed\|exception"

# Track changes related to specific files
tmux capture-pane -t session-name -p -S -1000 | grep "auth.go"

# Identify long-running tasks
gwq tmux list --sort duration

# Display only running sessions
gwq tmux list --filter running
```

## Benefits

1. **Process Persistence**: Commands continue execution even after terminal disconnection
2. **History Recording**: All output is automatically saved in tmux history
3. **Debugging Support**: tmux history search and analysis capabilities
4. **Monitoring Functionality**: Detailed understanding of execution status
5. **Extensible Design**: Can be used by any gwq feature requiring persistent processes

## Limitations

1. Works only in environments where tmux is installed
2. Resource consumption increases as the number of sessions grows
3. tmux history is limited by configured history-limit

## Summary

This tmux session management feature provides a generic foundation for running persistent processes in gwq. By focusing purely on process management and using tmux's native history features, it provides a simple and reliable system that can be used by various gwq features.
