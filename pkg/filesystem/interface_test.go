package filesystem

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStandardFileSystem_Stat(t *testing.T) {
	fs := NewStandardFileSystem()
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{
			name:      "existing file",
			path:      testFile,
			wantError: false,
		},
		{
			name:      "existing directory",
			path:      tmpDir,
			wantError: false,
		},
		{
			name:      "non-existent file",
			path:      filepath.Join(tmpDir, "nonexistent.txt"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := fs.Stat(tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("Stat() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && info == nil {
				t.Errorf("Stat() returned nil info for existing path")
			}
		})
	}
}

func TestStandardFileSystem_WriteFile_ReadFile(t *testing.T) {
	fs := NewStandardFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("Hello, World!")

	// Test WriteFile
	err := fs.WriteFile(testFile, testContent, 0644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Test ReadFile
	content, err := fs.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(content) != string(testContent) {
		t.Errorf("ReadFile() content = %v, want %v", string(content), string(testContent))
	}
}

func TestStandardFileSystem_MkdirAll(t *testing.T) {
	fs := NewStandardFileSystem()
	tmpDir := t.TempDir()

	// Test creating nested directories
	nestedDir := filepath.Join(tmpDir, "level1", "level2", "level3")
	err := fs.MkdirAll(nestedDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Verify directory was created
	info, err := fs.Stat(nestedDir)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Path is not a directory")
	}
}

func TestStandardFileSystem_Remove_RemoveAll(t *testing.T) {
	fs := NewStandardFileSystem()
	tmpDir := t.TempDir()

	// Test Remove (file)
	testFile := filepath.Join(tmpDir, "test.txt")
	err := fs.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = fs.Remove(testFile)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Verify file was removed
	if fs.Exists(testFile) {
		t.Errorf("File still exists after Remove()")
	}

	// Test RemoveAll (directory)
	testDir := filepath.Join(tmpDir, "testdir")
	err = fs.MkdirAll(filepath.Join(testDir, "subdir"), 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	err = fs.WriteFile(filepath.Join(testDir, "file.txt"), []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file in directory: %v", err)
	}

	err = fs.RemoveAll(testDir)
	if err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}

	// Verify directory was removed
	if fs.Exists(testDir) {
		t.Errorf("Directory still exists after RemoveAll()")
	}
}

func TestStandardFileSystem_Rename(t *testing.T) {
	fs := NewStandardFileSystem()
	tmpDir := t.TempDir()

	oldPath := filepath.Join(tmpDir, "old.txt")
	newPath := filepath.Join(tmpDir, "new.txt")
	testContent := []byte("test content")

	// Create original file
	err := fs.WriteFile(oldPath, testContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Rename file
	err = fs.Rename(oldPath, newPath)
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// Verify old file doesn't exist and new file exists with correct content
	if fs.Exists(oldPath) {
		t.Errorf("Old file still exists after rename")
	}

	if !fs.Exists(newPath) {
		t.Errorf("New file doesn't exist after rename")
	}

	content, err := fs.ReadFile(newPath)
	if err != nil {
		t.Fatalf("Failed to read renamed file: %v", err)
	}

	if string(content) != string(testContent) {
		t.Errorf("Renamed file content = %v, want %v", string(content), string(testContent))
	}
}

func TestStandardFileSystem_ReadDir(t *testing.T) {
	fs := NewStandardFileSystem()
	tmpDir := t.TempDir()

	// Create test files and directories
	testFiles := []string{"file1.txt", "file2.txt"}
	testDirs := []string{"dir1", "dir2"}

	for _, file := range testFiles {
		err := fs.WriteFile(filepath.Join(tmpDir, file), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	for _, dir := range testDirs {
		err := fs.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}
	}

	entries, err := fs.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	if len(entries) != len(testFiles)+len(testDirs) {
		t.Errorf("ReadDir() returned %d entries, want %d", len(entries), len(testFiles)+len(testDirs))
	}

	// Verify all expected entries are present
	names := make(map[string]bool)
	for _, entry := range entries {
		names[entry.Name()] = true
	}

	for _, file := range testFiles {
		if !names[file] {
			t.Errorf("Expected file %s not found in ReadDir() results", file)
		}
	}

	for _, dir := range testDirs {
		if !names[dir] {
			t.Errorf("Expected directory %s not found in ReadDir() results", dir)
		}
	}
}

func TestStandardFileSystem_FileOperations(t *testing.T) {
	fs := NewStandardFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")

	// Test Create
	file, err := fs.Create(testFile)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Test writing to file
	testContent := "Hello, World!"
	n, err := file.WriteString(testContent)
	if err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if n != len(testContent) {
		t.Errorf("WriteString() wrote %d bytes, want %d", n, len(testContent))
	}

	err = file.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Test Open and read
	file, err = fs.Open(testFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	content := make([]byte, len(testContent))
	n, err = file.Read(content)
	if err != nil && err != io.EOF {
		t.Fatalf("Read() error = %v", err)
	}
	if n != len(testContent) {
		t.Errorf("Read() read %d bytes, want %d", n, len(testContent))
	}
	if string(content) != testContent {
		t.Errorf("Read() content = %v, want %v", string(content), testContent)
	}

	err = file.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestStandardFileSystem_OpenFile(t *testing.T) {
	fs := NewStandardFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")

	// Test OpenFile with create and write flags
	file, err := fs.OpenFile(testFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}

	_, err = file.WriteString("test content")
	if err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}

	err = file.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Verify file was created
	if !fs.Exists(testFile) {
		t.Errorf("File was not created by OpenFile()")
	}
}

func TestStandardFileSystem_WorkingDirectory(t *testing.T) {
	fs := NewStandardFileSystem()

	// Get current directory
	originalDir, err := fs.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	// Resolve symlinks to handle macOS /var -> /private/var
	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}

	// Change directory
	err = fs.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	// Verify directory changed
	currentDir, err := fs.Getwd()
	if err != nil {
		t.Fatalf("Getwd() after Chdir() error = %v", err)
	}

	if currentDir != tmpDir {
		t.Errorf("Chdir() current directory = %v, want %v", currentDir, tmpDir)
	}

	// Restore original directory
	err = fs.Chdir(originalDir)
	if err != nil {
		t.Fatalf("Failed to restore original directory: %v", err)
	}
}

func TestStandardFileSystem_Exists(t *testing.T) {
	fs := NewStandardFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")

	// Test non-existent file
	if fs.Exists(testFile) {
		t.Errorf("Exists() returned true for non-existent file")
	}

	// Create file and test again
	err := fs.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if !fs.Exists(testFile) {
		t.Errorf("Exists() returned false for existing file")
	}

	// Test with directory
	if !fs.Exists(tmpDir) {
		t.Errorf("Exists() returned false for existing directory")
	}
}

func TestStandardFileSystem_IsDir(t *testing.T) {
	fs := NewStandardFileSystem()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	err := fs.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		wantDir  bool
		wantFile bool
	}{
		{
			name:     "directory",
			path:     tmpDir,
			wantDir:  true,
			wantFile: false,
		},
		{
			name:     "file",
			path:     testFile,
			wantDir:  false,
			wantFile: true,
		},
		{
			name:     "non-existent",
			path:     filepath.Join(tmpDir, "nonexistent"),
			wantDir:  false,
			wantFile: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isDir := fs.IsDir(tt.path)
			if isDir != tt.wantDir {
				t.Errorf("IsDir() = %v, want %v", isDir, tt.wantDir)
			}

			// Additional check: file should exist if it's either a dir or file
			exists := fs.Exists(tt.path)
			if exists != (tt.wantDir || tt.wantFile) {
				t.Errorf("Exists() = %v, want %v", exists, tt.wantDir || tt.wantFile)
			}
		})
	}
}

func TestStandardFileSystem_UserHomeDir(t *testing.T) {
	fs := NewStandardFileSystem()

	homeDir, err := fs.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	if homeDir == "" {
		t.Errorf("UserHomeDir() returned empty string")
	}

	// Home directory should exist
	if !fs.Exists(homeDir) {
		t.Errorf("UserHomeDir() returned non-existent directory: %s", homeDir)
	}

	// Home directory should be a directory
	if !fs.IsDir(homeDir) {
		t.Errorf("UserHomeDir() returned non-directory: %s", homeDir)
	}
}

// Mock FileSystem for testing interface compliance
type MockFileSystem struct {
	files map[string][]byte
}

func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files: make(map[string][]byte),
	}
}

func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if _, exists := m.files[name]; exists {
		return &mockFileInfo{name: filepath.Base(name), isDir: false}, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) Remove(name string) error {
	delete(m.files, name)
	return nil
}

func (m *MockFileSystem) RemoveAll(path string) error {
	for name := range m.files {
		if strings.HasPrefix(name, path) {
			delete(m.files, name)
		}
	}
	return nil
}

func (m *MockFileSystem) Rename(oldpath, newpath string) error {
	if data, exists := m.files[oldpath]; exists {
		m.files[newpath] = data
		delete(m.files, oldpath)
	}
	return nil
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return nil // Mock implementation
}

func (m *MockFileSystem) ReadDir(dirname string) ([]os.DirEntry, error) {
	return nil, nil // Mock implementation
}

func (m *MockFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	m.files[filename] = data
	return nil
}

func (m *MockFileSystem) ReadFile(filename string) ([]byte, error) {
	if data, exists := m.files[filename]; exists {
		return data, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) Create(name string) (File, error) {
	return &mockFile{name: name}, nil
}

func (m *MockFileSystem) Open(name string) (File, error) {
	return &mockFile{name: name}, nil
}

func (m *MockFileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return &mockFile{name: name}, nil
}

func (m *MockFileSystem) Getwd() (string, error) {
	return "/mock/dir", nil
}

func (m *MockFileSystem) Chdir(dir string) error {
	return nil
}

func (m *MockFileSystem) Exists(path string) bool {
	_, exists := m.files[path]
	return exists
}

func (m *MockFileSystem) IsDir(path string) bool {
	return false // Mock implementation - always return false for files
}

func (m *MockFileSystem) UserHomeDir() (string, error) {
	return "/mock/home", nil
}

// Mock implementations for testing
type mockFileInfo struct {
	name  string
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

type mockFile struct {
	name string
}

func (m *mockFile) Read(p []byte) (n int, err error)             { return 0, io.EOF }
func (m *mockFile) Write(p []byte) (n int, err error)            { return len(p), nil }
func (m *mockFile) Close() error                                 { return nil }
func (m *mockFile) Seek(offset int64, whence int) (int64, error) { return 0, nil }
func (m *mockFile) Stat() (os.FileInfo, error)                   { return &mockFileInfo{name: m.name}, nil }
func (m *mockFile) Sync() error                                  { return nil }
func (m *mockFile) Truncate(size int64) error                    { return nil }
func (m *mockFile) WriteString(s string) (int, error)            { return len(s), nil }
func (m *mockFile) Name() string                                 { return m.name }

func TestFileSystemInterface(t *testing.T) {
	// Test that MockFileSystem implements FileSystemInterface
	var fs FileSystemInterface = NewMockFileSystem()

	err := fs.WriteFile("test.txt", []byte("test"), 0644)
	if err != nil {
		t.Errorf("Interface implementation failed: %v", err)
	}

	data, err := fs.ReadFile("test.txt")
	if err != nil {
		t.Errorf("Interface implementation failed: %v", err)
	}
	if string(data) != "test" {
		t.Errorf("Expected 'test', got %v", string(data))
	}

	if !fs.Exists("test.txt") {
		t.Errorf("File should exist")
	}
}
