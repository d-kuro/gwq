package url

import (
	"testing"
)

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
		// SSH URL formats with slash (not colon)
		{
			name:     "ssh://git@ with slash format",
			input:    "ssh://git@github.com/user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		{
			name:     "ssh:// without git@ prefix",
			input:    "ssh://github.com/user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		// AWS CodeCommit credential helper format is no longer normalized
		// codecommit:: URLs will go through the default https:// prefix path
		{
			name:     "codecommit with profile passes through",
			input:    "codecommit::ap-northeast-1://myprofile@my-app-backend",
			expected: "https://codecommit::ap-northeast-1://myprofile@my-app-backend",
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

func TestParseRepositoryURL(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedHost  string
		expectedOwner string
		expectedRepo  string
		expectedPath  string
		expectError   bool
	}{
		// GitHub (existing compatibility)
		{
			name:          "GitHub HTTPS",
			input:         "https://github.com/user/repo.git",
			expectedHost:  "github.com",
			expectedOwner: "user",
			expectedRepo:  "repo",
			expectedPath:  "github.com/user/repo",
		},
		{
			name:          "GitHub SSH",
			input:         "git@github.com:user/repo.git",
			expectedHost:  "github.com",
			expectedOwner: "user",
			expectedRepo:  "repo",
			expectedPath:  "github.com/user/repo",
		},
		// GitLab subgroup support
		{
			name:          "GitLab subgroup HTTPS",
			input:         "https://gitlab.com/group/subgroup/project.git",
			expectedHost:  "gitlab.com",
			expectedOwner: "group/subgroup",
			expectedRepo:  "project",
			expectedPath:  "gitlab.com/group/subgroup/project",
		},
		{
			name:          "GitLab deep nesting SSH",
			input:         "git@gitlab.com:group/sub1/sub2/project.git",
			expectedHost:  "gitlab.com",
			expectedOwner: "group/sub1/sub2",
			expectedRepo:  "project",
			expectedPath:  "gitlab.com/group/sub1/sub2/project",
		},
		// CodeCommit HTTPS/SSH (standard URL format works)
		{
			name:          "CodeCommit HTTPS",
			input:         "https://git-codecommit.ap-northeast-1.amazonaws.com/v1/repos/my-app",
			expectedHost:  "git-codecommit.ap-northeast-1.amazonaws.com",
			expectedOwner: "v1/repos",
			expectedRepo:  "my-app",
			expectedPath:  "git-codecommit.ap-northeast-1.amazonaws.com/v1/repos/my-app",
		},
		{
			name:          "CodeCommit SSH",
			input:         "ssh://git-codecommit.ap-northeast-1.amazonaws.com/v1/repos/my-app",
			expectedHost:  "git-codecommit.ap-northeast-1.amazonaws.com",
			expectedOwner: "v1/repos",
			expectedRepo:  "my-app",
			expectedPath:  "git-codecommit.ap-northeast-1.amazonaws.com/v1/repos/my-app",
		},
		// codecommit:: credential helper format is not supported
		{
			name:        "codecommit credential helper format returns error",
			input:       "codecommit::ap-northeast-1://profile@repo",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseRepositoryURL(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseRepositoryURL(%s) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseRepositoryURL(%s) unexpected error: %v", tt.input, err)
				return
			}
			if info.Host != tt.expectedHost {
				t.Errorf("Host = %s, want %s", info.Host, tt.expectedHost)
			}
			if info.Owner != tt.expectedOwner {
				t.Errorf("Owner = %s, want %s", info.Owner, tt.expectedOwner)
			}
			if info.Repository != tt.expectedRepo {
				t.Errorf("Repository = %s, want %s", info.Repository, tt.expectedRepo)
			}
			if info.FullPath != tt.expectedPath {
				t.Errorf("FullPath = %s, want %s", info.FullPath, tt.expectedPath)
			}
		})
	}
}
