// Package url provides utilities for handling repository URLs and generating directory paths.
package url

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/d-kuro/gwq/internal/utils"
)

// RepositoryInfo contains parsed repository information.
type RepositoryInfo struct {
	Host       string // e.g., "github.com"
	Owner      string // e.g., "user1"
	Repository string // e.g., "myapp"
	FullPath   string // e.g., "github.com/user1/myapp"
}

// ParseRepositoryURL parses a git repository URL and extracts host, owner, and repository name.
func ParseRepositoryURL(repoURL string) (*RepositoryInfo, error) {
	// Handle different URL formats
	repoURL = normalizeURL(repoURL)

	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository URL: %w", err)
	}

	host := parsedURL.Host
	if host == "" {
		return nil, fmt.Errorf("no host found in URL: %s", repoURL)
	}

	// Extract path components
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < 2 {
		return nil, fmt.Errorf("invalid repository path: %s", parsedURL.Path)
	}

	owner := pathParts[0]
	repository := pathParts[1]

	// Remove .git suffix if present
	repository = strings.TrimSuffix(repository, ".git")

	fullPath := filepath.Join(host, owner, repository)

	return &RepositoryInfo{
		Host:       host,
		Owner:      owner,
		Repository: repository,
		FullPath:   fullPath,
	}, nil
}

// GenerateWorktreePath creates a worktree path based on repository info and branch name.
func GenerateWorktreePath(baseDir string, repoInfo *RepositoryInfo, branch string) string {
	// Sanitize branch name for filesystem
	safeBranch := sanitizeBranchName(branch)
	return filepath.Join(baseDir, repoInfo.FullPath, safeBranch)
}

// normalizeURL converts various git URL formats to a standard HTTP(S) format for parsing.
func normalizeURL(repoURL string) string {
	// Handle AWS CodeCommit credential helper format
	// codecommit::<region>://<profile>@<repo-name> or codecommit::<region>://<repo-name>
	if strings.HasPrefix(repoURL, "codecommit::") {
		if converted := normalizeCodeCommitURL(repoURL); converted != "" {
			return converted
		}
	}

	// Convert SSH formats to HTTPS for easier parsing
	switch {
	case strings.HasPrefix(repoURL, "git@"):
		// git@github.com:user/repo.git -> https://github.com/user/repo.git
		if host, path, found := strings.Cut(repoURL, ":"); found {
			host = strings.TrimPrefix(host, "git@")
			repoURL = fmt.Sprintf("https://%s/%s", host, path)
		}
	case strings.HasPrefix(repoURL, "ssh://git@"):
		// Handle both colon and slash formats:
		// ssh://git@github.com:user/repo.git -> https://github.com/user/repo.git
		// ssh://git@github.com/user/repo.git -> https://github.com/user/repo.git
		rest := strings.TrimPrefix(repoURL, "ssh://git@")
		if host, path, hasColon := strings.Cut(rest, ":"); hasColon {
			repoURL = fmt.Sprintf("https://%s/%s", host, path)
		} else {
			repoURL = "https://" + rest
		}
	case strings.HasPrefix(repoURL, "ssh://"):
		// ssh://github.com/user/repo.git -> https://github.com/user/repo.git
		repoURL = "https://" + strings.TrimPrefix(repoURL, "ssh://")
	}

	// Ensure https:// prefix
	if !strings.HasPrefix(repoURL, "http://") && !strings.HasPrefix(repoURL, "https://") {
		repoURL = "https://" + repoURL
	}

	return repoURL
}

// normalizeCodeCommitURL converts AWS CodeCommit credential helper URL to HTTPS format.
// Format: codecommit::<region>://<profile>@<repo-name> or codecommit::<region>://<repo-name>
// Result: https://git-codecommit.<region>.amazonaws.com/repos/<repo-name>
// Note: We use /repos/<repo-name> instead of /v1/repos/<repo-name> for simpler path parsing.
func normalizeCodeCommitURL(repoURL string) string {
	// Remove "codecommit::" prefix
	rest := strings.TrimPrefix(repoURL, "codecommit::")

	// Split by "://" to get region and repo info
	// Format: <region>://<profile>@<repo-name> or <region>://<repo-name>
	parts := strings.SplitN(rest, "://", 2)
	if len(parts) != 2 {
		return ""
	}

	region := parts[0]
	repoInfo := parts[1]

	// Extract repo name (remove profile@ if present)
	repoName := repoInfo
	if idx := strings.LastIndex(repoInfo, "@"); idx != -1 {
		repoName = repoInfo[idx+1:]
	}

	// Build HTTPS URL (using /repos/<repo-name> for simpler path parsing)
	return fmt.Sprintf("https://git-codecommit.%s.amazonaws.com/repos/%s", region, repoName)
}

// sanitizeBranchName converts branch names to filesystem-safe names.
func sanitizeBranchName(branch string) string {
	return utils.SanitizeForFilesystem(branch)
}

// ParseWorktreePath extracts repository info and branch from a worktree path.
func ParseWorktreePath(worktreePath, baseDir string) (*RepositoryInfo, string, error) {
	// Remove base directory from path
	relPath, err := filepath.Rel(baseDir, worktreePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get relative path: %w", err)
	}

	// Split into components: host/owner/repo/branch
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	if len(parts) < 4 {
		return nil, "", fmt.Errorf("invalid worktree path structure: %s", relPath)
	}

	host := parts[0]
	owner := parts[1]
	repository := parts[2]
	branch := strings.Join(parts[3:], "/") // Branch might contain slashes (converted to -)

	repoInfo := &RepositoryInfo{
		Host:       host,
		Owner:      owner,
		Repository: repository,
		FullPath:   filepath.Join(host, owner, repository),
	}

	return repoInfo, branch, nil
}
