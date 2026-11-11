//go:build !linux

package git

import (
	"errors"
	"os"
)

var errNoReflink = errors.New("reflink not supported")

func reflinkFile(src, dst string, perm os.FileMode) (os.FileInfo, error) { return nil, errNoReflink }

// statDataOf is unavailable here, so the donor index can never prove a clone.
func statDataOf(fi os.FileInfo) (statData, bool) { return statData{}, false }
