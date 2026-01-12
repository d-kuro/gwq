// Package template provides directory name template processing functionality.
package template

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/d-kuro/gwq/internal/url"
	"github.com/d-kuro/gwq/pkg/utils"
)

// TemplateData contains the data available for template processing.
type TemplateData struct {
	Host       string // e.g., "github.com"
	Owner      string // e.g., "user1"
	Repository string // e.g., "myapp"
	Branch     string // e.g., "feature/new-ui"
	Hash       string // Short hash of the repository URL + branch
}

// Processor handles template processing for worktree path generation.
type Processor struct {
	template      *template.Template
	sanitizeChars map[string]string
}

// New creates a new template processor.
func New(templateStr string, sanitizeChars map[string]string) (*Processor, error) {
	tmpl, err := template.New("worktree").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return &Processor{
		template:      tmpl,
		sanitizeChars: sanitizeChars,
	}, nil
}

// GeneratePath generates a worktree path using the configured template.
func (p *Processor) GeneratePath(baseDir string, repoInfo *url.RepositoryInfo, branch string) (string, error) {
	// Sanitize branch name only
	sanitizedBranch := p.sanitizeBranch(branch)

	// Create template data
	data := &TemplateData{
		Host:       repoInfo.Host,
		Owner:      repoInfo.Owner,
		Repository: repoInfo.Repository,
		Branch:     sanitizedBranch,
		Hash:       generateShortHash(repoInfo.FullPath + "/" + branch),
	}

	// Execute template
	var buf strings.Builder
	if err := p.template.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	relativePath := buf.String()

	// Join with base directory directly (no additional sanitization)
	// The template output should be used as-is, with only branch having been sanitized
	fullPath := filepath.Join(baseDir, relativePath)

	return fullPath, nil
}

// sanitizeBranch applies character sanitization rules to branch name only.
func (p *Processor) sanitizeBranch(branch string) string {
	sanitized := branch

	// Apply custom sanitize characters to branch name first
	for old, new := range p.sanitizeChars {
		sanitized = strings.ReplaceAll(sanitized, old, new)
	}

	// Then apply default filesystem sanitization to handle remaining problematic characters
	sanitized = utils.SanitizeForFilesystem(sanitized)

	return sanitized
}

// generateShortHash creates a short hash for the given input.
func generateShortHash(input string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:4]) // 8 character hex string
}
