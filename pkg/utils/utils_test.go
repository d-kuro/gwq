package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTildePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get user home directory: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Home directory",
			input:    home,
			expected: "~",
		},
		{
			name:     "Home subdirectory",
			input:    filepath.Join(home, "Documents"),
			expected: "~/Documents",
		},
		{
			name:     "Deep subdirectory",
			input:    filepath.Join(home, "ghq", "github.com", "d-kuro", "gwq"),
			expected: "~/ghq/github.com/d-kuro/gwq",
		},
		{
			name:     "Non-home path",
			input:    "/usr/local/bin",
			expected: "/usr/local/bin",
		},
		{
			name:     "Root path",
			input:    "/",
			expected: "/",
		},
		{
			name:     "Empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "Path similar to home but not home",
			input:    home + "extra",
			expected: home + "extra",
		},
		{
			name:     "Relative path",
			input:    "./relative/path",
			expected: "./relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TildePath(tt.input)
			if result != tt.expected {
				t.Errorf("TildePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTildePathWithDifferentSeparators(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get user home directory: %v", err)
	}

	// Test with unclean paths that might have extra separators
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Home with trailing slash",
			input:    home + string(filepath.Separator),
			expected: "~",
		},
		{
			name:     "Home subdirectory with double separators",
			input:    home + string(filepath.Separator) + string(filepath.Separator) + "Documents",
			expected: "~/Documents",
		},
		{
			name:     "Path with ./ elements",
			input:    filepath.Join(home, ".", "Documents", ".", "Projects"),
			expected: "~/Documents/Projects",
		},
		{
			name:     "Path with ../ elements",
			input:    filepath.Join(home, "Documents", "..", "Downloads"),
			expected: "~/Downloads",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TildePath(tt.input)
			if result != tt.expected {
				t.Errorf("TildePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeForFilesystem(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"feature/test", "feature-test"},
		{"bugfix:issue-123", "bugfix-issue-123"},
		{"feature\\windows", "feature-windows"}, // backslashes are replaced
		{"feat*ure", "feat-ure"},                // asterisks are replaced
		{"normal-branch", "normal-branch"},
		{"multiple//slashes", "multiple--slashes"},
		{"complex?path\"with<bad>chars|", "complex-path-with-bad-chars-"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SanitizeForFilesystem(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeForFilesystem(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEscapeForShell(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "string with double quotes",
			input:    `echo "hello"`,
			expected: `echo \"hello\"`,
		},
		{
			name:     "string with dollar signs",
			input:    "echo $HOME",
			expected: `echo \$HOME`,
		},
		{
			name:     "string with backticks",
			input:    "echo `date`",
			expected: "echo \\`date\\`",
		},
		{
			name:     "string with backslashes",
			input:    `path\to\file`,
			expected: `path\\to\\file`,
		},
		{
			name:     "complex command",
			input:    `git commit -m "Fix bug with $variable and \path"`,
			expected: `git commit -m \"Fix bug with \$variable and \\path\"`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "multiple special chars",
			input:    `"$test"` + "`" + `\`,
			expected: `\"\$test\"` + "\\`" + `\\`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeForShell(tt.input)
			if result != tt.expected {
				t.Errorf("EscapeForShell(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
