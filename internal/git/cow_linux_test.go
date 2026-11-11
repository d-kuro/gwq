//go:build linux

package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"
)

func TestReflinkRespectsUmask(t *testing.T) {
	if mask := os.Getenv("GWQ_TEST_REFLINK_UMASK"); mask != "" {
		n, err := strconv.ParseInt(mask, 8, 32)
		if err != nil {
			t.Fatal(err)
		}
		// This runs in a subprocess because umask is process-wide.
		unix.Umask(int(n))
		dir := t.TempDir()
		src := filepath.Join(dir, "source")
		if err := os.WriteFile(src, []byte("private checkout content"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, mode := range []os.FileMode{0o666, 0o777} {
			dst := filepath.Join(dir, mode.String())
			if _, err := reflinkFile(src, dst, mode); errors.Is(err, errNoReflink) {
				t.Skip("test filesystem does not support reflinks")
			} else if err != nil {
				t.Fatal(err)
			}
			fi, err := os.Stat(dst)
			if err != nil {
				t.Fatal(err)
			}
			if want := mode &^ os.FileMode(n); fi.Mode().Perm() != want {
				t.Errorf("mode = %o, want %o under umask %s", fi.Mode().Perm(), want, mask)
			}
		}
		return
	}
	for _, mask := range []string{"077", "000"} {
		t.Run(mask, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestReflinkRespectsUmask$", "-test.v")
			cmd.Env = append(os.Environ(), "GWQ_TEST_REFLINK_UMASK="+mask)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("umask subprocess: %v\n%s", err, out)
			}
			t.Log(string(out))
		})
	}
}
