package git

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // git object ids are SHA-1 in sha1 repositories
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// CowOptions controls copy-on-write worktree creation.
type CowOptions struct {
	// Fallback makes an unsupported situation (no reflink support, differing
	// attributes, ...) fall back to Git's normal checkout with a warning.
	// When false, such situations are errors.
	Fallback bool
	// Stderr receives the summary and warnings. Defaults to os.Stderr.
	Stderr io.Writer
}

// CowStats summarizes what a copy-on-write creation did.
type CowStats struct {
	Candidates    int      // tracked regular files in the target tree
	Cloned        int      // files reflinked and verified (includes LFS)
	LFSCloned     int      // LFS files reflinked from verified content
	Rejected      int      // clone attempted but verification failed
	RejectedPaths []string // paths whose cloned bytes did not match the expected object
	BytesShared   int64    // bytes of reflinked content
}

// errCowUnsupported marks situations where CoW creation cannot be done safely.
var errCowUnsupported = errors.New("copy-on-write not possible")

func cowUnsupported(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errCowUnsupported, fmt.Sprintf(format, a...))
}

// AddWorktreeCow creates a worktree like AddWorktree, but materializes files by
// reflinking verified content from an existing clean worktree and from the Git LFS
// object store, instead of writing every file again.
func (g *Git) AddWorktreeCow(path, branch string, createBranch bool, opts CowOptions) (err error) {
	w := opts.Stderr
	if w == nil {
		w = os.Stderr
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	_, statErr := os.Stat(abs)
	existed := statErr == nil

	args := []string{"worktree", "add", "--no-checkout"}
	if createBranch {
		args = append(args, "-b", branch, abs)
	} else {
		args = append(args, abs, branch)
	}
	if _, err := g.run(args...); err != nil {
		return fmt.Errorf("failed to add worktree: %w", err)
	}
	defer func() {
		if err != nil {
			g.rollbackCow(abs, branch, createBranch, existed)
		}
	}()

	stats, err := g.cowMaterialize(abs)
	if errors.Is(err, errCowUnsupported) {
		if !opts.Fallback {
			return fmt.Errorf("%w (use --cow-fallback to fall back to a normal checkout)", err)
		}
		_, _ = fmt.Fprintf(w, "[gwq] warning: %v; falling back to normal checkout (no storage sharing)\n", err)
		stats, err = CowStats{}, nil
	}
	if err != nil {
		return err
	}
	// Let Git materialize everything that was not verified. Entries seeded into the
	// index with matching stat data are left untouched, which preserves shared extents.
	if _, err := gitIn(abs, nil, "reset", "--hard", "-q"); err != nil {
		return fmt.Errorf("failed to check out remaining files: %w", err)
	}
	for i, p := range stats.RejectedPaths {
		if i == 5 {
			_, _ = fmt.Fprintf(w, "[gwq] warning: ... and %d more rejected files\n", len(stats.RejectedPaths)-5)
			break
		}
		_, _ = fmt.Fprintf(w, "[gwq] warning: %s: source content did not match the expected object; left to git checkout (not shared)\n", p)
	}
	_, _ = fmt.Fprintf(w, "[gwq] cow: %d/%d files reflinked (%d LFS), %.1f MiB shared, %d rejected\n",
		stats.Cloned, stats.Candidates, stats.LFSCloned, float64(stats.BytesShared)/(1<<20), stats.Rejected)
	return nil
}

// rollbackCow removes a worktree this call registered, plus the branch it created.
func (g *Git) rollbackCow(abs, branch string, createdBranch, dirExisted bool) {
	_, _ = g.run("worktree", "remove", "--force", "--force", abs)
	_, _ = g.run("worktree", "prune")
	if createdBranch {
		_, _ = g.run("branch", "-D", branch)
	}
	if dirExisted {
		_ = os.MkdirAll(abs, 0o755) // user-provided empty directory: leave it as it was
	}
}

type treeEntry struct {
	mode, sha string
}

type cowJob struct {
	path   string
	entry  treeEntry
	srcAbs string // reflink source
	lfs    *lfsPointer
}

type lfsPointer struct {
	oid  string
	size int64
}

func (g *Git) cowMaterialize(target string) (CowStats, error) {
	var stats CowStats

	if out, err := gitIn(target, nil, "rev-parse", "--show-object-format"); err != nil || strings.TrimSpace(string(out)) != "sha1" {
		return stats, cowUnsupported("only sha1 repositories are supported")
	}
	if v, _ := gitIn(target, nil, "config", "--get", "core.autocrlf"); strings.TrimSpace(string(v)) != "" && strings.TrimSpace(string(v)) != "false" {
		return stats, cowUnsupported("core.autocrlf is set")
	}
	if sp, _ := gitIn(target, nil, "config", "--get", "core.sparseCheckout"); strings.TrimSpace(string(sp)) == "true" {
		return stats, cowUnsupported("sparse checkout is enabled")
	}

	head, err := gitIn(target, nil, "rev-parse", "HEAD")
	if err != nil {
		return stats, err
	}
	targetCommit := strings.TrimSpace(string(head))
	targetTree, err := lsTree(target, targetCommit)
	if err != nil {
		return stats, err
	}

	source, err := g.pickCowSource(target, targetCommit)
	if err != nil {
		return stats, err
	}
	sourceTree, err := lsTree(source, "HEAD")
	if err != nil {
		return stats, err
	}

	// Attributes decide what checkout would write (filters, EOL, ident, encoding).
	// Materialize the target's .gitattributes first so check-attr evaluates exactly
	// the target's rules. Cloned bytes are separately verified against the target
	// blob (plain) or OID (LFS), so the source's own attributes do not matter.
	for p, e := range targetTree {
		if filepath.Base(p) != ".gitattributes" || !safeRelPath(p) {
			continue
		}
		blob, err := gitIn(target, nil, "cat-file", "blob", e.sha)
		if err != nil {
			return stats, err
		}
		dst := filepath.Join(target, p)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return stats, err
		}
		if err := os.WriteFile(dst, blob, 0o644); err != nil {
			return stats, err
		}
	}

	dirty, err := sourceUnsafePaths(source)
	if err != nil {
		return stats, err
	}

	// Candidate files: identical blob, regular file, clean in source.
	var cands []string
	for p, e := range targetTree {
		if e.mode != "100644" && e.mode != "100755" {
			continue
		}
		stats.Candidates++
		s, ok := sourceTree[p]
		if !ok || s != e || dirty.has(p) || !safeRelPath(p) || filepath.Base(p) == ".gitattributes" {
			continue
		}
		if fi, err := os.Lstat(filepath.Join(source, p)); err != nil || !fi.Mode().IsRegular() ||
			(fi.Mode()&0o111 != 0) != (e.mode == "100755") {
			continue
		}
		cands = append(cands, p)
	}

	attrs, err := checkAttrs(target, cands)
	if err != nil {
		return stats, err
	}

	store, err := g.lfsStore(target)
	if err != nil {
		return stats, err
	}

	var jobs []cowJob
	for _, p := range cands {
		a := attrs[p]
		if a["eol"] != "unspecified" || a["ident"] != "unspecified" || a["working-tree-encoding"] != "unspecified" {
			continue
		}
		switch a["filter"] {
		case "unspecified":
			if a["text"] != "unspecified" && a["text"] != "unset" {
				continue
			}
			jobs = append(jobs, cowJob{path: p, entry: targetTree[p], srcAbs: filepath.Join(source, p)})
		case "lfs":
			blob, err := gitIn(target, nil, "cat-file", "blob", targetTree[p].sha)
			if err != nil {
				continue
			}
			ptr, ok := parseLFSPointer(blob)
			if !ok {
				continue
			}
			obj := filepath.Join(store, ptr.oid[:2], ptr.oid[2:4], ptr.oid)
			if fi, err := os.Stat(obj); err != nil || !fi.Mode().IsRegular() || fi.Size() != ptr.size {
				continue // missing or wrong-sized object: let Git/LFS handle it
			}
			jobs = append(jobs, cowJob{path: p, entry: targetTree[p], srcAbs: obj, lfs: ptr})
		}
	}

	// Clone and verify in parallel.
	var (
		mu          sync.Mutex
		verified    []cowJob
		unsupported atomic.Bool
		wg          sync.WaitGroup
		sem         = make(chan struct{}, runtime.NumCPU())
	)
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if unsupported.Load() {
				return
			}
			dst := filepath.Join(target, j.path)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return
			}
			perm := os.FileMode(0o644)
			if j.entry.mode == "100755" {
				perm = 0o755
			}
			if err := reflinkFile(j.srcAbs, dst, perm); err != nil {
				if errors.Is(err, errNoReflink) {
					unsupported.Store(true)
				}
				return
			}
			size, ok := verifyClone(dst, j)
			mu.Lock()
			defer mu.Unlock()
			if !ok {
				_ = os.Remove(dst)
				stats.Rejected++
				stats.RejectedPaths = append(stats.RejectedPaths, j.path)
				return
			}
			verified = append(verified, j)
			stats.Cloned++
			stats.BytesShared += size
			if j.lfs != nil {
				stats.LFSCloned++
			}
		}()
	}
	wg.Wait()
	if unsupported.Load() {
		return stats, cowUnsupported("filesystem does not support reflinks between %s and %s", source, target)
	}
	if len(verified) == 0 {
		return stats, nil
	}

	// Attributes must be present before the index is refreshed (LFS clean filter).
	for p, e := range targetTree {
		if filepath.Base(p) != ".gitattributes" {
			continue
		}
		if !containsJob(verified, p) {
			blob, err := gitIn(target, nil, "cat-file", "blob", e.sha)
			if err != nil {
				return stats, err
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(target, p)), 0o755); err != nil {
				return stats, err
			}
			if err := os.WriteFile(filepath.Join(target, p), blob, 0o644); err != nil {
				return stats, err
			}
			verified = append(verified, cowJob{path: p, entry: e})
		}
	}

	// Seed the index, then let Git record stat data.
	var idx bytes.Buffer
	for _, j := range verified {
		fmt.Fprintf(&idx, "%s %s 0\t%s\x00", j.entry.mode, j.entry.sha, j.path)
	}
	if _, err := gitIn(target, idx.Bytes(), "update-index", "--add", "-z", "--index-info"); err != nil {
		return stats, err
	}
	if _, err := gitIn(target, nil, "update-index", "-q", "--really-refresh"); err != nil {
		return stats, err
	}
	return stats, nil
}

func containsJob(js []cowJob, p string) bool {
	for _, j := range js {
		if j.path == p {
			return true
		}
	}
	return false
}

func safeRelPath(p string) bool {
	if filepath.IsAbs(p) {
		return false
	}
	for _, c := range strings.Split(p, "/") {
		if c == ".." || c == "." || c == "" || strings.EqualFold(c, ".git") {
			return false
		}
	}
	return true
}

// verifyClone re-reads the cloned file and checks it against the expected object.
func verifyClone(dst string, j cowJob) (int64, bool) {
	f, err := os.Open(dst)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return 0, false
	}
	size := fi.Size()
	if j.lfs != nil {
		if size != j.lfs.size {
			return 0, false
		}
		h := sha256.New()
		n, err := io.Copy(h, f)
		return size, err == nil && n == size && hex.EncodeToString(h.Sum(nil)) == j.lfs.oid
	}
	h := sha1.New() //nolint:gosec
	fmt.Fprintf(h, "blob %d\x00", size)
	n, err := io.Copy(h, f)
	return size, err == nil && n == size && hex.EncodeToString(h.Sum(nil)) == j.entry.sha
}

func parseLFSPointer(b []byte) (*lfsPointer, bool) {
	if len(b) > 1024 {
		return nil, false
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) < 3 || lines[0] != "version https://git-lfs.github.com/spec/v1" {
		return nil, false
	}
	var oid string
	var size int64 = -1
	for _, l := range lines[1:] {
		k, v, ok := strings.Cut(l, " ")
		if !ok {
			return nil, false
		}
		switch k {
		case "oid":
			h, ok := strings.CutPrefix(v, "sha256:")
			if !ok || len(h) != 64 {
				return nil, false
			}
			if _, err := hex.DecodeString(h); err != nil || strings.ToLower(h) != h {
				return nil, false
			}
			oid = h
		case "size":
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n < 0 {
				return nil, false
			}
			size = n
		}
	}
	if oid == "" || size < 0 {
		return nil, false
	}
	return &lfsPointer{oid: oid, size: size}, true
}

// lfsStore returns the LFS object directory shared by all worktrees.
func (g *Git) lfsStore(target string) (string, error) {
	common, err := gitIn(target, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(string(common))
	storage := filepath.Join(dir, "lfs")
	if v, _ := gitIn(target, nil, "config", "--get", "lfs.storage"); strings.TrimSpace(string(v)) != "" {
		storage = strings.TrimSpace(string(v))
		if !filepath.IsAbs(storage) {
			storage = filepath.Join(dir, storage)
		}
	}
	return filepath.Join(storage, "objects"), nil
}

// pickCowSource chooses the worktree closest (in commits) to the target.
func (g *Git) pickCowSource(target, targetCommit string) (string, error) {
	wts, err := g.ListWorktrees()
	if err != nil {
		return "", err
	}
	best, bestDist := "", -1
	for _, wt := range wts {
		if sameFile(wt.Path, target) || wt.CommitHash == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(wt.Path, ".git")); err != nil {
			continue
		}
		out, err := gitIn(wt.Path, nil, "rev-list", "--left-right", "--count", "HEAD..."+targetCommit)
		if err != nil {
			continue
		}
		f := strings.Fields(string(out))
		if len(f) != 2 {
			continue
		}
		a, _ := strconv.Atoi(f[0])
		b, _ := strconv.Atoi(f[1])
		if bestDist < 0 || a+b < bestDist {
			best, bestDist = wt.Path, a+b
		}
	}
	if best == "" {
		return "", cowUnsupported("no usable source worktree")
	}
	return best, nil
}

func sameFile(a, b string) bool {
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}

type pathSet struct {
	exact    map[string]bool
	prefixes []string
}

func (s pathSet) has(p string) bool {
	if s.exact[p] {
		return true
	}
	for _, pre := range s.prefixes {
		if strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}

// sourceUnsafePaths lists paths in the source worktree that must not be copied:
// modified, untracked, ignored, unmerged, skip-worktree or assume-unchanged.
func sourceUnsafePaths(source string) (pathSet, error) {
	set := pathSet{exact: map[string]bool{}}
	out, err := gitIn(source, nil, "status", "--porcelain=v1", "-z", "--ignored=matching", "-uall")
	if err != nil {
		return set, err
	}
	recs := strings.Split(string(out), "\x00")
	for i := 0; i < len(recs); i++ {
		r := recs[i]
		if len(r) < 4 {
			continue
		}
		p := r[3:]
		if r[0] == 'R' || r[0] == 'C' || r[1] == 'R' || r[1] == 'C' {
			i++
			if i < len(recs) {
				set.exact[recs[i]] = true
			}
		}
		if strings.HasSuffix(p, "/") {
			set.prefixes = append(set.prefixes, p)
		} else {
			set.exact[p] = true
		}
	}
	lf, err := gitIn(source, nil, "ls-files", "-v", "-z")
	if err != nil {
		return set, err
	}
	for _, r := range strings.Split(string(lf), "\x00") {
		if len(r) > 2 && r[0] != 'H' {
			set.exact[r[2:]] = true
		}
	}
	return set, nil
}

func lsTree(dir, rev string) (map[string]treeEntry, error) {
	out, err := gitIn(dir, nil, "ls-tree", "-r", "-z", "--full-tree", rev)
	if err != nil {
		return nil, err
	}
	m := map[string]treeEntry{}
	for _, r := range strings.Split(string(out), "\x00") {
		meta, p, ok := strings.Cut(r, "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		if len(f) != 3 {
			continue
		}
		m[p] = treeEntry{mode: f[0], sha: f[2]}
	}
	return m, nil
}

// checkAttrs returns the attributes that affect checkout conversion, per path.
func checkAttrs(dir string, paths []string) (map[string]map[string]string, error) {
	res := map[string]map[string]string{}
	if len(paths) == 0 {
		return res, nil
	}
	in := strings.Join(paths, "\x00") + "\x00"
	out, err := gitIn(dir, []byte(in), "check-attr", "-z", "--stdin",
		"filter", "text", "eol", "ident", "working-tree-encoding")
	if err != nil {
		return nil, err
	}
	f := strings.Split(string(out), "\x00")
	for i := 0; i+2 < len(f); i += 3 {
		if res[f[i]] == nil {
			res[f[i]] = map[string]string{}
		}
		res[f[i]][f[i+1]] = f[i+2]
	}
	return res, nil
}

func gitIn(dir string, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
