# Worktree Status Design

## Overview

The worktree status feature provides a comprehensive view of all worktrees' current state, including git status, recent activity, and optional process information. This feature is essential for managing multiple AI coding agents working in parallel across different worktrees.

## Motivation

When working with multiple AI agents across different worktrees, it becomes challenging to:

- Track which worktrees have uncommitted changes
- Monitor active development across branches
- Identify stale or abandoned worktrees
- Understand which worktrees have active processes
- Quickly assess overall project health

The status command addresses these challenges by providing a simple, scriptable view of all worktree states that integrates well with Unix tooling.

## Command Interface

### Basic Usage

```bash
gwq status                   # Table view with basic status
gwq status --json           # JSON output for scripting
gwq status --watch          # Auto-refresh mode
gwq status --verbose        # Include additional details
```

### Options

````bash
-w, --watch                  # Auto-refresh (default: 5s)
-i, --interval <seconds>     # Refresh interval for watch mode
-f, --filter <status>        # Filter by status (modified, clean, stale)
-s, --sort <field>           # Sort by field (branch, modified, activity)
--json                       # Output as JSON
--csv                        # Output as CSV
-v, --verbose                # Show additional information
-g, --global                 # Show all worktrees from base directory
--show-processes             # Include running processes (slower)
--no-fetch                   # Skip remote status check (faster)

## Data Model

### WorktreeStatus Structure
```go
type WorktreeStatus struct {
    // Basic Information
    Path            string    `json:"path"`
    Branch          string    `json:"branch"`
    Repository      string    `json:"repository"`
    Remote          string    `json:"remote"`

    // Git Status
    GitStatus       GitStatus `json:"git_status"`

    // Activity Metrics
    LastModified    time.Time `json:"last_modified"`
    LastCommit      time.Time `json:"last_commit"`
    LastPush        time.Time `json:"last_push"`

    // Resource Usage
    DiskUsage       int64     `json:"disk_usage"`
    FileCount       int       `json:"file_count"`

    // Process Information
    ActiveProcesses []Process `json:"active_processes,omitempty"`

    // Health Indicators
    Health          Health    `json:"health"`
}

type GitStatus struct {
    // Working Tree Status
    Modified        int       `json:"modified"`
    Added           int       `json:"added"`
    Deleted         int       `json:"deleted"`
    Untracked       int       `json:"untracked"`

    // Index Status
    Staged          int       `json:"staged"`

    // Branch Status
    Ahead           int       `json:"ahead"`
    Behind          int       `json:"behind"`

    // Merge/Rebase Status
    InProgress      string    `json:"in_progress,omitempty"` // "merge", "rebase", "cherry-pick"
    Conflicts       int       `json:"conflicts"`
}

type Process struct {
    PID             int       `json:"pid"`
    Command         string    `json:"command"`
    StartTime       time.Time `json:"start_time"`
    CPUPercent      float64   `json:"cpu_percent"`
    MemoryMB        int       `json:"memory_mb"`
}

type Health struct {
    Status          HealthStatus `json:"status"` // healthy, warning, critical
    Issues          []string     `json:"issues,omitempty"`
}

type HealthStatus string

const (
    HealthStatusHealthy  HealthStatus = "healthy"
    HealthStatusWarning  HealthStatus = "warning"
    HealthStatusCritical HealthStatus = "critical"
)
````

## Display Modes

### 1. Table View (Default)

```
BRANCH              STATUS       CHANGES                      ACTIVITY
● main              up to date   -                            2 hours ago
  feature/auth      changed      5 added, 3 modified          10 mins ago
  feature/api       changed      12 added, 8 modified         5 mins ago
  bugfix/login      staged       2 added                      1 hour ago
  feature/old-ui    inactive     45 added, 23 modified        2 weeks ago
```

### 2. Verbose Table View (`--verbose`)

```
BRANCH              STATUS       CHANGES                      AHEAD/BEHIND  ACTIVITY       PROCESS
● main              up to date   -                            ↑0 ↓0         2 hours ago    -
  feature/auth      changed      5 added, 3 modified          ↑5 ↓2         10 mins ago    agent:8923
  feature/api       conflicted   12 added, 8 modified         ↑3 ↓0         5 mins ago     cursor:9102
  bugfix/login      staged       2 added                      ↑1 ↓0         1 hour ago     -
  feature/old-ui    inactive     45 added, 23 modified        ↑12 ↓5        2 weeks ago    -
```

### 3. Watch Mode (`--watch`)

```
Worktrees Status (github.com/user/project) - Updated: 10:30:45
Total: 5 | Changed: 2 | Up to date: 2 | Inactive: 1

BRANCH              STATUS       CHANGES                      ACTIVITY
● main              up to date   -                            2 hours ago
  feature/auth      changed      5 added, 3 modified          10 mins ago
  feature/api       changed      12 added, 8 modified         5 mins ago
  bugfix/login      staged       2 added                      1 hour ago
  feature/old-ui    inactive     45 added, 23 modified        2 weeks ago

[Press Ctrl+C to exit]
```

### 4. JSON Output Mode (`--json`)

```json
{
  "summary": {
    "total": 5,
    "changed": 2,
    "up_to_date": 2,
    "inactive": 1
  },
  "worktrees": [
    {
      "path": "~/worktrees/github.com/user/project/feature-api",
      "branch": "feature/api",
      "repository": "github.com/user/project",
      "status": "changed",
      "git_status": {
        "modified": 12,
        "added": 8,
        "deleted": 0,
        "untracked": 0,
        "staged": 0,
        "ahead": 3,
        "behind": 0,
        "conflicts": 0
      },
      "last_activity": "2024-01-15T10:30:00Z",
      "active_processes": [
        {
          "pid": 9102,
          "command": "cursor",
          "type": "ai_agent"
        }
      ]
    }
  ]
}
```

### 5. CSV Output Mode (`--csv`)

```csv
branch,status,modified,added,deleted,ahead,behind,last_activity,process
main,up to date,0,0,0,0,0,2024-01-15T08:30:00Z,
feature/auth,changed,5,3,0,5,2,2024-01-15T10:20:00Z,agent:8923
feature/api,changed,12,8,0,3,0,2024-01-15T10:25:00Z,cursor:9102
bugfix/login,staged,0,2,0,1,0,2024-01-15T09:30:00Z,
feature/old-ui,inactive,45,23,0,12,5,2024-01-01T10:00:00Z,
```

## Implementation Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────┐
│                    CLI Command Layer                     │
│                  (gwq status command)                    │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│                 Status Collector Service                 │
│         (Orchestrates data collection)                  │
└──────┬──────────┬──────────┬──────────┬────────────────┘
       │          │          │          │
┌──────▼────┐ ┌──▼────┐ ┌──▼────┐ ┌──▼──────┐
│    Git    │ │  FS   │ │Process│ │ Health  │
│ Collector │ │Scanner│ │Monitor│ │ Checker │
└───────────┘ └───────┘ └───────┘ └─────────┘
```

### Key Components

#### 1. Status Collector Service

```go
type StatusCollector interface {
    CollectAll(ctx context.Context) ([]WorktreeStatus, error)
    CollectOne(ctx context.Context, path string) (*WorktreeStatus, error)
    Watch(ctx context.Context, interval time.Duration) <-chan []WorktreeStatus
}
```

#### 2. Git Status Collector

```go
type GitCollector interface {
    GetStatus(path string) (*GitStatus, error)
    GetLastCommit(path string) (time.Time, string, error)
    GetRemoteStatus(path string) (ahead, behind int, error)
}
```

#### 3. Process Monitor

```go
type ProcessMonitor interface {
    GetActiveProcesses(path string) ([]Process, error)
    IsAIAgentRunning(path string) (bool, *Process, error)
}
```

#### 4. Health Checker

```go
type HealthChecker interface {
    CheckHealth(status *WorktreeStatus) Health
    GetHealthRules() []HealthRule
}

type HealthRule interface {
    Check(status *WorktreeStatus) (bool, []string)
    Severity() HealthStatus
}
```

### Data Collection Pipeline

```go
func (c *statusCollector) CollectAll(ctx context.Context) ([]WorktreeStatus, error) {
    // 1. Discover all worktrees
    worktrees, err := c.discovery.FindAll()
    if err != nil {
        return nil, err
    }

    // 2. Collect status in parallel
    results := make([]WorktreeStatus, len(worktrees))
    var wg sync.WaitGroup

    for i, wt := range worktrees {
        wg.Add(1)
        go func(idx int, worktree Worktree) {
            defer wg.Done()

            status := WorktreeStatus{
                Path:       worktree.Path,
                Branch:     worktree.Branch,
                Repository: worktree.Repository,
            }

            // Collect git status
            if gitStatus, err := c.git.GetStatus(worktree.Path); err == nil {
                status.GitStatus = *gitStatus
            }

            // Collect file system metrics
            if metrics, err := c.fs.GetMetrics(worktree.Path); err == nil {
                status.DiskUsage = metrics.DiskUsage
                status.FileCount = metrics.FileCount
                status.LastModified = metrics.LastModified
            }

            // Monitor processes
            if procs, err := c.process.GetActiveProcesses(worktree.Path); err == nil {
                status.ActiveProcesses = procs
            }

            // Check health
            status.Health = c.health.CheckHealth(&status)

            results[idx] = status
        }(i, wt)
    }

    wg.Wait()
    return results, nil
}
```

## Performance Considerations

### Caching Strategy

```go
type StatusCache struct {
    data      map[string]*WorktreeStatus
    mu        sync.RWMutex
    ttl       time.Duration
    lastFetch map[string]time.Time
}

func (c *StatusCache) Get(path string) (*WorktreeStatus, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    if status, ok := c.data[path]; ok {
        if time.Since(c.lastFetch[path]) < c.ttl {
            return status, true
        }
    }
    return nil, false
}
```

### Parallel Processing

- Use goroutines for concurrent status collection
- Limit concurrent git operations to avoid overwhelming the system
- Implement timeout for each collection operation

### Optimization Techniques

1. **Incremental Updates**: Only refresh changed worktrees in watch mode
2. **Selective Data Collection**: Skip expensive operations unless requested (e.g., `--show-processes`)
3. **Background Refresh**: Update cache in background for better responsiveness
4. **Process Filtering**: Only check for relevant processes (e.g., known AI agents)

## UI/UX Considerations

### Visual Indicators

- Status text clearly indicates worktree state
- Current worktree marked with bullet (●) when icons enabled
- Clear text-based formatting for easy reading

### Output Filtering & Processing

Since the output is designed to be scriptable, users can leverage Unix tools:

```bash
# Filter modified worktrees
gwq status --json | jq '.worktrees[] | select(.status == "modified")'

# Sort by last activity
gwq status --json | jq '.worktrees | sort_by(.last_activity) | reverse'

# Count worktrees by status
gwq status --json | jq '.summary'

# Show only branches with AI agents
gwq status --csv | grep -E "(agent|cursor|copilot)"

# Watch for changes in specific branches
gwq status --watch --filter "feature/*"
```

## Error Handling

### Graceful Degradation

- If git operations fail, show partial information
- Handle missing worktrees without crashing
- Timeout long-running operations

### Error Categories

1. **Git Errors**: Repository corruption, missing .git
2. **Permission Errors**: Cannot access worktree directory
3. **Process Errors**: Cannot retrieve process information
4. **Network Errors**: Cannot reach remote for status

## Integration Points

### With Existing Commands

- `gwq get`: Navigate to worktree from dashboard
- `gwq remove`: Remove stale worktrees identified by dashboard
- `gwq exec`: Execute commands in selected worktree

### External Tools

- Export data for external monitoring tools
- Integration with terminal multiplexers (tmux/screen)
- Support for custom health check scripts

## Testing Strategy

### Unit Tests

- Test each collector independently
- Mock git operations for predictable results
- Test health rules with various scenarios

### Integration Tests

- Test full pipeline with real worktrees
- Verify performance with many worktrees
- Test watch mode and refresh logic

### Output Tests

- Verify table formatting and alignment
- Test text output rendering
- Validate JSON and CSV output formats
- Test filtering and sorting options

## Future Enhancements

1. **Historical Tracking**: Store status history for trend analysis
2. **Notifications**: Alert when worktrees need attention
3. **Custom Health Rules**: User-defined health check configuration
4. **AI Agent Integration**: Special handling for known AI tools
5. **Web Dashboard**: Browser-based dashboard for remote monitoring
6. **Metrics Export**: Prometheus/Grafana integration

## Configuration

### Status Configuration

```toml
[status]
default_interval = 5       # Watch mode refresh interval (seconds)
show_processes = false     # Show processes by default (slower)
fetch_remote = true        # Check remote status by default
cache_ttl = 30            # Cache TTL in seconds

[status.filters]
stale_threshold = 14      # Days before marking as stale
large_changeset = 50      # Files threshold for warning

[status.display]
# Color output removed due to table alignment issues
relative_time = true      # Show "2 hours ago" vs timestamps
tilde_home = true         # Show ~ instead of full home path
```

## Implementation Timeline

1. **Phase 1**: Basic status collection and table display
2. **Phase 2**: JSON and CSV output formats
3. **Phase 3**: Watch mode and filtering options
4. **Phase 4**: Process monitoring integration (optional)
5. **Phase 5**: Performance optimization and caching
