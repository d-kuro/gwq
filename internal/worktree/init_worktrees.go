package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// gitignoreContent is the content of the .gitignore file created in .worktrees directory.
// Using "*" ensures all files and directories in .worktrees are ignored by git.
// This also makes the .worktrees directory itself invisible to git status since
// git doesn't track empty directories.
const gitignoreContent = "*\n"

// readmeContent is the content of the README.md file created in .worktrees directory.
const readmeContent = `# Git worktrees added by gwq

This directory contains Git worktrees created with gwq.

- Do NOT edit files here from parent directory contexts.
- Each subdirectory is an independent Git worktree.
- A .gitignore file ensures everything under it is ignored.

For more information, visit: https://github.com/d-kuro/gwq
`

// InitWorktreesDir initializes the .worktrees directory with necessary files.
// If autoFiles is true, it creates .gitignore and README.md.
// Existing files are not overwritten.
func InitWorktreesDir(worktreesDir string, autoFiles bool) error {
	// Create the directory (and any parent directories) if it doesn't exist
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return err
	}

	if !autoFiles {
		return nil
	}

	// Create .gitignore if it doesn't exist
	gitignorePath := filepath.Join(worktreesDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
			return err
		}
	}

	// Create README.md if it doesn't exist
	readmePath := filepath.Join(worktreesDir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
			return err
		}
	}

	return nil
}

// GetWorktreesDir returns the path to the .worktrees directory under the repository root.
func GetWorktreesDir(repoRoot, worktreesDir string) string {
	return filepath.Join(repoRoot, worktreesDir)
}

// GenerateGhqWorktreePath generates the worktree path for ghq mode.
// The path will be: {repoRoot}/{worktreesDir}/{sanitized-branch}
func GenerateGhqWorktreePath(repoRoot, worktreesDir, branch string, sanitizeChars map[string]string) string {
	// Sanitize the branch name
	sanitizedBranch := sanitizeBranchName(branch, sanitizeChars)

	return filepath.Join(repoRoot, worktreesDir, sanitizedBranch)
}

// ValidateWorktreesDir validates that the worktreesDir is a safe relative path.
// It returns an error if the path is absolute, uses tilde expansion, or attempts path traversal.
func ValidateWorktreesDir(worktreesDir string) error {
	if filepath.IsAbs(worktreesDir) || strings.HasPrefix(worktreesDir, "~") {
		return fmt.Errorf("worktrees_dir must be a relative path, got: %s", worktreesDir)
	}
	cleanedDir := filepath.Clean(worktreesDir)
	if strings.HasPrefix(cleanedDir, "..") {
		return fmt.Errorf("worktrees_dir must not escape repository root, got: %s", worktreesDir)
	}
	return nil
}

// sanitizeBranchName replaces special characters in a branch name using the provided mapping.
// Keys are sorted to ensure deterministic replacement order.
func sanitizeBranchName(branch string, sanitizeChars map[string]string) string {
	// Sort keys for deterministic order
	keys := make([]string, 0, len(sanitizeChars))
	for k := range sanitizeChars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := branch
	for _, from := range keys {
		result = strings.ReplaceAll(result, from, sanitizeChars[from])
	}
	return result
}
