//go:build linux && !appengine && go1.24

package osfs

import (
	"os"
	"syscall"
)

// notDirError returns the error for dirPath when opening it with
// O_DIRECTORY failed with ENOTDIR. From Go 1.24 os.ReadDir opens with
// O_DIRECTORY too, so its error is the same open failure.
func notDirError(dirPath string) error {
	return &os.PathError{Op: "open", Path: dirPath, Err: syscall.ENOTDIR}
}
