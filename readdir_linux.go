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

			// Convert d_type to fs.FileMode
			var mode fs.FileMode
			switch dirent.Type {
			case syscall.DT_REG:
				mode = 0
			case syscall.DT_DIR:
				mode = fs.ModeDir
			case syscall.DT_LNK:
				mode = fs.ModeSymlink
			case syscall.DT_BLK:
				mode = fs.ModeDevice
			case syscall.DT_CHR:
				mode = fs.ModeDevice | fs.ModeCharDevice
			case syscall.DT_FIFO:
				mode = fs.ModeNamedPipe
			case syscall.DT_SOCK:
				mode = fs.ModeSocket
			case syscall.DT_UNKNOWN:
				// Fallback to lstat for filesystems that don't support d_type
				fullPath := dirPath + "/" + name
				info, err := os.Lstat(fullPath)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return nil, err
				}
				mode = info.Mode() & fs.ModeType
			default:
				// Unknown type, skip
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
