package template

import (
	"path/filepath"
	"testing"

	"github.com/d-kuro/gwq/internal/url"
)

func TestProcessor_GeneratePath(t *testing.T) {
	tests := []struct {
		name          string
		template      string
		sanitizeChars map[string]string
		baseDir       string
		repoInfo      *url.RepositoryInfo
		branch        string
		expected      string
		expectError   bool
	}{
		{
			name:     "default template",
			template: "{{.Host}}/{{.Owner}}/{{.Repository}}/{{.Branch}}",
			baseDir:  "/tmp/worktrees",
			repoInfo: &url.RepositoryInfo{
				Host:       "github.com",
				Owner:      "user1",
				Repository: "myapp",
				FullPath:   "github.com/user1/myapp",
			},
			branch:   "feature/new-ui",
			expected: filepath.Join("/tmp/worktrees", "github.com/user1/myapp/feature-new-ui"),
		},
		{
			name:     "template with .git",
			template: "{{.Host}}/{{.Owner}}/{{.Repository}}/.git/{{.Branch}}",
			baseDir:  "/tmp/worktrees",
			repoInfo: &url.RepositoryInfo{
				Host:       "github.com",
				Owner:      "user1",
				Repository: "myapp",
				FullPath:   "github.com/user1/myapp",
			},
			branch:   "feature/auth",
			expected: filepath.Join("/tmp/worktrees", "github.com/user1/myapp/.git/feature-auth"),
		},
		{
			name:     "template with custom sanitization",
			template: "{{.Repository}}-{{.Branch}}",
			sanitizeChars: map[string]string{
				"/": "_",
				":": "-",
			},
			baseDir: "/tmp/worktrees",
			repoInfo: &url.RepositoryInfo{
				Host:       "github.com",
				Owner:      "user1",
				Repository: "myapp",
				FullPath:   "github.com/user1/myapp",
			},
			branch:   "feature/auth:v2",
			expected: filepath.Join("/tmp/worktrees", "myapp-feature_auth-v2"),
		},
		{
			name:     "template with hash",
			template: "{{.Repository}}-{{.Hash}}",
			baseDir:  "/tmp/worktrees",
			repoInfo: &url.RepositoryInfo{
				Host:       "github.com",
				Owner:      "user1",
				Repository: "myapp",
				FullPath:   "github.com/user1/myapp",
			},
			branch:   "main",
			expected: filepath.Join("/tmp/worktrees", "myapp-dff4fa3c"), // Hash of "github.com/user1/myapp/main"
		},
		{
			name:        "invalid template",
			template:    "{{.Invalid}}",
			baseDir:     "/tmp/worktrees",
			repoInfo:    &url.RepositoryInfo{},
			branch:      "main",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := New(tt.template, tt.sanitizeChars)
			if err != nil {
				if tt.expectError {
					return
				}
				t.Fatalf("Failed to create processor: %v", err)
			}

			result, err := processor.GeneratePath(tt.baseDir, tt.repoInfo, tt.branch)
			if err != nil {
				if tt.expectError {
					return
				}
				t.Fatalf("GeneratePath failed: %v", err)
			}

			if tt.expectError {
				t.Fatalf("Expected error but got result: %s", result)
			}

			if result != tt.expected {
				t.Errorf("GeneratePath() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestTemplateData_Hash(t *testing.T) {
	// Test that hash generation is consistent
	hash1 := generateShortHash("github.com/user1/myapp/main")
	hash2 := generateShortHash("github.com/user1/myapp/main")

	if hash1 != hash2 {
		t.Errorf("Hash generation is not consistent: %s != %s", hash1, hash2)
	}

	// Test that different inputs produce different hashes
	hash3 := generateShortHash("github.com/user1/myapp/feature")
	if hash1 == hash3 {
		t.Errorf("Different inputs produced same hash: %s == %s", hash1, hash3)
	}

	// Test hash length
	if len(hash1) != 8 {
		t.Errorf("Hash length should be 8, got %d", len(hash1))
	}
}

func TestProcessor_SanitizeBranch(t *testing.T) {
	tests := []struct {
		name          string
		sanitizeChars map[string]string
		input         string
		expected      string
	}{
		{
			name: "default sanitization",
			sanitizeChars: map[string]string{
				"/": "-",
				":": "-",
			},
			input:    "feature/auth:v2",
			expected: "feature-auth-v2",
		},
		{
			name: "custom sanitization",
			sanitizeChars: map[string]string{
				"/": "_",
				" ": "-",
			},
			input:    "feature/new ui",
			expected: "feature_new-ui",
		},
		{
			name:          "no sanitization rules",
			sanitizeChars: nil,
			input:         "simple-branch",
			expected:      "simple-branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &Processor{
				sanitizeChars: tt.sanitizeChars,
			}

			result := processor.sanitizeBranch(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeBranch() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestProcessor_GeneratePath_BranchOnlySanitization(t *testing.T) {
	// Test that sanitize_chars only applies to branch name, not the entire path
	processor, err := New("{{.Host}}/{{.Owner}}/{{.Repository}}/.git/{{.Branch}}", map[string]string{
		"/": "_",
	})
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	repoInfo := &url.RepositoryInfo{
		Host:       "github.com",
		Owner:      "user1",
		Repository: "myapp",
		FullPath:   "github.com/user1/myapp",
	}

	result, err := processor.GeneratePath("/tmp/worktrees", repoInfo, "feature/auth")
	if err != nil {
		t.Fatalf("GeneratePath failed: %v", err)
	}

	// The path should keep slashes in the template but sanitize them in the branch name
	expected := filepath.Join("/tmp/worktrees", "github.com/user1/myapp/.git/feature_auth")
	if result != expected {
		t.Errorf("GeneratePath() = %s, want %s", result, expected)
	}
}

func TestNewDisplayProcessor(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		expectError bool
	}{
		{
			name:        "valid template",
			template:    "{{.Host}}/{{.Owner}}/{{.Repository}}:{{.Branch}}",
			expectError: false,
		},
		{
			name:        "simple template",
			template:    "{{.Repository}}:{{.Branch}}",
			expectError: false,
		},
		{
			name:        "invalid template syntax",
			template:    "{{.Repository",
			expectError: true,
		},
		{
			name:        "empty template",
			template:    "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDisplayProcessor(tt.template)
			if (err != nil) != tt.expectError {
				t.Errorf("NewDisplayProcessor() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestDisplayProcessor_Format(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		data        *DisplayTemplateData
		expected    string
		expectError bool
	}{
		{
			name:     "full format with path",
			template: "{{.Host}}/{{.Owner}}/{{.Repository}}:{{.Branch}} ({{.Path}})",
			data: &DisplayTemplateData{
				Host:       "github.com",
				Owner:      "user",
				Repository: "myapp",
				Branch:     "feature",
				Path:       "~/ghq/github.com/user/myapp",
			},
			expected: "github.com/user/myapp:feature (~/ghq/github.com/user/myapp)",
		},
		{
			name:     "simple without path",
			template: "{{.Repository}}:{{.Branch}}",
			data: &DisplayTemplateData{
				Repository: "myapp",
				Branch:     "feature",
			},
			expected: "myapp:feature",
		},
		{
			name:     "path first",
			template: "{{.Path}} [{{.Branch}}]",
			data: &DisplayTemplateData{
				Branch: "feature",
				Path:   "~/project",
			},
			expected: "~/project [feature]",
		},
		{
			name:     "with IsMain conditional - main worktree",
			template: "{{.Repository}}{{if not .IsMain}}:{{.Branch}}{{end}}",
			data: &DisplayTemplateData{
				Repository: "myapp",
				Branch:     "main",
				IsMain:     true,
			},
			expected: "myapp",
		},
		{
			name:     "with IsMain conditional - worktree",
			template: "{{.Repository}}{{if not .IsMain}}:{{.Branch}}{{end}}",
			data: &DisplayTemplateData{
				Repository: "myapp",
				Branch:     "feature",
				IsMain:     false,
			},
			expected: "myapp:feature",
		},
		{
			name:     "owner/repo format",
			template: "{{.Owner}}/{{.Repository}}:{{.Branch}} ({{.Path}})",
			data: &DisplayTemplateData{
				Owner:      "user",
				Repository: "myapp",
				Branch:     "feature",
				Path:       "~/ghq/github.com/user/myapp",
			},
			expected: "user/myapp:feature (~/ghq/github.com/user/myapp)",
		},
		{
			name:     "branch only with path",
			template: "{{.Branch}} ({{.Path}})",
			data: &DisplayTemplateData{
				Branch: "feature",
				Path:   "~/ghq/github.com/user/myapp",
			},
			expected: "feature (~/ghq/github.com/user/myapp)",
		},
		{
			name:     "invalid field reference",
			template: "{{.InvalidField}}",
			data: &DisplayTemplateData{
				Repository: "myapp",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := NewDisplayProcessor(tt.template)
			if err != nil {
				t.Fatalf("Failed to create processor: %v", err)
			}

			result, err := processor.Format(tt.data)
			if (err != nil) != tt.expectError {
				t.Errorf("Format() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError && result != tt.expected {
				t.Errorf("Format() = %q, want %q", result, tt.expected)
			}
		})
	}
}
