//go:build !linux && !darwin && !windows

package osfs

import (
	"io/fs"
	"os"
)

// readDirOptimized falls back to os.ReadDir for platforms without specific optimizations
func readDirOptimized(dirPath string) ([]fs.DirEntry, error) {
	return os.ReadDir(dirPath)
}
