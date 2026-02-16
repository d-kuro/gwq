package worktree

import (
	"context"
	"fmt"
	"strings"
)

// SetupCommandResult represents the result of executing a setup command.
type SetupCommandResult struct {
	Command string
	Output  string
	Err     error
}

// RunSetupCommands executes each command in the given directory using the provided executor.
// It returns a slice of results, one per command, containing output and any error.
func RunSetupCommands(ctx context.Context, executor interface {
	ExecuteInDirWithOutput(ctx context.Context, dir, name string, args ...string) (string, error)
}, dir string, commands []string) []SetupCommandResult {
	var results []SetupCommandResult
	for _, cmd := range commands {
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			continue
		}
		cmd = strings.Join(parts, " ")

		name := parts[0]
		args := parts[1:]
		output, err := executor.ExecuteInDirWithOutput(ctx, dir, name, args...)

		var cmdErr error
		if err != nil {
			cmdErr = fmt.Errorf("%s: %w", cmd, err)
		}

		results = append(results, SetupCommandResult{
			Command: cmd,
			Output:  output,
			Err:     cmdErr,
		})
	}
	return results
}
