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
// A pattern that matches a directory copies it recursively, skipping git metadata and dstRoot itself when it lies
// within the directory. Symlinked files are copied as regular files; symlinked directories are followed only when a
// pattern names them literally. Each file is copied at most once.
// Errors are returned for each failed copy, but copying continues for all files.
func CopyFilesWithGlob(fs filesystem.FileSystemInterface, srcRoot, dstRoot string, patterns []string) []error {
	c := &fileCopier{fs: fs, srcRoot: srcRoot, dstRoot: dstRoot, seen: map[string]bool{}}
	if info, err := fs.Stat(dstRoot); err == nil {
		c.dstInfo = info
	}
	for _, pattern := range patterns {
		c.copyPattern(pattern)
	}
	return c.errs
}

// fileCopier holds the state of a single CopyFilesWithGlob call.
type fileCopier struct {
	fs      filesystem.FileSystemInterface
	srcRoot string
	dstRoot string
	dstInfo os.FileInfo     // dstRoot, which must never be copied into itself
	seen    map[string]bool // source paths already handled
	errs    []error
}

// copyPattern processes a single glob pattern and copies matching files and directories.
func (c *fileCopier) copyPattern(pattern string) {
	// relPath is relative to srcRoot. WithNoFollow keeps ** out of symlinked
	// directories and reports wildcard matches that are symlinks as such.
	err := doublestar.GlobWalk(os.DirFS(c.srcRoot), pattern, func(relPath string, d os.DirEntry) error {
		if !isGitMetadata(relPath) {
			c.copyEntry(filepath.Join(c.srcRoot, relPath), d)
		}
		return nil
	}, doublestar.WithNoFollow())
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("invalid glob pattern %q: %w", pattern, err))
	}
}

// copyEntry copies a file, or a directory recursively, unless it was already handled.
func (c *fileCopier) copyEntry(srcPath string, d os.DirEntry) {
	if c.seen[srcPath] {
		return
	}
	c.seen[srcPath] = true

	isDir := d.IsDir()
	if d.Type()&os.ModeSymlink != 0 {
		info, err := c.fs.Stat(srcPath)
		if err != nil {
			c.errs = append(c.errs, fmt.Errorf("skip broken symlink %q: %w", srcPath, err))
			return
		}
		if info.IsDir() {
			c.errs = append(c.errs, fmt.Errorf("skip symlinked directory %q", srcPath))
			return
		}
		isDir = false
	}

	if isDir {
		c.copyDirectory(srcPath)
		return
	}
	if err := copySingleFile(c.fs, c.srcRoot, c.dstRoot, srcPath); err != nil {
		c.errs = append(c.errs, err)
	}
}

// copyDirectory recursively copies a directory and its contents.
func (c *fileCopier) copyDirectory(srcDir string) {
	if info, err := c.fs.Stat(srcDir); err == nil && c.dstInfo != nil && os.SameFile(info, c.dstInfo) {
		return
	}

	entries, err := c.fs.ReadDir(srcDir)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("read directory %q: %w", srcDir, err))
		return
	}

	relPath, err := filepath.Rel(c.srcRoot, srcDir)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("compute relative path for %q: %w", srcDir, err))
		return
	}
	dstPath := filepath.Join(c.dstRoot, relPath)
	if err := c.fs.MkdirAll(dstPath, 0755); err != nil {
		c.errs = append(c.errs, fmt.Errorf("create directory for %q: %w", dstPath, err))
		return
	}

	for _, entry := range entries {
		if !isGitMetadata(entry.Name()) {
			c.copyEntry(filepath.Join(srcDir, entry.Name()), entry)
		}
	}
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

	dstFile, err := fs.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
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
