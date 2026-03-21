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
	repository := pathParts[len(pathParts)-1]

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
	// Convert SSH format to HTTPS format for easier parsing
	if strings.HasPrefix(repoURL, "git@") {
		// git@github.com:user/repo.git -> https://github.com/user/repo.git
		if host, path, found := strings.Cut(repoURL, ":"); found {
			host = strings.TrimPrefix(host, "git@")
			repoURL = fmt.Sprintf("https://%s/%s", host, path)
		}
	} else if strings.HasPrefix(repoURL, "ssh://git@") {
		// ssh://git@github.com:user/repo.git -> https://github.com/user/repo.git
		repoURL = strings.TrimPrefix(repoURL, "ssh://")
		if host, path, found := strings.Cut(repoURL, ":"); found {
			host = strings.TrimPrefix(host, "git@")
			repoURL = fmt.Sprintf("https://%s/%s", host, path)
		}
	} else if isSCPLikeURL(repoURL) {
		// SCP-like format without git@ prefix (e.g., SSH config alias)
		// workgit:myorg/myrepo.git -> https://workgit/myorg/myrepo.git
		host, path, _ := strings.Cut(repoURL, ":")
		repoURL = fmt.Sprintf("https://%s/%s", host, path)
	}

	// Ensure https:// prefix
	if !strings.HasPrefix(repoURL, "http://") && !strings.HasPrefix(repoURL, "https://") {
		repoURL = "https://" + repoURL
	}

	return repoURL
}

// isSCPLikeURL checks if a URL string uses SCP-like syntax (host:path)
// without a git@ prefix. This handles SSH config aliases like "workgit:org/repo.git".
//
// Limitation: "host:123/path" where the first path segment is all digits is treated
// as a port number (host:port/path), not SCP-like. This means SSH aliases with
// numeric-only first path segments are not detected. This is an acceptable tradeoff
// since such paths are extremely rare in practice.
func isSCPLikeURL(rawURL string) bool {
	if strings.Contains(rawURL, "://") {
		return false
	}

	if strings.HasPrefix(rawURL, "git@") {
		return false
	}

	// Exclude bracketed IPv6 addresses (e.g., "[::1]:8080/user/repo")
	if strings.HasPrefix(rawURL, "[") {
		return false
	}

	_, after, found := strings.Cut(rawURL, ":")
	if !found || after == "" {
		return false
	}

	// Check if the segment before the first '/' is all digits (a port number).
	portOrPath, _, _ := strings.Cut(after, "/")

	for _, c := range portOrPath {
		if c < '0' || c > '9' {
			return true
		}
	}

	return false
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
