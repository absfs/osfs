//go:build linux && !appengine

package osfs

import (
	"bytes"
	"io/fs"
	"os"
	"syscall"
	"unsafe"
)

// bufSize is larger than the default 8KB used by os.ReadDir for better performance
const bufSize = 32 * 1024

// readDirOptimized uses syscall.Getdents directly with a larger buffer for improved performance.
// This avoids allocations and extracts d_type without calling lstat.
//
// Its errors are os.ReadDir's, op included: "open" when the name cannot be
// opened and "readdirent" when it cannot be read, including ENOTDIR for a
// name that exists but is not a directory.
func readDirOptimized(dirPath string) ([]fs.DirEntry, error) {
	fd, err := openDir(dirPath)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)

	var entries []fs.DirEntry
	buf := make([]byte, bufSize)

	for {
		n, err := ignoringEINTR(func() (int, error) { return syscall.Getdents(fd, buf) })
		if err != nil {
			return nil, &os.PathError{Op: "readdirent", Path: dirPath, Err: err}
		}
		if n == 0 {
			break
		}

		// Parse dirents from buffer
		bufp := 0
		for bufp < n {
			dirent := (*syscall.Dirent)(unsafe.Pointer(&buf[bufp]))
			bufp += int(dirent.Reclen)

			// Skip if inode is 0 (deleted file)
			if dirent.Ino == 0 {
				continue
			}

			// Extract name
			nameBuf := (*[256]byte)(unsafe.Pointer(&dirent.Name[0]))
			nameLen := bytes.IndexByte(nameBuf[:], 0)
			if nameLen < 0 {
				continue
			}

			name := string(nameBuf[:nameLen])
			if name == "." || name == ".." {
				continue
			}

			mode, ok, err := direntType(dirent.Type, dirPath, name)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}

			entries = append(entries, &dirEntry{
				name:    name,
				typ:     mode,
				dirPath: dirPath,
			})
		}
	}

	// Sort entries by name for consistency with os.ReadDir
	sortDirEntries(entries)
	if entries == nil {
		// Like os.ReadDir, never return a nil slice for an empty directory.
		entries = []fs.DirEntry{}
	}
	return entries, nil
}

// openDir opens dirPath for getdents. Like os.Open it sets O_CLOEXEC, so a
// process started while the directory is being read does not inherit the
// descriptor, and it retries an open interrupted by a signal. O_DIRECTORY
// keeps it from opening anything else; a FIFO would block.
//
// os.ReadDir opens without O_DIRECTORY, so when dirPath names something
// other than a directory its open succeeds and its read fails ENOTDIR as
// "readdirent", but when an element before the last is not a directory its
// open fails ENOTDIR as "open". An ENOTDIR here is told apart the same way,
// by opening dirPath again without O_DIRECTORY, non-blocking so a FIFO
// cannot hang it. That open also gives os's error when dirPath cannot be
// opened at all, such as a file without read permission. It runs only on
// this failure path.
func openDir(dirPath string) (int, error) {
	const flags = syscall.O_RDONLY | syscall.O_CLOEXEC
	fd, err := openRetry(dirPath, flags|syscall.O_DIRECTORY)
	if err == nil {
		return fd, nil
	}
	if err != syscall.ENOTDIR {
		return -1, &os.PathError{Op: "open", Path: dirPath, Err: err}
	}
	fd, err = openRetry(dirPath, flags|syscall.O_NONBLOCK|syscall.O_NOCTTY)
	if err != nil {
		return -1, &os.PathError{Op: "open", Path: dirPath, Err: err}
	}
	syscall.Close(fd)
	return -1, &os.PathError{Op: "readdirent", Path: dirPath, Err: syscall.ENOTDIR}
}

// openRetry opens path read-only with flags, retrying on EINTR as os.Open
// does.
func openRetry(path string, flags int) (int, error) {
	return ignoringEINTR(func() (int, error) { return syscall.Open(path, flags, 0) })
}

// ignoringEINTR calls fn until it fails with something other than EINTR, as
// the os package does around open and getdents.
func ignoringEINTR(fn func() (int, error)) (int, error) {
	for {
		n, err := fn()
		if err != syscall.EINTR {
			return n, err
		}
	}
}

// direntType converts a dirent's d_type to fs.DirEntry type bits. On
// filesystems that report DT_UNKNOWN it asks os.Lstat. ok is false for an
// entry to leave out: one removed since it was listed, or an unknown d_type.
func direntType(dtype uint8, dirPath, name string) (mode fs.FileMode, ok bool, err error) {
	switch dtype {
	case syscall.DT_REG:
		return 0, true, nil
	case syscall.DT_DIR:
		return fs.ModeDir, true, nil
	case syscall.DT_LNK:
		return fs.ModeSymlink, true, nil
	case syscall.DT_BLK:
		return fs.ModeDevice, true, nil
	case syscall.DT_CHR:
		return fs.ModeDevice | fs.ModeCharDevice, true, nil
	case syscall.DT_FIFO:
		return fs.ModeNamedPipe, true, nil
	case syscall.DT_SOCK:
		return fs.ModeSocket, true, nil
	case syscall.DT_UNKNOWN:
		info, err := os.Lstat(dirPath + "/" + name)
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		return info.Mode() & fs.ModeType, true, nil
	}
	return 0, false, nil
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
	return os.Lstat(d.dirPath + "/" + d.name)
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
