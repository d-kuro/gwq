package git

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func cowGit(t *testing.T, dir, input string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func cowRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	return NewTestRepository(t).Path
}

func cowWrite(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

func requireReflink(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "source")
	cowWrite(t, src, "reflink probe", 0o600)
	if err := reflinkFile(src, filepath.Join(dir, "clone"), 0o600); errors.Is(err, errNoReflink) {
		t.Skip("test filesystem does not support reflinks")
	} else if err != nil {
		t.Fatal(err)
	}
}

func TestCowPostCheckoutHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test hook uses a shell script")
	}
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			repo := cowRepo(t)
			head := cowGit(t, repo, "", "rev-parse", "HEAD")
			hooks := t.TempDir()
			cowGit(t, repo, "", "config", "core.hooksPath", hooks)
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > hook-args\npwd > hook-cwd\necho hook-output\n"
			if fail {
				script += "exit 7\n"
			}
			cowWrite(t, filepath.Join(hooks, "post-checkout"), script, 0o755)
			dst := filepath.Join(t.TempDir(), "cow")
			var output bytes.Buffer
			err := New(repo).AddWorktreeCow(dst, "cow", true, CowOptions{Fallback: true, Stderr: &output})
			if fail && err == nil || !fail && err != nil {
				t.Fatalf("hook failure=%v, creation error=%v", fail, err)
			}
			if !strings.Contains(output.String(), "hook-output") {
				t.Fatal("hook output did not reach the diagnostic writer")
			}
			args, err := os.ReadFile(filepath.Join(dst, "hook-args"))
			if err != nil {
				t.Fatal(err)
			}
			want := strings.Repeat("0", len(head)) + "\n" + head + "\n1\n"
			if string(args) != want {
				t.Fatalf("hook args = %q, want %q", args, want)
			}
			cwd, err := os.ReadFile(filepath.Join(dst, "hook-cwd"))
			if err != nil || !sameFile(strings.TrimSpace(string(cwd)), dst) {
				t.Fatalf("hook cwd = %q, error=%v", cwd, err)
			}
			if got := cowGit(t, dst, "", "rev-parse", "HEAD"); got != head {
				t.Fatalf("completed worktree lost after hook: %s", got)
			}
		})
	}
}

func TestCowAttributeCheckout(t *testing.T) {
	requireReflink(t)
	for _, kind := range []string{"crlf-attributes", "executable-attributes", "symlink-attributes", "legacy-crlf"} {
		t.Run(kind, func(t *testing.T) {
			repo := cowRepo(t)
			cowGit(t, repo, "", "checkout", "-b", "target")
			attrs := filepath.Join(repo, ".gitattributes")
			path := ".gitattributes"
			switch kind {
			case "crlf-attributes":
				cowWrite(t, attrs, ".gitattributes text eol=crlf\n", 0o644)
			case "executable-attributes":
				cowWrite(t, attrs, "*.bin -text\n", 0o755)
			case "symlink-attributes":
				cowWrite(t, filepath.Join(repo, "attributes-target"), "*.bin -text\n", 0o644)
				if err := os.Symlink("attributes-target", attrs); err != nil {
					t.Fatal(err)
				}
			case "legacy-crlf":
				cowGit(t, repo, "", "config", "core.eol", "crlf")
				cowWrite(t, attrs, "README.md crlf\n", 0o644)
				path = "README.md"
			}
			cowGit(t, repo, "", "add", ".")
			cowGit(t, repo, "", "commit", "-qm", "target attributes")
			cowGit(t, repo, "", "checkout", "main")
			dst := filepath.Join(t.TempDir(), "cow")
			if err := New(repo).AddWorktreeCow(dst, "target", false, CowOptions{Stderr: io.Discard}); err != nil {
				t.Fatal(err)
			}
			normal := filepath.Join(t.TempDir(), "normal")
			cowGit(t, repo, "", "worktree", "add", "--detach", normal, "target")
			got, err := os.ReadFile(filepath.Join(dst, path))
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(normal, path))
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("checkout bytes = %q, normal = %q, error=%v", got, want, err)
			}
			gotInfo, err := os.Lstat(filepath.Join(dst, path))
			if err != nil {
				t.Fatal(err)
			}
			wantInfo, err := os.Lstat(filepath.Join(normal, path))
			if err != nil || gotInfo.Mode() != wantInfo.Mode() {
				t.Fatalf("checkout mode = %s, normal info = %v, error=%v", gotInfo.Mode(), wantInfo, err)
			}
			status, err := gitIn(dst, nil, "status", "--porcelain")
			if err != nil || len(status) != 0 {
				t.Fatalf("new checkout is dirty: %s, error=%v", status, err)
			}
		})
	}
}

func TestCowRejectsEscapingAttributePath(t *testing.T) {
	repo := cowRepo(t)
	blob := cowGit(t, repo, "", "rev-parse", "HEAD:README.md")
	attrs := cowGit(t, repo, "100644 blob "+blob+"\t.gitattributes\n", "mktree")
	escaped := cowGit(t, repo, "040000 tree "+attrs+"\tescaped\n", "mktree")
	tree := cowGit(t, repo, "040000 tree "+escaped+"\t..\n100644 blob "+blob+"\tREADME.md\n", "mktree")
	commit := cowGit(t, repo, "", "commit-tree", tree, "-p", "HEAD", "-m", "malformed tree")
	cowGit(t, repo, "", "branch", "target", commit)
	parent := t.TempDir()
	dst := filepath.Join(parent, "cow")
	if err := New(repo).AddWorktreeCow(dst, "target", false, CowOptions{Stderr: io.Discard}); err == nil {
		t.Fatal("expected invalid tree to be rejected")
	}
	if _, err := os.Stat(filepath.Join(parent, "escaped")); !os.IsNotExist(err) {
		t.Fatalf("invalid tree wrote outside checkout: %v", err)
	}
	if list := cowGit(t, repo, "", "worktree", "list", "--porcelain"); strings.Contains(list, dst) {
		t.Fatalf("failed checkout remains registered: %s", list)
	}
}

func TestCowLFSCheckoutPolicy(t *testing.T) {
	requireReflink(t)
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("git-lfs is not installed")
	}
	for _, policy := range []string{"hydrate", "skip-env", "skip-filter", "exclude", "include-other", "lfsconfig-exclude"} {
		t.Run(policy, func(t *testing.T) {
			repo := cowRepo(t)
			cowGit(t, repo, "", "lfs", "install", "--local")
			cowWrite(t, filepath.Join(repo, ".gitattributes"), "*.bin filter=lfs diff=lfs merge=lfs -text\n", 0o644)
			content := strings.Repeat("hydrated LFS bytes\n", 1024)
			cowWrite(t, filepath.Join(repo, "asset.bin"), content, 0o644)
			if policy == "lfsconfig-exclude" {
				cowWrite(t, filepath.Join(repo, ".lfsconfig"), "[lfs]\n\tfetchexclude = *.bin\n", 0o644)
			}
			cowGit(t, repo, "", "add", ".")
			cowGit(t, repo, "", "commit", "-qm", "LFS asset")
			switch policy {
			case "skip-env":
				t.Setenv("GIT_LFS_SKIP_SMUDGE", "1")
			case "skip-filter":
				cowGit(t, repo, "", "config", "filter.lfs.process", "git-lfs filter-process --skip")
			case "exclude":
				cowGit(t, repo, "", "config", "lfs.fetchexclude", "*.bin")
			case "include-other":
				cowGit(t, repo, "", "config", "lfs.fetchinclude", "other.bin")
			}
			dst := filepath.Join(t.TempDir(), "cow")
			var output bytes.Buffer
			if err := New(repo).AddWorktreeCow(dst, "cow", true, CowOptions{Stderr: &output}); err != nil {
				t.Fatal(err)
			}
			normal := filepath.Join(t.TempDir(), "normal")
			cowGit(t, repo, "", "worktree", "add", "--detach", normal, "HEAD")
			got, err := os.ReadFile(filepath.Join(dst, "asset.bin"))
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(normal, "asset.bin"))
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("LFS checkout differs from native Git: lengths %d vs %d, error=%v", len(got), len(want), err)
			}
			if policy == "hydrate" {
				if string(got) != content || !strings.Contains(output.String(), "(1 LFS)") {
					t.Fatalf("expected hydrated reflink: %s", output.String())
				}
			} else if !bytes.HasPrefix(got, []byte("version https://git-lfs.github.com/spec/v1\n")) || !strings.Contains(output.String(), "(0 LFS)") {
				t.Fatalf("expected pointer without LFS cloning: %s", output.String())
			}
		})
	}
}
