package worktree

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// capturedCall records one invocation of the fake Executor.
type capturedCall struct {
	dir  string
	name string
	args []string
}

// fakeExecutor implements the Executor interface. Tests use it to assert the
// exact name/args passed through from RunSetupCommands.
type fakeExecutor struct {
	calls   []capturedCall
	outputs []string
	errs    []error
}

var _ Executor = (*fakeExecutor)(nil)

func (f *fakeExecutor) ExecuteInDirWithOutput(_ context.Context, dir, name string, args ...string) (string, error) {
	f.calls = append(f.calls, capturedCall{dir: dir, name: name, args: append([]string(nil), args...)})

	var out string
	if len(f.outputs) > 0 {
		out = f.outputs[0]
		f.outputs = f.outputs[1:]
	}

	var err error
	if len(f.errs) > 0 {
		err = f.errs[0]
		f.errs = f.errs[1:]
	}

	return out, err
}

func TestRunSetupCommands_ShellInvocation(t *testing.T) {
	exec := &fakeExecutor{
		outputs: []string{"ok1", "ok2"},
		errs:    []error{nil, errors.New("fail")},
	}
	dir := t.TempDir()
	ctx := context.Background()
	cmds := []string{
		"echo \"hello world\"",
		"ln -s ~/foo bar && touch baz",
	}

	results := RunSetupCommands(ctx, exec, dir, cmds)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for i, want := range cmds {
		call := exec.calls[i]
		if call.dir != dir {
			t.Errorf("index %d: dir = %q; want %q", i, call.dir, dir)
		}
		if call.name != "sh" {
			t.Errorf("index %d: name = %q; want \"sh\"", i, call.name)
		}
		wantArgs := []string{"-c", want}
		if !reflect.DeepEqual(call.args, wantArgs) {
			t.Errorf("index %d: args = %v; want %v", i, call.args, wantArgs)
		}
	}

	if results[0].Err != nil {
		t.Errorf("results[0].Err = %v; want nil", results[0].Err)
	}
	if results[0].Output != "ok1" {
		t.Errorf("results[0].Output = %q; want \"ok1\"", results[0].Output)
	}
	if results[0].Command != cmds[0] {
		t.Errorf("results[0].Command = %q; want %q", results[0].Command, cmds[0])
	}
	if results[1].Err == nil {
		t.Errorf("results[1].Err = nil; want non-nil")
	}
	if results[1].Output != "ok2" {
		t.Errorf("results[1].Output = %q; want \"ok2\"", results[1].Output)
	}
}

func TestRunSetupCommands_SkipEmpty(t *testing.T) {
	exec := &fakeExecutor{}
	dir := t.TempDir()
	ctx := context.Background()

	results := RunSetupCommands(ctx, exec, dir, []string{"", "   ", "echo hi", "\t\n"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result after skipping empties, got %d", len(results))
	}
	if results[0].Command != "echo hi" {
		t.Errorf("results[0].Command = %q; want \"echo hi\"", results[0].Command)
	}
	if len(exec.calls) != 1 {
		t.Errorf("expected 1 executor call, got %d", len(exec.calls))
	}
}

func TestRunSetupCommands_IndexAlignment(t *testing.T) {
	// Regression: the legacy implementation appended to errs only on failure,
	// so outputs[i] and errs[i] drifted. The new []SetupResult shape gives
	// each command a self-contained entry, even when earlier ones succeeded.
	exec := &fakeExecutor{
		outputs: []string{"o1", "o2", "o3"},
		errs:    []error{nil, errors.New("boom"), nil},
	}
	ctx := context.Background()
	results := RunSetupCommands(ctx, exec, "/dir", []string{"a", "b", "c"})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Err != nil || results[0].Output != "o1" || results[0].Command != "a" {
		t.Errorf("result 0 = %+v; want {Command: a, Output: o1, Err: nil}", results[0])
	}
	if results[1].Err == nil || results[1].Output != "o2" || results[1].Command != "b" {
		t.Errorf("result 1 = %+v; want {Command: b, Output: o2, Err: non-nil}", results[1])
	}
	if results[2].Err != nil || results[2].Output != "o3" || results[2].Command != "c" {
		t.Errorf("result 2 = %+v; want {Command: c, Output: o3, Err: nil}", results[2])
	}
}
