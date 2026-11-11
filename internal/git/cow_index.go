package git

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

// statData is the part of struct stat that Git records in an index entry. Like the
// index, it keeps only the low 32 bits of each value.
type statData struct {
	ctimeSec, ctimeNsec uint32
	mtimeSec, mtimeNsec uint32
	dev, ino            uint32
	uid, gid, size      uint32
}

// indexEntry is one stage-0 index entry.
type indexEntry struct {
	path string
	mode uint32
	oid  [20]byte
	stat statData
}

// donorIndex is a verified snapshot of the donor worktree's index.
type donorIndex struct {
	entries map[string]*indexEntry
	mtimeNs int64 // modification time of the index file, for racy-clean detection
}

const (
	indexHeaderSize   = 12
	indexFixedSize    = 62
	indexMaxSize      = 1 << 30
	indexFlagAssume   = 0x8000
	indexFlagExtended = 0x4000
	indexFlagStage    = 0x3000
	indexFlagNameMask = 0x0fff
	indexExtSkipWT    = 0x4000
	indexExtIntent    = 0x2000
)

var errIndexUnusable = errors.New("donor index cannot be used")

func indexUnusable(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errIndexUnusable, fmt.Sprintf(format, a...))
}

func parseMode(s string) (uint32, error) {
	n, err := strconv.ParseUint(s, 8, 32)
	return uint32(n), err
}

// readDonorIndex snapshots and parses the index file at path. The snapshot is
// retried until the file is stable, and its SHA-1 trailer must verify.
func readDonorIndex(path string) (*donorIndex, error) {
	var data []byte
	var mtimeNs int64
	stable := false
	for range 5 {
		before, err := os.Stat(path)
		if err != nil {
			return nil, indexUnusable("%v", err)
		}
		if !before.Mode().IsRegular() || before.Size() > indexMaxSize {
			return nil, indexUnusable("not a regular file of reasonable size")
		}
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, indexUnusable("%v", err)
		}
		after, err := os.Stat(path)
		if err != nil {
			return nil, indexUnusable("%v", err)
		}
		if os.SameFile(before, after) && before.ModTime().Equal(after.ModTime()) && before.Size() == after.Size() && int64(len(data)) == after.Size() {
			mtimeNs, stable = after.ModTime().UnixNano(), true
			break
		}
	}
	if !stable {
		return nil, indexUnusable("index keeps changing")
	}
	entries, err := parseIndex(data)
	if err != nil {
		return nil, err
	}
	m := make(map[string]*indexEntry, len(entries))
	for i := range entries {
		m[entries[i].path] = &entries[i]
	}
	return &donorIndex{entries: m, mtimeNs: mtimeNs}, nil
}

// parseIndex parses an index (version 2 or 3) with a valid checksum. It refuses
// anything that makes the donor's stat data an unreliable proof: other versions,
// split or sparse index, unmerged or skip-worktree entries, and unsafe paths.
// Entries marked assume-unchanged or intent-to-add are left out.
func parseIndex(data []byte) ([]indexEntry, error) {
	if len(data) < indexHeaderSize+sha1.Size {
		return nil, indexUnusable("truncated")
	}
	body, sum := data[:len(data)-sha1.Size], data[len(data)-sha1.Size:]
	if got := sha1.Sum(body); !bytes.Equal(got[:], sum) {
		return nil, indexUnusable("checksum mismatch")
	}
	if string(body[:4]) != "DIRC" {
		return nil, indexUnusable("bad signature")
	}
	version, count := binary.BigEndian.Uint32(body[4:8]), binary.BigEndian.Uint32(body[8:12])
	if version != 2 && version != 3 {
		return nil, indexUnusable("unsupported index version %d", version)
	}
	if uint64(count)*(indexFixedSize+2) > uint64(len(body)) {
		return nil, indexUnusable("entry count exceeds file size")
	}
	entries := make([]indexEntry, 0, count)
	off := indexHeaderSize
	for range count {
		if off+indexFixedSize > len(body) {
			return nil, indexUnusable("truncated entry")
		}
		b := body[off:]
		var e indexEntry
		e.stat = statData{
			ctimeSec: binary.BigEndian.Uint32(b[0:]), ctimeNsec: binary.BigEndian.Uint32(b[4:]),
			mtimeSec: binary.BigEndian.Uint32(b[8:]), mtimeNsec: binary.BigEndian.Uint32(b[12:]),
			dev: binary.BigEndian.Uint32(b[16:]), ino: binary.BigEndian.Uint32(b[20:]),
			uid: binary.BigEndian.Uint32(b[28:]), gid: binary.BigEndian.Uint32(b[32:]), size: binary.BigEndian.Uint32(b[36:]),
		}
		e.mode = binary.BigEndian.Uint32(b[24:])
		copy(e.oid[:], b[40:60])
		flags := binary.BigEndian.Uint16(b[60:])
		hlen := indexFixedSize
		var ext uint16
		if flags&indexFlagExtended != 0 {
			if version < 3 || off+indexFixedSize+2 > len(body) {
				return nil, indexUnusable("bad extended flags")
			}
			ext = binary.BigEndian.Uint16(b[62:])
			hlen += 2
		}
		if flags&indexFlagStage != 0 {
			return nil, indexUnusable("unmerged entries")
		}
		if ext&indexExtSkipWT != 0 {
			return nil, indexUnusable("skip-worktree entries (sparse checkout)")
		}
		nul := bytes.IndexByte(b[hlen:], 0)
		if nul <= 0 || off+hlen+nul > len(body) {
			return nil, indexUnusable("bad entry path")
		}
		e.path = string(b[hlen : hlen+nul])
		if !safeRelPath(e.path) {
			return nil, indexUnusable("unsafe path %q", e.path)
		}
		size := (hlen + nul + 8) &^ 7
		if off+size > len(body) {
			return nil, indexUnusable("truncated entry padding")
		}
		off += size
		if flags&indexFlagAssume == 0 && ext&indexExtIntent == 0 {
			entries = append(entries, e)
		}
	}
	for off < len(body) {
		if off+8 > len(body) {
			return nil, indexUnusable("truncated extension")
		}
		sig := string(body[off : off+4])
		size := uint64(binary.BigEndian.Uint32(body[off+4:]))
		if sig == "link" || sig == "sdir" {
			return nil, indexUnusable("%s extension (split or sparse index)", sig)
		}
		if sig[0] < 'A' || sig[0] > 'Z' {
			return nil, indexUnusable("required extension %q", sig)
		}
		if uint64(off)+8+size > uint64(len(body)) {
			return nil, indexUnusable("extension exceeds file size")
		}
		off += 8 + int(size)
	}
	return entries, nil
}

// writeIndex writes a version 2 index for entries, which must already be sorted by
// path, through index.lock and a rename. Fails if index.lock exists.
func writeIndex(path string, entries []indexEntry) (err error) {
	var buf bytes.Buffer
	buf.WriteString("DIRC")
	_ = binary.Write(&buf, binary.BigEndian, [2]uint32{2, uint32(len(entries))})
	for _, e := range entries {
		if !safeRelPath(e.path) {
			return fmt.Errorf("refusing to index unsafe path %q", e.path)
		}
		s := e.stat
		_ = binary.Write(&buf, binary.BigEndian, [10]uint32{s.ctimeSec, s.ctimeNsec, s.mtimeSec, s.mtimeNsec, s.dev, s.ino, e.mode, s.uid, s.gid, s.size})
		buf.Write(e.oid[:])
		_ = binary.Write(&buf, binary.BigEndian, uint16(min(len(e.path), indexFlagNameMask)))
		buf.WriteString(e.path)
		buf.Write(make([]byte, 8-(indexFixedSize+len(e.path))%8))
	}
	sum := sha1.Sum(buf.Bytes())
	buf.Write(sum[:])

	lock := path + ".lock"
	f, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(lock)
		}
	}()
	if _, err = io.Copy(f, &buf); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(lock, path)
}

// cleanAgainst reports whether fi, a fresh lstat of the donor file, still matches
// the donor's index entry exactly (every stat field except dev), and is not
// racily clean: its mtime must be strictly older than the index file's.
func (d *donorIndex) cleanAgainst(e *indexEntry, fi os.FileInfo) bool {
	if !fi.Mode().IsRegular() || fi.ModTime().UnixNano() >= d.mtimeNs {
		return false
	}
	sd, ok := statDataOf(fi)
	if !ok {
		return false
	}
	want := e.stat
	want.dev, sd.dev = 0, 0 // dev is not compared by default Git builds
	return sd == want && (fi.Mode()&0o111 != 0) == (e.mode&0o111 != 0)
}

func oidFromHex(s string) ([20]byte, bool) {
	var oid [20]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(oid) {
		return oid, false
	}
	copy(oid[:], b)
	return oid, true
}
