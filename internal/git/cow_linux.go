//go:build linux

package git

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

var errNoReflink = errors.New("reflink not supported")

// reflinkFile clones src into a new file dst (FICLONE); it never hard links.
// It returns the stat of dst taken right after the clone.
func reflinkFile(src, dst string, perm os.FileMode) (os.FileInfo, error) {
	in, err := os.OpenFile(src, os.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	// Apply the caller's umask at creation, just like a normal Git checkout.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW, perm)
	if err != nil {
		return nil, err
	}
	if err := unix.IoctlFileClone(int(out.Fd()), int(in.Fd())); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		switch err {
		case unix.EOPNOTSUPP, unix.EXDEV, unix.EINVAL, unix.ENOTTY, unix.ENOSYS:
			return nil, errNoReflink
		}
		return nil, err
	}
	fi, err := out.Stat()
	if err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return nil, err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return nil, err
	}
	return fi, nil
}

// statDataOf extracts the index stat fields from fi, which must come from os.Lstat,
// os.Stat or File.Stat.
func statDataOf(fi os.FileInfo) (statData, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return statData{}, false
	}
	return statData{
		ctimeSec: uint32(st.Ctim.Sec), ctimeNsec: uint32(st.Ctim.Nsec),
		mtimeSec: uint32(st.Mtim.Sec), mtimeNsec: uint32(st.Mtim.Nsec),
		dev: uint32(st.Dev), ino: uint32(st.Ino),
		uid: st.Uid, gid: st.Gid, size: uint32(st.Size),
	}, true
}
