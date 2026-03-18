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
