package command

import (
	"context"
	"io"
)

// CommandExecutor defines the interface for executing system commands
type CommandExecutor interface {
	// Execute runs a command and returns error only
	Execute(ctx context.Context, name string, args ...string) error

	// ExecuteWithOutput runs a command and returns output and error
	ExecuteWithOutput(ctx context.Context, name string, args ...string) (string, error)

	// ExecuteInDir runs a command in a specific directory
	ExecuteInDir(ctx context.Context, dir, name string, args ...string) error

	// ExecuteInDirWithOutput runs a command in a specific directory and returns output
	ExecuteInDirWithOutput(ctx context.Context, dir, name string, args ...string) (string, error)

	// ExecuteWithStreams runs a command with custom input/output streams
	ExecuteWithStreams(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error

	// ExecuteWithEnv runs a command with custom environment variables
	ExecuteWithEnv(ctx context.Context, env []string, name string, args ...string) error

	// ExecuteWithEnvInDir runs a command with custom environment and directory
	ExecuteWithEnvInDir(ctx context.Context, env []string, dir, name string, args ...string) error
}

// CommandOptions holds configuration for command execution
type CommandOptions struct {
	WorkingDir  string
	Environment []string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

// AdvancedCommandExecutor provides more flexible command execution
type AdvancedCommandExecutor interface {
	// ExecuteWithOptions runs a command with comprehensive options
	ExecuteWithOptions(ctx context.Context, name string, args []string, opts *CommandOptions) error

	// ExecuteWithOptionsAndOutput runs a command with options and returns output
	ExecuteWithOptionsAndOutput(ctx context.Context, name string, args []string, opts *CommandOptions) (string, error)
}
