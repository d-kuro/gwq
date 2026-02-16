// Package ghq provides integration with the ghq repository management tool.
package ghq

import (
	"fmt"
	"os/exec"
	"strings"
)

// CommandExecutor defines the interface for executing shell commands.
type CommandExecutor interface {
	Run(name string, args ...string) (string, error)
}

// DefaultExecutor is the default implementation of CommandExecutor.
type DefaultExecutor struct{}

// Run executes a command and returns its output.
func (e *DefaultExecutor) Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// Client provides methods for interacting with the ghq command.
type Client struct {
	executor CommandExecutor
}

// NewClient creates a new ghq Client with the default executor.
func NewClient() *Client {
	return &Client{
		executor: &DefaultExecutor{},
	}
}

// NewClientWithExecutor creates a new ghq Client with a custom executor (for testing).
func NewClientWithExecutor(executor CommandExecutor) *Client {
	return &Client{
		executor: executor,
	}
}

// GetRoots returns all ghq root directories.
// Executes `ghq root --all` to retrieve the list.
func (c *Client) GetRoots() ([]string, error) {
	output, err := c.executor.Run("ghq", "root", "--all")
	if err != nil {
		return nil, fmt.Errorf("failed to get ghq roots: %w", err)
	}

	return parseLines(output), nil
}

// ListRepositories returns all repositories managed by ghq.
// Executes `ghq list -p` to retrieve the full paths.
func (c *Client) ListRepositories() ([]string, error) {
	output, err := c.executor.Run("ghq", "list", "-p")
	if err != nil {
		return nil, fmt.Errorf("failed to list ghq repositories: %w", err)
	}

	return parseLines(output), nil
}

// IsInstalled checks if ghq is installed and available.
func (c *Client) IsInstalled() bool {
	_, err := c.executor.Run("ghq", "root")
	return err == nil
}

// parseLines splits output into non-empty lines.
func parseLines(output string) []string {
	lines := strings.Split(output, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}

	return result
}
