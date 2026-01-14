package cmd

import (
	"fmt"
	"os"
	"os/exec"
)

// LaunchShell launches an interactive shell in the specified directory.
// This function is used by commands that support the --stay flag.
func LaunchShell(dir string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	fmt.Printf("Launching shell in: %s\n", dir)
	fmt.Println("Type 'exit' to return to the original directory")

	shellCmd := exec.Command(shell)
	shellCmd.Dir = dir
	shellCmd.Env = os.Environ()
	shellCmd.Stdin = os.Stdin
	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr

	return shellCmd.Run()
}
