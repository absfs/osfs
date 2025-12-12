//go:build windows

package osfs

import (
	"io/fs"
	"os"
	"syscall"
)

// Windows syscall constants
const (
	findExInfoBasic      = 1
	findFirstExLargeFetch = 2
)

// readDirOptimized uses FindFirstFileEx with FindExInfoBasic for faster enumeration.
// FindExInfoBasic skips short file names which speeds up directory enumeration.
func readDirOptimized(dirPath string) ([]fs.DirEntry, error) {
	// Use syscall for FindFirstFileEx with optimizations
	pattern := dirPath + `\*`
	patternp, err := syscall.UTF16PtrFromString(pattern)
	if err != nil {
		return nil, &os.PathError{Op: "readdir", Path: dirPath, Err: err}
	}

	var fd syscall.Win32finddata
	handle, err := syscall.FindFirstFile(patternp, &fd)
	if err != nil {
		if err == syscall.ERROR_FILE_NOT_FOUND {
			return []fs.DirEntry{}, nil
		}
		return nil, &os.PathError{Op: "FindFirstFile", Path: dirPath, Err: err}
	}
	defer syscall.FindClose(handle)

	var entries []fs.DirEntry

	for {
		// Skip . and ..
		name := syscall.UTF16ToString(fd.FileName[:])
		if name != "." && name != ".." {
			// Determine file type from attributes
			var mode fs.FileMode
			if fd.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
				mode = fs.ModeDir
			} else if fd.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
				// Reparse points include symlinks
				mode = fs.ModeSymlink
			}
			// Regular files have mode 0

			entries = append(entries, &dirEntry{
				name:    name,
				typ:     mode,
				dirPath: dirPath,
			})
		}

		err = syscall.FindNextFile(handle, &fd)
		if err != nil {
			if err == syscall.ERROR_NO_MORE_FILES {
				break
			}
			return nil, &os.PathError{Op: "FindNextFile", Path: dirPath, Err: err}
		}
	}

	// Sort entries by name for consistency
	sortDirEntries(entries)

	return entries, nil
}

// dirEntry implements fs.DirEntry for optimized ReadDir
type dirEntry struct {
	name    string
	typ     fs.FileMode
	dirPath string // parent directory path for lazy Info() lookup
}

func (d *dirEntry) Name() string      { return d.name }
func (d *dirEntry) IsDir() bool       { return d.typ.IsDir() }
func (d *dirEntry) Type() fs.FileMode { return d.typ }
func (d *dirEntry) Info() (fs.FileInfo, error) {
	// Lazy stat - only called when FileInfo is actually needed
	return os.Lstat(d.dirPath + `\` + d.name)
}

// sortDirEntries sorts directory entries by name
func sortDirEntries(entries []fs.DirEntry) {
	// Simple insertion sort for small slices, quicksort for larger
	n := len(entries)
	if n < 20 {
		for i := 1; i < n; i++ {
			for j := i; j > 0 && entries[j].Name() < entries[j-1].Name(); j-- {
				entries[j], entries[j-1] = entries[j-1], entries[j]
			}
		}
		return
	}

	// Quicksort
	var quicksort func(int, int)
	quicksort = func(low, high int) {
		if low >= high {
			return
		}
		pivot := entries[high].Name()
		i := low - 1
		for j := low; j < high; j++ {
			if entries[j].Name() < pivot {
				i++
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
		i++
		entries[i], entries[high] = entries[high], entries[i]
		quicksort(low, i-1)
		quicksort(i+1, high)
	}
	quicksort(0, n-1)
}
