package worktree

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/d-kuro/gwq/internal/filesystem"
)

// CopyFilesWithGlob copies files from srcRoot to dstRoot, supporting glob patterns and preserving directory structure.
// Errors are returned for each failed copy, but copying continues for all files.
func CopyFilesWithGlob(fs filesystem.FileSystemInterface, srcRoot, dstRoot string, patterns []string) []error {
	var errs []error
	for _, pattern := range patterns {
		patternErrs := copyFilesForPattern(fs, srcRoot, dstRoot, pattern)
		errs = append(errs, patternErrs...)
	}
	return errs
}

// copyFilesForPattern processes a single glob pattern and copies matching files.
func copyFilesForPattern(fs filesystem.FileSystemInterface, srcRoot, dstRoot, pattern string) []error {
	var errs []error

	// matches are relative paths from srcRoot
	matches, err := doublestar.Glob(os.DirFS(srcRoot), pattern)
	if err != nil {
		return []error{fmt.Errorf("invalid glob pattern %q: %w", pattern, err)}
	}

	for _, relPath := range matches {
		if isGitMetadata(relPath) {
			continue
		}

		srcPath := filepath.Join(srcRoot, relPath)
		info, err := fs.Stat(srcPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("stat %q: %w", srcPath, err))
			continue
		}
		if info.IsDir() {
			errs = append(errs, copyDirectory(fs, srcRoot, dstRoot, srcPath)...)
			continue
		}

		if err := copySingleFile(fs, srcRoot, dstRoot, srcPath); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// copyDirectory recursively copies a directory and its contents.
func copyDirectory(fs filesystem.FileSystemInterface, srcRoot, dstRoot, srcDir string) []error {
	var errs []error

	entries, err := fs.ReadDir(srcDir)
	if err != nil {
		return []error{fmt.Errorf("read directory %q: %w", srcDir, err)}
	}

	relPath, err := filepath.Rel(srcRoot, srcDir)
	if err == nil {
		dstPath := filepath.Join(dstRoot, relPath)
		if err := fs.MkdirAll(dstPath, 0755); err != nil {
			errs = append(errs, fmt.Errorf("create directory for %q: %w", dstPath, err))
		}
	}

	for _, entry := range entries {
		path := filepath.Join(srcDir, entry.Name())
		if entry.IsDir() {
			errs = append(errs, copyDirectory(fs, srcRoot, dstRoot, path)...)
		} else {
			if err := copySingleFile(fs, srcRoot, dstRoot, path); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

// copySingleFile copies a single file from srcPath to the corresponding path under dstRoot.
func copySingleFile(fs filesystem.FileSystemInterface, srcRoot, dstRoot, srcPath string) (retErr error) {
	relPath, err := filepath.Rel(srcRoot, srcPath)
	if err != nil {
		return fmt.Errorf("compute relative path for %q: %w", srcPath, err)
	}

	dstPath := filepath.Join(dstRoot, relPath)
	if err := fs.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("create directory for %q: %w", dstPath, err)
	}

	srcFile, err := fs.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", srcPath, err)
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil && retErr == nil {
			retErr = fmt.Errorf("close source file %q: %w", srcPath, closeErr)
		}
	}()

	// Creating dstPath truncates it. When it resolves to the source file, e.g.
	// through a symlink checked out in both worktrees, that would empty it.
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source file %q: %w", srcPath, err)
	}
	if dstInfo, err := fs.Stat(dstPath); err == nil && os.SameFile(srcInfo, dstInfo) {
		return nil
	}

	dstFile, err := fs.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create destination file %q: %w", dstPath, err)
	}
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil && retErr == nil {
			retErr = fmt.Errorf("close destination file %q: %w", dstPath, closeErr)
		}
	}()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy %q to %q: %w", srcPath, dstPath, err)
	}

	return nil
}

// isGitMetadata reports whether a slash-separated relative path has a .git
// element. Copying it would clobber the new worktree's own .git file. The
// comparison ignores case for case-insensitive filesystems.
func isGitMetadata(relPath string) bool {
	return slices.ContainsFunc(strings.Split(relPath, "/"), func(elem string) bool {
		return strings.EqualFold(elem, ".git")
	})
}
