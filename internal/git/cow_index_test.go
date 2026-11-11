package git

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testIndexBytes(version uint32, paths ...string) []byte {
	var buf bytes.Buffer
	buf.WriteString("DIRC")
	_ = binary.Write(&buf, binary.BigEndian, [2]uint32{version, uint32(len(paths))})
	for _, p := range paths {
		_ = binary.Write(&buf, binary.BigEndian, [10]uint32{1, 2, 3, 4, 5, 6, 0o100644, 7, 8, 9})
		buf.Write(make([]byte, 20))
		_ = binary.Write(&buf, binary.BigEndian, uint16(len(p)))
		buf.WriteString(p)
		buf.Write(make([]byte, 8-(indexFixedSize+len(p))%8))
	}
	sum := sha1.Sum(buf.Bytes())
	return append(buf.Bytes(), sum[:]...)
}

func TestIndexRoundTrip(t *testing.T) {
	var entries []indexEntry
	for i := 1; i <= 20; i++ { // every padding length
		e := indexEntry{path: strings.Repeat("p", i), mode: 0o100755, stat: statData{1, 2, 3, 4, 5, 6, 7, 8, uint32(i)}}
		e.oid[0] = byte(i)
		entries = append(entries, e)
	}
	path := filepath.Join(t.TempDir(), "index")
	if err := writeIndex(path, entries); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("index.lock left behind: %v", err)
	}
	d, err := readDonorIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.entries) != len(entries) {
		t.Fatalf("read %d entries, wrote %d", len(d.entries), len(entries))
	}
	for _, e := range entries {
		if got := d.entries[e.path]; got == nil || *got != e {
			t.Fatalf("%s: read %+v, wrote %+v", e.path, got, e)
		}
	}
	// An existing index.lock means another Git process owns the index.
	if err := os.WriteFile(path+".lock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeIndex(path, entries); err == nil {
		t.Fatal("writeIndex ignored an existing index.lock")
	}
}

func TestIndexRejectsUnsafePaths(t *testing.T) {
	for _, p := range []string{"a/../b", "../b", "/etc/passwd", "a//b", "./a", ".git/hooks/post-checkout", "dir/.GIT/config", "a/."} {
		if _, err := parseIndex(testIndexBytes(2, p)); !errors.Is(err, errIndexUnusable) {
			t.Errorf("parseIndex accepted %q: %v", p, err)
		}
		if err := writeIndex(filepath.Join(t.TempDir(), "index"), []indexEntry{{path: p}}); err == nil {
			t.Errorf("writeIndex accepted %q", p)
		}
	}
	if _, err := parseIndex(testIndexBytes(2, "ok/path")); err != nil {
		t.Fatal(err)
	}
}

func TestIndexRejectsUnusableIndexes(t *testing.T) {
	good := testIndexBytes(2, "a")
	corrupt := bytes.Clone(good)
	corrupt[20] ^= 1 // flips a stat byte without fixing the checksum
	trailer := func(version uint32, ext string) []byte {
		b := testIndexBytes(version, "a")
		b = b[:len(b)-sha1.Size]
		b = append(b, ext...)
		b = binary.BigEndian.AppendUint32(b, 0)
		sum := sha1.Sum(b)
		return append(b, sum[:]...)
	}
	for name, data := range map[string][]byte{
		"checksum":    corrupt,
		"truncated":   good[:10],
		"version 4":   testIndexBytes(4, "a"),
		"split index": trailer(2, "link"),
		"sparse":      trailer(2, "sdir"),
		"required":    trailer(2, "zzzz"),
	} {
		if _, err := parseIndex(data); !errors.Is(err, errIndexUnusable) {
			t.Errorf("%s: parseIndex = %v", name, err)
		}
	}
	if _, err := parseIndex(trailer(2, "TREE")); err != nil {
		t.Errorf("optional extension rejected: %v", err)
	}
}
