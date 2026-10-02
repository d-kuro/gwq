package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/d-kuro/gwq/internal/filesystem"
)

func TestCopyFilesWithGlob(t *testing.T) {
	tests := []struct {
		name        string
		dirs        []string
		files       map[string]string
		patterns    []string
		expected    []string
		notExpected []string
	}{
		{
			name: "skips gitdir file",
			files: map[string]string{
				".git":       "gitdir: ./.bare\n",
				".gitignore": "ignore",
				".env":       "env",
			},
			patterns:    []string{"*", ".GIT"},
			expected:    []string{".gitignore", ".env"},
			notExpected: []string{".git"},
		},
		{
			name: "skips .git directory",
			dirs: []string{".git/refs"},
			files: map[string]string{
				".git/config": "[core]\n",
				".env":        "env",
			},
			patterns:    []string{"**/*"},
			expected:    []string{".env"},
			notExpected: []string{".git"},
		},
		{
			name: "single file and wildcard",
			dirs: []string{"templates", "config"},
			files: map[string]string{
				"templates/.env.example": "env",
				"config/a.json":          "a",
				"config/b.json":          "b",
			},
			patterns: []string{"templates/.env.example", "config/*.json"},
			expected: []string{
				"templates/.env.example",
				"config/a.json",
				"config/b.json",
			},
		},
		{
			name: "double star recursive",
			dirs: []string{"configs", "configs/dev", "configs/dev/secrets", "other"},
			files: map[string]string{
				"configs/base.yaml":           "base",
				"configs/dev/app.yaml":        "app",
				"configs/dev/secrets/db.yaml": "db",
				"other/ignore.txt":            "ignore",
			},
			patterns: []string{"configs/**"},
			expected: []string{
				"configs/base.yaml",
				"configs/dev/app.yaml",
				"configs/dev/secrets/db.yaml",
			},
			notExpected: []string{"other/ignore.txt"},
		},
		{
			name: "double star with suffix filter",
			dirs: []string{"templates/layouts", "templates/partials/common", "src"},
			files: map[string]string{
				"templates/base.tmpl":                "base",
				"templates/layouts/main.tmpl":        "main",
				"templates/partials/common/nav.tmpl": "nav",
				"templates/README.md":                "readme",
				"src/main.go":                        "go",
			},
			patterns: []string{"templates/**/*.tmpl"},
			expected: []string{
				"templates/base.tmpl",
				"templates/layouts/main.tmpl",
				"templates/partials/common/nav.tmpl",
			},
			notExpected: []string{"templates/README.md", "src/main.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcDir := t.TempDir()
			dstDir := t.TempDir()

			// Create directories
			for _, dir := range tt.dirs {
				if err := os.MkdirAll(filepath.Join(srcDir, dir), 0755); err != nil {
					t.Fatalf("failed to create dir %s: %v", dir, err)
				}
			}

			// Create files
			for path, content := range tt.files {
				if err := os.WriteFile(filepath.Join(srcDir, path), []byte(content), 0644); err != nil {
					t.Fatalf("failed to write %s: %v", path, err)
				}
			}

			fs := filesystem.NewStandardFileSystem()
			errs := CopyFilesWithGlob(fs, srcDir, dstDir, tt.patterns)
			if len(errs) != 0 {
				t.Errorf("expected no errors, got %v", errs)
			}

			// Check expected files exist
			for _, rel := range tt.expected {
				path := filepath.Join(dstDir, rel)
				if _, err := os.Stat(path); err != nil {
					t.Errorf("expected %s to be copied, err: %v", rel, err)
				}
			}

			// Check notExpected files don't exist
			for _, rel := range tt.notExpected {
				path := filepath.Join(dstDir, rel)
				if _, err := os.Stat(path); err == nil {
					t.Errorf("expected %s to NOT be copied", rel)
				}
			}
		})
	}
}

func TestCopyFilesWithGlob_SkipsSameFile(t *testing.T) {
	tests := []struct {
		name       string
		link       string // created in both worktrees, pointing at linkTarget
		linkTarget string // relative to the shared directory
		pattern    string
	}{
		{
			name:       "symlinked file in both worktrees",
			link:       ".env",
			linkTarget: "data.env",
			pattern:    ".env",
		},
		{
			name:       "file under a symlinked directory in both worktrees",
			link:       "config",
			linkTarget: ".",
			pattern:    "config/*.env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shared := t.TempDir()
			sharedFile := filepath.Join(shared, "data.env")
			if err := os.WriteFile(sharedFile, []byte("secret"), 0644); err != nil {
				t.Fatalf("failed to write shared file: %v", err)
			}
			srcDir := t.TempDir()
			dstDir := t.TempDir()
			for _, dir := range []string{srcDir, dstDir} {
				if err := os.Symlink(filepath.Join(shared, tt.linkTarget), filepath.Join(dir, tt.link)); err != nil {
					t.Fatalf("failed to create symlink: %v", err)
				}
			}

			fs := filesystem.NewStandardFileSystem()
			errs := CopyFilesWithGlob(fs, srcDir, dstDir, []string{tt.pattern})
			if len(errs) != 0 {
				t.Errorf("expected no errors, got %v", errs)
			}
			got, err := os.ReadFile(sharedFile)
			if err != nil {
				t.Fatalf("failed to read shared file: %v", err)
			}
			if string(got) != "secret" {
				t.Errorf("shared file content = %q, want %q", got, "secret")
			}
		})
	}
}
