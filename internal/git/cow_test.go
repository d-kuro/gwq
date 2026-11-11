package git

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
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
	if _, err := reflinkFile(src, filepath.Join(dir, "clone"), 0o600); errors.Is(err, errNoReflink) {
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

// cowFiles commits the given files (path -> contents; a trailing "*" on the path
// makes the file executable) on the current branch.
func cowFiles(t *testing.T, repo string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		mode := os.FileMode(0o644)
		if strings.HasSuffix(p, "*") {
			p, mode = strings.TrimSuffix(p, "*"), 0o755
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repo, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		cowWrite(t, filepath.Join(repo, p), c, mode)
	}
	cowGit(t, repo, "", "add", "-A")
	cowGit(t, repo, "", "commit", "-qm", "files")
}

// cowSettle ages the tracked files of a worktree and refreshes its index, so that
// the index stat data is trusted: every file is older than the index file.
func cowSettle(t *testing.T, repo string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	for _, p := range strings.Split(cowGit(t, repo, "", "ls-files"), "\n") {
		if fi, err := os.Lstat(filepath.Join(repo, p)); err == nil && fi.Mode().IsRegular() {
			if err := os.Chtimes(filepath.Join(repo, p), old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	cowRefresh(t, repo)
}

// cowRefresh records the current stat data of every file in the index. Git compares
// whole seconds only, so a plain refresh can keep stale nanoseconds; start from
// zero stat data to record every field afresh.
func cowRefresh(t *testing.T, repo string) {
	t.Helper()
	cowGit(t, repo, "", "read-tree", "HEAD")
	cowGit(t, repo, "", "update-index", "--refresh")
}

func cowIndexPath(t *testing.T, repo string) string {
	t.Helper()
	return cowGit(t, repo, "", "rev-parse", "--path-format=absolute", "--git-path", "index")
}

type cowSummary struct {
	cloned, candidates, lfs, rejected, left, hashed int
	proof                                           string
	output                                          string
}

var cowSummaryRE = regexp.MustCompile(`(\d+)/(\d+) files reflinked \((\d+) LFS\), [\d.]+ MiB shared, (\d+) rejected, (\d+) left to git, (\d+) proven by hashing \((.+) proof\)`)

func cowAdd(t *testing.T, repo, branch string, create bool, verify bool) (string, cowSummary) {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "cow")
	var out bytes.Buffer
	if err := New(repo).AddWorktreeCow(dst, branch, create, CowOptions{Verify: verify, Stderr: &out}); err != nil {
		t.Fatalf("AddWorktreeCow: %v\n%s", err, out.String())
	}
	m := cowSummaryRE.FindStringSubmatch(out.String())
	if m == nil {
		t.Fatalf("no summary in output: %s", out.String())
	}
	n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
	return dst, cowSummary{n(1), n(2), n(3), n(4), n(5), n(6), m[7], out.String()}
}

// cowCheckWorktree asserts the worktree is exactly rev: clean status, HEAD, every
// tracked file's bytes and mode match the tree, and no file is left untracked.
func cowCheckWorktree(t *testing.T, dst, rev string) {
	t.Helper()
	if st := cowGit(t, dst, "", "status", "--porcelain", "--untracked-files=all"); st != "" {
		t.Fatalf("worktree is not clean:\n%s", st)
	}
	if got, want := cowGit(t, dst, "", "rev-parse", "HEAD"), cowGit(t, dst, "", "rev-parse", rev); got != want {
		t.Fatalf("HEAD = %s, want %s", got, want)
	}
	for _, line := range strings.Split(cowGit(t, dst, "", "ls-tree", "-r", rev), "\n") {
		meta, p, _ := strings.Cut(line, "\t")
		f := strings.Fields(meta)
		if len(f) != 3 || f[0] == "160000" {
			continue
		}
		fi, err := os.Lstat(filepath.Join(dst, p))
		if err != nil {
			t.Fatal(err)
		}
		if f[0] == "120000" {
			if fi.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("%s is not a symlink", p)
			}
			continue
		}
		got, err := os.ReadFile(filepath.Join(dst, p))
		if err != nil {
			t.Fatal(err)
		}
		if sum := cowGit(t, dst, string(got), "hash-object", "--path", p, "--stdin"); sum != f[2] {
			t.Fatalf("%s: content hashes to %s, tree has %s", p, sum, f[2])
		}
		if (fi.Mode()&0o111 != 0) != (f[0] == "100755") {
			t.Fatalf("%s: mode %s, tree has %s", p, fi.Mode(), f[0])
		}
	}
}

var cowDebugRE = regexp.MustCompile(`(?m)^(.+)\n  ctime: (\d+):(\d+)\n  mtime: (\d+):(\d+)\n  dev: \d+\tino: (\d+)\n  uid: (\d+)\tgid: (\d+)\n  size: (\d+)\tflags: \w+$`)

// cowCheckIndexStat asserts the index records exactly the stat data of every
// regular file, so Git trusts it without hashing; it returns how many it checked.
func cowCheckIndexStat(t *testing.T, dst string) int {
	t.Helper()
	checked := 0
	for _, m := range cowDebugRE.FindAllStringSubmatch(cowGit(t, dst, "", "ls-files", "--debug"), -1) {
		fi, err := os.Lstat(filepath.Join(dst, m[1]))
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		sd, ok := statDataOf(fi)
		if !ok {
			t.Skip("stat data unavailable on this platform")
		}
		u := func(i int) uint32 { v, _ := strconv.ParseUint(m[i], 10, 32); return uint32(v) }
		want := statData{u(2), u(3), u(4), u(5), 0, u(6), u(7), u(8), u(9)}
		sd.dev = 0
		if sd != want {
			t.Fatalf("%s: index stat %+v, file %+v", m[1], want, sd)
		}
		checked++
	}
	return checked
}

var cowBaseFiles = map[string]string{
	"a.txt":         "alpha\n",
	"dir/b.txt":     "bravo\n",
	"dir/sub/c.txt": "charlie\n",
	"run.sh*":       "#!/bin/sh\n",
	"empty.txt":     "",
}

const cowRegular = 6 // cowBaseFiles plus README.md

func cowBaseRepo(t *testing.T) string {
	t.Helper()
	requireReflink(t)
	repo := cowRepo(t)
	cowFiles(t, repo, cowBaseFiles)
	if err := os.Symlink("a.txt", filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	cowGit(t, repo, "", "add", "link")
	cowGit(t, repo, "", "commit", "-qm", "symlink")
	cowSettle(t, repo)
	return repo
}

func TestCowDonorIndexProof(t *testing.T) {
	repo := cowBaseRepo(t)
	dst, sum := cowAdd(t, repo, "cow", true, false)
	if sum.proof != "donor index" || sum.cloned != cowRegular || sum.candidates != cowRegular || sum.hashed != 0 || sum.rejected != 0 || sum.left != 1 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	cowCheckWorktree(t, dst, "HEAD")
	if n := cowCheckIndexStat(t, dst); n != cowRegular {
		t.Fatalf("checked index stat of %d files, want %d", n, cowRegular)
	}
	if got := cowGit(t, dst, "", "ls-files", "-s"); got != cowGit(t, repo, "", "ls-files", "-s") {
		t.Fatalf("index differs from the donor index:\n%s", got)
	}
}

func TestCowVerify(t *testing.T) {
	repo := cowBaseRepo(t)
	dst, sum := cowAdd(t, repo, "cow", true, true)
	if sum.proof != "hash" || sum.cloned != cowRegular || sum.hashed != cowRegular || sum.left != 1 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	cowCheckWorktree(t, dst, "HEAD")
	if n := cowCheckIndexStat(t, dst); n != cowRegular {
		t.Fatalf("checked index stat of %d files, want %d", n, cowRegular)
	}
}

func TestCowDirtyDonorFileIsNotCloned(t *testing.T) {
	for _, kind := range []string{"content", "touch"} {
		t.Run(kind, func(t *testing.T) {
			repo := cowBaseRepo(t)
			p := filepath.Join(repo, "dir", "b.txt")
			if kind == "content" {
				cowWrite(t, p, "BRAVO\n", 0o644) // same size, so only the timestamps tell
			} else if err := os.Chtimes(p, time.Now(), time.Now()); err != nil {
				t.Fatal(err)
			}
			dst, sum := cowAdd(t, repo, "cow", true, false)
			if sum.cloned != cowRegular-1 || sum.hashed != 0 {
				t.Fatalf("unexpected summary: %+v", sum)
			}
			cowCheckWorktree(t, dst, "HEAD")
			if got, _ := os.ReadFile(filepath.Join(dst, "dir", "b.txt")); string(got) != "bravo\n" {
				t.Fatalf("target has donor's dirty content %q", got)
			}
		})
	}
}

func TestCowRacilyCleanDonorFileIsNotCloned(t *testing.T) {
	repo := cowBaseRepo(t)
	a := filepath.Join(repo, "a.txt")
	racy := time.Now().Add(-30 * time.Minute)
	if err := os.Chtimes(a, racy, racy); err != nil {
		t.Fatal(err)
	}
	cowRefresh(t, repo)
	if _, sum := cowAdd(t, repo, "cow", true, false); sum.cloned != cowRegular {
		t.Fatalf("a.txt is older than the index and must be cloned: %+v", sum)
	}
	// The index file is as old as a.txt: a later same-timestamp edit would be invisible.
	if err := os.Chtimes(cowIndexPath(t, repo), racy, racy); err != nil {
		t.Fatal(err)
	}
	dst, sum := cowAdd(t, repo, "cow2", true, false)
	if sum.cloned != cowRegular-1 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	cowCheckWorktree(t, dst, "HEAD")
}

func TestCowStagedDonorChangeIsNotCloned(t *testing.T) {
	repo := cowBaseRepo(t)
	cowWrite(t, filepath.Join(repo, "a.txt"), "staged!\n", 0o644)
	cowGit(t, repo, "", "add", "a.txt")
	dst, sum := cowAdd(t, repo, "cow", true, false)
	if sum.cloned != cowRegular-1 || sum.hashed != 0 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	cowCheckWorktree(t, dst, "HEAD")
	if got, _ := os.ReadFile(filepath.Join(dst, "a.txt")); string(got) != "alpha\n" {
		t.Fatalf("staged donor content leaked into the target: %q", got)
	}
}

func TestCowDivergentTarget(t *testing.T) {
	repo := cowBaseRepo(t)
	cowGit(t, repo, "", "checkout", "-q", "-b", "target")
	cowFiles(t, repo, map[string]string{"dir/b.txt": "changed\n", "new.txt": "new\n", "run.sh*": "#!/bin/sh\necho hi\n"})
	cowGit(t, repo, "", "rm", "-q", "dir/sub/c.txt")
	cowGit(t, repo, "", "commit", "-qm", "remove")
	cowGit(t, repo, "", "checkout", "-q", "main")
	cowSettle(t, repo)
	dst, sum := cowAdd(t, repo, "target", false, false)
	// a.txt, README.md and empty.txt are identical in donor and target.
	if sum.cloned != 3 || sum.hashed != 0 || sum.proof != "donor index" {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	cowCheckWorktree(t, dst, "target")
	if n := cowCheckIndexStat(t, dst); n != cowRegular-1+1 {
		t.Fatalf("checked index stat of %d files", n)
	}
}

func TestCowDonorIndexFallsBackToHashing(t *testing.T) {
	for _, kind := range []string{"version-4", "unmerged", "skip-worktree"} {
		t.Run(kind, func(t *testing.T) {
			repo := cowBaseRepo(t)
			switch kind {
			case "version-4":
				cowGit(t, repo, "", "update-index", "--index-version", "4")
			case "unmerged":
				cowGit(t, repo, "100644 "+cowGit(t, repo, "", "rev-parse", "HEAD:a.txt")+" 2\ta.txt\n", "update-index", "--index-info")
			case "skip-worktree":
				cowGit(t, repo, "", "update-index", "--skip-worktree", "a.txt")
			}
			dst, sum := cowAdd(t, repo, "cow", true, false)
			if sum.proof != "hash" || !strings.Contains(sum.output, "proving clones by hashing") {
				t.Fatalf("expected a visible fallback to hashing: %+v", sum)
			}
			if sum.hashed != sum.cloned || sum.cloned == 0 {
				t.Fatalf("unexpected summary: %+v", sum)
			}
			if kind != "unmerged" { // an unmerged donor index leaves the donor itself dirty
				cowCheckWorktree(t, dst, "HEAD")
			}
		})
	}
}

func TestCowLFSDonorProof(t *testing.T) {
	requireReflink(t)
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("git-lfs is not installed")
	}
	content := strings.Repeat("hydrated LFS bytes\n", 1024)
	setup := func(t *testing.T) string {
		repo := cowRepo(t)
		cowGit(t, repo, "", "lfs", "install", "--local")
		cowFiles(t, repo, map[string]string{".gitattributes": "*.bin filter=lfs diff=lfs merge=lfs -text\n", "asset.bin": content})
		return repo
	}
	pointer := "version https://git-lfs.github.com/spec/v1\n"
	t.Run("hydrated donor, hydrated target", func(t *testing.T) {
		repo := setup(t)
		cowSettle(t, repo)
		dst, sum := cowAdd(t, repo, "cow", true, false)
		// README.md and asset.bin come from the donor; .gitattributes is Git's.
		if sum.cloned != 2 || sum.lfs != 1 || sum.hashed != 0 {
			t.Fatalf("unexpected summary: %+v", sum)
		}
		cowCheckWorktree(t, dst, "HEAD")
		if got, _ := os.ReadFile(filepath.Join(dst, "asset.bin")); string(got) != content {
			t.Fatal("asset.bin is not hydrated")
		}
	})
	t.Run("pointer donor, hydrated target uses the store with hashing", func(t *testing.T) {
		repo := setup(t)
		if err := os.Remove(filepath.Join(repo, "asset.bin")); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("git", "-C", repo, "checkout", "--", "asset.bin")
		cmd.Env = append(os.Environ(), "GIT_LFS_SKIP_SMUDGE=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		if got, _ := os.ReadFile(filepath.Join(repo, "asset.bin")); !strings.HasPrefix(string(got), pointer) {
			t.Fatal("donor asset.bin is not a pointer")
		}
		cowSettle(t, repo)
		dst, sum := cowAdd(t, repo, "cow", true, false)
		if sum.cloned != 2 || sum.lfs != 1 || sum.hashed != 1 {
			t.Fatalf("unexpected summary: %+v", sum)
		}
		cowCheckWorktree(t, dst, "HEAD")
		if got, _ := os.ReadFile(filepath.Join(dst, "asset.bin")); string(got) != content {
			t.Fatal("asset.bin is not hydrated")
		}
	})
	t.Run("pointer donor, pointer-only target", func(t *testing.T) {
		repo := setup(t)
		t.Setenv("GIT_LFS_SKIP_SMUDGE", "1")
		if err := os.Remove(filepath.Join(repo, "asset.bin")); err != nil {
			t.Fatal(err)
		}
		cowGit(t, repo, "", "checkout", "--", "asset.bin")
		cowSettle(t, repo)
		dst, sum := cowAdd(t, repo, "cow", true, false)
		if sum.cloned != 2 || sum.lfs != 0 || sum.hashed != 0 {
			t.Fatalf("unexpected summary: %+v", sum)
		}
		cowCheckWorktree(t, dst, "HEAD")
		if got, _ := os.ReadFile(filepath.Join(dst, "asset.bin")); !strings.HasPrefix(string(got), pointer) {
			t.Fatal("asset.bin is not a pointer")
		}
	})
	t.Run("hydrated donor, pointer-only target", func(t *testing.T) {
		repo := setup(t)
		cowSettle(t, repo)
		t.Setenv("GIT_LFS_SKIP_SMUDGE", "1")
		dst, sum := cowAdd(t, repo, "cow", true, false)
		if sum.cloned != 1 || sum.lfs != 0 {
			t.Fatalf("hydrated donor bytes must not become a pointer: %+v", sum)
		}
		cowCheckWorktree(t, dst, "HEAD")
	})
}
