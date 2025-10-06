package worktree

import (
	"io"
	"path/filepath"

	"github.com/d-kuro/gwq/pkg/filesystem"
)

// CopyFilesWithGlob copies files from srcRoot to dstRoot, supporting glob patterns and preserving directory structure.
// Errors are returned for each failed copy, but copying continues for all files.
func CopyFilesWithGlob(fs filesystem.FileSystemInterface, srcRoot, dstRoot string, patterns []string) []error {
	var errs []error
	for _, pattern := range patterns {
		// Expand pattern relative to srcRoot
		globPattern := filepath.Join(srcRoot, pattern)
		matches, err := filepath.Glob(globPattern)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, srcPath := range matches {
			// Only copy files (not directories)
			info, err := fs.Stat(srcPath)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if info.IsDir() {
				continue
			}
			relPath, err := filepath.Rel(srcRoot, srcPath)
			if err != nil {
				errs = append(errs, err)
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
				srcFile.Close()
				errs = append(errs, err)
				continue
			}
			_, err = io.Copy(dstFile, srcFile)
			srcFile.Close()
			dstFile.Close()
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}
