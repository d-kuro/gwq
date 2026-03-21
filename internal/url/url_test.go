package url

import (
	"testing"
)

func TestParseRepositoryURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantHost  string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "standard github https",
			input:     "https://github.com/user/repo",
			wantHost:  "github.com",
			wantOwner: "user",
			wantRepo:  "repo",
		},
		{
			name:      "github https with .git suffix",
			input:     "https://github.com/user/repo.git",
			wantHost:  "github.com",
			wantOwner: "user",
			wantRepo:  "repo",
		},
		{
			name:      "github ssh format",
			input:     "git@github.com:user/repo.git",
			wantHost:  "github.com",
			wantOwner: "user",
			wantRepo:  "repo",
		},
		{
			name:      "gitlab nested group - 3 levels",
			input:     "https://gitlab.com/org/team/repo",
			wantHost:  "gitlab.com",
			wantOwner: "org",
			wantRepo:  "repo",
		},
		{
			name:      "gitlab nested group - 4 levels",
			input:     "https://gitlab.com/org/team/suborg/repo",
			wantHost:  "gitlab.com",
			wantOwner: "org",
			wantRepo:  "repo",
		},
		{
			name:      "gitlab nested group with .git suffix",
			input:     "https://gitlab.com/org/team/suborg/repo.git",
			wantHost:  "gitlab.com",
			wantOwner: "org",
			wantRepo:  "repo",
		},
		{
			name:      "gitlab nested group ssh format",
			input:     "git@gitlab.com:org/team/suborg/repo.git",
			wantHost:  "gitlab.com",
			wantOwner: "org",
			wantRepo:  "repo",
		},
		{
			name:      "SSH config alias",
			input:     "workgit:myorg/myrepo.git",
			wantHost:  "workgit",
			wantOwner: "myorg",
			wantRepo:  "myrepo",
		},
		{
			name:      "SSH config alias without .git",
			input:     "myalias:owner/repo",
			wantHost:  "myalias",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "SSH config alias with nested path",
			input:     "myhost:org/team/repo.git",
			wantHost:  "myhost",
			wantOwner: "org",
			wantRepo:  "repo",
		},
		{
			name:      "git@ with SSH config alias",
			input:     "git@workgit:org/repo.git",
			wantHost:  "workgit",
			wantOwner: "org",
			wantRepo:  "repo",
		},
		{
			name:      "URL with port number",
			input:     "localhost:8080/user/repo",
			wantHost:  "localhost:8080",
			wantOwner: "user",
			wantRepo:  "repo",
		},
		{
			name:    "single path component is invalid",
			input:   "https://github.com/user",
			wantErr: true,
		},
		{
			name:    "no host",
			input:   "/user/repo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseRepositoryURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseRepositoryURL(%s) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRepositoryURL(%s) unexpected error: %v", tt.input, err)
			}
			if info.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", info.Host, tt.wantHost)
			}
			if info.Owner != tt.wantOwner {
				t.Errorf("Owner = %q, want %q", info.Owner, tt.wantOwner)
			}
			if info.Repository != tt.wantRepo {
				t.Errorf("Repository = %q, want %q", info.Repository, tt.wantRepo)
			}
		})
	}
}

func TestIsSCPLikeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "basic SSH config alias",
			input:    "workgit:myorg/myrepo.git",
			expected: true,
		},
		{
			name:     "alias without .git",
			input:    "myalias:owner/repo",
			expected: true,
		},
		{
			name:     "port number URL",
			input:    "localhost:8080/user/repo",
			expected: false,
		},
		{
			name:     "port only without path",
			input:    "localhost:8080",
			expected: false,
		},
		{
			name:     "URL with scheme",
			input:    "https://github.com/user/repo",
			expected: false,
		},
		{
			name:     "git@ prefix",
			input:    "git@github.com:user/repo.git",
			expected: false,
		},
		{
			name:     "empty path after colon",
			input:    "host:",
			expected: false,
		},
		{
			name:     "no colon",
			input:    "github.com/user/repo",
			expected: false,
		},
		{
			name:     "colon followed by slash",
			input:    "host:/user/repo",
			expected: false,
		},
		{
			name:     "bracketed IPv6 address",
			input:    "[::1]:8080/user/repo",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSCPLikeURL(tt.input)
			if result != tt.expected {
				t.Errorf("isSCPLikeURL(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "git@ format",
			input:    "git@github.com:user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		{
			name:     "ssh://git@ format",
			input:    "ssh://git@github.com:user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		{
			name:     "https format unchanged",
			input:    "https://github.com/user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		{
			name:     "http format unchanged",
			input:    "http://github.com/user/repo.git",
			expected: "http://github.com/user/repo.git",
		},
		{
			name:     "plain url gets https prefix",
			input:    "github.com/user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		{
			name:     "SSH config alias SCP format",
			input:    "workgit:myorg/myrepo.git",
			expected: "https://workgit/myorg/myrepo.git",
		},
		{
			name:     "SSH config alias without .git",
			input:    "myalias:owner/repo",
			expected: "https://myalias/owner/repo",
		},
		{
			name:     "SSH config alias with nested path",
			input:    "myhost:org/team/repo.git",
			expected: "https://myhost/org/team/repo.git",
		},
		{
			name:     "URL with port number is not SCP",
			input:    "localhost:8080/user/repo",
			expected: "https://localhost:8080/user/repo",
		},
		{
			name:     "git@ with SSH config alias",
			input:    "git@workgit:org/repo.git",
			expected: "https://workgit/org/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeURL(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
