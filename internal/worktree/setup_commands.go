package worktree

import (
	"context"
	"fmt"
	"strings"
)

// RunSetupCommands executes each command in the given directory using the provided executor.
// It returns a slice of errors (one per failed command, if any), and a slice of outputs (stdout+stderr per command).
func RunSetupCommands(ctx context.Context, executor interface {
	ExecuteInDirWithOutput(ctx context.Context, dir, name string, args ...string) (string, error)
}, dir string, commands []string) (outputs []string, errs []error) {
	for _, cmd := range commands {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}

		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			continue
		}

		name := parts[0]
		args := parts[1:]
		output, err := executor.ExecuteInDirWithOutput(ctx, dir, name, args...)
		outputs = append(outputs, output)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", cmd, err))
		}
	}
	return outputs, errs
}
