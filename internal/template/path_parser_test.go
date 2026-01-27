package template

import (
	"testing"
)

func TestNewPathParser(t *testing.T) {
	tests := []struct {
		name          string
		templateStr   string
		sanitizeChars map[string]string
		wantSegments  []string
		wantErr       bool
	}{
		{
			name:          "standard ghq template",
			templateStr:   "{{.Host}}/{{.Owner}}/{{.Repository}}/{{.Branch}}",
			sanitizeChars: map[string]string{"/": "-"},
			wantSegments:  []string{"Host", "Owner", "Repository", "Branch"},
			wantErr:       false,
		},
		{
			name:          "repository and branch only",
			templateStr:   "{{.Repository}}/{{.Branch}}",
			sanitizeChars: nil,
			wantSegments:  []string{"Repository", "Branch"},
			wantErr:       false,
		},
		{
			name:          "branch only",
			templateStr:   "{{.Branch}}",
			sanitizeChars: nil,
			wantSegments:  []string{"Branch"},
			wantErr:       false,
		},
		{
			name:          "template with hash",
			templateStr:   "{{.Repository}}/{{.Hash}}",
			sanitizeChars: nil,
			wantSegments:  nil,
			wantErr:       true, // Hash is not supported
		},
		{
			name:          "template with fixed prefix",
			templateStr:   "worktrees/{{.Repository}}/{{.Branch}}",
			sanitizeChars: nil,
			wantSegments:  []string{"Repository", "Branch"},
			wantErr:       false,
		},
		{
			name:          "empty template",
			templateStr:   "",
			sanitizeChars: nil,
			wantSegments:  nil,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, err := NewPathParser(tt.templateStr, tt.sanitizeChars)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewPathParser() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NewPathParser() unexpected error: %v", err)
				return
			}
			if len(parser.segments) != len(tt.wantSegments) {
				t.Errorf("segments = %v, want %v", parser.segments, tt.wantSegments)
				return
			}
			for i, seg := range tt.wantSegments {
				if parser.segments[i] != seg {
					t.Errorf("segment[%d] = %q, want %q", i, parser.segments[i], seg)
				}
			}
		})
	}
}

func TestPathParser_ParsePath(t *testing.T) {
	tests := []struct {
		name          string
		templateStr   string
		sanitizeChars map[string]string
		worktreePath  string
		baseDir       string
		wantHost      string
		wantOwner     string
		wantRepo      string
		wantBranch    string
		wantErr       bool
	}{
		{
			name:          "standard ghq path",
			templateStr:   "{{.Host}}/{{.Owner}}/{{.Repository}}/{{.Branch}}",
			sanitizeChars: map[string]string{"/": "-"},
			worktreePath:  "/Users/test/ghq/github.com/user/myapp/feature-auth",
			baseDir:       "/Users/test/ghq",
			wantHost:      "github.com",
			wantOwner:     "user",
			wantRepo:      "myapp",
			wantBranch:    "feature-auth",
			wantErr:       false,
		},
		{
			name:          "repository and branch only",
			templateStr:   "{{.Repository}}/{{.Branch}}",
			sanitizeChars: nil,
			worktreePath:  "/Users/test/worktrees/myapp/feature",
			baseDir:       "/Users/test/worktrees",
			wantHost:      "",
			wantOwner:     "",
			wantRepo:      "myapp",
			wantBranch:    "feature",
			wantErr:       false,
		},
		{
			name:          "branch only",
			templateStr:   "{{.Branch}}",
			sanitizeChars: nil,
			worktreePath:  "/Users/test/worktrees/feature-branch",
			baseDir:       "/Users/test/worktrees",
			wantHost:      "",
			wantOwner:     "",
			wantRepo:      "",
			wantBranch:    "feature-branch",
			wantErr:       false,
		},
		{
			name:          "path with fixed prefix",
			templateStr:   "worktrees/{{.Repository}}/{{.Branch}}",
			sanitizeChars: nil,
			worktreePath:  "/Users/test/ghq/worktrees/myapp/feature",
			baseDir:       "/Users/test/ghq",
			wantHost:      "",
			wantOwner:     "",
			wantRepo:      "myapp",
			wantBranch:    "feature",
			wantErr:       false,
		},
		{
			name:          "path not under baseDir",
			templateStr:   "{{.Repository}}/{{.Branch}}",
			sanitizeChars: nil,
			worktreePath:  "/other/path/myapp/feature",
			baseDir:       "/Users/test/worktrees",
			wantErr:       true,
		},
		{
			name:          "not enough path segments",
			templateStr:   "{{.Host}}/{{.Owner}}/{{.Repository}}/{{.Branch}}",
			sanitizeChars: nil,
			worktreePath:  "/Users/test/ghq/github.com/user",
			baseDir:       "/Users/test/ghq",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, err := NewPathParser(tt.templateStr, tt.sanitizeChars)
			if err != nil {
				t.Fatalf("NewPathParser() error: %v", err)
			}

			info, err := parser.ParsePath(tt.worktreePath, tt.baseDir)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParsePath() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ParsePath() unexpected error: %v", err)
				return
			}

			if info.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", info.Host, tt.wantHost)
			}
			if info.Owner != tt.wantOwner {
				t.Errorf("Owner = %q, want %q", info.Owner, tt.wantOwner)
			}
			if info.Repository != tt.wantRepo {
				t.Errorf("Repository = %q, want %q", info.Repository, tt.wantRepo)
			}
			if info.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", info.Branch, tt.wantBranch)
			}
		})
	}
}

func TestPathParser_ParsePath_AbsolutePath(t *testing.T) {
	parser, err := NewPathParser("{{.Repository}}/{{.Branch}}", nil)
	if err != nil {
		t.Fatalf("NewPathParser() error: %v", err)
	}

	// Test with absolute paths (simulating tilde-expanded paths)
	info, err := parser.ParsePath("/home/user/worktrees/myapp/feature", "/home/user/worktrees")
	if err != nil {
		t.Errorf("ParsePath() unexpected error: %v", err)
		return
	}

	if info.Repository != "myapp" {
		t.Errorf("Repository = %q, want %q", info.Repository, "myapp")
	}
	if info.Branch != "feature" {
		t.Errorf("Branch = %q, want %q", info.Branch, "feature")
	}
}
