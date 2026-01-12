package worktree

import (
	"io"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/d-kuro/gwq/pkg/filesystem"
)

// CopyFilesWithGlob copies files from srcRoot to dstRoot, supporting glob patterns and preserving directory structure.
// Errors are returned for each failed copy, but copying continues for all files.
func CopyFilesWithGlob(fs filesystem.FileSystemInterface, srcRoot, dstRoot string, patterns []string) []error {
	var errs []error
	for _, pattern := range patterns {
		// matches are relative paths from srcRoot
		matches, err := doublestar.Glob(os.DirFS(srcRoot), pattern)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, relPath := range matches {
			// matches are relative paths from srcRoot
			srcPath := filepath.Join(srcRoot, relPath)

			// Only copy files (not directories)
			info, err := fs.Stat(srcPath)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if info.IsDir() {
				continue
			}

			dstPath := filepath.Join(dstRoot, relPath)
			if err := fs.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				errs = append(errs, err)
				continue
			}

			srcFile, err := fs.Open(srcPath)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			dstFile, err := fs.Create(dstPath)
			if err != nil {
				_ = srcFile.Close()
				errs = append(errs, err)
				continue
			}

			_, err = io.Copy(dstFile, srcFile)
			_ = srcFile.Close()
			if closeErr := dstFile.Close(); closeErr != nil {
				errs = append(errs, closeErr)
			}
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}
