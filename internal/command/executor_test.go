package command

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestStandardExecutor_Execute(t *testing.T) {
	executor := NewStandardExecutor()
	ctx := context.Background()

	tests := []struct {
		name      string
		command   string
		args      []string
		wantError bool
	}{
		{
			name:      "successful command",
			command:   "echo",
			args:      []string{"hello"},
			wantError: false,
		},
		{
			name:      "command with multiple args",
			command:   "echo",
			args:      []string{"hello", "world"},
			wantError: false,
		},
		{
			name:      "non-existent command",
			command:   "nonexistentcommand123",
			args:      []string{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.Execute(ctx, tt.command, tt.args...)
			if (err != nil) != tt.wantError {
				t.Errorf("Execute() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestStandardExecutor_ExecuteWithOutput(t *testing.T) {
	executor := NewStandardExecutor()
	ctx := context.Background()

	tests := []struct {
		name         string
		command      string
		args         []string
		wantContains string
		wantError    bool
	}{
		{
			name:         "echo command",
			command:      "echo",
			args:         []string{"hello world"},
			wantContains: "hello world",
			wantError:    false,
		},
		{
			name:         "date command",
			command:      "date",
			args:         []string{"+%Y"},
			wantContains: "20", // Should contain year starting with 20
			wantError:    false,
		},
		{
			name:      "failing command",
			command:   "false",
			args:      []string{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executor.ExecuteWithOutput(ctx, tt.command, tt.args...)
			if (err != nil) != tt.wantError {
				t.Errorf("ExecuteWithOutput() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && !strings.Contains(output, tt.wantContains) {
				t.Errorf("ExecuteWithOutput() output = %v, want to contain %v", output, tt.wantContains)
			}
		})
	}
}

func TestStandardExecutor_ExecuteInDir(t *testing.T) {
	executor := NewStandardExecutor()
	ctx := context.Background()

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		dir       string
		command   string
		args      []string
		wantError bool
	}{
		{
			name:      "execute in valid directory",
			dir:       tmpDir,
			command:   "pwd",
			args:      []string{},
			wantError: false,
		},
		{
			name:      "execute in non-existent directory",
			dir:       "/nonexistent/directory",
			command:   "pwd",
			args:      []string{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.ExecuteInDir(ctx, tt.dir, tt.command, tt.args...)
			if (err != nil) != tt.wantError {
				t.Errorf("ExecuteInDir() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestStandardExecutor_ExecuteInDirWithOutput(t *testing.T) {
	executor := NewStandardExecutor()
	ctx := context.Background()

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	output, err := executor.ExecuteInDirWithOutput(ctx, tmpDir, "pwd")
	if err != nil {
		t.Fatalf("ExecuteInDirWithOutput() error = %v", err)
	}

	// The output should contain the temp directory path
	if !strings.Contains(output, tmpDir) {
		t.Errorf("ExecuteInDirWithOutput() output = %v, want to contain %v", output, tmpDir)
	}
}

func TestStandardExecutor_ExecuteWithStreams(t *testing.T) {
	executor := NewStandardExecutor()
	ctx := context.Background()

	// Test with custom streams
	stdin := strings.NewReader("test input")
	var stdout, stderr bytes.Buffer

	// Use 'cat' command which echoes stdin to stdout
	err := executor.ExecuteWithStreams(ctx, stdin, &stdout, &stderr, "cat")
	if err != nil {
		t.Fatalf("ExecuteWithStreams() error = %v", err)
	}

	if stdout.String() != "test input" {
		t.Errorf("ExecuteWithStreams() stdout = %v, want %v", stdout.String(), "test input")
	}
}

func TestStandardExecutor_ExecuteWithEnv(t *testing.T) {
	executor := NewStandardExecutor()
	ctx := context.Background()

	// Set a custom environment variable
	env := []string{"TEST_VAR=test_value"}

	err := executor.ExecuteWithEnv(ctx, env, "sh", "-c", "test \"$TEST_VAR\" = \"test_value\"")
	if err != nil {
		t.Errorf("ExecuteWithEnv() error = %v", err)
	}
}

func TestStandardExecutor_ExecuteWithEnvInDir(t *testing.T) {
	executor := NewStandardExecutor()
	ctx := context.Background()

	tmpDir := t.TempDir()
	env := []string{"TEST_VAR=test_value"}

	err := executor.ExecuteWithEnvInDir(ctx, env, tmpDir, "sh", "-c", "test \"$TEST_VAR\" = \"test_value\"")
	if err != nil {
		t.Errorf("ExecuteWithEnvInDir() error = %v", err)
	}
}

func TestStandardExecutor_ExecuteWithOptions(t *testing.T) {
	executor := NewStandardExecutor()
	ctx := context.Background()

	tmpDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	opts := &CommandOptions{
		WorkingDir:  tmpDir,
		Environment: []string{"TEST_VAR=test_value"},
		Stdin:       strings.NewReader(""),
		Stdout:      &stdout,
		Stderr:      &stderr,
	}

	err := executor.ExecuteWithOptions(ctx, "pwd", []string{}, opts)
	if err != nil {
		t.Errorf("ExecuteWithOptions() error = %v", err)
	}

	if !strings.Contains(stdout.String(), tmpDir) {
		t.Errorf("ExecuteWithOptions() output should contain working directory")
	}
}

func TestStandardExecutor_ExecuteWithOptionsAndOutput(t *testing.T) {
	executor := NewStandardExecutor()
	ctx := context.Background()

	tmpDir := t.TempDir()

	opts := &CommandOptions{
		WorkingDir:  tmpDir,
		Environment: []string{"TEST_VAR=test_value"},
	}

	output, err := executor.ExecuteWithOptionsAndOutput(ctx, "pwd", []string{}, opts)
	if err != nil {
		t.Errorf("ExecuteWithOptionsAndOutput() error = %v", err)
	}

	if !strings.Contains(output, tmpDir) {
		t.Errorf("ExecuteWithOptionsAndOutput() output = %v, want to contain %v", output, tmpDir)
	}
}

func TestStandardExecutor_CancelledContext(t *testing.T) {
	executor := NewStandardExecutor()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := executor.Execute(ctx, "sleep", "1")
	if err == nil {
		t.Error("Execute() should fail with cancelled context")
	}
}

// Mock executor for testing interface compliance
type MockExecutor struct {
	executeFunc func(ctx context.Context, name string, args ...string) error
	outputFunc  func(ctx context.Context, name string, args ...string) (string, error)
}

func (m *MockExecutor) Execute(ctx context.Context, name string, args ...string) error {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, name, args...)
	}
	return nil
}

func (m *MockExecutor) ExecuteWithOutput(ctx context.Context, name string, args ...string) (string, error) {
	if m.outputFunc != nil {
		return m.outputFunc(ctx, name, args...)
	}
	return "mock output", nil
}

func (m *MockExecutor) ExecuteInDir(ctx context.Context, dir, name string, args ...string) error {
	return m.Execute(ctx, name, args...)
}

func (m *MockExecutor) ExecuteInDirWithOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	return m.ExecuteWithOutput(ctx, name, args...)
}

func (m *MockExecutor) ExecuteWithStreams(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	return m.Execute(ctx, name, args...)
}

func (m *MockExecutor) ExecuteWithEnv(ctx context.Context, env []string, name string, args ...string) error {
	return m.Execute(ctx, name, args...)
}

func (m *MockExecutor) ExecuteWithEnvInDir(ctx context.Context, env []string, dir, name string, args ...string) error {
	return m.Execute(ctx, name, args...)
}

func TestCommandExecutorInterface(t *testing.T) {
	// Test that MockExecutor implements CommandExecutor interface
	var executor CommandExecutor = &MockExecutor{}
	ctx := context.Background()

	err := executor.Execute(ctx, "test")
	if err != nil {
		t.Errorf("Interface implementation failed: %v", err)
	}

	output, err := executor.ExecuteWithOutput(ctx, "test")
	if err != nil {
		t.Errorf("Interface implementation failed: %v", err)
	}
	if output != "mock output" {
		t.Errorf("Expected 'mock output', got %v", output)
	}
}

func TestAdvancedCommandExecutorInterface(t *testing.T) {
	// Test that StandardExecutor implements AdvancedCommandExecutor interface
	var executor AdvancedCommandExecutor = NewStandardExecutor()
	ctx := context.Background()

	opts := &CommandOptions{
		WorkingDir: os.TempDir(),
	}

	err := executor.ExecuteWithOptions(ctx, "echo", []string{"test"}, opts)
	if err != nil {
		t.Errorf("AdvancedCommandExecutor implementation failed: %v", err)
	}

	output, err := executor.ExecuteWithOptionsAndOutput(ctx, "echo", []string{"test"}, opts)
	if err != nil {
		t.Errorf("AdvancedCommandExecutor implementation failed: %v", err)
	}
	if !strings.Contains(output, "test") {
		t.Errorf("Expected output to contain 'test', got %v", output)
	}
}
