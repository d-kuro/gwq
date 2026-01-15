package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	// EnvShellDepth is the environment variable name for tracking shell nesting depth.
	EnvShellDepth = "GWQ_SHELL_DEPTH"

	// DefaultNestWarningThreshold is the default depth at which to show a warning.
	DefaultNestWarningThreshold = 2
)

// getShellDepth returns the current shell nesting depth from environment variable.
// Returns 0 if the variable is not set or invalid.
func getShellDepth() int {
	depthStr := os.Getenv(EnvShellDepth)
	if depthStr == "" {
		return 0
	}
	depth, err := strconv.Atoi(depthStr)
	if err != nil || depth < 0 {
		return 0
	}
	return depth
}

// updateEnvVar updates or adds an environment variable in the given slice.
func updateEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// LaunchShell launches an interactive shell in the specified directory.
// This function is used by commands that support the --stay flag.
func LaunchShell(dir string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	currentDepth := getShellDepth()
	newDepth := currentDepth + 1

	// Show warning if nesting is deep
	if newDepth >= DefaultNestWarningThreshold {
		fmt.Fprintf(os.Stderr, "Warning: Shell nesting depth is %d. You need to type 'exit' %d time(s) to return to the original shell.\n", newDepth, newDepth)
		fmt.Fprintf(os.Stderr, "         To avoid nesting, use: cd $(gwq get <pattern>)\n")
	}

	fmt.Printf("Launching shell in: %s\n", dir)
	fmt.Println("Type 'exit' to return to the previous directory")

	shellCmd := exec.Command(shell)
	shellCmd.Dir = dir

	// Copy current environment and update depth
	env := os.Environ()
	env = updateEnvVar(env, EnvShellDepth, strconv.Itoa(newDepth))
	shellCmd.Env = env

	shellCmd.Stdin = os.Stdin
	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr

	return shellCmd.Run()
}
