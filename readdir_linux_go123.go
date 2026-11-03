//go:build linux && !appengine && !go1.24

package osfs

import (
	"os"
	"syscall"
)

// Linux values the syscall package does not export; they are the same on
// every architecture.
const (
	atFDCWD   = -0x64 // AT_FDCWD
	atEACCESS = 0x200 // AT_EACCESS: check with the effective IDs, as open does
	rOK       = 0x4   // R_OK
)

// notDirError returns the error for dirPath when opening it with
// O_DIRECTORY failed with ENOTDIR, as os.ReadDir reports it before Go 1.24.
//
// Go 1.23's os.ReadDir opens without O_DIRECTORY. When an element before the
// last is not a directory its open fails ENOTDIR as "open". When dirPath
// itself names something other than a directory its open usually succeeds
// and its read fails ENOTDIR as "readdirent"; the open fails instead for a
// file the caller may not read (EACCES) and for a socket (ENXIO).
//
// notDirError tells these apart with stat and access, never by opening
// dirPath: opening a FIFO, a tty or another device has side effects, such
// as releasing a writer blocked on the FIFO or resetting a serial board.
// So where os.ReadDir would block on a FIFO, or a device's open would fail,
// notDirError gives "readdirent" with ENOTDIR.
func notDirError(dirPath string) error {
	var st syscall.Stat_t
	if err := syscall.Stat(dirPath, &st); err != nil || st.Mode&syscall.S_IFMT == syscall.S_IFDIR {
		// An element before the last is not a directory, or dirPath
		// changed since the open.
		return &os.PathError{Op: "open", Path: dirPath, Err: syscall.ENOTDIR}
	}
	if st.Mode&syscall.S_IFMT == syscall.S_IFSOCK {
		return &os.PathError{Op: "open", Path: dirPath, Err: syscall.ENXIO}
	}
	if err := syscall.Faccessat(atFDCWD, dirPath, rOK, atEACCESS); err != nil {
		return &os.PathError{Op: "open", Path: dirPath, Err: err}
	}
	return &os.PathError{Op: "readdirent", Path: dirPath, Err: syscall.ENOTDIR}
}
