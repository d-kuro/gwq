package cmd

import (
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
