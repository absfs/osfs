//go:build linux && !appengine && !go1.24

package osfs

import (
	"os"
	"syscall"
)

// notDirError returns the error for dirPath when opening it with
// O_DIRECTORY failed with ENOTDIR, as os.ReadDir reports it before Go 1.24.
//
// Go 1.23's os.ReadDir opens without O_DIRECTORY, so when dirPath names
// something other than a directory its open succeeds and its read fails
// ENOTDIR as "readdirent", but when an element before the last is not a
// directory its open fails ENOTDIR as "open". notDirError tells the two
// apart the same way, by opening dirPath again without O_DIRECTORY,
// non-blocking so that a FIFO cannot hang it (os.ReadDir would block). That
// open also gives os's error when dirPath cannot be opened at all, such as
// a file without read permission.
func notDirError(dirPath string) error {
	fd, err := openRetry(dirPath, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NONBLOCK|syscall.O_NOCTTY)
	if err != nil {
		return &os.PathError{Op: "open", Path: dirPath, Err: err}
	}
	syscall.Close(fd)
	return &os.PathError{Op: "readdirent", Path: dirPath, Err: syscall.ENOTDIR}
}
