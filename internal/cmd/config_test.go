package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestConfigSetValueTypes(t *testing.T) {
	for _, local := range []bool{false, true} {
		name := "global"
		if local {
			name = "local"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Chdir(home)
			if err := os.MkdirAll(filepath.Join(home, ".config", "gwq"), 0755); err != nil {
				t.Fatal(err)
			}
			configSetLocal = local
			t.Cleanup(func() { configSetLocal = false; viper.Reset() })
			for _, tc := range []struct {
				key, input string
				want       any
			}{
				{"worktree.basedir", "2026-worktrees", "2026-worktrees"},
				{"naming.template", "2026/{{.Branch}}", "2026/{{.Branch}}"},
				{"worktree.auto_mkdir", "true", true},
				{"finder.preview", "false", false},
				{"finder.preview_size", "42", int64(42)},
			} {
				if err := runConfigSet(&cobra.Command{}, []string{tc.key, tc.input}); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(home, ".config", "gwq", "config.toml")
				if local {
					path = filepath.Join(home, ".gwq.toml")
				}
				stored := viper.New()
				stored.SetConfigFile(path)
				if err := stored.ReadInConfig(); err != nil {
					t.Fatal(err)
				}
				if got := stored.Get(tc.key); !reflect.DeepEqual(got, tc.want) {
					t.Errorf("set %s %q: stored %#v (%T), want %#v (%T)", tc.key, tc.input, got, got, tc.want, tc.want)
				}
			}
		})
	}
}
