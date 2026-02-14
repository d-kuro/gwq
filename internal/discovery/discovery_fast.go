// Package discovery provides filesystem-based global worktree discovery.
package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/d-kuro/gwq/internal/url"
)

// ============================================================================
// Phase 3: Git file direct reading (zero process spawn)
// ============================================================================

// readWorktreeDetailsFast reads worktree details by directly reading git files.
// Returns an error if any step fails (caller should fall back to git commands).
func readWorktreeDetailsFast(worktreePath string) (repoURL string, repoInfo *url.RepositoryInfo, branch, commitHash string, err error) {
	// Read .git file to get gitdir
	gitFile := filepath.Join(worktreePath, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		return "", nil, "", "", err
	}

	contentStr := strings.TrimSpace(string(content))

	// Validate gitdir: prefix before processing
	if !strings.HasPrefix(contentStr, "gitdir:") {
		return "", nil, "", "", fmt.Errorf("invalid .git file format: missing gitdir: prefix")
	}

	gitdirRaw := strings.TrimSpace(strings.TrimPrefix(contentStr, "gitdir:"))

	// Resolve relative paths
	var gitdir string
	if filepath.IsAbs(gitdirRaw) {
		gitdir = gitdirRaw
	} else {
		gitdir = filepath.Join(worktreePath, gitdirRaw)
	}

	// Read HEAD file to get branch or commit hash
	headPath := filepath.Join(gitdir, "HEAD")
	headContent, err := os.ReadFile(headPath)
	if err != nil {
		return "", nil, "", "", err
	}

	branch, commitHash, needsFallback := parseBranchOrCommitFromHead(string(headContent))
	if needsFallback {
		return "", nil, "", "", fmt.Errorf("unsupported ref format")
	}

	// Find main .git directory using commondir file
	mainGitDir := findMainGitDirWithCommondir(gitdir)

	// Get commit hash from refs if not already obtained (detached HEAD case)
	if commitHash == "" && branch != "HEAD" {
		commitHash, err = readCommitFromRefSafe(mainGitDir, branch)
		if err != nil {
			return "", nil, "", "", err
		}
	}

	// Read remote URL from config
	configPath := filepath.Join(mainGitDir, "config")
	repoURL, err = parseRemoteFromConfigSimple(configPath)
	if err != nil {
		return "", nil, "", "", err
	}

	repoInfo, _ = url.ParseRepositoryURL(repoURL)

	return repoURL, repoInfo, branch, commitHash, nil
}

// parseBranchOrCommitFromHead parses HEAD content and returns branch name or commit hash.
// Only refs/heads/ is supported. Other ref types trigger fallback.
func parseBranchOrCommitFromHead(content string) (branch, commit string, needsFallback bool) {
	content = strings.TrimSpace(content)

	// Normal branch reference: "ref: refs/heads/main"
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/"), "", false
	}

	// Detached HEAD: content is a 40-character hex hash
	if len(content) == 40 && isHexString(content) {
		return "HEAD", content, false
	}

	// Other ref formats (refs/tags/, refs/remotes/, etc.) - need git command
	return "", "", true
}

// isHexString checks if a string is a valid hexadecimal string.
func isHexString(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// findMainGitDirWithCommondir finds the main .git directory using commondir file.
func findMainGitDirWithCommondir(gitdir string) string {
	commondirPath := filepath.Join(gitdir, "commondir")
	if content, err := os.ReadFile(commondirPath); err == nil {
		commondir := strings.TrimSpace(string(content))
		if filepath.IsAbs(commondir) {
			return commondir
		}
		return filepath.Join(gitdir, commondir)
	}
	// No commondir file - use heuristic
	return findMainGitDir(gitdir)
}

// findMainGitDir finds the main .git directory from a worktree gitdir path.
// Example: /path/to/repo/.git/worktrees/branch-name → /path/to/repo/.git
func findMainGitDir(gitdir string) string {
	dir := gitdir
	for {
		base := filepath.Base(dir)
		parent := filepath.Dir(dir)
		if base == "worktrees" {
			if filepath.Base(parent) == ".git" {
				return parent
			}
		}
		if parent == dir {
			break // Reached root
		}
		dir = parent
	}
	return gitdir
}

// readCommitFromRefSafe reads commit hash from refs/heads/branch.
// Only attempts to read loose refs. For packed-refs/reftable, returns error.
func readCommitFromRefSafe(mainGitDir, branch string) (string, error) {
	refPath := filepath.Join(mainGitDir, "refs", "heads", branch)
	content, err := os.ReadFile(refPath)
	if err != nil {
		return "", fmt.Errorf("ref file not found, fallback to git command: %w", err)
	}
	return strings.TrimSpace(string(content)), nil
}

// parseRemoteFromConfigSimple parses remote origin URL from git config.
// Does not support include/includeIf directives.
func parseRemoteFromConfigSimple(configPath string) (string, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	contentStr := string(content)
	contentLower := strings.ToLower(contentStr)

	// Bail out if config has includes (too complex to parse)
	if strings.Contains(contentLower, "[include]") ||
		strings.Contains(contentLower, "[includeif") {
		return "", fmt.Errorf("config includes external files")
	}

	// Simple INI-style parsing
	lines := strings.Split(contentStr, "\n")
	inOrigin := false
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		lineLower := strings.ToLower(trimmedLine)

		// Check for origin section header (case-insensitive)
		if lineLower == `[remote "origin"]` {
			inOrigin = true
			continue
		}

		// Check for any other section header
		if inOrigin && strings.HasPrefix(trimmedLine, "[") {
			break // Left origin section
		}

		// Parse url key with flexible whitespace handling
		if inOrigin {
			// Check if line starts with "url" (case-insensitive)
			if strings.HasPrefix(lineLower, "url") {
				rest := trimmedLine[3:] // Remove "url"
				rest = strings.TrimLeft(rest, " \t")
				if value, found := strings.CutPrefix(rest, "="); found {
					// Found url = value
					value = strings.TrimSpace(value)
					if value != "" {
						return value, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("origin remote not found")
}

// extractWorktreeInfoWithFallback tries fast extraction first, falls back to git commands.
func extractWorktreeInfoWithFallback(worktreePath string) (*GlobalWorktreeEntry, error) {
	repoURL, repoInfo, branch, commitHash, err := readWorktreeDetailsFast(worktreePath)
	if err != nil {
		return extractWorktreeInfo(worktreePath)
	}

	entry := &GlobalWorktreeEntry{
		RepositoryURL:  repoURL,
		RepositoryInfo: repoInfo,
		Branch:         branch,
		Path:           worktreePath,
		CommitHash:     commitHash,
		IsMain:         false,
	}
	entry.loaded.Store(true)
	return entry, nil
}
