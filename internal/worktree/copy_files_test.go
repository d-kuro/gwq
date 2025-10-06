package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/d-kuro/gwq/pkg/filesystem"
)

func TestCopyFilesWithGlob(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source files and directories
	if err := os.MkdirAll(filepath.Join(srcDir, "templates"), 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "config"), 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "templates", ".env.example"), []byte("env"), 0644); err != nil {
		t.Fatalf("failed to write .env.example: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config", "a.json"), []byte("a"), 0644); err != nil {
		t.Fatalf("failed to write a.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config", "b.json"), []byte("b"), 0644); err != nil {
		t.Fatalf("failed to write b.json: %v", err)
	}

	fs := filesystem.NewStandardFileSystem()

	errs := CopyFilesWithGlob(fs, srcDir, dstDir, []string{"templates/.env.example", "config/*.json"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}

	check := func(rel string) {
		path := filepath.Join(dstDir, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to be copied, err: %v", rel, err)
		}
	}
	check("templates/.env.example")
	check("config/a.json")
	check("config/b.json")
}
