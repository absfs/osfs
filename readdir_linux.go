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
func readDirOptimized(dirPath string) ([]fs.DirEntry, error) {
	fd, err := syscall.Open(dirPath, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: dirPath, Err: err}
	}
	defer syscall.Close(fd)

	var entries []fs.DirEntry
	buf := make([]byte, bufSize)

	for {
		n, err := syscall.Getdents(fd, buf)
		if err != nil {
			return nil, &os.PathError{Op: "getdents", Path: dirPath, Err: err}
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
