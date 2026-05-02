package worktree

import (
	"context"
	"strings"
)

// Executor is the minimal contract needed to run a setup command.
// command.NewStandardExecutor() satisfies it; tests supply fakes.
type Executor interface {
	ExecuteInDirWithOutput(ctx context.Context, dir, name string, args ...string) (string, error)
}

// SetupResult is the outcome of running one setup command. Each field is
// self-contained so callers do not need to correlate parallel output/error
// slices by index.
type SetupResult struct {
	Command string
	Output  string
	Err     error
}

// RunSetupCommands runs each non-empty command string via `sh -c` in the
// given directory. It returns one SetupResult per command actually executed
// (empty or whitespace-only commands are skipped silently).
func RunSetupCommands(ctx context.Context, executor Executor, dir string, commands []string) []SetupResult {
	results := make([]SetupResult, 0, len(commands))
	for _, cmd := range commands {
		trimmed := strings.TrimSpace(cmd)
		if trimmed == "" {
			continue
		}
		output, err := executor.ExecuteInDirWithOutput(ctx, dir, "sh", "-c", trimmed)
		results = append(results, SetupResult{Command: trimmed, Output: output, Err: err})
	}
	return results
}
