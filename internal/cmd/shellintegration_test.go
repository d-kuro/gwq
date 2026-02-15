package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestCompletionBash_LaunchShellTrue(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("cd.launch_shell", true)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	completionBashCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "bash"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "__GWQ_CD_SHIM") {
		t.Error("launch_shell=true should NOT include wrapper function")
	}
}

func TestCompletionBash_LaunchShellFalse(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("cd.launch_shell", false)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	completionBashCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "bash"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "__GWQ_CD_SHIM") {
		t.Error("launch_shell=false should include wrapper function with __GWQ_CD_SHIM")
	}
	if !strings.Contains(output, "gwq()") {
		t.Error("launch_shell=false should include gwq() wrapper function")
	}
}

func TestCompletionCmd_Structure(t *testing.T) {
	if completionCmd.Use != "completion [bash|zsh|fish]" {
		t.Errorf("completionCmd.Use = %q, want %q", completionCmd.Use, "completion [bash|zsh|fish]")
	}

	// Verify subcommands exist
	subcommands := completionCmd.Commands()
	names := make(map[string]bool)
	for _, cmd := range subcommands {
		names[cmd.Name()] = true
	}
	for _, expected := range []string{"bash", "zsh", "fish"} {
		if !names[expected] {
			t.Errorf("completion command should have %q subcommand", expected)
		}
	}
}
