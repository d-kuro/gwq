package filesystem

import (
	"io"
	"os"
)

// FileSystemInterface defines the contract for file system operations
type FileSystemInterface interface {
	// File operations
	Stat(name string) (os.FileInfo, error)
	Remove(name string) error
	RemoveAll(path string) error
	Rename(oldpath, newpath string) error

	// Directory operations
	MkdirAll(path string, perm os.FileMode) error
	ReadDir(dirname string) ([]os.DirEntry, error)

	// File content operations
	WriteFile(filename string, data []byte, perm os.FileMode) error
	ReadFile(filename string) ([]byte, error)

	// File handle operations
	Create(name string) (File, error)
	Open(name string) (File, error)
	OpenFile(name string, flag int, perm os.FileMode) (File, error)

	// Working directory operations
	Getwd() (string, error)
	Chdir(dir string) error

	// Utility operations
	Exists(path string) bool
	IsDir(path string) bool
	UserHomeDir() (string, error)
}

// File interface abstracts file operations
type File interface {
	io.Reader
	io.Writer
	io.Closer
	io.Seeker
	Stat() (os.FileInfo, error)
	Sync() error
	Truncate(size int64) error
	WriteString(s string) (int, error)
	Name() string
}

// StandardFileSystem implements FileSystemInterface using standard library
type StandardFileSystem struct{}

// NewStandardFileSystem creates a new StandardFileSystem
func NewStandardFileSystem() *StandardFileSystem {
	return &StandardFileSystem{}
}

// Stat returns file info
func (fs *StandardFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// Remove removes a file
func (fs *StandardFileSystem) Remove(name string) error {
	return os.Remove(name)
}

// RemoveAll removes a path and any children it contains
func (fs *StandardFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Rename renames a file or directory
func (fs *StandardFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// MkdirAll creates a directory path
func (fs *StandardFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// ReadDir reads directory contents
func (fs *StandardFileSystem) ReadDir(dirname string) ([]os.DirEntry, error) {
	return os.ReadDir(dirname)
}

// WriteFile writes data to a file
func (fs *StandardFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// ReadFile reads file contents
func (fs *StandardFileSystem) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// Create creates a new file
func (fs *StandardFileSystem) Create(name string) (File, error) {
	return os.Create(name)
}

// Open opens a file for reading
func (fs *StandardFileSystem) Open(name string) (File, error) {
	return os.Open(name)
}

// OpenFile opens a file with flags and permissions
func (fs *StandardFileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return os.OpenFile(name, flag, perm)
}

// Getwd returns current working directory
func (fs *StandardFileSystem) Getwd() (string, error) {
	return os.Getwd()
}

// Chdir changes working directory
func (fs *StandardFileSystem) Chdir(dir string) error {
	return os.Chdir(dir)
}

// Exists checks if a path exists
func (fs *StandardFileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir checks if a path is a directory
func (fs *StandardFileSystem) IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// UserHomeDir returns user home directory
func (fs *StandardFileSystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}
