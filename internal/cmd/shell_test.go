package cmd

import (
	"testing"
)

func TestGetShellDepth(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     int
	}{
		{
			name:     "not set",
			envValue: "",
			want:     0,
		},
		{
			name:     "zero",
			envValue: "0",
			want:     0,
		},
		{
			name:     "positive",
			envValue: "3",
			want:     3,
		},
		{
			name:     "negative treated as zero",
			envValue: "-1",
			want:     0,
		},
		{
			name:     "invalid treated as zero",
			envValue: "abc",
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv handles save/restore automatically
			// Empty string is treated as 0 by getShellDepth
			t.Setenv(EnvShellDepth, tt.envValue)

			got := getShellDepth()
			if got != tt.want {
				t.Errorf("getShellDepth() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUpdateEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		env      []string
		key      string
		value    string
		expected []string
	}{
		{
			name:     "add new variable",
			env:      []string{"PATH=/usr/bin"},
			key:      "GWQ_SHELL_DEPTH",
			value:    "1",
			expected: []string{"PATH=/usr/bin", "GWQ_SHELL_DEPTH=1"},
		},
		{
			name:     "update existing variable",
			env:      []string{"PATH=/usr/bin", "GWQ_SHELL_DEPTH=1"},
			key:      "GWQ_SHELL_DEPTH",
			value:    "2",
			expected: []string{"PATH=/usr/bin", "GWQ_SHELL_DEPTH=2"},
		},
		{
			name:     "empty env",
			env:      []string{},
			key:      "GWQ_SHELL_DEPTH",
			value:    "1",
			expected: []string{"GWQ_SHELL_DEPTH=1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := updateEnvVar(tt.env, tt.key, tt.value)
			if len(got) != len(tt.expected) {
				t.Errorf("updateEnvVar() returned %d elements, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("updateEnvVar()[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}
