package ghq

import (
	"errors"
	"strings"
	"testing"
)

// MockCommandExecutor is a mock implementation of CommandExecutor for testing.
type MockCommandExecutor struct {
	RunFunc func(name string, args ...string) (string, error)
}

func (m *MockCommandExecutor) Run(name string, args ...string) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(name, args...)
	}
	return "", nil
}

func TestGetRoots(t *testing.T) {
	tests := []struct {
		name        string
		mockOutput  string
		mockError   error
		wantRoots   []string
		wantErr     bool
		errContains string
	}{
		{
			name:       "single root",
			mockOutput: "/home/user/ghq\n",
			mockError:  nil,
			wantRoots:  []string{"/home/user/ghq"},
			wantErr:    false,
		},
		{
			name:       "multiple roots",
			mockOutput: "/home/user/ghq\n/home/user/work/ghq\n",
			mockError:  nil,
			wantRoots:  []string{"/home/user/ghq", "/home/user/work/ghq"},
			wantErr:    false,
		},
		{
			name:       "empty output",
			mockOutput: "",
			mockError:  nil,
			wantRoots:  []string{},
			wantErr:    false,
		},
		{
			name:       "output with extra newlines",
			mockOutput: "\n/home/user/ghq\n\n/home/user/work/ghq\n\n",
			mockError:  nil,
			wantRoots:  []string{"/home/user/ghq", "/home/user/work/ghq"},
			wantErr:    false,
		},
		{
			name:        "command error",
			mockOutput:  "",
			mockError:   errors.New("ghq not found"),
			wantRoots:   nil,
			wantErr:     true,
			errContains: "failed to get ghq roots",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				executor: &MockCommandExecutor{
					RunFunc: func(name string, args ...string) (string, error) {
						if name != "ghq" || len(args) != 2 || args[0] != "root" || args[1] != "--all" {
							t.Errorf("unexpected command: %s %v", name, args)
						}
						return tt.mockOutput, tt.mockError
					},
				},
			}

			roots, err := client.GetRoots()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRoots() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !containsString(err.Error(), tt.errContains) {
					t.Errorf("GetRoots() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if !equalStringSlices(roots, tt.wantRoots) {
				t.Errorf("GetRoots() = %v, want %v", roots, tt.wantRoots)
			}
		})
	}
}

func TestListRepositories(t *testing.T) {
	tests := []struct {
		name        string
		mockOutput  string
		mockError   error
		wantRepos   []string
		wantErr     bool
		errContains string
	}{
		{
			name:       "multiple repositories",
			mockOutput: "/home/user/ghq/github.com/user/repo1\n/home/user/ghq/github.com/user/repo2\n",
			mockError:  nil,
			wantRepos:  []string{"/home/user/ghq/github.com/user/repo1", "/home/user/ghq/github.com/user/repo2"},
			wantErr:    false,
		},
		{
			name:       "empty output",
			mockOutput: "",
			mockError:  nil,
			wantRepos:  []string{},
			wantErr:    false,
		},
		{
			name:        "command error",
			mockOutput:  "",
			mockError:   errors.New("ghq not found"),
			wantRepos:   nil,
			wantErr:     true,
			errContains: "failed to list ghq repositories",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				executor: &MockCommandExecutor{
					RunFunc: func(name string, args ...string) (string, error) {
						if name != "ghq" || len(args) != 2 || args[0] != "list" || args[1] != "-p" {
							t.Errorf("unexpected command: %s %v", name, args)
						}
						return tt.mockOutput, tt.mockError
					},
				},
			}

			repos, err := client.ListRepositories()
			if (err != nil) != tt.wantErr {
				t.Errorf("ListRepositories() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !containsString(err.Error(), tt.errContains) {
					t.Errorf("ListRepositories() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if !equalStringSlices(repos, tt.wantRepos) {
				t.Errorf("ListRepositories() = %v, want %v", repos, tt.wantRepos)
			}
		})
	}
}

func TestIsInstalled(t *testing.T) {
	tests := []struct {
		name       string
		mockOutput string
		mockError  error
		want       bool
	}{
		{
			name:       "ghq installed",
			mockOutput: "/home/user/ghq\n",
			mockError:  nil,
			want:       true,
		},
		{
			name:       "ghq not installed",
			mockOutput: "",
			mockError:  errors.New("ghq: command not found"),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				executor: &MockCommandExecutor{
					RunFunc: func(name string, args ...string) (string, error) {
						return tt.mockOutput, tt.mockError
					},
				},
			}

			got := client.IsInstalled()
			if got != tt.want {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsGhqManaged(t *testing.T) {
	tests := []struct {
		name     string
		repoPath string
		ghqRoots []string
		want     bool
	}{
		{
			name:     "path under ghq root",
			repoPath: "/home/user/ghq/github.com/user/repo",
			ghqRoots: []string{"/home/user/ghq"},
			want:     true,
		},
		{
			name:     "path under second ghq root",
			repoPath: "/home/user/work/ghq/github.com/user/repo",
			ghqRoots: []string{"/home/user/ghq", "/home/user/work/ghq"},
			want:     true,
		},
		{
			name:     "path not under any ghq root",
			repoPath: "/home/user/projects/repo",
			ghqRoots: []string{"/home/user/ghq"},
			want:     false,
		},
		{
			name:     "empty ghq roots",
			repoPath: "/home/user/ghq/github.com/user/repo",
			ghqRoots: []string{},
			want:     false,
		},
		{
			name:     "partial match should not work",
			repoPath: "/home/user/ghq-backup/github.com/user/repo",
			ghqRoots: []string{"/home/user/ghq"},
			want:     false,
		},
		{
			name:     "exact ghq root path",
			repoPath: "/home/user/ghq",
			ghqRoots: []string{"/home/user/ghq"},
			want:     false, // ghq root itself is not a repository
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGhqManaged(tt.repoPath, tt.ghqRoots)
			if got != tt.want {
				t.Errorf("IsGhqManaged(%q, %v) = %v, want %v", tt.repoPath, tt.ghqRoots, got, tt.want)
			}
		})
	}
}

// Helper functions for tests

func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
