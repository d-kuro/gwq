package template

import (
	"strings"
	"testing"
)

func TestRenderCommands(t *testing.T) {
	data := &TemplateData{
		Host:       "github.com",
		Owner:      "d-kuro",
		Repository: "gwq",
		Branch:     "feature/new-ui",
		Hash:       "a1b2c3d4",
		Path:       "/tmp/worktrees/gwq/feature-new-ui",
	}

	tests := []struct {
		name          string
		commands      []string
		wantRendered  []string // expected Rendered for each input index; "" means expect Err
		wantErrSubstr []string // expected substring in Err for each input index; "" means expect no Err
	}{
		{
			name: "all variables render",
			commands: []string{
				"echo {{.Branch}} {{.Path}}",
				"zellij action new-tab --name {{.Branch}} -- nvim .",
				"mkdir -p {{.Path}}/.local",
			},
			wantRendered: []string{
				"echo feature/new-ui /tmp/worktrees/gwq/feature-new-ui",
				"zellij action new-tab --name feature/new-ui -- nvim .",
				"mkdir -p /tmp/worktrees/gwq/feature-new-ui/.local",
			},
			wantErrSubstr: []string{"", "", ""},
		},
		{
			name:          "parse error is captured per command",
			commands:      []string{"echo {{.Branch}}", "echo {{ invalid"},
			wantRendered:  []string{"echo feature/new-ui", ""},
			wantErrSubstr: []string{"", "parse command"},
		},
		{
			name:          "missing key is an error (missingkey=error)",
			commands:      []string{"echo {{.NoSuchVar}}"},
			wantRendered:  []string{""},
			wantErrSubstr: []string{"execute template"},
		},
		{
			name:          "empty input is a no-op",
			commands:      []string{},
			wantRendered:  []string{},
			wantErrSubstr: []string{},
		},
		{
			name:          "first failure does not short-circuit subsequent commands",
			commands:      []string{"echo {{.NoSuchVar}}", "echo {{.Branch}}", "echo {{.Path}}"},
			wantRendered:  []string{"", "echo feature/new-ui", "echo /tmp/worktrees/gwq/feature-new-ui"},
			wantErrSubstr: []string{"execute template", "", ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderCommands(tc.commands, data)

			if len(got) != len(tc.commands) {
				t.Fatalf("len mismatch: got %d results for %d commands", len(got), len(tc.commands))
			}

			for i, rc := range got {
				if rc.Source != tc.commands[i] {
					t.Errorf("index %d: Source = %q; want %q", i, rc.Source, tc.commands[i])
				}

				wantErr := tc.wantErrSubstr[i] != ""
				if wantErr {
					if rc.Err == nil {
						t.Errorf("index %d: expected error containing %q, got nil", i, tc.wantErrSubstr[i])
					} else if !strings.Contains(rc.Err.Error(), tc.wantErrSubstr[i]) {
						t.Errorf("index %d: error %q does not contain %q", i, rc.Err.Error(), tc.wantErrSubstr[i])
					}
					if rc.Rendered != "" {
						t.Errorf("index %d: Rendered should be empty on error, got %q", i, rc.Rendered)
					}
					continue
				}

				if rc.Err != nil {
					t.Errorf("index %d: unexpected error: %v", i, rc.Err)
				}
				if rc.Rendered != tc.wantRendered[i] {
					t.Errorf("index %d: Rendered = %q; want %q", i, rc.Rendered, tc.wantRendered[i])
				}
			}
		})
	}
}
