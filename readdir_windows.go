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
				typ:     entryType(dirPath, name, fd.FileAttributes, fd.Reserved0),
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
// dirPath. It does not double a trailing separator, and it keeps a bare
// drive designator drive-relative: "C:" lists the current directory on C:,
// as os.ReadDir("C:") does, so its pattern is "C:*", not the root's "C:\*".
func findPattern(dirPath string) string {
	return joinNative(dirPath, "*")
}

// entryType returns the fs.DirEntry type bits os.ReadDir would report for
// an entry with the given attributes and reparse tag (the find data's
// Reserved0). Plain files and directories are decided from the attributes.
// Reparse points (symlinks, junctions, dedup and cloud files) are typed by
// os.Lstat, which applies os.ReadDir's own reparse-tag rules, including the
// winsymlink GODEBUG setting: a directory symlink is ModeSymlink, not
// ModeDir. If Lstat fails (the entry was removed or replaced since it was
// listed, or cannot be opened) the type comes from the tag instead.
func entryType(dirPath, name string, attrs, tag uint32) fs.FileMode {
	if attrs&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		if info, err := os.Lstat(joinNative(dirPath, name)); err == nil {
			return info.Mode().Type()
		}
		return reparseType(attrs, tag)
	}
	if attrs&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return fs.ModeDir
	}
	return 0
}

// Reparse tags the syscall package does not name.
const (
	reparseTagDedup         = 0x80000013 // IO_REPARSE_TAG_DEDUP
	reparseTagAFUnix        = 0x80000023 // IO_REPARSE_TAG_AF_UNIX
	reparseTagNameSurrogate = 0x20000000 // IsReparseTagNameSurrogate bit
)

// reparseType returns the type bits the os package (Go 1.23 and later, with
// the default winsymlink setting) gives a reparse point with these
// attributes and tag. A name surrogate (symlink, junction) is never a
// directory; a symlink is ModeSymlink, an AF_UNIX socket ModeSocket, a dedup
// file a plain file or directory, and any other tag ModeIrregular.
func reparseType(attrs, tag uint32) fs.FileMode {
	var m fs.FileMode
	if tag&reparseTagNameSurrogate == 0 && attrs&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		m |= fs.ModeDir
	}
	switch tag {
	case syscall.IO_REPARSE_TAG_SYMLINK:
		m |= fs.ModeSymlink
	case reparseTagAFUnix:
		m |= fs.ModeSocket
	case reparseTagDedup:
		// Treated as the plain file or directory it stands for.
	default:
		m |= fs.ModeIrregular
	}
	return m
}

// joinNative appends name to dirPath with one separator between them. A
// bare drive designator ("C:") gets none, so "C:name" stays relative to the
// current directory on that drive, as it is for the os package.
func joinNative(dirPath, name string) string {
	n := len(dirPath)
	if (n > 0 && os.IsPathSeparator(dirPath[n-1])) || isDriveDesignator(dirPath) {
		return dirPath + name
	}
	return dirPath + `\` + name
}

// isDriveDesignator reports whether p is a drive letter and colon alone.
func isDriveDesignator(p string) bool {
	return len(p) == 2 && p[1] == ':'
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
