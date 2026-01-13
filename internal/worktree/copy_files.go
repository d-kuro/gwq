package worktree

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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
		srcPath := filepath.Join(srcRoot, relPath)
		info, err := fs.Stat(srcPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("stat %q: %w", srcPath, err))
			continue
		}
		if info.IsDir() {
			continue
		}

		if err := copySingleFile(fs, srcRoot, dstRoot, srcPath); err != nil {
			errs = append(errs, err)
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
