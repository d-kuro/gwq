package worktree

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

type fakeExecutor struct {
	calls   []string
	outputs []string
	errs    []error
}

func (f *fakeExecutor) ExecuteInDirWithOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	f.calls = append(f.calls, filepath.Join(dir, name+" "+filepath.Join(args...)))

	if len(f.outputs) > 0 {
		out := f.outputs[0]
		f.outputs = f.outputs[1:]

		var err error
		if len(f.errs) > 0 {
			err = f.errs[0]
			f.errs = f.errs[1:]
		}

		return out, err
	}

	return "", nil
}

func TestRunSetupCommands(t *testing.T) {
	exec := &fakeExecutor{
		outputs: []string{"ok1", "ok2"},
		errs:    []error{nil, errors.New("fail")},
	}
	dir := t.TempDir()
	ctx := context.Background()
	cmds := []string{"echo foo", "failcmd bar"}

	results := RunSetupCommands(ctx, exec, dir, cmds)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results[0].Output != "ok1" {
		t.Errorf("expected first output 'ok1', got %q", results[0].Output)
	}
	if results[0].Err != nil {
		t.Errorf("expected first command to succeed, got error: %v", results[0].Err)
	}
	if results[1].Output != "ok2" {
		t.Errorf("expected second output 'ok2', got %q", results[1].Output)
	}
	if results[1].Err == nil {
		t.Error("expected second command to fail, got nil error")
	}
}
