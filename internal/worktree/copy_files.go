package worktree

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/d-kuro/gwq/internal/filesystem"
)

// CopyFilesWithGlob copies files from srcRoot to dstRoot, supporting glob patterns and preserving directory structure.
// A pattern that matches a directory copies it recursively. Git metadata, and dstRoot itself when it lies within
// srcRoot, are never copied. Symlinked files are copied as regular files; symlinked directories are followed only
// when a pattern names them literally. Each file is copied at most once.
// The returned errors cover failed copies and skipped symlinks; copying continues for all files.
func CopyFilesWithGlob(fs filesystem.FileSystemInterface, srcRoot, dstRoot string, patterns []string) []error {
	c := &fileCopier{
		fs:      fs,
		srcRoot: srcRoot,
		dstRoot: dstRoot,
		dstRel:  relativeInside(srcRoot, dstRoot),
		copied:  map[string]bool{},
		skipped: map[string]bool{},
	}
	if info, err := fs.Stat(dstRoot); err == nil {
		c.dstInfo = info
	}
	for _, pattern := range patterns {
		c.copyPattern(pattern)
	}
	return c.errs
}

// fileCopier holds the state of a single CopyFilesWithGlob call. Relative
// paths are slash-separated and relative to srcRoot and dstRoot.
type fileCopier struct {
	fs      filesystem.FileSystemInterface
	srcRoot string
	dstRoot string
	dstRel  string          // dstRoot relative to srcRoot when it lies within it, e.g. basedir = "./worktrees"
	dstInfo os.FileInfo     // dstRoot, to recognize it through symlinks or case variants
	copied  map[string]bool // relative paths already copied
	skipped map[string]bool // relative paths of symlinks already reported as skipped
	errs    []error
}

// copyPattern processes a single glob pattern and copies matching files and directories.
func (c *fileCopier) copyPattern(pattern string) {
	// Collect the matches before copying so the walk never sees files this
	// copy creates. WithNoFollow keeps ** out of symlinked directories and
	// reports wildcard matches that are symlinks as such.
	type match struct {
		relPath string
		d       os.DirEntry
	}
	var matches []match
	err := doublestar.GlobWalk(os.DirFS(c.srcRoot), pattern, func(relPath string, d os.DirEntry) error {
		matches = append(matches, match{relPath, d})
		return nil
	}, doublestar.WithNoFollow())
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("invalid glob pattern %q: %w", pattern, err))
		return
	}
	for _, m := range matches {
		c.copyEntry(m.relPath, m.d)
	}
}

// copyEntry copies a file, or a directory recursively, unless it was already
// copied or must never be copied.
func (c *fileCopier) copyEntry(relPath string, d os.DirEntry) {
	if c.copied[relPath] || isGitMetadata(relPath) || c.isDestination(relPath) {
		return
	}

	srcPath := filepath.Join(c.srcRoot, relPath)
	if d.Type()&os.ModeSymlink != 0 {
		info, err := c.fs.Stat(srcPath)
		if err != nil {
			c.skip(relPath, fmt.Errorf("skip broken symlink %q: %w", srcPath, err))
			return
		}
		if info.IsDir() {
			c.skip(relPath, fmt.Errorf("skip symlinked directory %q", srcPath))
			return
		}
	}
	c.copied[relPath] = true

	dstPath := filepath.Join(c.dstRoot, relPath)
	if d.IsDir() {
		if info, err := c.fs.Stat(srcPath); err == nil && os.SameFile(info, c.dstInfo) {
			return // dstRoot reached through a symlink or a case variant
		}
		c.copyDirectory(relPath, srcPath, dstPath)
		return
	}
	if err := copySingleFile(c.fs, srcPath, dstPath); err != nil {
		c.errs = append(c.errs, err)
	}
}

// copyDirectory recursively copies a directory and its contents.
func (c *fileCopier) copyDirectory(relPath, srcPath, dstPath string) {
	entries, err := c.fs.ReadDir(srcPath)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("read directory %q: %w", srcPath, err))
		return
	}
	if err := c.fs.MkdirAll(dstPath, 0755); err != nil {
		c.errs = append(c.errs, fmt.Errorf("create directory for %q: %w", dstPath, err))
		return
	}
	for _, entry := range entries {
		c.copyEntry(path.Join(relPath, entry.Name()), entry)
	}
}

// skip reports a skipped symlink once, even when several patterns reach it.
// It is not marked as copied, so a pattern naming it literally still follows it.
func (c *fileCopier) skip(relPath string, err error) {
	if !c.skipped[relPath] {
		c.skipped[relPath] = true
		c.errs = append(c.errs, err)
	}
}

// isDestination reports whether relPath is dstRoot or lies under it.
func (c *fileCopier) isDestination(relPath string) bool {
	return c.dstRel != "" && (relPath == c.dstRel || strings.HasPrefix(relPath, c.dstRel+"/"))
}

// relativeInside returns dst relative to src as a slash-separated path when
// dst lies within src, and "" otherwise.
func relativeInside(src, dst string) string {
	src, err := filepath.EvalSymlinks(src)
	if err != nil {
		return ""
	}
	dst, err = filepath.EvalSymlinks(dst)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(src, dst)
	if err != nil || !filepath.IsLocal(rel) {
		return ""
	}
	return filepath.ToSlash(rel)
}

// copySingleFile copies the file at srcPath to dstPath, creating parent directories as needed.
func copySingleFile(fs filesystem.FileSystemInterface, srcPath, dstPath string) (retErr error) {
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
