//go:build linux

package git

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

var errNoReflink = errors.New("reflink not supported")

// reflinkFile clones src into a new file dst (FICLONE); it never hard links.
func reflinkFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	// Apply the caller's umask at creation, just like a normal Git checkout.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if err := unix.IoctlFileClone(int(out.Fd()), int(in.Fd())); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		switch err {
		case unix.EOPNOTSUPP, unix.EXDEV, unix.EINVAL, unix.ENOTTY, unix.ENOSYS:
			return errNoReflink
		}
		return err
	}
	return out.Close()
}
