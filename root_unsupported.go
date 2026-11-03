//go:build !(go1.25 && unix)

package osfs

import (
	"errors"
	"os"

	"github.com/absfs/absfs"
)

// openRoot reports that this build cannot confine a filesystem.
func openRoot(dir string) (absfs.SymlinkFileSystem, error) {
	return nil, &os.PathError{Op: "openroot", Path: dir, Err: errors.ErrUnsupported}
}
