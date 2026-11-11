package git

import (
	"bufio"
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
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CowOptions controls copy-on-write worktree creation.
type CowOptions struct {
	// Fallback makes an unsupported situation (no reflink support, differing
	// attributes, ...) fall back to Git's normal checkout with a warning.
	// When false, such situations are errors.
	Fallback bool
	// Verify proves every clone by hashing it against its Git blob or LFS SHA-256
	// instead of trusting the donor worktree's index stat data.
	Verify bool
	// Stderr receives the summary and warnings. Defaults to os.Stderr.
	Stderr io.Writer
}

// CowStats summarizes what a copy-on-write creation did.
type CowStats struct {
	Candidates    int      // tracked regular files in the target tree
	Cloned        int      // files reflinked and proven (includes LFS)
	LFSCloned     int      // LFS files reflinked from proven hydrated content
	Rejected      int      // clone attempted but the proof failed
	RejectedPaths []string // paths whose clone could not be proven equal to the expected object
	BytesShared   int64    // bytes of reflinked content
	LeftToGit     int      // tree entries Git checked out itself
	Hashed        int      // clones proven by hashing rather than donor index stat data
	Proof         string   // "donor index" or "hash"
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
	checkedOut := false
	defer func() {
		if err != nil && !checkedOut {
			g.rollbackCow(abs, branch, createBranch, existed)
		}
	}()

	stats, err := g.cowMaterialize(abs, opts, w)
	if errors.Is(err, errCowUnsupported) {
		if !opts.Fallback {
			return fmt.Errorf("%w (use --cow-fallback to fall back to a normal checkout)", err)
		}
		_, _ = fmt.Fprintf(w, "[gwq] warning: %v; falling back to normal checkout (no storage sharing)\n", err)
		stats, err = CowStats{}, nil
		if _, err := gitIn(abs, nil, "reset", "--hard", "-q"); err != nil {
			return fmt.Errorf("failed to check out remaining files: %w", err)
		}
	}
	if err != nil {
		return err
	}
	// Like git worktree add, leave a completed checkout in place if its hook fails.
	checkedOut = true
	for i, p := range stats.RejectedPaths {
		if i == 5 {
			_, _ = fmt.Fprintf(w, "[gwq] warning: ... and %d more rejected files\n", len(stats.RejectedPaths)-5)
			break
		}
		_, _ = fmt.Fprintf(w, "[gwq] warning: %s: clone could not be proven equal to the expected object; left to git checkout (not shared)\n", p)
	}
	_, _ = fmt.Fprintf(w, "[gwq] cow: %d/%d files reflinked (%d LFS), %.1f MiB shared, %d rejected, %d left to git, %d proven by hashing (%s proof)\n",
		stats.Cloned, stats.Candidates, stats.LFSCloned, float64(stats.BytesShared)/(1<<20), stats.Rejected, stats.LeftToGit, stats.Hashed, stats.Proof)
	head, err := gitIn(abs, nil, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	newOID := strings.TrimSpace(string(head))
	hook := exec.Command("git", "hook", "run", "--ignore-missing", "post-checkout", "--", strings.Repeat("0", len(newOID)), newOID, "1")
	hook.Dir, hook.Stdout, hook.Stderr = abs, w, w
	if err := hook.Run(); err != nil {
		return fmt.Errorf("post-checkout hook failed: %w", err)
	}
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
	path    string
	entry   treeEntry
	srcAbs  string      // reflink source
	lfs     *lfsPointer // hydrated LFS content expected in the clone
	donor   *indexEntry // donor index entry proving the clone; nil means proof by hashing
	pointer []byte      // pointer-only LFS checkout: the exact bytes expected in the clone
}

type lfsPointer struct {
	oid  string
	size int64
}

type clonedFile struct {
	job  cowJob
	stat statData
	size int64
}

func (g *Git) cowMaterialize(target string, opts CowOptions, w io.Writer) (CowStats, error) {
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
	paths := make([]string, 0, len(targetTree))
	for p := range targetTree {
		if !safeRelPath(p) {
			return stats, fmt.Errorf("refusing unsafe path %q in the target tree", p)
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)

	source, err := g.pickCowSource(target, targetCommit)
	if err != nil {
		return stats, err
	}

	// Without --verify a clone is proven by the donor worktree's index. If that index
	// cannot serve as proof, every clone is proven by hashing instead.
	var donor *donorIndex
	stats.Proof = "hash"
	if !opts.Verify {
		if donor, err = donorIndexFor(source); err != nil {
			_, _ = fmt.Fprintf(w, "[gwq] warning: %v; proving clones by hashing\n", err)
		} else {
			stats.Proof = "donor index"
		}
	}

	// Let Git validate the complete tree and check out attributes with normal
	// mode and conversion semantics. Never write raw tree paths ourselves.
	if _, err := gitIn(target, nil, "read-tree", targetCommit); err != nil {
		return stats, err
	}
	var attrPaths bytes.Buffer
	for _, p := range paths {
		if filepath.Base(p) == ".gitattributes" {
			attrPaths.WriteString(p)
			attrPaths.WriteByte(0)
		}
	}
	if attrPaths.Len() > 0 {
		if _, err := gitIn(target, attrPaths.Bytes(), "checkout-index", "--force", "-z", "--stdin"); err != nil {
			return stats, err
		}
	}

	var (
		sourceTree map[string]treeEntry
		dirty      pathSet
	)
	if donor == nil {
		if sourceTree, err = lsTree(source, "HEAD"); err != nil {
			return stats, err
		}
		if dirty, err = sourceUnsafePaths(source); err != nil {
			return stats, err
		}
	}

	// Candidate files: identical blob and mode as the target tree, regular file, and
	// clean in the donor (by its index stat data, or by status when hashing).
	var cands []string
	proven := map[string]*indexEntry{} // candidates whose donor file is stat-clean
	for _, p := range paths {
		e := targetTree[p]
		if e.mode != "100644" && e.mode != "100755" {
			continue
		}
		stats.Candidates++
		if filepath.Base(p) == ".gitattributes" {
			continue
		}
		var de *indexEntry
		if donor != nil {
			mode, _ := parseMode(e.mode)
			oid, ok := oidFromHex(e.sha)
			if de = donor.entries[p]; !ok || de == nil || de.mode != mode || de.oid != oid {
				continue
			}
		} else if s, ok := sourceTree[p]; !ok || s != e || dirty.has(p) {
			continue
		}
		fi, err := os.Lstat(filepath.Join(source, p))
		if err != nil || !fi.Mode().IsRegular() || (fi.Mode()&0o111 != 0) != (e.mode == "100755") {
			continue
		}
		cands = append(cands, p)
		if donor != nil && donor.cleanAgainst(de, fi) {
			proven[p] = de
		}
	}

	attrs, err := checkAttrs(target, cands)
	if err != nil {
		return stats, err
	}
	var donorAttrs map[string]map[string]string
	if donor != nil {
		// The donor's bytes only equal the blob if the donor itself applied no
		// conversion; stat-clean says nothing about content that Git converted.
		provenPaths := make([]string, 0, len(proven))
		for p := range proven {
			provenPaths = append(provenPaths, p)
		}
		if donorAttrs, err = checkAttrs(source, provenPaths); err != nil {
			return stats, err
		}
	}

	policy, err := g.lfsPolicy(target)
	if err != nil {
		return stats, err
	}
	var lfsOIDs []string
	for _, p := range cands {
		if a := attrs[p]; plainConversion(a) && a["filter"] == "lfs" && (policy.store != "" || policy.pointerOnly) {
			lfsOIDs = append(lfsOIDs, targetTree[p].sha)
		}
	}
	blobs, err := catSmallBlobs(target, lfsOIDs, 1024)
	if err != nil {
		return stats, err
	}

	var jobs []cowJob
	for _, p := range cands {
		a := attrs[p]
		if !plainConversion(a) {
			continue
		}
		j := cowJob{path: p, entry: targetTree[p], srcAbs: filepath.Join(source, p)}
		de := proven[p]
		switch a["filter"] {
		case "unspecified":
			if a["text"] != "unspecified" && a["text"] != "unset" {
				continue
			}
			if donor != nil {
				if de == nil || !plainText(donorAttrs[p]) {
					continue // not proven: left to git
				}
				j.donor = de
			}
			jobs = append(jobs, j)
		case "lfs":
			if policy.store == "" && !policy.pointerOnly {
				continue // respect LFS checkout settings; let Git run the filter
			}
			blob := blobs[targetTree[p].sha]
			ptr, ok := parseLFSPointer(blob)
			if !ok {
				continue
			}
			if de != nil && donorAttrs[p]["filter"] == "lfs" && plainConversion(donorAttrs[p]) {
				// The donor file is usable only if its size is what the target's LFS
				// policy produces; hydrated and pointer-only checkouts differ in size.
				switch {
				case policy.pointerOnly && int64(de.stat.size) == int64(len(blob)):
					j.donor, j.pointer = de, blob
					jobs = append(jobs, j)
					continue
				case !policy.pointerOnly && ptr.size != int64(len(blob)) && int64(de.stat.size) == ptr.size:
					j.donor, j.lfs = de, ptr
					jobs = append(jobs, j)
					continue
				}
			}
			if policy.store == "" {
				continue
			}
			obj := filepath.Join(policy.store, ptr.oid[:2], ptr.oid[2:4], ptr.oid)
			if fi, err := os.Stat(obj); err != nil || !fi.Mode().IsRegular() || fi.Size() != ptr.size {
				continue // missing or wrong-sized object: let Git/LFS handle it
			}
			j.srcAbs, j.lfs = obj, ptr
			jobs = append(jobs, j)
		}
	}

	// Clone and prove in parallel.
	var (
		mu          sync.Mutex
		cloned      = map[string]clonedFile{}
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
			perm := os.FileMode(0o666)
			if j.entry.mode == "100755" {
				perm = 0o777
			}
			fi, err := reflinkFile(j.srcAbs, dst, perm)
			if err != nil {
				if errors.Is(err, errNoReflink) {
					unsupported.Store(true)
				}
				return
			}
			sd, ok := statDataOf(fi)
			var size int64
			if ok {
				size, ok = proveClone(donor, j, dst, fi)
			}
			mu.Lock()
			defer mu.Unlock()
			if !ok {
				_ = os.Remove(dst)
				stats.Rejected++
				stats.RejectedPaths = append(stats.RejectedPaths, j.path)
				return
			}
			cloned[j.path] = clonedFile{job: j, stat: sd, size: size}
		}()
	}
	wg.Wait()
	if unsupported.Load() {
		return stats, cowUnsupported("filesystem does not support reflinks between %s and %s", source, target)
	}
	sort.Strings(stats.RejectedPaths)

	// Write the index with the new files' own stat data, so Git trusts it without
	// hashing anything. Entries not cloned carry zero stat data until Git checks
	// them out below.
	var maxMtime uint32
	entries := make([]indexEntry, 0, len(paths))
	var leftover bytes.Buffer
	for _, p := range paths {
		e := targetTree[p]
		mode, err := parseMode(e.mode)
		if err != nil {
			return stats, fmt.Errorf("bad mode %q for %s", e.mode, p)
		}
		ie := indexEntry{path: p, mode: mode}
		var ok bool
		if ie.oid, ok = oidFromHex(e.sha); !ok {
			return stats, fmt.Errorf("bad object id %q for %s", e.sha, p)
		}
		if c, ok := cloned[p]; ok {
			ie.stat = c.stat
			maxMtime = max(maxMtime, c.stat.mtimeSec)
			stats.Cloned++
			stats.BytesShared += c.size
			if c.job.lfs != nil {
				stats.LFSCloned++
			}
			if c.job.donor == nil {
				stats.Hashed++
			}
		} else {
			leftover.WriteString(p)
			leftover.WriteByte(0)
		}
		entries = append(entries, ie)
	}
	stats.LeftToGit = len(entries) - stats.Cloned
	if stats.Cloned > 0 {
		// A file modified in the same second as the index is written could keep its
		// stat data; let the index be strictly newer than every clone.
		if d := time.Until(time.Unix(int64(maxMtime)+1, 0).Add(20 * time.Millisecond)); d > 0 {
			time.Sleep(d)
		}
	}
	idxPath, err := gitIn(target, nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return stats, err
	}
	if err := writeIndex(strings.TrimSpace(string(idxPath)), entries); err != nil {
		return stats, fmt.Errorf("failed to write index: %w", err)
	}
	if leftover.Len() > 0 {
		if _, err := gitIn(target, leftover.Bytes(), "checkout-index", "-f", "-u", "-z", "--stdin"); err != nil {
			return stats, fmt.Errorf("failed to check out remaining files: %w", err)
		}
	}
	return stats, nil
}

// plainConversion reports whether no checkout conversion other than a filter or
// text attribute applies.
func plainConversion(a map[string]string) bool {
	return a["eol"] == "unspecified" && a["ident"] == "unspecified" && a["working-tree-encoding"] == "unspecified" && a["crlf"] == "unspecified"
}

// plainText reports whether a path is checked out byte for byte.
func plainText(a map[string]string) bool {
	return plainConversion(a) && a["filter"] == "unspecified" && (a["text"] == "unspecified" || a["text"] == "unset")
}

// donorIndexFor loads the donor worktree's index as proof of clean files. An error
// means the index cannot serve as proof; callers prove clones by hashing instead.
func donorIndexFor(source string) (*donorIndex, error) {
	if v, _ := gitIn(source, nil, "config", "--get", "core.autocrlf"); strings.TrimSpace(string(v)) != "" && strings.TrimSpace(string(v)) != "false" {
		return nil, indexUnusable("core.autocrlf is set in the donor")
	}
	path, err := gitIn(source, nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return nil, indexUnusable("%v", err)
	}
	return readDonorIndex(strings.TrimSpace(string(path)))
}

// proveClone decides whether the clone at dst equals the object the target tree
// expects, and returns its size. fi is the stat of dst taken right after cloning.
func proveClone(donor *donorIndex, j cowJob, dst string, fi os.FileInfo) (int64, bool) {
	if j.donor == nil {
		return verifyClone(dst, j)
	}
	// The donor file must be unchanged since before the clone started.
	cur, err := os.Lstat(j.srcAbs)
	if err != nil || !donor.cleanAgainst(j.donor, cur) {
		return 0, false
	}
	if !fi.Mode().IsRegular() || uint32(fi.Size()) != j.donor.stat.size {
		return 0, false
	}
	if j.pointer != nil {
		// A hydrated file can share the pointer's size; compare the actual bytes.
		got, err := os.ReadFile(dst)
		if err != nil || !bytes.Equal(got, j.pointer) {
			return 0, false
		}
	}
	return fi.Size(), true
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

// lfsPolicy describes how Git/LFS will check out LFS files in a worktree.
type lfsPolicy struct {
	store       string // hydrate from this object directory
	pointerOnly bool   // every LFS file is checked out as its pointer
}

// lfsPolicy returns the LFS checkout policy of target. The zero value means
// checkout settings require Git/LFS to decide for each file.
func (g *Git) lfsPolicy(target string) (lfsPolicy, error) {
	if v := os.Getenv("GIT_LFS_SKIP_SMUDGE"); v != "" {
		skip, err := strconv.ParseBool(v)
		if err != nil || skip {
			return lfsPolicy{pointerOnly: err == nil}, nil
		}
	}
	// Git uses the process filter when it is configured, else the smudge filter.
	process, _ := gitIn(target, nil, "config", "--get", "filter.lfs.process")
	smudge, _ := gitIn(target, nil, "config", "--get", "filter.lfs.smudge")
	processSkip, smudgeSkip := strings.Contains(string(process), "--skip"), strings.Contains(string(smudge), "--skip")
	if processSkip || smudgeSkip {
		hasProcess := strings.TrimSpace(string(process)) != ""
		return lfsPolicy{pointerOnly: hasProcess && processSkip || !hasProcess && smudgeSkip}, nil
	}
	// git-lfs resolves .lfsconfig (including HEAD when it is not checked out)
	// and lfs.storage itself. Avoid duplicating its configuration precedence.
	env, err := gitIn(target, nil, "lfs", "env")
	if err != nil {
		return lfsPolicy{}, nil // missing git-lfs: ordinary Git checkout owns the behavior
	}
	var store string
	for _, line := range strings.Split(string(env), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "LocalMediaDir":
			store = value
		case "FetchInclude", "FetchExclude":
			if value != "" {
				return lfsPolicy{}, nil
			}
		}
	}
	return lfsPolicy{store: store}, nil
}

// catSmallBlobs reads blobs of at most limit bytes with one git process. Larger or
// missing blobs are absent from the result.
func catSmallBlobs(dir string, oids []string, limit int64) (map[string][]byte, error) {
	res := map[string][]byte{}
	if len(oids) == 0 {
		return res, nil
	}
	cmd := exec.Command("git", "cat-file", "--batch")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	var in bytes.Buffer
	for _, o := range oids {
		in.WriteString(o)
		in.WriteByte('\n')
	}
	cmd.Stdin = &in
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	r := bufio.NewReader(out)
	var readErr error
	for range oids {
		line, err := r.ReadString('\n')
		if err != nil {
			readErr = err
			break
		}
		f := strings.Fields(line)
		if len(f) != 3 { // "<oid> missing"
			continue
		}
		size, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil {
			readErr = err
			break
		}
		if size > limit || f[1] != "blob" {
			if _, err := io.CopyN(io.Discard, r, size+1); err != nil {
				readErr = err
				break
			}
			continue
		}
		buf := make([]byte, size+1) // content plus the trailing newline
		if _, err := io.ReadFull(r, buf); err != nil {
			readErr = err
			break
		}
		res[f[0]] = buf[:size]
	}
	_, _ = io.Copy(io.Discard, r)
	if err := cmd.Wait(); err != nil || readErr != nil {
		return nil, fmt.Errorf("git cat-file --batch: %v %v %s", err, readErr, strings.TrimSpace(stderr.String()))
	}
	return res, nil
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
		"filter", "text", "eol", "ident", "working-tree-encoding", "crlf")
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
