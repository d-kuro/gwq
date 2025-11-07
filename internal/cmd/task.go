package cmd

import (
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage Claude Code tasks and automated development",
	Long: `Manage Claude Code tasks and automated development with dependency management.

This command provides a task queue system for Claude Code that enables:
- Automated task execution with dependency resolution
- Priority-based scheduling
- Resource management and parallelism control
- Integration with existing gwq worktree management

All commands should be executed from the git repository root directory.`,
	Example: `  # List all tasks
  gwq task list

  # View task execution logs
  gwq task logs
  gwq task logs exec-a1b2c3

  # Start worker to process tasks
  gwq task worker start --parallel 2

  # Check worker status
  gwq task worker status

  # View task details
  gwq task show task-id`,
}

func init() {
	rootCmd.AddCommand(taskCmd)
}
