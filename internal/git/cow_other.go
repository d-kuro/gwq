//go:build !linux

package git

import (
	"errors"
	"os"
)

var errNoReflink = errors.New("reflink not supported")

func reflinkFile(src, dst string, perm os.FileMode) error { return errNoReflink }
