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
		// AWS CodeCommit credential helper format
		{
			name:     "codecommit with profile",
			input:    "codecommit::ap-northeast-1://myprofile@my-app-backend",
			expected: "https://git-codecommit.ap-northeast-1.amazonaws.com/repos/my-app-backend",
		},
		{
			name:     "codecommit without profile",
			input:    "codecommit::us-east-1://simple-repo",
			expected: "https://git-codecommit.us-east-1.amazonaws.com/repos/simple-repo",
		},
		{
			name:     "codecommit with different region",
			input:    "codecommit::eu-west-1://dev-profile@frontend-app",
			expected: "https://git-codecommit.eu-west-1.amazonaws.com/repos/frontend-app",
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

func TestParseRepositoryURL_CodeCommit(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedHost  string
		expectedOwner string
		expectedRepo  string
		expectedPath  string
		expectError   bool
	}{
		{
			name:          "codecommit with profile",
			input:         "codecommit::ap-northeast-1://myprofile@my-app-backend",
			expectedHost:  "git-codecommit.ap-northeast-1.amazonaws.com",
			expectedOwner: "repos",
			expectedRepo:  "my-app-backend",
			expectedPath:  "git-codecommit.ap-northeast-1.amazonaws.com/repos/my-app-backend",
		},
		{
			name:          "codecommit without profile",
			input:         "codecommit::us-east-1://simple-repo",
			expectedHost:  "git-codecommit.us-east-1.amazonaws.com",
			expectedOwner: "repos",
			expectedRepo:  "simple-repo",
			expectedPath:  "git-codecommit.us-east-1.amazonaws.com/repos/simple-repo",
		},
		{
			name:          "codecommit with different region",
			input:         "codecommit::eu-west-1://dev-profile@frontend-app",
			expectedHost:  "git-codecommit.eu-west-1.amazonaws.com",
			expectedOwner: "repos",
			expectedRepo:  "frontend-app",
			expectedPath:  "git-codecommit.eu-west-1.amazonaws.com/repos/frontend-app",
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
