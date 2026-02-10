package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/d-kuro/gwq/pkg/models"
)

func TestCalculateSummary(t *testing.T) {
	tests := []struct {
		name     string
		statuses []*models.WorktreeStatus
		want     statusSummary
	}{
		{
			name:     "empty statuses",
			statuses: []*models.WorktreeStatus{},
			want:     statusSummary{Total: 0, Modified: 0, Clean: 0, Stale: 0},
		},
		{
			name: "mixed statuses",
			statuses: []*models.WorktreeStatus{
				{Status: models.WorktreeStatusClean},
				{Status: models.WorktreeStatusModified},
				{Status: models.WorktreeStatusModified},
				{Status: models.WorktreeStatusStale},
			},
			want: statusSummary{Total: 4, Modified: 2, Clean: 1, Stale: 1},
		},
		{
			name: "all clean",
			statuses: []*models.WorktreeStatus{
				{Status: models.WorktreeStatusClean},
				{Status: models.WorktreeStatusClean},
			},
			want: statusSummary{Total: 2, Modified: 0, Clean: 2, Stale: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSummary(tt.statuses)
			if got != tt.want {
				t.Errorf("calculateSummary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterStatuses(t *testing.T) {
	statuses := []*models.WorktreeStatus{
		{Branch: "main", Status: models.WorktreeStatusClean},
		{Branch: "feature1", Status: models.WorktreeStatusModified},
		{Branch: "feature2", Status: models.WorktreeStatusModified},
		{Branch: "old", Status: models.WorktreeStatusStale},
	}

	tests := []struct {
		name   string
		filter string
		want   int
	}{
		{
			name:   "filter modified",
			filter: "modified",
			want:   2,
		},
		{
			name:   "filter clean",
			filter: "clean",
			want:   1,
		},
		{
			name:   "filter stale",
			filter: "stale",
			want:   1,
		},
		{
			name:   "invalid filter",
			filter: "invalid",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterStatuses(statuses, tt.filter)
			if len(got) != tt.want {
				t.Errorf("filterStatuses() returned %d items, want %d", len(got), tt.want)
			}
		})
	}
}

func TestFormatActivity(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "zero time",
			time:     time.Time{},
			expected: "unknown",
		},
		{
			name:     "just now",
			time:     now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "minutes ago",
			time:     now.Add(-5 * time.Minute),
			expected: "5 mins ago",
		},
		{
			name:     "1 minute ago",
			time:     now.Add(-1 * time.Minute),
			expected: "1 min ago",
		},
		{
			name:     "hours ago",
			time:     now.Add(-3 * time.Hour),
			expected: "3 hours ago",
		},
		{
			name:     "1 hour ago",
			time:     now.Add(-1 * time.Hour),
			expected: "1 hour ago",
		},
		{
			name:     "days ago",
			time:     now.Add(-2 * 24 * time.Hour),
			expected: "2 days ago",
		},
		{
			name:     "1 day ago",
			time:     now.Add(-1 * 24 * time.Hour),
			expected: "1 day ago",
		},
		{
			name:     "weeks ago",
			time:     now.Add(-14 * 24 * time.Hour),
			expected: "2 weeks ago",
		},
		{
			name:     "1 week ago",
			time:     now.Add(-7 * 24 * time.Hour),
			expected: "1 week ago",
		},
		{
			name:     "months ago",
			time:     now.Add(-60 * 24 * time.Hour),
			expected: "2 months ago",
		},
		{
			name:     "1 month ago",
			time:     now.Add(-30 * 24 * time.Hour),
			expected: "1 month ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatActivity(tt.time)
			if got != tt.expected {
				t.Errorf("formatActivity() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFormatChanges(t *testing.T) {
	tests := []struct {
		name     string
		status   models.GitStatus
		expected string
	}{
		{
			name:     "no changes",
			status:   models.GitStatus{},
			expected: "-",
		},
		{
			name: "only added",
			status: models.GitStatus{
				Added: 5,
			},
			expected: "5 added",
		},
		{
			name: "only modified",
			status: models.GitStatus{
				Modified: 3,
			},
			expected: "3 modified",
		},
		{
			name: "only deleted",
			status: models.GitStatus{
				Deleted: 2,
			},
			expected: "2 deleted",
		},
		{
			name: "only untracked",
			status: models.GitStatus{
				Untracked: 4,
			},
			expected: "4 untracked",
		},
		{
			name: "mixed changes",
			status: models.GitStatus{
				Added:     5,
				Modified:  3,
				Deleted:   2,
				Untracked: 1,
			},
			expected: "5 added, 3 modified, 2 deleted, 1 untracked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatChanges(tt.status)
			if got != tt.expected {
				t.Errorf("formatChanges() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFormatAheadBehind(t *testing.T) {
	tests := []struct {
		name     string
		ahead    int
		behind   int
		expected string
	}{
		{
			name:     "no ahead or behind",
			ahead:    0,
			behind:   0,
			expected: "↑0 ↓0",
		},
		{
			name:     "ahead only",
			ahead:    5,
			behind:   0,
			expected: "↑5 ↓0",
		},
		{
			name:     "behind only",
			ahead:    0,
			behind:   3,
			expected: "↑0 ↓3",
		},
		{
			name:     "both ahead and behind",
			ahead:    2,
			behind:   4,
			expected: "↑2 ↓4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAheadBehind(tt.ahead, tt.behind)
			if got != tt.expected {
				t.Errorf("formatAheadBehind() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFormatProcess(t *testing.T) {
	tests := []struct {
		name      string
		processes []models.ProcessInfo
		expected  string
	}{
		{
			name:      "no processes",
			processes: []models.ProcessInfo{},
			expected:  "-",
		},
		{
			name: "single process",
			processes: []models.ProcessInfo{
				{PID: 1234, Command: "claude"},
			},
			expected: "claude:1234",
		},
		{
			name: "multiple processes",
			processes: []models.ProcessInfo{
				{PID: 1234, Command: "claude"},
				{PID: 5678, Command: "cursor"},
			},
			expected: "claude:1234,cursor:5678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatProcess(tt.processes)
			if got != tt.expected {
				t.Errorf("formatProcess() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseGitStatusFiles(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name:     "simple modified file",
			output:   " M file.txt\x00",
			expected: []string{"file.txt"},
		},
		{
			name:     "file with spaces",
			output:   " M path with spaces/file name.txt\x00",
			expected: []string{"path with spaces/file name.txt"},
		},
		{
			name:     "renamed file in index",
			output:   "R  old.txt\x00new.txt\x00",
			expected: []string{"new.txt"},
		},
		{
			name:     "copied file in index",
			output:   "C  source.txt\x00dest.txt\x00",
			expected: []string{"dest.txt"},
		},
		{
			name:     "mixed changes",
			output:   " M modified.txt\x00R  old.txt\x00renamed.txt\x00?? untracked.txt\x00",
			expected: []string{"modified.txt", "renamed.txt", "untracked.txt"},
		},
		{
			name:     "rename in worktree (Y side)",
			output:   " R old.txt\x00new.txt\x00",
			expected: []string{"new.txt"},
		},
		{
			name:     "copy in worktree (Y side)",
			output:   " C source.txt\x00copy.txt\x00",
			expected: []string{"copy.txt"},
		},
		{
			name:     "file starting with space",
			output:   " M  leading-space.txt\x00",
			expected: []string{" leading-space.txt"},
		},
		{
			name:     "added file",
			output:   "A  newfile.txt\x00",
			expected: []string{"newfile.txt"},
		},
		{
			name:     "deleted file",
			output:   " D deleted.txt\x00",
			expected: []string{"deleted.txt"},
		},
		{
			name:     "untracked file",
			output:   "?? untracked.txt\x00",
			expected: []string{"untracked.txt"},
		},
		{
			name:     "ignored file (if shown)",
			output:   "!! ignored.txt\x00",
			expected: []string{"ignored.txt"},
		},
		{
			name:     "empty output",
			output:   "",
			expected: nil,
		},
		{
			name:     "only null terminator",
			output:   "\x00",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitStatusFiles(tt.output)
			if len(result) != len(tt.expected) {
				t.Errorf("parseGitStatusFiles() returned %d items, want %d", len(result), len(tt.expected))
				t.Errorf("got: %v, want: %v", result, tt.expected)
				return
			}
			for i, got := range result {
				if got != tt.expected[i] {
					t.Errorf("parseGitStatusFiles()[%d] = %q, want %q", i, got, tt.expected[i])
				}
			}
		})
	}
}

func TestExtractRepoNameFromPath_SkipsDotPrefixedSegments(t *testing.T) {
	collector := &StatusCollector{}
	path := filepath.Join(string(filepath.Separator), "home", "user", ".ghq", "github.com", "owner", "repo")

	got := collector.extractRepoNameFromPath(path)
	want := filepath.Join("github.com", "owner", "repo")
	if got != want {
		t.Fatalf("extractRepoNameFromPath() = %q, want %q", got, want)
	}
}

func TestExtractGhqStyleRepo_SkipsDotPrefixedSegments(t *testing.T) {
	collector := &StatusCollector{}
	path := filepath.Join(string(filepath.Separator), "home", "user", ".ghq", "github.com", "owner", "repo")

	got := collector.extractGhqStyleRepo(path)
	want := filepath.Join("github.com", "owner", "repo")
	if got != want {
		t.Fatalf("extractGhqStyleRepo() = %q, want %q", got, want)
	}
}

func TestExtractRepoFromPathParts(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		wantRepo string
		wantOK   bool
	}{
		{
			name:     "normal ghq path",
			parts:    []string{"", "home", "user", "ghq", "github.com", "owner", "repo"},
			wantRepo: filepath.Join("github.com", "owner", "repo"),
			wantOK:   true,
		},
		{
			name:     "skip dot-prefixed host-like segment",
			parts:    []string{"", "home", "user", ".ghq", "github.com", "owner", "repo"},
			wantRepo: filepath.Join("github.com", "owner", "repo"),
			wantOK:   true,
		},
		{
			name:     "reject .worktrees as repository segment",
			parts:    []string{"", "home", "user", "github.com", "owner", ".worktrees"},
			wantRepo: "",
			wantOK:   false,
		},
		{
			name:     "no host segment",
			parts:    []string{"", "home", "user", "workspace", "owner", "repo"},
			wantRepo: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRepo, gotOK := extractRepoFromPathParts(tt.parts)
			if gotOK != tt.wantOK {
				t.Fatalf("extractRepoFromPathParts() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotRepo != tt.wantRepo {
				t.Fatalf("extractRepoFromPathParts() repo = %q, want %q", gotRepo, tt.wantRepo)
			}
		})
	}
}
