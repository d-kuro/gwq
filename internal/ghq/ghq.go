// Package ghq provides integration with the ghq repository management tool.
package ghq

import (
	"fmt"
	"os/exec"
	"path/filepath"
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

// IsGhqManaged checks if a repository path is under any of the ghq roots.
// The path must be a subdirectory of a ghq root (not the root itself).
func IsGhqManaged(repoPath string, ghqRoots []string) bool {
	// Clean and normalize the path
	repoPath = filepath.Clean(repoPath)

	for _, root := range ghqRoots {
		root = filepath.Clean(root)

		// Check if repoPath is a subdirectory of root
		// Use filepath.Rel to properly handle path comparisons
		rel, err := filepath.Rel(root, repoPath)
		if err != nil {
			continue
		}

		// If rel doesn't start with "..", it's under the root
		// Also ensure it's not the root itself (rel should not be ".")
		if !strings.HasPrefix(rel, "..") && rel != "." {
			return true
		}
	}

	return false
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
