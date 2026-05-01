package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
)

const (
	trustStoreFilename = "trusted_configs.json"
	trustStoreVersion  = 1
	trustStorePerm     = 0o600
	promptPreviewLimit = 4096 // Cap prompt file preview at 4 KiB to avoid flooding the terminal with a hostile or accidentally-large .gwq.toml, which could scroll the [y/N] prompt off-screen.
)

// trustPrompter asks the user whether to trust an untrusted local config.
type trustPrompter interface {
	PromptTrust(absPath string, content []byte) (bool, error)
}

// TrustStore manages trusted local `.gwq.toml` files keyed by (absolute path, sha256).
// Zero value is useful: an empty, in-memory-only store.
type TrustStore struct {
	path    string
	entries []trustEntry
}

type trustEntry struct {
	Path      string    `json:"path"`
	SHA256    string    `json:"sha256"`
	TrustedAt time.Time `json:"trusted_at"`
}

type trustFile struct {
	Version int          `json:"version"`
	Entries []trustEntry `json:"entries"`
}

// LoadTrustStore reads the trust store from disk.
// Missing file → empty store, not an error.
// Malformed JSON or unknown version → empty store (non-fatal; startup must not be blocked).
func LoadTrustStore(path string) (*TrustStore, error) {
	s := &TrustStore{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, fmt.Errorf("read trust store %s: %w", path, err)
	}
	if len(data) == 0 {
		return s, nil
	}

	var tf trustFile
	if err := json.Unmarshal(data, &tf); err != nil {
		fmt.Fprintf(os.Stderr, "gwq: ignoring malformed trust store %s: %v\n", path, err)
		return s, nil
	}
	if tf.Version != trustStoreVersion {
		fmt.Fprintf(os.Stderr, "gwq: ignoring trust store %s with unknown version %d\n", path, tf.Version)
		return s, nil
	}

	s.entries = tf.Entries
	return s, nil
}

// IsTrusted reports whether (absPath, sha256) is registered.
func (s *TrustStore) IsTrusted(absPath, sha256 string) bool {
	if s == nil {
		return false
	}
	for _, e := range s.entries {
		if e.Path == absPath && e.SHA256 == sha256 {
			return true
		}
	}
	return false
}

// Add registers (absPath, sha256) as trusted and persists to disk with atomic rename.
// Refuses to write if the trust store path is a symlink (anti-tampering).
// If s.path is empty, the store is in-memory only and Add just updates entries.
func (s *TrustStore) Add(absPath, sha256 string) error {
	// Update or insert entry (dedupe by path; sha256 may differ).
	now := time.Now().UTC()
	replaced := false
	for i := range s.entries {
		if s.entries[i].Path == absPath {
			s.entries[i].SHA256 = sha256
			s.entries[i].TrustedAt = now
			replaced = true
			break
		}
	}
	if !replaced {
		s.entries = append(s.entries, trustEntry{
			Path:      absPath,
			SHA256:    sha256,
			TrustedAt: now,
		})
	}

	if s.path == "" {
		return nil
	}
	return s.save()
}

// save serializes entries and writes them atomically.
func (s *TrustStore) save() error {
	if info, err := os.Lstat(s.path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write trust store: %s is a symlink", s.path)
		}
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create trust store dir: %w", err)
	}

	tf := trustFile{
		Version: trustStoreVersion,
		Entries: s.entries,
	}
	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trust store: %w", err)
	}

	// os.CreateTemp uses O_CREATE|O_EXCL with a random suffix and mode 0600,
	// so an attacker cannot pre-place a symlink at the temp path to redirect
	// the write through.
	f, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create trust store temp: %w", err)
	}
	tmpPath := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write trust store temp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close trust store temp: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename trust store: %w", err)
	}
	return nil
}

// defaultTrustStorePath returns the canonical trust store location under the gwq config dir.
func defaultTrustStorePath() string {
	return filepath.Join(getConfigDir(), trustStoreFilename)
}

// computeSHA256 returns the hex-encoded SHA-256 of data.
// Bytes-only variant (intentional: no path-taking helper, to avoid TOCTOU between hash and parse).
func computeSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// normalizeConfigPath returns the absolute, symlink-resolved path for a local config file.
// Returns an error if the resolved target is not a regular file (directory / FIFO / socket / device).
func normalizeConfigPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("abs path %s: %w", path, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks %s: %w", abs, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", resolved, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file (mode %s)", resolved, info.Mode())
	}
	return resolved, nil
}

// isStdinInteractive reports whether stdin is attached to a terminal.
func isStdinInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// stdioPrompter asks the user via stdin/stderr. Never writes to stdout —
// shell integration (`gwq cd`, completion) uses stdout as a protocol.
type stdioPrompter struct {
	in  io.Reader
	out io.Writer
}

func newStdioPrompter() *stdioPrompter {
	return &stdioPrompter{in: os.Stdin, out: os.Stderr}
}

// PromptTrust shows the file contents and asks for trust confirmation.
// Returns true only on "y" / "yes" (case-insensitive). EOF / error → false.
//
// The file content is untrusted input. Control bytes (C0/C1 escapes, DEL, CR)
// are escaped before output so ANSI/OSC sequences in a malicious .gwq.toml
// cannot clear the screen, move the cursor, or forge the [y/N] prompt.
func (p *stdioPrompter) PromptTrust(absPath string, content []byte) (bool, error) {
	preview := content
	truncated := false
	if len(preview) > promptPreviewLimit {
		preview = preview[:promptPreviewLimit]
		truncated = true
	}

	var b strings.Builder
	b.WriteString("\ngwq: untrusted local config detected:\n")
	fmt.Fprintf(&b, "  path: %s\n", sanitizeForTerminal(absPath))
	fmt.Fprintf(&b, "  size: %d bytes\n\n", len(content))
	b.WriteString("--- file contents ---\n")
	b.WriteString(strings.TrimRight(sanitizeForTerminal(string(preview)), "\n"))
	if truncated {
		fmt.Fprintf(&b, "\n... (truncated, showing first %d of %d bytes)", promptPreviewLimit, len(content))
	}
	b.WriteString("\n--------------------\n\n")
	b.WriteString("Trust this file and load it? [y/N]: ")
	if _, err := io.WriteString(p.out, b.String()); err != nil {
		return false, fmt.Errorf("write prompt: %w", err)
	}

	var response string
	if _, err := fmt.Fscanln(p.in, &response); err != nil {
		return false, nil
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}

// sanitizeForTerminal replaces terminal control characters with a visible
// `\xHH` escape so untrusted input cannot inject ANSI/OSC sequences, overwrite
// output via carriage return, or otherwise forge the surrounding prompt. Tab
// and newline pass through unchanged since they are needed for normal layout.
// C1 controls (U+0080–U+009F) are also escaped because modern terminals still
// honor them when they arrive via UTF-8 (`0xC2 0x80`..`0xC2 0x9F`).
func sanitizeForTerminal(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			fmt.Fprintf(&b, "\\x%02x", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
