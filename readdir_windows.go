//go:build windows

package osfs

import (
	"io/fs"
	"os"
	"syscall"
)

// readDirOptimized lists dirPath with FindFirstFile/FindNextFile, which
// returns each entry's name and attributes in one call per entry.
//
// Its errors are os.ReadDir's. FindFirstFile fails differently from
// os.ReadDir: ERROR_PATH_NOT_FOUND for a missing name where os reports
// ERROR_FILE_NOT_FOUND, and ERROR_DIRECTORY for a regular file or a dangling
// symlink, which does not match fs.ErrNotExist. So any enumeration failure
// is answered by os.ReadDir itself, which gives the same error (op, path and
// errno) a caller of os.ReadDir would see, or the listing if the failure was
// one os.ReadDir does not share. Failures are rare, so this costs nothing on
// the common path.
func readDirOptimized(dirPath string) ([]fs.DirEntry, error) {
	patternp, err := syscall.UTF16PtrFromString(findPattern(dirPath))
	if err != nil {
		return os.ReadDir(dirPath)
	}

	var fd syscall.Win32finddata
	handle, err := syscall.FindFirstFile(patternp, &fd)
	if err != nil {
		return os.ReadDir(dirPath)
	}
	defer syscall.FindClose(handle)

	var entries []fs.DirEntry
	for {
		name := syscall.UTF16ToString(fd.FileName[:])
		if name != "." && name != ".." {
			entries = append(entries, &dirEntry{
				name:    name,
				typ:     entryType(dirPath, name, fd.FileAttributes),
				dirPath: dirPath,
			})
		}

		err = syscall.FindNextFile(handle, &fd)
		if err == syscall.ERROR_NO_MORE_FILES {
			break
		}
		if err != nil {
			return os.ReadDir(dirPath)
		}
	}

	sortDirEntries(entries)
	if entries == nil {
		entries = []fs.DirEntry{}
	}
	return entries, nil
}

// findPattern returns the FindFirstFile pattern matching every entry of
// dirPath, without doubling a trailing separator ("C:\" gives "C:\*").
func findPattern(dirPath string) string {
	if n := len(dirPath); n > 0 && os.IsPathSeparator(dirPath[n-1]) {
		return dirPath + "*"
	}
	return dirPath + `\*`
}

// entryType returns the fs.DirEntry type bits os.ReadDir would report for
// an entry with the given attributes. Plain files and directories are
// decided from the attributes. Reparse points (symlinks, junctions, dedup
// and cloud files) are typed by os.Lstat, which applies the same reparse-tag
// rules os.ReadDir does: a directory symlink is ModeSymlink, not ModeDir.
func entryType(dirPath, name string, attrs uint32) fs.FileMode {
	if attrs&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		if info, err := os.Lstat(joinNative(dirPath, name)); err == nil {
			return info.Mode().Type()
		}
		// Gone since it was listed; report what the attributes say.
	}
	if attrs&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return fs.ModeDir
	}
	return 0
}

// joinNative appends name to dirPath with one separator between them.
func joinNative(dirPath, name string) string {
	if n := len(dirPath); n > 0 && os.IsPathSeparator(dirPath[n-1]) {
		return dirPath + name
	}
	return dirPath + `\` + name
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
	return os.Lstat(joinNative(d.dirPath, d.name))
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
