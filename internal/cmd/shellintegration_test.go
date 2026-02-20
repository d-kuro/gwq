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

func TestCompletionBash_OutputsCompletionScript(t *testing.T) {
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
	if !strings.Contains(output, "__start_gwq") {
		t.Error("bash completion should contain __start_gwq")
	}
	if !strings.Contains(output, "COMPREPLY") {
		t.Error("bash completion should contain COMPREPLY")
	}
}

func TestCompletionFish(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("cd.launch_shell", true)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	completionFishCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "fish"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "complete -c gwq") {
		t.Error("fish completion should contain 'complete -c gwq'")
	}
}

func TestCompletionZsh(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("cd.launch_shell", true)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	completionZshCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "zsh"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "compdef") {
		t.Error("zsh completion should contain 'compdef'")
	}
}

func TestCompletionZsh_LaunchShellFalse(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("cd.launch_shell", false)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	completionZshCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "zsh"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "compdef") {
		t.Error("zsh completion should contain 'compdef'")
	}
	if !strings.Contains(output, "__GWQ_CD_SHIM") {
		t.Error("launch_shell=false should include wrapper function with __GWQ_CD_SHIM")
	}
}

func TestCompletionFish_LaunchShellFalse(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("cd.launch_shell", false)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	completionFishCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "fish"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "complete -c gwq") {
		t.Error("fish completion should contain 'complete -c gwq'")
	}
	if !strings.Contains(output, "__GWQ_CD_SHIM") {
		t.Error("launch_shell=false should include wrapper function with __GWQ_CD_SHIM")
	}
}

func TestCompletionPowershell(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	completionPowershellCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "powershell"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Register-ArgumentCompleter") {
		t.Error("powershell completion should contain Register-ArgumentCompleter")
	}
}

func TestCompletionInvalidSubcommand(t *testing.T) {
	rootCmd.SetArgs([]string{"completion", "invalid"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid subcommand, got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected 'unknown command' in error, got %q", err.Error())
	}
}

func TestCompletionCmd_Structure(t *testing.T) {
	if completionCmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("completionCmd.Use = %q, want %q", completionCmd.Use, "completion [bash|zsh|fish|powershell]")
	}

	// Verify subcommands exist
	subcommands := completionCmd.Commands()
	names := make(map[string]bool)
	for _, cmd := range subcommands {
		names[cmd.Name()] = true
	}
	for _, expected := range []string{"bash", "zsh", "fish", "powershell"} {
		if !names[expected] {
			t.Errorf("completion command should have %q subcommand", expected)
		}
	}
}
