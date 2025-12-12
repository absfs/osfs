//go:build darwin

package osfs

import (
	"io/fs"
	"os"
)

// readDirOptimized uses os.ReadDir on Darwin which is optimized and uses
// getattrlistbulk syscall internally. This is 2x faster than syscall.Getdirentries
// because Darwin's getdirentries is simulated and slow.
func readDirOptimized(dirPath string) ([]fs.DirEntry, error) {
	return os.ReadDir(dirPath)
}
