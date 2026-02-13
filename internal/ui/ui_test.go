package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/d-kuro/gwq/pkg/models"
)

func TestNewPrinter(t *testing.T) {
	tests := []struct {
		name   string
		config *models.UIConfig
		want   *Printer
	}{
		{
			name: "WithIcons",
			config: &models.UIConfig{
				Icons: true,
			},
			want: &Printer{
				useIcons: true,
			},
		},
		{
			name: "WithoutIcons",
			config: &models.UIConfig{
				Icons: false,
			},
			want: &Printer{
				useIcons: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.config)
			if p.useIcons != tt.want.useIcons {
				t.Errorf("useIcons = %v, want %v", p.useIcons, tt.want.useIcons)
			}
		})
	}
}

func TestPrintWorktrees(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	worktrees := []models.Worktree{
		{
			Path:       "/path/to/main",
			Branch:     "main",
			CommitHash: "abc123def456",
			IsMain:     true,
			CreatedAt:  time.Now(),
		},
		{
			Path:       "/path/to/feature",
			Branch:     "feature/test",
			CommitHash: "def456abc789",
			IsMain:     false,
			CreatedAt:  time.Now().Add(-24 * time.Hour),
		},
	}

	p := New(&models.UIConfig{Icons: true})

	// Test simple output
	p.PrintWorktrees(worktrees, false)
	_ = w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)
	os.Stdout = oldStdout

	// Verify output contains expected content
	if !strings.Contains(output, "BRANCH") {
		t.Error("Output should contain BRANCH header")
	}
	if !strings.Contains(output, "PATH") {
		t.Error("Output should contain PATH header")
	}
	if !strings.Contains(output, "main") {
		t.Error("Output should contain main branch")
	}
	if !strings.Contains(output, "feature/test") {
		t.Error("Output should contain feature/test branch")
	}
	if !strings.Contains(output, "●") {
		t.Error("Output should contain main worktree marker when icons enabled")
	}
}

func TestPrintWorktreesVerbose(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	worktrees := []models.Worktree{
		{
			Path:       "/path/to/main",
			Branch:     "main",
			CommitHash: "abc123def456789",
			IsMain:     true,
			CreatedAt:  time.Now(),
		},
	}

	p := New(&models.UIConfig{})
	p.PrintWorktrees(worktrees, true)
	_ = w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)
	os.Stdout = oldStdout

	// Verify verbose output
	if !strings.Contains(output, "COMMIT") {
		t.Error("Verbose output should contain COMMIT header")
	}
	if !strings.Contains(output, "CREATED") {
		t.Error("Verbose output should contain CREATED header")
	}
	if !strings.Contains(output, "TYPE") {
		t.Error("Verbose output should contain TYPE header")
	}
	if !strings.Contains(output, "abc123de") {
		t.Error("Verbose output should contain truncated commit hash")
	}
	if !strings.Contains(output, "main") {
		t.Error("Verbose output should contain worktree type")
	}
}

func TestPrintWorktreesEmpty(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p := New(&models.UIConfig{})
	p.PrintWorktrees([]models.Worktree{}, false)
	_ = w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)
	os.Stdout = oldStdout

	if !strings.Contains(output, "No worktrees found") {
		t.Error("Empty worktrees should print 'No worktrees found'")
	}
}

func TestPrintWorktreesJSON(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	now := time.Now().Truncate(time.Second)
	worktrees := []models.Worktree{
		{
			Path:       "/path/to/main",
			Branch:     "main",
			CommitHash: "abc123",
			IsMain:     true,
			CreatedAt:  now,
		},
	}

	p := New(&models.UIConfig{})
	err := p.PrintWorktreesJSON(worktrees)
	if err != nil {
		t.Fatalf("PrintWorktreesJSON() error = %v", err)
	}

	_ = w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = oldStdout

	// Verify JSON output
	var decoded []models.Worktree
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON output: %v", err)
	}

	if len(decoded) != 1 {
		t.Errorf("Expected 1 worktree in JSON, got %d", len(decoded))
	}

	if decoded[0].Path != worktrees[0].Path {
		t.Errorf("JSON worktree path = %s, want %s", decoded[0].Path, worktrees[0].Path)
	}
}

func TestPrintBranches(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	branches := []models.Branch{
		{
			Name:      "main",
			IsCurrent: true,
			IsRemote:  false,
			LastCommit: models.CommitInfo{
				Hash:    "abc123",
				Message: "Initial commit with a very long message that should be truncated",
				Author:  "Test User",
				Date:    time.Now(),
			},
		},
		{
			Name:      "origin/feature",
			IsCurrent: false,
			IsRemote:  true,
			LastCommit: models.CommitInfo{
				Hash:    "def456",
				Message: "Feature commit",
				Author:  "Another User",
				Date:    time.Now().Add(-48 * time.Hour),
			},
		},
	}

	p := New(&models.UIConfig{Icons: true})
	p.PrintBranches(branches)
	_ = w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)
	os.Stdout = oldStdout

	// Verify output
	if !strings.Contains(output, "BRANCH") {
		t.Error("Output should contain BRANCH header")
	}
	if !strings.Contains(output, "LAST COMMIT") {
		t.Error("Output should contain LAST COMMIT header")
	}
	if !strings.Contains(output, "AUTHOR") {
		t.Error("Output should contain AUTHOR header")
	}
	if !strings.Contains(output, "DATE") {
		t.Error("Output should contain DATE header")
	}
	if !strings.Contains(output, "* main") {
		t.Error("Output should contain current branch marker")
	}
	if !strings.Contains(output, "→ origin/feature") {
		t.Error("Output should contain remote branch marker")
	}
	if !strings.Contains(output, "...") {
		t.Error("Long commit message should be truncated")
	}
}

func TestPrintConfig(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	settings := map[string]any{
		"worktree": map[string]any{
			"basedir":    "~/worktrees",
			"auto_mkdir": true,
		},
		"ui": map[string]any{
			"color": true,
			"icons": false,
		},
		"simple": "value",
	}

	p := New(&models.UIConfig{})
	p.PrintConfig(settings)
	_ = w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)
	os.Stdout = oldStdout

	// Verify output contains all settings
	expectedLines := []string{
		"worktree.basedir = ~/worktrees",
		"worktree.auto_mkdir = true",
		"ui.color = true",
		"ui.icons = false",
		"simple = value",
	}

	for _, expected := range expectedLines {
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain '%s'", expected)
		}
	}
}

func TestPrintError(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	p := New(&models.UIConfig{})
	testErr := fmt.Errorf("test error message")
	p.PrintError(testErr)

	_ = w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)
	os.Stderr = oldStderr

	expected := "Error: test error message\n"
	if output != expected {
		t.Errorf("PrintError() output = %q, want %q", output, expected)
	}
}

func TestPrintSuccess(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p := New(&models.UIConfig{})
	p.PrintSuccess("Operation completed successfully")

	_ = w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)
	os.Stdout = oldStdout

	expected := "Operation completed successfully\n"
	if output != expected {
		t.Errorf("PrintSuccess() output = %q, want %q", output, expected)
	}
}

func TestPrintWorktreePath(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p := New(&models.UIConfig{})
	p.PrintWorktreePath("/path/to/worktree")

	_ = w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)
	os.Stdout = oldStdout

	expected := "/path/to/worktree\n"
	if output != expected {
		t.Errorf("PrintWorktreePath() output = %q, want %q", output, expected)
	}
}

func TestTruncateHash(t *testing.T) {
	p := &Printer{}

	tests := []struct {
		input    string
		expected string
	}{
		{"abc123def456789", "abc123de"},
		{"short", "short"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := p.truncateHash(tt.input)
			if result != tt.expected {
				t.Errorf("truncateHash(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateMessage(t *testing.T) {
	p := &Printer{}

	tests := []struct {
		message  string
		maxLen   int
		expected string
	}{
		{"This is a very long commit message that should be truncated", 20, "This is a very lo..."},
		{"Short message", 20, "Short message"},
		{"20CharactersExactly!", 20, "20CharactersExactly!"},
		{"21 characters message", 20, "21 characters mes..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			result := p.truncateMessage(tt.message, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateMessage(%s, %d) = %s, want %s", tt.message, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	p := &Printer{}
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "ZeroTime",
			time:     time.Time{},
			expected: "unknown",
		},
		{
			name:     "30MinutesAgo",
			time:     now.Add(-30 * time.Minute),
			expected: "30 minutes ago",
		},
		{
			name:     "2HoursAgo",
			time:     now.Add(-2 * time.Hour),
			expected: "2 hours ago",
		},
		{
			name:     "3DaysAgo",
			time:     now.Add(-3 * 24 * time.Hour),
			expected: "3 days ago",
		},
		{
			name:     "2WeeksAgo",
			time:     now.Add(-14 * 24 * time.Hour),
			expected: now.Add(-14 * 24 * time.Hour).Format("2006-01-02"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.formatTime(tt.time)
			if result != tt.expected {
				t.Errorf("formatTime() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestPrintConfigRecursive(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p := &Printer{}

	// Test nested configuration
	data := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": "deep value",
			},
			"simple": 42,
		},
		"root": "root value",
	}

	p.printConfigRecursive("", data)
	_ = w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)
	os.Stdout = oldStdout

	// Verify nested values are printed with correct paths
	expectedLines := []string{
		"level1.level2.level3 = deep value",
		"level1.simple = 42",
		"root = root value",
	}

	for _, expected := range expectedLines {
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain '%s'", expected)
		}
	}
}

func TestPrintConfigRecursiveWithPrefix(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p := &Printer{}

	// Test with initial prefix
	data := map[string]any{
		"key": "value",
	}

	p.printConfigRecursive("prefix", data)
	_ = w.Close()
	out, _ := io.ReadAll(r)
	output := string(out)
	os.Stdout = oldStdout

	expected := "prefix.key = value\n"
	if output != expected {
		t.Errorf("printConfigRecursive() output = %q, want %q", output, expected)
	}
}
