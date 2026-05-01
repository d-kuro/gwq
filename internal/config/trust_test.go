package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func TestComputeSHA256(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "empty",
			data: []byte{},
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "hello",
			data: []byte("hello"),
			want: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
		{
			name: "multiline",
			data: []byte("line1\nline2\n"),
			want: "2751a3a2f303ad21752038085e2b8c5f98ecff61a2e4ebbd43506a941725be80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeSHA256(tt.data); got != tt.want {
				t.Errorf("computeSHA256(%q) = %s, want %s", tt.data, got, tt.want)
			}
		})
	}
}

func TestLoadTrustStore(t *testing.T) {
	validJSON := func(t *testing.T) []byte {
		t.Helper()
		data, err := json.Marshal(trustFile{
			Version: trustStoreVersion,
			Entries: []trustEntry{
				{Path: "/abs/path/.gwq.toml", SHA256: "abc"},
			},
		})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return data
	}

	tests := []struct {
		name       string
		writeBytes func(t *testing.T) ([]byte, bool) // (data, write)
		wantLen    int
	}{
		{
			name: "missing file",
			writeBytes: func(*testing.T) ([]byte, bool) {
				return nil, false
			},
			wantLen: 0,
		},
		{
			name: "empty file",
			writeBytes: func(*testing.T) ([]byte, bool) {
				return []byte{}, true
			},
			wantLen: 0,
		},
		{
			name:       "valid json",
			writeBytes: func(t *testing.T) ([]byte, bool) { return validJSON(t), true },
			wantLen:    1,
		},
		{
			name: "malformed json",
			writeBytes: func(*testing.T) ([]byte, bool) {
				return []byte("{not json"), true
			},
			wantLen: 0,
		},
		{
			name: "unknown version",
			writeBytes: func(t *testing.T) ([]byte, bool) {
				data, err := json.Marshal(trustFile{Version: 999, Entries: []trustEntry{{Path: "/x"}}})
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				return data, true
			},
			wantLen: 0,
		},
		{
			name: "top-level array (legacy shape)",
			writeBytes: func(*testing.T) ([]byte, bool) {
				return []byte(`[{"path":"/x","sha256":"y"}]`), true
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "trusted_configs.json")
			if data, write := tt.writeBytes(t); write {
				if err := os.WriteFile(path, data, trustStorePerm); err != nil {
					t.Fatalf("write: %v", err)
				}
			}
			s, err := LoadTrustStore(path)
			if err != nil {
				t.Fatalf("LoadTrustStore() error = %v", err)
			}
			if got := len(s.entries); got != tt.wantLen {
				t.Errorf("entries len = %d, want %d", got, tt.wantLen)
			}
			if s.path != path {
				t.Errorf("path = %s, want %s", s.path, path)
			}
		})
	}
}

func TestTrustStore_IsTrusted(t *testing.T) {
	s := &TrustStore{
		entries: []trustEntry{
			{Path: "/a", SHA256: "hash-a"},
			{Path: "/b", SHA256: "hash-b"},
		},
	}
	tests := []struct {
		name string
		path string
		sha  string
		want bool
	}{
		{"match a", "/a", "hash-a", true},
		{"match b", "/b", "hash-b", true},
		{"path mismatch", "/c", "hash-a", false},
		{"sha mismatch", "/a", "other", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.IsTrusted(tt.path, tt.sha); got != tt.want {
				t.Errorf("IsTrusted(%s, %s) = %v, want %v", tt.path, tt.sha, got, tt.want)
			}
		})
	}

	t.Run("empty store", func(t *testing.T) {
		var empty TrustStore
		if empty.IsTrusted("/a", "x") {
			t.Error("empty store should not trust anything")
		}
	})

	t.Run("nil store", func(t *testing.T) {
		var nilStore *TrustStore
		if nilStore.IsTrusted("/a", "x") {
			t.Error("nil store should not trust anything")
		}
	})
}

func TestTrustStore_Add_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trusted_configs.json")
	s, err := LoadTrustStore(path)
	if err != nil {
		t.Fatalf("LoadTrustStore: %v", err)
	}
	if err := s.Add("/abs/foo", "sha-foo"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Add("/abs/bar", "sha-bar"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != trustStorePerm {
		t.Errorf("perm = %o, want %o", perm, trustStorePerm)
	}

	reloaded, err := LoadTrustStore(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reloaded.entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(reloaded.entries))
	}
	if !reloaded.IsTrusted("/abs/foo", "sha-foo") {
		t.Error("expected /abs/foo to be trusted")
	}
	if !reloaded.IsTrusted("/abs/bar", "sha-bar") {
		t.Error("expected /abs/bar to be trusted")
	}
}

func TestTrustStore_Add_DedupeUpdatesHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trusted_configs.json")
	s, _ := LoadTrustStore(path)

	if err := s.Add("/abs/foo", "hash-v1"); err != nil {
		t.Fatalf("Add v1: %v", err)
	}
	if err := s.Add("/abs/foo", "hash-v2"); err != nil {
		t.Fatalf("Add v2: %v", err)
	}

	if len(s.entries) != 1 {
		t.Errorf("entries len = %d, want 1 (should dedupe by path)", len(s.entries))
	}
	if !s.IsTrusted("/abs/foo", "hash-v2") {
		t.Error("expected updated hash to be trusted")
	}
	if s.IsTrusted("/abs/foo", "hash-v1") {
		t.Error("old hash should no longer be trusted after update")
	}
}

func TestTrustStore_Add_InMemory(t *testing.T) {
	var s TrustStore // zero value: path == "" → in-memory only
	if err := s.Add("/abs/foo", "sha"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !s.IsTrusted("/abs/foo", "sha") {
		t.Error("expected in-memory add to work")
	}
}

func TestTrustStore_Add_AtomicNoTempLeftover(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trusted_configs.json")
	s, _ := LoadTrustStore(path)
	if err := s.Add("/abs/foo", "sha"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("stale temp file left behind: %s", e.Name())
		}
	}
}

// TestTrustStore_Add_TempSymlinkAttackResisted ensures that save() cannot be
// tricked into following a pre-placed symlink at the temp path and clobbering
// an arbitrary file. os.CreateTemp uses O_CREATE|O_EXCL with a random suffix,
// so decoys at predictable names are never opened for write.
func TestTrustStore_Add_TempSymlinkAttackResisted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test not reliable on Windows")
	}
	dir := t.TempDir()
	victimDir := t.TempDir()
	victim := filepath.Join(victimDir, "victim")
	if err := os.WriteFile(victim, []byte("DO-NOT-OVERWRITE"), 0o600); err != nil {
		t.Fatalf("write victim: %v", err)
	}

	// Plant decoy symlinks at names an attacker might guess.
	decoys := []string{
		"trusted_configs.json.tmp",
		"trusted_configs.json.tmp-",
		"trusted_configs.json.tmp-0",
	}
	for _, d := range decoys {
		if err := os.Symlink(victim, filepath.Join(dir, d)); err != nil {
			t.Fatalf("symlink %s: %v", d, err)
		}
	}

	s := &TrustStore{path: filepath.Join(dir, "trusted_configs.json")}
	if err := s.Add("/abs/foo", "sha"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	data, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("read victim: %v", err)
	}
	if string(data) != "DO-NOT-OVERWRITE" {
		t.Errorf("decoy symlink was followed: victim content = %q", data)
	}
	// Decoys themselves should remain as symlinks (untouched).
	for _, d := range decoys {
		info, err := os.Lstat(filepath.Join(dir, d))
		if err != nil {
			t.Errorf("decoy %s missing: %v", d, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("decoy %s is no longer a symlink", d)
		}
	}

	// Trust store itself was written correctly.
	reloaded, err := LoadTrustStore(s.path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reloaded.IsTrusted("/abs/foo", "sha") {
		t.Error("trust entry not persisted")
	}
}

func TestTrustStore_Add_SymlinkRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test not reliable on Windows")
	}
	dir := t.TempDir()
	realTarget := filepath.Join(dir, "evil_target.json")
	if err := os.WriteFile(realTarget, []byte("original"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	storePath := filepath.Join(dir, "trusted_configs.json")
	if err := os.Symlink(realTarget, storePath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	s := &TrustStore{path: storePath}
	err := s.Add("/abs/foo", "sha")
	if err == nil {
		t.Fatal("expected Add to reject symlink target")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink, got: %v", err)
	}

	data, readErr := os.ReadFile(realTarget)
	if readErr != nil {
		t.Fatalf("read target: %v", readErr)
	}
	if string(data) != "original" {
		t.Errorf("symlink target was modified: %q", data)
	}
}

func TestNormalizeConfigPath(t *testing.T) {
	dir := t.TempDir()
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(dir): %v", err)
	}

	regularFile := filepath.Join(dir, "regular.toml")
	if err := os.WriteFile(regularFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	symlinkFile := filepath.Join(dir, "link.toml")
	if err := os.Symlink(regularFile, symlinkFile); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	t.Run("regular file", func(t *testing.T) {
		got, err := normalizeConfigPath(regularFile)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		want := filepath.Join(realDir, "regular.toml")
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("symlink to regular file", func(t *testing.T) {
		got, err := normalizeConfigPath(symlinkFile)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		want := filepath.Join(realDir, "regular.toml")
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("directory rejected", func(t *testing.T) {
		_, err := normalizeConfigPath(subdir)
		if err == nil {
			t.Fatal("expected error for directory")
		}
		if !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("error should mention regular file: %v", err)
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		_, err := normalizeConfigPath(filepath.Join(dir, "does-not-exist"))
		if err == nil {
			t.Fatal("expected error for nonexistent path")
		}
	})

	if runtime.GOOS != "windows" {
		t.Run("FIFO rejected", func(t *testing.T) {
			fifoPath := filepath.Join(dir, "fifo")
			if err := syscall.Mkfifo(fifoPath, 0o600); err != nil {
				t.Skipf("Mkfifo unavailable: %v", err)
			}
			_, err := normalizeConfigPath(fifoPath)
			if err == nil {
				t.Fatal("expected FIFO to be rejected")
			}
			if !strings.Contains(err.Error(), "not a regular file") {
				t.Errorf("error should mention regular file: %v", err)
			}
		})
	}
}

func TestStdioPrompter_PromptTrust(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"y lowercase", "y\n", true},
		{"Y uppercase", "Y\n", true},
		{"yes lowercase", "yes\n", true},
		{"YES uppercase", "YES\n", true},
		{"n", "n\n", false},
		{"empty line", "\n", false},
		{"arbitrary string", "maybe\n", false},
		{"EOF", "", false},
		{"leading whitespace", "  y\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			p := &stdioPrompter{
				in:  strings.NewReader(tt.input),
				out: &out,
			}
			got, err := p.PromptTrust("/abs/foo/.gwq.toml", []byte("setup_commands = [\"x\"]"))
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != tt.want {
				t.Errorf("PromptTrust(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if !strings.Contains(out.String(), "/abs/foo/.gwq.toml") {
				t.Error("prompt should include path")
			}
			if !strings.Contains(out.String(), "setup_commands") {
				t.Error("prompt should include file contents")
			}
		})
	}

	t.Run("oversized content is truncated", func(t *testing.T) {
		var out strings.Builder
		p := &stdioPrompter{
			in:  strings.NewReader("n\n"),
			out: &out,
		}
		big := bytes.Repeat([]byte("A"), promptPreviewLimit*2)
		if _, err := p.PromptTrust("/abs/big.toml", big); err != nil {
			t.Fatalf("err: %v", err)
		}
		display := out.String()
		if !strings.Contains(display, "truncated") {
			t.Error("prompt should announce truncation for oversized content")
		}
		if strings.Count(display, "A") >= len(big) {
			t.Errorf("prompt should not contain full oversized payload (%d bytes)", len(big))
		}
	})

	t.Run("control bytes in content are escaped", func(t *testing.T) {
		var out strings.Builder
		p := &stdioPrompter{in: strings.NewReader("n\n"), out: &out}
		hostile := []byte("safe\n\x1b[2J\x1b]0;forged title\x07\rinjected\x7fend")
		if _, err := p.PromptTrust("/abs/evil.toml", hostile); err != nil {
			t.Fatalf("err: %v", err)
		}
		display := out.String()
		// Raw ESC (0x1b), BEL (0x07), CR (0x0d), DEL (0x7f) must not appear.
		for _, b := range []byte{0x1b, 0x07, 0x0d, 0x7f} {
			if strings.ContainsRune(display, rune(b)) {
				t.Errorf("raw control byte 0x%02x leaked into prompt output", b)
			}
		}
		// Escaped forms should be visible instead.
		for _, want := range []string{`\x1b`, `\x07`, `\x0d`, `\x7f`} {
			if !strings.Contains(display, want) {
				t.Errorf("expected %q in escaped output, got: %q", want, display)
			}
		}
		if !strings.Contains(display, "safe") || !strings.Contains(display, "injected") {
			t.Error("printable text should still appear around the escaped bytes")
		}
	})

	t.Run("control bytes in path are escaped", func(t *testing.T) {
		var out strings.Builder
		p := &stdioPrompter{in: strings.NewReader("n\n"), out: &out}
		if _, err := p.PromptTrust("/tmp/\x1b[2K/.gwq.toml", []byte("x")); err != nil {
			t.Fatalf("err: %v", err)
		}
		if strings.ContainsRune(out.String(), 0x1b) {
			t.Error("raw ESC in path leaked into prompt output")
		}
		if !strings.Contains(out.String(), `\x1b`) {
			t.Errorf("expected escaped ESC in path, got: %q", out.String())
		}
	})
}

func TestSanitizeForTerminal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain ascii", "hello world", "hello world"},
		{"tab and newline kept", "a\tb\nc", "a\tb\nc"},
		{"ansi escape", "pre\x1b[31mred\x1b[0mpost", `pre\x1b[31mred\x1b[0mpost`},
		{"carriage return", "a\rb", `a\x0db`},
		{"BEL and DEL", "x\x07y\x7fz", `x\x07y\x7fz`},
		{"unicode preserved", "日本語", "日本語"},
		{"C1 control via utf8", "a\u009bb", `a\x9bb`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeForTerminal(tt.in); got != tt.want {
				t.Errorf("sanitizeForTerminal(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDefaultTrustStorePath(t *testing.T) {
	t.Setenv("HOME", "/tmp/fake-home")
	got := defaultTrustStorePath()
	if !strings.HasSuffix(got, "trusted_configs.json") {
		t.Errorf("defaultTrustStorePath() = %s, want suffix trusted_configs.json", got)
	}
	if !strings.Contains(got, "gwq") {
		t.Errorf("defaultTrustStorePath() = %s, want to contain gwq", got)
	}
}

func FuzzLoadTrustStore(f *testing.F) {
	// Seed with a valid JSON document.
	validJSON, err := json.Marshal(trustFile{
		Version: trustStoreVersion,
		Entries: []trustEntry{{Path: "/x", SHA256: "abc"}},
	})
	if err != nil {
		f.Fatalf("seed marshal: %v", err)
	}
	f.Add(validJSON)
	f.Add([]byte(""))
	f.Add([]byte("{"))
	f.Add([]byte(`{"version":0,"entries":[]}`))
	f.Add([]byte(`{"version":1}`))
	f.Add([]byte(`{"version":1,"entries":[{}]}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(t.TempDir(), "trusted_configs.json")
		if err := os.WriteFile(path, data, trustStorePerm); err != nil {
			t.Fatalf("write: %v", err)
		}
		s, err := LoadTrustStore(path)
		if err != nil {
			// Load must not return an error for arbitrary bytes;
			// malformed input falls back to empty store.
			t.Fatalf("LoadTrustStore returned error for fuzz input: %v", err)
		}
		// Any loaded entry must be self-consistent: IsTrusted on the same
		// (path, sha) pair must return true. Breaking this would mean an
		// attacker could inject an entry that's stored but ineffective, or
		// vice versa.
		for _, e := range s.entries {
			if !s.IsTrusted(e.Path, e.SHA256) {
				t.Errorf("loaded entry not reported as trusted: %+v", e)
			}
		}
	})
}
