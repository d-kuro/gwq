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

	outputs, errs := RunSetupCommands(ctx, exec, dir, cmds)
	if len(outputs) != 2 {
		t.Errorf("expected 2 outputs, got %d", len(outputs))
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}
