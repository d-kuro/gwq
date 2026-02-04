package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWorktreesDir(t *testing.T) {
	t.Run("creates directory and files when they don't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoRoot := filepath.Join(tmpDir, "repo")
		if err := os.MkdirAll(repoRoot, 0755); err != nil {
			t.Fatalf("failed to create repo dir: %v", err)
		}

		worktreesDir := filepath.Join(repoRoot, ".worktrees")

		err := InitWorktreesDir(worktreesDir, true)
		if err != nil {
			t.Fatalf("InitWorktreesDir() error = %v", err)
		}

		// Check directory was created
		info, err := os.Stat(worktreesDir)
		if err != nil {
			t.Fatalf("directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory, got file")
		}

		// Check .gitignore was created
		gitignorePath := filepath.Join(worktreesDir, ".gitignore")
		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}
		if string(content) != gitignoreContent {
			t.Errorf("gitignore content = %q, want %q", string(content), gitignoreContent)
		}

		// Check README.md was created
		readmePath := filepath.Join(worktreesDir, "README.md")
		content, err = os.ReadFile(readmePath)
		if err != nil {
			t.Fatalf("failed to read README.md: %v", err)
		}
		if string(content) != readmeContent {
			t.Errorf("README.md content = %q, want %q", string(content), readmeContent)
		}
	})

	t.Run("does not create files when autoFiles is false", func(t *testing.T) {
		tmpDir := t.TempDir()
		worktreesDir := filepath.Join(tmpDir, ".worktrees")

		err := InitWorktreesDir(worktreesDir, false)
		if err != nil {
			t.Fatalf("InitWorktreesDir() error = %v", err)
		}

		// Check directory was created
		_, err = os.Stat(worktreesDir)
		if err != nil {
			t.Fatalf("directory not created: %v", err)
		}

		// Check .gitignore was NOT created
		gitignorePath := filepath.Join(worktreesDir, ".gitignore")
		_, err = os.Stat(gitignorePath)
		if !os.IsNotExist(err) {
			t.Error("expected .gitignore to not exist")
		}

		// Check README.md was NOT created
		readmePath := filepath.Join(worktreesDir, "README.md")
		_, err = os.Stat(readmePath)
		if !os.IsNotExist(err) {
			t.Error("expected README.md to not exist")
		}
	})

	t.Run("does not overwrite existing files", func(t *testing.T) {
		tmpDir := t.TempDir()
		worktreesDir := filepath.Join(tmpDir, ".worktrees")
		if err := os.MkdirAll(worktreesDir, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}

		// Create existing .gitignore with custom content
		customContent := "custom content\n"
		gitignorePath := filepath.Join(worktreesDir, ".gitignore")
		if err := os.WriteFile(gitignorePath, []byte(customContent), 0644); err != nil {
			t.Fatalf("failed to write .gitignore: %v", err)
		}

		err := InitWorktreesDir(worktreesDir, true)
		if err != nil {
			t.Fatalf("InitWorktreesDir() error = %v", err)
		}

		// Check .gitignore was NOT overwritten
		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}
		if string(content) != customContent {
			t.Errorf("gitignore was overwritten, got %q, want %q", string(content), customContent)
		}
	})

	t.Run("creates directory with nested path", func(t *testing.T) {
		tmpDir := t.TempDir()
		worktreesDir := filepath.Join(tmpDir, "deep", "nested", ".worktrees")

		err := InitWorktreesDir(worktreesDir, true)
		if err != nil {
			t.Fatalf("InitWorktreesDir() error = %v", err)
		}

		// Check directory was created
		_, err = os.Stat(worktreesDir)
		if err != nil {
			t.Fatalf("directory not created: %v", err)
		}
	})
}

func TestGetWorktreesDir(t *testing.T) {
	tests := []struct {
		name         string
		repoRoot     string
		worktreesDir string
		expectedPath string
	}{
		{
			name:         "default worktrees dir",
			repoRoot:     "/home/user/ghq/github.com/user/repo",
			worktreesDir: ".worktrees",
			expectedPath: "/home/user/ghq/github.com/user/repo/.worktrees",
		},
		{
			name:         "custom worktrees dir",
			repoRoot:     "/home/user/ghq/github.com/user/repo",
			worktreesDir: ".wt",
			expectedPath: "/home/user/ghq/github.com/user/repo/.wt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetWorktreesDir(tt.repoRoot, tt.worktreesDir)
			if got != tt.expectedPath {
				t.Errorf("GetWorktreesDir() = %v, want %v", got, tt.expectedPath)
			}
		})
	}
}

func TestGenerateGhqWorktreePath(t *testing.T) {
	tests := []struct {
		name          string
		repoRoot      string
		worktreesDir  string
		branch        string
		sanitizeChars map[string]string
		wantContains  string
	}{
		{
			name:          "simple branch name",
			repoRoot:      "/home/user/ghq/github.com/user/repo",
			worktreesDir:  ".worktrees",
			branch:        "feature-auth",
			sanitizeChars: map[string]string{"/": "-"},
			wantContains:  ".worktrees/feature-auth",
		},
		{
			name:          "branch with slash",
			repoRoot:      "/home/user/ghq/github.com/user/repo",
			worktreesDir:  ".worktrees",
			branch:        "feature/new-ui",
			sanitizeChars: map[string]string{"/": "-"},
			wantContains:  ".worktrees/feature-new-ui",
		},
		{
			name:          "branch with multiple special chars",
			repoRoot:      "/home/user/ghq/github.com/user/repo",
			worktreesDir:  ".worktrees",
			branch:        "feature/auth:v2",
			sanitizeChars: map[string]string{"/": "-", ":": "-"},
			wantContains:  ".worktrees/feature-auth-v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateGhqWorktreePath(tt.repoRoot, tt.worktreesDir, tt.branch, tt.sanitizeChars)
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("GenerateGhqWorktreePath() = %v, want to contain %v", got, tt.wantContains)
			}
		})
	}
}
