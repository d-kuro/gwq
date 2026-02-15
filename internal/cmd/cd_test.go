package cmd

import (
	"os"
	"testing"
)

func TestCdCmd_Structure(t *testing.T) {
	// Test that cdCmd is properly configured
	if cdCmd.Use != "cd [pattern]" {
		t.Errorf("cdCmd.Use = %q, want %q", cdCmd.Use, "cd [pattern]")
	}

	if cdCmd.Short == "" {
		t.Error("cdCmd.Short should not be empty")
	}

	if cdCmd.RunE == nil {
		t.Error("cdCmd.RunE should not be nil")
	}
}

func TestCdCmd_Flags(t *testing.T) {
	// Test that global flag is defined
	flag := cdCmd.Flags().Lookup("global")
	if flag == nil {
		t.Fatal("global flag should be defined")
	}

	if flag.Shorthand != "g" {
		t.Errorf("global flag shorthand = %q, want %q", flag.Shorthand, "g")
	}
}

func TestCdCmd_ValidArgs(t *testing.T) {
	// Test that ValidArgsFunction is set
	if cdCmd.ValidArgsFunction == nil {
		t.Error("cdCmd.ValidArgsFunction should not be nil")
	}
}

func TestCdCmd_EnvCdShimConstant(t *testing.T) {
	if envCdShim != "__GWQ_CD_SHIM" {
		t.Errorf("envCdShim = %q, want %q", envCdShim, "__GWQ_CD_SHIM")
	}
}

func TestCdCmd_ShimEnvIsNotSetByDefault(t *testing.T) {
	// Verify the environment variable is not set in the test environment
	val := os.Getenv(envCdShim)
	if val == "1" {
		t.Skip("__GWQ_CD_SHIM is set in the test environment")
	}
}
