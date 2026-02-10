package finder

import (
	"strings"
	"testing"
	"time"

	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/template"
	"github.com/d-kuro/gwq/internal/tmux"
	"github.com/d-kuro/gwq/pkg/models"
)

// newTestDisplayProcessor creates a DisplayProcessor for testing purposes.
func newTestDisplayProcessor(templateStr string) (*template.DisplayProcessor, error) {
	return template.NewDisplayProcessor(templateStr)
}

func TestNew(t *testing.T) {
	g := &git.Git{}
	config := &models.FinderConfig{Preview: true}

	finder := New(g, config)

	if finder == nil {
		t.Fatal("New() returned nil")
	}
	if finder.git != g {
		t.Error("git instance not set correctly")
	}
	if finder.config != config {
		t.Error("config not set correctly")
	}
	if finder.useTildeHome {
		t.Error("useTildeHome should be false by default")
	}
}

func TestNewWithUI(t *testing.T) {
	g := &git.Git{}
	config := &models.FinderConfig{Preview: true}
	uiConfig := &models.UIConfig{TildeHome: true}

	finder := NewWithUI(g, config, uiConfig, nil)

	if finder == nil {
		t.Fatal("NewWithUI() returned nil")
	}
	if finder.git != g {
		t.Error("git instance not set correctly")
	}
	if finder.config != config {
		t.Error("config not set correctly")
	}
	if !finder.useTildeHome {
		t.Error("useTildeHome should be true when set in UI config")
	}
}

func TestNewWithUI_WithDisplayTemplate(t *testing.T) {
	g := &git.Git{}
	config := &models.FinderConfig{Preview: false}
	uiConfig := &models.UIConfig{TildeHome: false}
	namingConfig := &models.NamingConfig{
		DisplayTemplate: "{{.Repository}}:{{.Branch}}",
	}

	finder := NewWithUI(g, config, uiConfig, namingConfig)

	if finder == nil {
		t.Fatal("NewWithUI() returned nil")
	}
	if finder.displayProcessor == nil {
		t.Error("displayProcessor should be set when DisplayTemplate is configured")
	}
}

func TestNewWithUI_InvalidDisplayTemplate(t *testing.T) {
	g := &git.Git{}
	config := &models.FinderConfig{Preview: false}
	uiConfig := &models.UIConfig{TildeHome: false}
	namingConfig := &models.NamingConfig{
		DisplayTemplate: "{{.Invalid", // Invalid template syntax
	}

	finder := NewWithUI(g, config, uiConfig, namingConfig)

	if finder == nil {
		t.Fatal("NewWithUI() returned nil")
	}
	if finder.displayProcessor != nil {
		t.Error("displayProcessor should be nil for invalid template")
	}
}

func TestSelectWorktree_EmptyList(t *testing.T) {
	finder := &Finder{}
	worktrees := []models.Worktree{}

	result, err := finder.SelectWorktree(worktrees)

	if err == nil {
		t.Error("Expected error for empty worktrees list")
	}
	if result != nil {
		t.Error("Expected nil result for empty worktrees list")
	}
	if !strings.Contains(err.Error(), "no worktrees available") {
		t.Errorf("Expected 'no worktrees available' error, got: %v", err)
	}
}

func TestSelectBranch_EmptyList(t *testing.T) {
	finder := &Finder{}
	branches := []models.Branch{}

	result, err := finder.SelectBranch(branches)

	if err == nil {
		t.Error("Expected error for empty branches list")
	}
	if result != nil {
		t.Error("Expected nil result for empty branches list")
	}
	if !strings.Contains(err.Error(), "no branches available") {
		t.Errorf("Expected 'no branches available' error, got: %v", err)
	}
}

func TestSelectMultipleWorktrees_EmptyList(t *testing.T) {
	finder := &Finder{}
	worktrees := []models.Worktree{}

	result, err := finder.SelectMultipleWorktrees(worktrees)

	if err == nil {
		t.Error("Expected error for empty worktrees list")
	}
	if result != nil {
		t.Error("Expected nil result for empty worktrees list")
	}
	if !strings.Contains(err.Error(), "no worktrees available") {
		t.Errorf("Expected 'no worktrees available' error, got: %v", err)
	}
}

func TestSelectSession_EmptyList(t *testing.T) {
	finder := &Finder{}
	sessions := []*tmux.Session{}

	result, err := finder.SelectSession(sessions)

	if err == nil {
		t.Error("Expected error for empty sessions list")
	}
	if result != nil {
		t.Error("Expected nil result for empty sessions list")
	}
	if !strings.Contains(err.Error(), "no tmux sessions available") {
		t.Errorf("Expected 'no tmux sessions available' error, got: %v", err)
	}
}

func TestSelectMultipleSessions_EmptyList(t *testing.T) {
	finder := &Finder{}
	sessions := []*tmux.Session{}

	result, err := finder.SelectMultipleSessions(sessions)

	if err == nil {
		t.Error("Expected error for empty sessions list")
	}
	if result != nil {
		t.Error("Expected nil result for empty sessions list")
	}
	if !strings.Contains(err.Error(), "no tmux sessions available") {
		t.Errorf("Expected 'no tmux sessions available' error, got: %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "just now",
			duration: 30 * time.Second,
			expected: "just now",
		},
		{
			name:     "1 minute",
			duration: 1 * time.Minute,
			expected: "1 min",
		},
		{
			name:     "multiple minutes",
			duration: 5 * time.Minute,
			expected: "5 mins",
		},
		{
			name:     "1 hour",
			duration: 1 * time.Hour,
			expected: "1 hour",
		},
		{
			name:     "multiple hours",
			duration: 3 * time.Hour,
			expected: "3 hours",
		},
		{
			name:     "1 day",
			duration: 24 * time.Hour,
			expected: "1 day",
		},
		{
			name:     "multiple days",
			duration: 5 * 24 * time.Hour,
			expected: "5 days",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %s, expected %s", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestTruncateHash(t *testing.T) {
	tests := []struct {
		name     string
		hash     string
		expected string
	}{
		{
			name:     "short hash",
			hash:     "abc123",
			expected: "abc123",
		},
		{
			name:     "exact 8 chars",
			hash:     "abc12345",
			expected: "abc12345",
		},
		{
			name:     "long hash",
			hash:     "abc1234567890def",
			expected: "abc12345",
		},
		{
			name:     "empty hash",
			hash:     "",
			expected: "",
		},
		{
			name:     "full git hash",
			hash:     "a1b2c3d4e5f6789012345678901234567890abcd",
			expected: "a1b2c3d4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateHash(tt.hash)
			if result != tt.expected {
				t.Errorf("truncateHash(%s) = %s, expected %s", tt.hash, result, tt.expected)
			}
		})
	}
}

func TestTruncateMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		maxLen   int
		expected string
	}{
		{
			name:     "short message",
			message:  "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			message:  "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long message",
			message:  "this is a very long commit message",
			maxLen:   10,
			expected: "this is...",
		},
		{
			name:     "empty message",
			message:  "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "max length too small",
			message:  "hello",
			maxLen:   3,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateMessage(tt.message, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateMessage(%s, %d) = %s, expected %s", tt.message, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestGenerateSessionPreview(t *testing.T) {
	startTime := time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)
	session := &tmux.Session{
		SessionName: "test-session",
		Context:     "test-context",
		Identifier:  "test-id",
		Command:     "vim test.go",
		StartTime:   startTime,
		WorkingDir:  "/home/user/project",
		Metadata: map[string]string{
			"branch": "main",
			"repo":   "test-repo",
		},
	}

	finder := &Finder{}
	preview := finder.generateSessionPreview(session, 20)

	expectedContent := []string{
		"Session: test-session",
		"Context: test-context",
		"Identifier: test-id",
		"Command: vim test.go",
		"Directory: /home/user/project",
		"Metadata:",
		"branch: main",
		"repo: test-repo",
	}

	for _, content := range expectedContent {
		if !strings.Contains(preview, content) {
			t.Errorf("Expected preview to contain '%s', but it didn't. Preview:\n%s", content, preview)
		}
	}
}

func TestGenerateSessionPreview_NoWorkingDir(t *testing.T) {
	session := &tmux.Session{
		SessionName: "test-session",
		Context:     "test-context",
		Identifier:  "test-id",
		Command:     "vim test.go",
		StartTime:   time.Now(),
	}

	finder := &Finder{}
	preview := finder.generateSessionPreview(session, 20)

	if strings.Contains(preview, "Directory:") {
		t.Error("Expected preview to not contain 'Directory:' when WorkingDir is empty")
	}
}

func TestGenerateSessionPreview_MaxLines(t *testing.T) {
	session := &tmux.Session{
		SessionName: "test-session",
		Context:     "test-context",
		Identifier:  "test-id",
		Command:     "vim test.go",
		StartTime:   time.Now(),
		WorkingDir:  "/home/user/project",
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	finder := &Finder{}
	preview := finder.generateSessionPreview(session, 5) // Limit to 5 lines

	lines := strings.Split(preview, "\n")
	if len(lines) > 5 {
		t.Errorf("Expected preview to have at most 5 lines, got %d lines", len(lines))
	}
}

func TestGenerateWorktreePreview_WithoutGit(t *testing.T) {
	wt := models.Worktree{
		Branch:     "feature-branch",
		Path:       "/home/user/project/feature",
		CommitHash: "abc1234567890def",
		CreatedAt:  time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC),
		IsMain:     false,
	}

	finder := &Finder{git: nil} // No git instance
	preview := finder.generateWorktreePreview(wt, 20)

	expectedContent := []string{
		"Branch: feature-branch",
		"Path: /home/user/project/feature",
		"Commit: abc12345",
		"Created: 2023-06-15 10:30",
		"Type: Additional worktree",
	}

	for _, content := range expectedContent {
		if !strings.Contains(preview, content) {
			t.Errorf("Expected preview to contain '%s', but it didn't. Preview:\n%s", content, preview)
		}
	}

	// Should not contain recent commits since git is nil
	if strings.Contains(preview, "Recent commits:") {
		t.Error("Expected preview to not contain 'Recent commits:' when git is nil")
	}
}

func TestGenerateWorktreePreview_MainWorktree(t *testing.T) {
	wt := models.Worktree{
		Branch:     "main",
		Path:       "/home/user/project",
		CommitHash: "abc1234567890def",
		CreatedAt:  time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC),
		IsMain:     true,
	}

	finder := &Finder{git: nil}
	preview := finder.generateWorktreePreview(wt, 20)

	if !strings.Contains(preview, "Type: Main worktree") {
		t.Error("Expected preview to contain 'Type: Main worktree' for main worktree")
	}
}

func TestGenerateWorktreePreview_WithTildeHome(t *testing.T) {
	wt := models.Worktree{
		Branch:     "feature-branch",
		Path:       "/home/user/project/feature",
		CommitHash: "abc1234567890def",
		CreatedAt:  time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC),
		IsMain:     false,
	}

	finder := &Finder{
		git:          nil,
		useTildeHome: true,
	}
	preview := finder.generateWorktreePreview(wt, 20)

	// Note: The actual tilde replacement behavior depends on utils.TildePath implementation
	// This test verifies the method is called, not the exact output
	if !strings.Contains(preview, "Path:") {
		t.Error("Expected preview to contain 'Path:' field")
	}
}

func TestGenerateBranchPreview_Current(t *testing.T) {
	branch := models.Branch{
		Name:      "main",
		IsCurrent: true,
		IsRemote:  false,
		LastCommit: models.CommitInfo{
			Hash:    "abc1234567890def",
			Message: "Initial commit",
			Author:  "John Doe",
			Date:    time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC),
		},
	}

	finder := &Finder{}
	preview := finder.generateBranchPreview(branch, 20)

	expectedContent := []string{
		"Branch: main",
		"Type: Current",
		"Last commit: Initial commit",
		"Author: John Doe",
		"Date: 2023-06-15 10:30",
		"Hash: abc12345",
	}

	for _, content := range expectedContent {
		if !strings.Contains(preview, content) {
			t.Errorf("Expected preview to contain '%s', but it didn't. Preview:\n%s", content, preview)
		}
	}
}

func TestGenerateBranchPreview_Remote(t *testing.T) {
	branch := models.Branch{
		Name:      "origin/feature",
		IsCurrent: false,
		IsRemote:  true,
		LastCommit: models.CommitInfo{
			Hash:    "def1234567890abc",
			Message: "Add feature implementation",
			Author:  "Jane Smith",
			Date:    time.Date(2023, 6, 15, 11, 45, 0, 0, time.UTC),
		},
	}

	finder := &Finder{}
	preview := finder.generateBranchPreview(branch, 20)

	if !strings.Contains(preview, "Type: Remote") {
		t.Error("Expected preview to contain 'Type: Remote' for remote branch")
	}
}

func TestGenerateBranchPreview_Local(t *testing.T) {
	branch := models.Branch{
		Name:      "feature-branch",
		IsCurrent: false,
		IsRemote:  false,
		LastCommit: models.CommitInfo{
			Hash:    "def1234567890abc",
			Message: "Work in progress",
			Author:  "Developer",
			Date:    time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC),
		},
	}

	finder := &Finder{}
	preview := finder.generateBranchPreview(branch, 20)

	if !strings.Contains(preview, "Type: Local") {
		t.Error("Expected preview to contain 'Type: Local' for local branch")
	}
}

func TestGenerateBranchPreview_MaxLines(t *testing.T) {
	branch := models.Branch{
		Name:      "test-branch",
		IsCurrent: false,
		IsRemote:  false,
		LastCommit: models.CommitInfo{
			Hash:    "abc123",
			Message: "Test commit",
			Author:  "Test Author",
			Date:    time.Now(),
		},
	}

	finder := &Finder{}
	preview := finder.generateBranchPreview(branch, 3) // Limit to 3 lines

	lines := strings.Split(preview, "\n")
	if len(lines) > 3 {
		t.Errorf("Expected preview to have at most 3 lines, got %d lines", len(lines))
	}
}

func TestBuildSessionFinderOptions_WithPreview(t *testing.T) {
	sessions := []*tmux.Session{
		{SessionName: "test-session"},
	}

	finder := &Finder{
		config: &models.FinderConfig{Preview: true},
	}

	opts := finder.buildSessionFinderOptions(sessions)

	if len(opts) < 2 {
		t.Error("Expected at least 2 options when preview is enabled")
	}
}

func TestBuildSessionFinderOptions_WithoutPreview(t *testing.T) {
	sessions := []*tmux.Session{
		{SessionName: "test-session"},
	}

	finder := &Finder{
		config: &models.FinderConfig{Preview: false},
	}

	opts := finder.buildSessionFinderOptions(sessions)

	if len(opts) != 1 {
		t.Errorf("Expected exactly 1 option when preview is disabled, got %d", len(opts))
	}
}

func TestFormatSessionForDisplay(t *testing.T) {
	sessions := []*tmux.Session{
		{
			Context:    "project1",
			Identifier: "session1",
			Command:    "vim main.go",
		},
		{
			Context:    "project2",
			Identifier: "session2",
			Command:    "make test",
		},
	}

	finder := &Finder{}
	formatter := finder.formatSessionForDisplay(sessions)

	result0 := formatter(0)
	expected0 := "project1/session1 - vim main.go"
	if result0 != expected0 {
		t.Errorf("formatSessionForDisplay(0) = %s, expected %s", result0, expected0)
	}

	result1 := formatter(1)
	expected1 := "project2/session2 - make test"
	if result1 != expected1 {
		t.Errorf("formatSessionForDisplay(1) = %s, expected %s", result1, expected1)
	}
}

func TestExtractBranchName(t *testing.T) {
	tests := []struct {
		name          string
		displayBranch string
		expected      string
	}{
		{
			name:          "owner/repo:branch format",
			displayBranch: "github.com/user/myapp:feature",
			expected:      "feature",
		},
		{
			name:          "simple branch name",
			displayBranch: "feature",
			expected:      "feature",
		},
		{
			name:          "main repo without colon",
			displayBranch: "github.com/user/myapp",
			expected:      "github.com/user/myapp",
		},
		{
			name:          "branch with slash",
			displayBranch: "github.com/user/myapp:feature/new-ui",
			expected:      "feature/new-ui",
		},
		{
			name:          "empty string",
			displayBranch: "",
			expected:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBranchName(tt.displayBranch)
			if result != tt.expected {
				t.Errorf("extractBranchName(%q) = %q, want %q", tt.displayBranch, result, tt.expected)
			}
		})
	}
}

func TestFormatWorktreeForDisplay_WithTemplate(t *testing.T) {
	finder := &Finder{
		useTildeHome: false,
		useIcons:     false,
	}

	// Set up display processor manually for testing
	processor, err := newTestDisplayProcessor("{{.Owner}}/{{.Repository}}:{{.Branch}}")
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	finder.displayProcessor = processor

	wt := models.Worktree{
		Branch: "github.com/user/myapp:feature",
		Path:   "/path/to/worktree",
		IsMain: false,
		RepositoryInfo: &models.RepositoryInfo{
			Host:       "github.com",
			Owner:      "user",
			Repository: "myapp",
		},
	}

	result := finder.formatWorktreeForDisplay(wt)
	expected := "user/myapp:feature"

	if result != expected {
		t.Errorf("formatWorktreeForDisplay() = %q, want %q", result, expected)
	}
}

func TestFormatWorktreeForDisplay_FallbackWhenNoTemplate(t *testing.T) {
	finder := &Finder{
		useTildeHome:     false,
		useIcons:         false,
		showPath:         true,
		displayProcessor: nil, // No template configured
	}

	wt := models.Worktree{
		Branch: "github.com/user/myapp:feature",
		Path:   "/path/to/worktree",
		IsMain: false,
		RepositoryInfo: &models.RepositoryInfo{
			Host:       "github.com",
			Owner:      "user",
			Repository: "myapp",
		},
	}

	result := finder.formatWorktreeForDisplay(wt)
	expected := "github.com/user/myapp:feature (/path/to/worktree)"

	if result != expected {
		t.Errorf("formatWorktreeForDisplay() = %q, want %q", result, expected)
	}
}

func TestFormatWorktreeForDisplay_FallbackWhenNoRepositoryInfo(t *testing.T) {
	processor, err := newTestDisplayProcessor("{{.Owner}}/{{.Repository}}:{{.Branch}}")
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	finder := &Finder{
		useTildeHome:     false,
		useIcons:         false,
		showPath:         true,
		displayProcessor: processor,
	}

	wt := models.Worktree{
		Branch:         "feature",
		Path:           "/path/to/worktree",
		IsMain:         false,
		RepositoryInfo: nil, // No repo info
	}

	result := finder.formatWorktreeForDisplay(wt)
	expected := "feature (/path/to/worktree)"

	if result != expected {
		t.Errorf("formatWorktreeForDisplay() = %q, want %q", result, expected)
	}
}

func TestFormatWorktreeForDisplay_TemplateWithPath(t *testing.T) {
	finder := &Finder{
		useTildeHome: false,
		useIcons:     false,
	}

	processor, err := newTestDisplayProcessor("{{.Repository}}:{{.Branch}} ({{.Path}})")
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	finder.displayProcessor = processor

	wt := models.Worktree{
		Branch: "github.com/user/myapp:feature",
		Path:   "/path/to/worktree",
		IsMain: false,
		RepositoryInfo: &models.RepositoryInfo{
			Host:       "github.com",
			Owner:      "user",
			Repository: "myapp",
		},
	}

	result := finder.formatWorktreeForDisplay(wt)
	expected := "myapp:feature (/path/to/worktree)"

	if result != expected {
		t.Errorf("formatWorktreeForDisplay() = %q, want %q", result, expected)
	}
}

func TestFormatWorktreeForDisplay_MainWorktreeWithConditional(t *testing.T) {
	finder := &Finder{
		useTildeHome: false,
		useIcons:     false,
	}

	processor, err := newTestDisplayProcessor("{{.Repository}}{{if not .IsMain}}:{{.Branch}}{{end}}")
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	finder.displayProcessor = processor

	// Main worktree
	wtMain := models.Worktree{
		Branch: "github.com/user/myapp",
		Path:   "/path/to/main",
		IsMain: true,
		RepositoryInfo: &models.RepositoryInfo{
			Host:       "github.com",
			Owner:      "user",
			Repository: "myapp",
		},
	}

	resultMain := finder.formatWorktreeForDisplay(wtMain)
	expectedMain := "myapp"
	if resultMain != expectedMain {
		t.Errorf("formatWorktreeForDisplay(main) = %q, want %q", resultMain, expectedMain)
	}

	// Non-main worktree
	wtBranch := models.Worktree{
		Branch: "github.com/user/myapp:feature",
		Path:   "/path/to/feature",
		IsMain: false,
		RepositoryInfo: &models.RepositoryInfo{
			Host:       "github.com",
			Owner:      "user",
			Repository: "myapp",
		},
	}

	resultBranch := finder.formatWorktreeForDisplay(wtBranch)
	expectedBranch := "myapp:feature"
	if resultBranch != expectedBranch {
		t.Errorf("formatWorktreeForDisplay(branch) = %q, want %q", resultBranch, expectedBranch)
	}
}

func TestFormatWorktreeForDisplay_ShowPathFalse(t *testing.T) {
	f := &Finder{
		useTildeHome:     false,
		useIcons:         false,
		showPath:         false,
		displayProcessor: nil,
	}

	wt := models.Worktree{
		Branch: "github.com/user/myapp",
		Path:   "/home/user/ghq/github.com/user/myapp",
		IsMain: true,
	}

	result := f.formatWorktreeForDisplay(wt)
	// showPath=false: path should not appear
	if strings.Contains(result, "/home/user/ghq") {
		t.Errorf("expected no path in display, got %q", result)
	}
	// Branch should still be shown
	if !strings.Contains(result, "github.com/user/myapp") {
		t.Errorf("expected branch in display, got %q", result)
	}
}

func TestFormatWorktreeForDisplay_ShowPathFalse_NonMainWorktree(t *testing.T) {
	f := &Finder{
		useTildeHome:     false,
		useIcons:         false,
		showPath:         false,
		displayProcessor: nil,
	}

	wt := models.Worktree{
		Branch: "github.com/user/myapp:feature",
		Path:   "/home/user/ghq/github.com/user/myapp/.worktrees/feature",
		IsMain: false,
	}

	result := f.formatWorktreeForDisplay(wt)
	expected := "github.com/user/myapp:feature"
	if result != expected {
		t.Errorf("formatWorktreeForDisplay() = %q, want %q", result, expected)
	}
}

func TestFormatWorktreeForDisplay_ShowPathFalse_WithTemplate(t *testing.T) {
	processor, err := newTestDisplayProcessor("{{.Owner}}/{{.Repository}}:{{.Branch}}")
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	f := &Finder{
		useTildeHome:     false,
		useIcons:         false,
		showPath:         false,
		displayProcessor: processor,
	}

	wt := models.Worktree{
		Branch: "github.com/user/myapp:feature",
		Path:   "/home/user/ghq/github.com/user/myapp/.worktrees/feature",
		IsMain: false,
		RepositoryInfo: &models.RepositoryInfo{
			Host:       "github.com",
			Owner:      "user",
			Repository: "myapp",
		},
	}

	result := f.formatWorktreeForDisplay(wt)
	expected := "user/myapp:feature"
	if result != expected {
		t.Errorf("formatWorktreeForDisplay() = %q, want %q", result, expected)
	}
}

func TestFormatWorktreeForDisplay_ShowPathTrue(t *testing.T) {
	f := &Finder{
		useTildeHome:     false,
		useIcons:         false,
		showPath:         true,
		displayProcessor: nil,
	}

	wt := models.Worktree{
		Branch: "main",
		Path:   "/home/user/project",
		IsMain: true,
	}

	result := f.formatWorktreeForDisplay(wt)
	if !strings.Contains(result, "(/home/user/project)") {
		t.Errorf("expected path in display when showPath=true, got %q", result)
	}
}

func TestSetShowPath(t *testing.T) {
	f := &Finder{showPath: true}
	if !f.showPath {
		t.Fatal("showPath should be true initially")
	}
	f.SetShowPath(false)
	if f.showPath {
		t.Fatal("showPath should be false after SetShowPath(false)")
	}
	f.SetShowPath(true)
	if !f.showPath {
		t.Fatal("showPath should be true after SetShowPath(true)")
	}
}

func TestFormatWorktreeForDisplay_IconsEnabled_MainHasVisiblePrefix(t *testing.T) {
	f := &Finder{
		useIcons: true,
		showPath: false,
	}

	wt := models.Worktree{
		Branch: "github.com/user/myapp",
		IsMain: true,
	}

	got := f.formatWorktreeForDisplay(wt)
	if got == wt.Branch {
		t.Fatalf("expected visible prefix for main worktree when icons are enabled, got %q", got)
	}
}

// Benchmark tests
func BenchmarkTruncateHash(b *testing.B) {
	hash := "a1b2c3d4e5f6789012345678901234567890abcd"
	for i := 0; i < b.N; i++ {
		truncateHash(hash)
	}
}

func BenchmarkTruncateMessage(b *testing.B) {
	message := "This is a very long commit message that needs to be truncated for display purposes"
	for i := 0; i < b.N; i++ {
		truncateMessage(message, 50)
	}
}

func BenchmarkFormatDuration(b *testing.B) {
	duration := 3*time.Hour + 25*time.Minute
	for i := 0; i < b.N; i++ {
		formatDuration(duration)
	}
}

func BenchmarkGenerateSessionPreview(b *testing.B) {
	session := &tmux.Session{
		SessionName: "test-session",
		Context:     "test-context",
		Identifier:  "test-id",
		Command:     "vim test.go",
		StartTime:   time.Now(),
		WorkingDir:  "/home/user/project",
		Metadata: map[string]string{
			"branch": "main",
			"repo":   "test-repo",
		},
	}

	finder := &Finder{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		finder.generateSessionPreview(session, 20)
	}
}

func BenchmarkGenerateWorktreePreview(b *testing.B) {
	wt := models.Worktree{
		Branch:     "feature-branch",
		Path:       "/home/user/project/feature",
		CommitHash: "abc1234567890def",
		CreatedAt:  time.Now(),
		IsMain:     false,
	}

	finder := &Finder{git: nil}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		finder.generateWorktreePreview(wt, 20)
	}
}

func BenchmarkGenerateBranchPreview(b *testing.B) {
	branch := models.Branch{
		Name:      "feature-branch",
		IsCurrent: false,
		IsRemote:  false,
		LastCommit: models.CommitInfo{
			Hash:    "abc1234567890def",
			Message: "Add new feature implementation",
			Author:  "Developer",
			Date:    time.Now(),
		},
	}

	finder := &Finder{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		finder.generateBranchPreview(branch, 20)
	}
}
