package template

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// PathParser parses worktree paths based on naming template configuration.
// It extracts Host, Owner, Repository, and Branch information from paths.
//
// Limitations:
//   - Only supports templates where each variable is a separate path segment
//   - Templates like "{{.Repository}}-{{.Branch}}" (variables in same segment) are NOT supported
//   - Templates containing {{.Hash}} are not supported (cannot reverse hash)
//   - Extra path segments beyond the template are ignored (e.g., "repo/branch/subdir" matches "{{.Repository}}/{{.Branch}}")
type PathParser struct {
	segments      []string // Ordered variable names from template (e.g., ["Host", "Owner", "Repository", "Branch"])
	fixedPrefixes []string // Fixed prefix segments before variables (e.g., ["worktrees"] for "worktrees/{{.Branch}}")
}

// ParsedPathInfo contains information extracted from a worktree path.
type ParsedPathInfo struct {
	Host       string // e.g., "github.com"
	Owner      string // e.g., "user"
	Repository string // e.g., "myapp"
	Branch     string // Sanitized branch name (e.g., "feature-auth" from "feature/auth")
}

// supportedVariables defines the variables that can be parsed from paths.
var supportedVariables = map[string]bool{
	"Host":       true,
	"Owner":      true,
	"Repository": true,
	"Branch":     true,
}

// templateVarRegex matches Go template variables like {{.VarName}}
var templateVarRegex = regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)

// NewPathParser creates a new PathParser from a template string.
// Returns an error if the template contains unsupported variables (like Hash)
// or if the template is empty.
// The sanitizeChars parameter is accepted for API compatibility but currently unused.
func NewPathParser(templateStr string, _ map[string]string) (*PathParser, error) {
	if templateStr == "" {
		return nil, fmt.Errorf("template string is empty")
	}

	matches := templateVarRegex.FindAllStringSubmatch(templateStr, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no variables found in template")
	}

	var segments []string
	for _, match := range matches {
		varName := match[1]
		if !supportedVariables[varName] {
			return nil, fmt.Errorf("unsupported variable in template: %s (only Host, Owner, Repository, Branch are supported)", varName)
		}
		segments = append(segments, varName)
	}

	return &PathParser{
		segments:      segments,
		fixedPrefixes: extractFixedPrefixes(templateStr),
	}, nil
}

// extractFixedPrefixes extracts fixed path segments before the first template variable.
func extractFixedPrefixes(templateStr string) []string {
	loc := templateVarRegex.FindStringIndex(templateStr)
	if loc == nil || loc[0] == 0 {
		return nil
	}

	prefix := strings.TrimSuffix(templateStr[:loc[0]], "/")
	if prefix == "" {
		return nil
	}
	return strings.Split(prefix, "/")
}

// ParsePath parses a worktree path and extracts information based on the template.
// worktreePath should be an absolute path to the worktree.
// baseDir is the base directory (e.g., ghq root or worktree base directory).
func (p *PathParser) ParsePath(worktreePath, baseDir string) (*ParsedPathInfo, error) {
	worktreePath = filepath.Clean(worktreePath)
	baseDir = filepath.Clean(baseDir)

	if !strings.HasPrefix(worktreePath, baseDir+string(filepath.Separator)) && worktreePath != baseDir {
		return nil, fmt.Errorf("path %s is not under base directory %s", worktreePath, baseDir)
	}

	relPath, err := filepath.Rel(baseDir, worktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative path: %w", err)
	}

	pathSegments := strings.Split(relPath, string(filepath.Separator))

	if err := p.verifyPrefixes(pathSegments); err != nil {
		return nil, err
	}

	varSegments := pathSegments[len(p.fixedPrefixes):]
	if len(varSegments) < len(p.segments) {
		return nil, fmt.Errorf("not enough path segments: expected %d, got %d", len(p.segments), len(varSegments))
	}

	return p.extractInfo(varSegments), nil
}

// verifyPrefixes checks that path segments match the expected fixed prefixes.
func (p *PathParser) verifyPrefixes(pathSegments []string) error {
	for i, expected := range p.fixedPrefixes {
		if i >= len(pathSegments) {
			return fmt.Errorf("path does not match template prefix: expected %q at position %d", expected, i)
		}
		if pathSegments[i] != expected {
			return fmt.Errorf("path does not match template prefix: expected %q, got %q at position %d", expected, pathSegments[i], i)
		}
	}
	return nil
}

// extractInfo extracts ParsedPathInfo from variable segments based on template configuration.
func (p *PathParser) extractInfo(varSegments []string) *ParsedPathInfo {
	info := &ParsedPathInfo{}
	for i, varName := range p.segments {
		switch varName {
		case "Host":
			info.Host = varSegments[i]
		case "Owner":
			info.Owner = varSegments[i]
		case "Repository":
			info.Repository = varSegments[i]
		case "Branch":
			info.Branch = varSegments[i]
		}
	}
	return info
}
