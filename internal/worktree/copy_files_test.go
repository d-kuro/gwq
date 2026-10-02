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
		dst         string // destination relative to the source; empty means a separate directory
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
		{
			name: "copy directory",
			dirs: []string{"versions/1.20/run/config", "versions/1.20/run/logs"},
			files: map[string]string{
				"versions/1.20/run/config/settings.json": "settings",
				"versions/1.20/run/eula.txt":             "eula",
			},
			patterns: []string{"versions/*/run"},
			expected: []string{
				"versions/1.20/run/config/settings.json",
				"versions/1.20/run/eula.txt",
			},
		},
		{
			name: "directory copy skips nested git metadata",
			dirs: []string{"vendor/lib/.git/objects", "vendor/sub"},
			files: map[string]string{
				"vendor/lib/.git/config": "[core]\n",
				"vendor/lib/a.txt":       "a",
				"vendor/sub/.git":        "gitdir: ../../.git/modules/sub\n",
				"vendor/sub/b.txt":       "b",
			},
			patterns:    []string{"vendor"},
			expected:    []string{"vendor/lib/a.txt", "vendor/sub/b.txt"},
			notExpected: []string{"vendor/lib/.git", "vendor/sub/.git"},
		},
		{
			name:        "destination inside the source, directory match",
			dirs:        []string{"worktrees/feature", "worktrees/other"},
			files:       map[string]string{"worktrees/other/note.txt": "note", "worktrees/feature/own.txt": "own"},
			dst:         "worktrees/feature",
			patterns:    []string{"worktrees"},
			expected:    []string{"worktrees/other/note.txt"},
			notExpected: []string{"worktrees/feature"},
		},
		{
			name:        "destination inside the source, double star match",
			dirs:        []string{"worktrees/feature", "worktrees/other"},
			files:       map[string]string{"worktrees/other/note.txt": "note", "worktrees/feature/own.txt": "own"},
			dst:         "worktrees/feature",
			patterns:    []string{"worktrees/**"},
			expected:    []string{"worktrees/other/note.txt"},
			notExpected: []string{"worktrees/feature"},
		},
		{
			name:        "destination inside the source, suffix filter match",
			dirs:        []string{"worktrees/feature", "worktrees/other"},
			files:       map[string]string{"worktrees/other/note.txt": "note", "worktrees/feature/own.txt": "own"},
			dst:         "worktrees/feature",
			patterns:    []string{"**/*.txt"},
			expected:    []string{"worktrees/other/note.txt"},
			notExpected: []string{"worktrees/feature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcDir := t.TempDir()
			dstDir := t.TempDir()
			if tt.dst != "" {
				dstDir = filepath.Join(srcDir, tt.dst)
			}

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

// countingFS counts how often each destination file is opened for writing.
type countingFS struct {
	filesystem.FileSystemInterface
	writes map[string]int
}

func (c *countingFS) OpenFile(name string, flag int, perm os.FileMode) (filesystem.File, error) {
	if flag&os.O_CREATE != 0 {
		c.writes[name]++
	}
	return c.FileSystemInterface.OpenFile(name, flag, perm)
}

func TestCopyFilesWithGlob_CopiesEachFileOnce(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "config", "a", "b"), 0755); err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config", "a", "b", "x.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	fs := &countingFS{FileSystemInterface: filesystem.NewStandardFileSystem(), writes: map[string]int{}}
	errs := CopyFilesWithGlob(fs, srcDir, dstDir, []string{"config/**", "config"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	dst := filepath.Join(dstDir, "config", "a", "b", "x.json")
	if fs.writes[dst] != 1 {
		t.Errorf("%s written %d times, want 1", dst, fs.writes[dst])
	}
}

func TestCopyFilesWithGlob_DoesNotFollowSymlinkedDirectories(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
	}{
		{name: "directory match", patterns: []string{"run"}},
		{name: "double star match", patterns: []string{"run/**"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcDir := t.TempDir()
			dstDir := t.TempDir()
			target := t.TempDir()
			if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("target"), 0644); err != nil {
				t.Fatalf("failed to write target: %v", err)
			}
			run := filepath.Join(srcDir, "run")
			if err := os.MkdirAll(run, 0755); err != nil {
				t.Fatalf("failed to create directory: %v", err)
			}
			if err := os.Symlink(target, filepath.Join(run, "dir-link")); err != nil {
				t.Fatalf("failed to create symlink: %v", err)
			}
			if err := os.Symlink(filepath.Join(target, "file.txt"), filepath.Join(run, "file-link")); err != nil {
				t.Fatalf("failed to create symlink: %v", err)
			}

			fs := filesystem.NewStandardFileSystem()
			errs := CopyFilesWithGlob(fs, srcDir, dstDir, tt.patterns)
			if len(errs) != 1 {
				t.Errorf("expected one warning for dir-link, got %v", errs)
			}
			if _, err := os.Lstat(filepath.Join(dstDir, "run", "dir-link")); err == nil {
				t.Error("expected dir-link to be skipped")
			}
			if got, _ := os.ReadFile(filepath.Join(dstDir, "run", "file-link")); string(got) != "target" {
				t.Errorf("file-link content = %q, want %q", got, "target")
			}
		})
	}
}

func TestCopyFilesWithGlob_FollowsLiteralSymlinkedDirectory(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
	}{
		{name: "after a directory match", patterns: []string{"config", "config/link"}},
		{name: "before a directory match", patterns: []string{"config/link", "config"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcDir := t.TempDir()
			dstDir := t.TempDir()
			target := t.TempDir()
			if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("target"), 0644); err != nil {
				t.Fatalf("failed to write target: %v", err)
			}
			if err := os.MkdirAll(filepath.Join(srcDir, "config"), 0755); err != nil {
				t.Fatalf("failed to create directory: %v", err)
			}
			if err := os.Symlink(target, filepath.Join(srcDir, "config", "link")); err != nil {
				t.Fatalf("failed to create symlink: %v", err)
			}

			fs := filesystem.NewStandardFileSystem()
			CopyFilesWithGlob(fs, srcDir, dstDir, tt.patterns)
			if got, _ := os.ReadFile(filepath.Join(dstDir, "config", "link", "file.txt")); string(got) != "target" {
				t.Errorf("config/link/file.txt content = %q, want %q", got, "target")
			}
		})
	}
}

func TestCopyFilesWithGlob_SkipsDestinationThroughSymlink(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(srcDir, "worktrees", "feature")
	for _, dir := range []string{dstDir, filepath.Join(srcDir, "worktrees", "other")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(srcDir, "worktrees", "other", "note.txt"), []byte("note"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.Symlink("worktrees", filepath.Join(srcDir, "alias")); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	fs := filesystem.NewStandardFileSystem()
	if errs := CopyFilesWithGlob(fs, srcDir, dstDir, []string{"alias"}); len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "alias", "other", "note.txt")); err != nil {
		t.Errorf("expected sibling content to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "alias", "feature")); err == nil {
		t.Error("destination was copied into itself")
	}
}

func TestCopyFilesWithGlob_CopiesDirectoryDifferingFromDestinationOnlyInCase(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(srcDir, "worktrees", "feature")
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatalf("failed to create destination: %v", err)
	}
	other := filepath.Join(srcDir, "WORKTREES", "FEATURE")
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if info, err := os.Stat(dstDir); err == nil {
		if otherInfo, err := os.Stat(other); err == nil && os.SameFile(info, otherInfo) {
			t.Skip("filesystem is case-insensitive")
		}
	}
	if err := os.WriteFile(filepath.Join(other, "x.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	fs := filesystem.NewStandardFileSystem()
	if errs := CopyFilesWithGlob(fs, srcDir, dstDir, []string{"WORKTREES"}); len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "WORKTREES", "FEATURE", "x.txt")); err != nil {
		t.Errorf("expected WORKTREES/FEATURE/x.txt to be copied: %v", err)
	}
}

func TestCopyFilesWithGlob_PreservesFileMode(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "bin"), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	script := filepath.Join(srcDir, "bin", "run.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	fs := filesystem.NewStandardFileSystem()
	if errs := CopyFilesWithGlob(fs, srcDir, dstDir, []string{"bin"}); len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	info, err := os.Stat(filepath.Join(dstDir, "bin", "run.sh"))
	if err != nil {
		t.Fatalf("expected run.sh to be copied: %v", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Errorf("run.sh mode = %v, want it to stay executable", info.Mode().Perm())
	}
}
