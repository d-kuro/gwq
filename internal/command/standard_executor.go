package command

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
)

// StandardExecutor implements CommandExecutor using os/exec
type StandardExecutor struct{}

// NewStandardExecutor creates a new StandardExecutor
func NewStandardExecutor() *StandardExecutor {
	return &StandardExecutor{}
}

// Execute runs a command and returns error only
func (e *StandardExecutor) Execute(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Run()
}

// ExecuteWithOutput runs a command and returns output and error
func (e *StandardExecutor) ExecuteWithOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %w, stderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// ExecuteInDir runs a command in a specific directory
func (e *StandardExecutor) ExecuteInDir(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

// ExecuteInDirWithOutput runs a command in a specific directory and returns output
func (e *StandardExecutor) ExecuteInDirWithOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %w, stderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// ExecuteWithStreams runs a command with custom input/output streams
func (e *StandardExecutor) ExecuteWithStreams(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// ExecuteWithEnv runs a command with custom environment variables
func (e *StandardExecutor) ExecuteWithEnv(ctx context.Context, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	return cmd.Run()
}

// ExecuteWithEnvInDir runs a command with custom environment and directory
func (e *StandardExecutor) ExecuteWithEnvInDir(ctx context.Context, env []string, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = dir
	return cmd.Run()
}

// applyOptions applies CommandOptions to an exec.Cmd.
// If captureOutput is true, stdout and stderr from opts are ignored (for output capture).
func applyOptions(cmd *exec.Cmd, opts *CommandOptions, captureOutput bool) {
	if opts == nil {
		return
	}

	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}
	if opts.Environment != nil {
		cmd.Env = opts.Environment
	}
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}
	if !captureOutput {
		if opts.Stdout != nil {
			cmd.Stdout = opts.Stdout
		}
		if opts.Stderr != nil {
			cmd.Stderr = opts.Stderr
		}
	}
}

// ExecuteWithOptions runs a command with comprehensive options
func (e *StandardExecutor) ExecuteWithOptions(ctx context.Context, name string, args []string, opts *CommandOptions) error {
	cmd := exec.CommandContext(ctx, name, args...)
	applyOptions(cmd, opts, false)
	return cmd.Run()
}

// ExecuteWithOptionsAndOutput runs a command with options and returns output
func (e *StandardExecutor) ExecuteWithOptionsAndOutput(ctx context.Context, name string, args []string, opts *CommandOptions) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	applyOptions(cmd, opts, true)

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %w, stderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
