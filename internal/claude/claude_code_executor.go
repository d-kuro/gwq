package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/d-kuro/gwq/internal/config"
	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/worktree"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/d-kuro/gwq/pkg/system"
	"github.com/d-kuro/gwq/pkg/utils"
)

// ClaudeCodeExecutor handles the actual execution of Claude Code commands
type ClaudeCodeExecutor struct {
	config *models.ClaudeConfig
	system system.SystemInterface
}

// NewClaudeCodeExecutor creates a new Claude Code executor
func NewClaudeCodeExecutor(config *models.ClaudeConfig) *ClaudeCodeExecutor {
	return &ClaudeCodeExecutor{
		config: config,
		system: system.NewStandardSystem(),
	}
}

// NewClaudeCodeExecutorWithSystem creates a new Claude Code executor with custom system interface
func NewClaudeCodeExecutorWithSystem(config *models.ClaudeConfig, sys system.SystemInterface) *ClaudeCodeExecutor {
	return &ClaudeCodeExecutor{
		config: config,
		system: sys,
	}
}

// Execute runs Claude Code and captures the output
func (cce *ClaudeCodeExecutor) Execute(ctx context.Context, execution *UnifiedExecution, logFile string) (*ExecutionResult, error) {
	// Create named pipe for output capture
	pipePath, cleanup, err := cce.createNamedPipe(execution.ExecutionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create named pipe: %w", err)
	}
	defer cleanup()

	// Start log capture in background
	logCaptureDone := cce.startLogCapture(pipePath, logFile, execution)

	// Setup and validate execution environment
	if err := cce.ensureWorktreeExists(execution); err != nil {
		return &ExecutionResult{
			Success:  false,
			ExitCode: 1,
			Error:    fmt.Sprintf("failed to ensure worktree exists: %v", err),
		}, err
	}

	// Execute the Claude command
	cmd, err := cce.setupCommandExecution(ctx, execution, pipePath)
	if err != nil {
		return &ExecutionResult{
			Success:  false,
			ExitCode: 1,
			Error:    fmt.Sprintf("failed to setup command: %v", err),
		}, err
	}

	exitCode, cmdErr := cce.executeCommand(cmd)

	// Handle post-execution cleanup
	cce.handlePostExecution(ctx, execution)

	// Collect and return results
	return cce.collectExecutionResult(exitCode, cmdErr, logCaptureDone, execution)
}

// buildClaudeCommand builds the appropriate Claude command
func (cce *ClaudeCodeExecutor) buildClaudeCommand(execution *UnifiedExecution) string {
	args := []string{cce.config.Executable}

	// Add standard arguments for task execution
	args = append(args, "--dangerously-skip-permissions", "--output-format", "stream-json")

	// Add the prompt
	args = append(args, "-p", fmt.Sprintf(`"%s"`, utils.EscapeForShell(execution.Prompt)))

	return strings.Join(args, " ")
}

// captureLogOutput captures the JSON output from Claude
func (cce *ClaudeCodeExecutor) captureLogOutput(pipePath, logFile string, execution *UnifiedExecution) error {
	// Open pipe for reading
	pipe, err := os.OpenFile(pipePath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open pipe: %w", err)
	}
	defer func() {
		if err := pipe.Close(); err != nil {
			fmt.Printf("Warning: failed to close pipe: %v\n", err)
		}
	}()

	// Create log file
	log, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Printf("Warning: failed to close log file: %v\n", err)
		}
	}()

	// Read and process JSON stream
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Add timestamp and execution context to each JSON line
		var jsonData map[string]interface{}
		if err := json.Unmarshal([]byte(line), &jsonData); err == nil {
			// Enhance with execution context
			jsonData["timestamp"] = time.Now().Format(time.RFC3339)
			jsonData["execution_id"] = execution.ExecutionID
			jsonData["execution_type"] = execution.ExecutionType

			// Extract cost and model info if available
			if jsonData["type"] == "result" {
				if cost, ok := jsonData["cost_usd"].(float64); ok {
					execution.CostUSD = cost
				}
				if duration, ok := jsonData["duration_ms"].(float64); ok {
					execution.DurationMS = int64(duration)
				}
			}

			if jsonData["type"] == "system" && jsonData["subtype"] == "init" {
				if model, ok := jsonData["model"].(string); ok {
					execution.Model = model
				}
			}

			// Write enhanced JSON line
			enhancedLine, _ := json.Marshal(jsonData)
			if _, err := fmt.Fprintf(log, "%s\n", enhancedLine); err != nil {
				fmt.Printf("Warning: failed to write enhanced log line: %v\n", err)
			}
		} else {
			// If not valid JSON, write as-is with execution context
			contextLine := fmt.Sprintf(`{"type":"raw","content":"%s","execution_id":"%s","timestamp":"%s"}`,
				escapeJSONString(line), execution.ExecutionID, time.Now().Format(time.RFC3339))
			if _, err := fmt.Fprintln(log, contextLine); err != nil {
				fmt.Printf("Warning: failed to write log line: %v\n", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading from pipe: %w", err)
	}

	return nil
}

// detectChangedFiles detects files that were changed during execution
func (cce *ClaudeCodeExecutor) detectChangedFiles(execution *UnifiedExecution) []string {
	workingDir := execution.WorkingDir

	// For task executions, use the worktree path if available
	if execution.TaskInfo != nil && execution.TaskInfo.WorktreePath != "" {
		workingDir = execution.TaskInfo.WorktreePath
	}

	if workingDir == "" {
		return []string{}
	}

	// Use git to find changed files
	cmd := exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = workingDir
	output, err := cmd.Output()
	if err != nil {
		// If git diff fails, try git status for untracked files
		cmd = exec.Command("git", "status", "--porcelain")
		cmd.Dir = workingDir
		output, err = cmd.Output()
		if err != nil {
			return []string{}
		}

		// Parse git status output
		var files []string
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if len(line) > 3 {
				files = append(files, strings.TrimSpace(line[3:]))
			}
		}
		return files
	}

	// Parse git diff output
	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 1 && files[0] == "" {
		return []string{}
	}

	return files
}

// createNamedPipe creates a named pipe for output capture
func (cce *ClaudeCodeExecutor) createNamedPipe(executionID string) (string, func(), error) {
	pipePath := fmt.Sprintf("/tmp/gwq-claude-%s.pipe", executionID)
	if err := cce.system.CreateNamedPipe(pipePath, 0600); err != nil {
		return "", nil, err
	}

	cleanup := func() {
		if err := cce.system.RemoveFile(pipePath); err != nil {
			fmt.Printf("Warning: failed to remove pipe: %v\n", err)
		}
	}

	return pipePath, cleanup, nil
}

// startLogCapture starts log capture in a background goroutine
func (cce *ClaudeCodeExecutor) startLogCapture(pipePath, logFile string, execution *UnifiedExecution) <-chan error {
	logCaptureDone := make(chan error, 1)
	go func() {
		logCaptureDone <- cce.captureLogOutput(pipePath, logFile, execution)
	}()
	return logCaptureDone
}

// setupCommandExecution creates and configures the command for execution
func (cce *ClaudeCodeExecutor) setupCommandExecution(ctx context.Context, execution *UnifiedExecution, pipePath string) (*exec.Cmd, error) {
	// Build the Claude command
	claudeCmd := cce.buildClaudeCommand(execution)
	fullCmd := fmt.Sprintf("%s | tee %s", claudeCmd, pipePath)

	// Create command with context
	cmd := exec.CommandContext(ctx, "bash", "-c", fullCmd)
	cmd.Dir = execution.WorkingDir

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("CLAUDE_EXECUTION_ID=%s", execution.ExecutionID),
		fmt.Sprintf("CLAUDE_SESSION_ID=%s", execution.SessionID),
	)

	return cmd, nil
}

// executeCommand starts the command and waits for completion
func (cce *ClaudeCodeExecutor) executeCommand(cmd *exec.Cmd) (int, error) {
	// Start the command
	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("failed to start Claude command: %w", err)
	}

	// Wait for command completion
	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return exitCode, err
}

// handlePostExecution handles cleanup after command execution
func (cce *ClaudeCodeExecutor) handlePostExecution(ctx context.Context, execution *UnifiedExecution) {
	// Wait for tmux session to terminate (for task executions)
	if execution.ExecutionType == ExecutionTypeTask && execution.TmuxSession != "" {
		cce.waitForTmuxSessionTermination(ctx, execution.TmuxSession)
	}
}

// collectExecutionResult collects execution results and builds the final result
func (cce *ClaudeCodeExecutor) collectExecutionResult(exitCode int, cmdErr error, logCaptureDone <-chan error, execution *UnifiedExecution) (*ExecutionResult, error) {
	// Wait for log capture to complete
	logErr := <-logCaptureDone

	// Detect changed files
	changedFiles := cce.detectChangedFiles(execution)

	// Create result
	result := &ExecutionResult{
		Success:      exitCode == 0 && cmdErr == nil,
		ExitCode:     exitCode,
		FilesChanged: changedFiles,
	}

	// Handle command execution errors
	if cmdErr != nil {
		result.Error = cmdErr.Error()
	}

	// Handle log capture errors
	if logErr != nil {
		if result.Error != "" {
			result.Error += fmt.Sprintf("; log capture error: %v", logErr)
		} else {
			result.Error = fmt.Sprintf("log capture error: %v", logErr)
		}
	}

	return result, nil
}

// WatchOutput provides real-time output watching for an execution
func (cce *ClaudeCodeExecutor) WatchOutput(ctx context.Context, execution *UnifiedExecution, logFile string) error {
	// Open log file
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: failed to close log file: %v\n", err)
		}
	}()

	// Follow the file
	reader := bufio.NewReader(file)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// Check if execution is still running
					if execution.Status == ExecutionStatusRunning {
						time.Sleep(100 * time.Millisecond)
						continue
					}
					return nil
				}
				return err
			}

			// Parse and format JSON for display
			cce.displayLogLine(line)
		}
	}
}

// displayLogLine formats and displays a log line
func (cce *ClaudeCodeExecutor) displayLogLine(line string) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		fmt.Print(line)
		return
	}

	// Format based on type
	switch data["type"] {
	case "assistant":
		if msg, ok := data["message"].(map[string]interface{}); ok {
			if content, ok := msg["content"].([]interface{}); ok {
				for _, item := range content {
					if contentItem, ok := item.(map[string]interface{}); ok {
						if contentItem["type"] == "text" {
							if text, ok := contentItem["text"].(string); ok {
								fmt.Printf("🤖 %s\n", text)
							}
						}
					}
				}
			}
		}
	case "user":
		// Tool results
		if msg, ok := data["message"].(map[string]interface{}); ok {
			if content, ok := msg["content"].([]interface{}); ok {
				for _, item := range content {
					if contentItem, ok := item.(map[string]interface{}); ok {
						if contentItem["type"] == "tool_result" {
							if result, ok := contentItem["content"].(string); ok {
								fmt.Printf("📊 Tool Result:\n%s\n", result)
							}
						}
					}
				}
			}
		}
	case "result":
		if result, ok := data["result"].(string); ok {
			fmt.Printf("\n✅ Result: %s\n", result)
		}
		if cost, ok := data["cost_usd"].(float64); ok {
			fmt.Printf("💰 Cost: $%.4f\n", cost)
		}
	case "raw":
		if content, ok := data["content"].(string); ok {
			fmt.Printf("📝 %s\n", content)
		}
	}
}

// escapeJSONString escapes a string for JSON
func escapeJSONString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// ensureWorktreeExists ensures that the worktree exists for task executions
func (cce *ClaudeCodeExecutor) ensureWorktreeExists(execution *UnifiedExecution) error {
	// Only handle task executions with TaskInfo
	if execution.ExecutionType != ExecutionTypeTask || execution.TaskInfo == nil {
		return nil
	}

	// Only verify worktree if specified
	if execution.TaskInfo.Worktree == "" {
		return nil
	}

	// Load config and create worktree manager
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize git from repository root
	g := git.New(execution.Repository)

	// Create worktree manager
	wm := worktree.New(g, cfg)

	// Check if worktree exists using gwq logic
	worktreePath, err := wm.GetWorktreePath(execution.TaskInfo.Worktree)
	if err != nil {
		// Worktree doesn't exist - check if we should create it
		if execution.TaskInfo.AutoCreateWorktree && execution.TaskInfo.BaseBranch != "" {
			// Create the worktree from the base branch
			fmt.Printf("Creating worktree '%s' from base branch '%s'...\n",
				execution.TaskInfo.Worktree, execution.TaskInfo.BaseBranch)

			if err := wm.AddFromBase(execution.TaskInfo.Worktree, execution.TaskInfo.BaseBranch, ""); err != nil {
				return fmt.Errorf("failed to create worktree '%s' from base branch '%s': %w",
					execution.TaskInfo.Worktree, execution.TaskInfo.BaseBranch, err)
			}

			// Try to get the worktree path again after creation
			worktreePath, err = wm.GetWorktreePath(execution.TaskInfo.Worktree)
			if err != nil {
				return fmt.Errorf("failed to get worktree path after creation: %w", err)
			}
		} else {
			// Return the original error if auto-create is not enabled or no base branch specified
			return fmt.Errorf("worktree '%s' does not exist, please create it first using 'gwq add %s': %w",
				execution.TaskInfo.Worktree, execution.TaskInfo.Worktree, err)
		}
	}

	// Verify worktree directory exists and is accessible
	if _, statErr := os.Stat(worktreePath); statErr != nil {
		return fmt.Errorf("worktree path '%s' is not accessible: %w", worktreePath, statErr)
	}

	// Update working directory to the worktree path
	execution.WorkingDir = worktreePath
	if execution.TaskInfo != nil {
		execution.TaskInfo.WorktreePath = worktreePath
	}

	return nil
}

// waitForTmuxSessionTermination waits for a tmux session to terminate
func (cce *ClaudeCodeExecutor) waitForTmuxSessionTermination(ctx context.Context, sessionName string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if tmux session still exists
			cmd := exec.Command("tmux", "has-session", "-t", sessionName)
			if err := cmd.Run(); err != nil {
				// Session doesn't exist anymore, task is complete
				return
			}
		}
	}
}
