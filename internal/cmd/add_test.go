package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestHandleAddPostCreate(t *testing.T) {
	t.Parallel()

	expAt := time.Now().Add(24 * time.Hour)

	tests := []struct {
		name           string
		inShim         bool
		autoCdOnAdd    bool
		stay           bool
		expiresAt      *time.Time
		wantStdout     string
		wantStderrHas  []string
		wantShellCalls int
	}{
		{
			name:           "shim+stay: path on stdout, msg on stderr",
			inShim:         true,
			stay:           true,
			wantStdout:     "/wt/path\n",
			wantStderrHas:  []string{"Created worktree for branch 'foo'"},
			wantShellCalls: 0,
		},
		{
			name:           "shim+auto_cd_on_add: path on stdout",
			inShim:         true,
			autoCdOnAdd:    true,
			wantStdout:     "/wt/path\n",
			wantStderrHas:  []string{"Created worktree for branch 'foo'"},
			wantShellCalls: 0,
		},
		{
			name:           "shim+neither: stdout strictly empty",
			inShim:         true,
			wantStdout:     "",
			wantStderrHas:  []string{"Created worktree for branch 'foo'"},
			wantShellCalls: 0,
		},
		{
			name:           "shim+stay+expires: two stderr lines, one stdout line",
			inShim:         true,
			stay:           true,
			expiresAt:      &expAt,
			wantStdout:     "/wt/path\n",
			wantStderrHas:  []string{"Created worktree", "Worktree expires at"},
			wantShellCalls: 0,
		},
		{
			name:           "nonshim+stay: launchShell called, msg on stdout",
			stay:           true,
			wantStdout:     "Created worktree for branch 'foo'\n",
			wantShellCalls: 1,
		},
		{
			name:           "nonshim+no stay: no shell, msg on stdout",
			wantStdout:     "Created worktree for branch 'foo'\n",
			wantShellCalls: 0,
		},
		{
			name:           "auto_cd_on_add=true but no shim: no effect",
			autoCdOnAdd:    true,
			wantStdout:     "Created worktree for branch 'foo'\n",
			wantShellCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			shellCalls := 0
			fakeLaunch := func(path string) error {
				shellCalls++
				if path != "/wt/path" {
					t.Errorf("launchShell path = %q; want %q", path, "/wt/path")
				}
				return nil
			}

			handleAddPostCreate(
				&stdout, &stderr,
				tt.inShim, tt.autoCdOnAdd,
				addResult{
					Branch:    "foo",
					Path:      "/wt/path",
					Stay:      tt.stay,
					ExpiresAt: tt.expiresAt,
				},
				fakeLaunch,
			)

			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("stdout = %q; want %q", got, tt.wantStdout)
			}
			for _, sub := range tt.wantStderrHas {
				if !strings.Contains(stderr.String(), sub) {
					t.Errorf("stderr = %q; want to contain %q", stderr.String(), sub)
				}
			}
			if shellCalls != tt.wantShellCalls {
				t.Errorf("launchShell calls = %d; want %d", shellCalls, tt.wantShellCalls)
			}
		})
	}
}
