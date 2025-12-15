package osfs

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"time"

	"github.com/absfs/absfs"
)

// FileSystem implements absfs.SymlinkFileSystem using the host OS filesystem.
// All paths are Unix-style (forward slashes) for consistency with absfs conventions.
// On Windows, drive letters are represented as /c/, /d/, etc.
type FileSystem struct {
	cwd string // Unix-style path, e.g., "/c/Users/foo" on Windows, "/home/user" on Unix
}

// NewFS creates a new FileSystem rooted at the OS current working directory.
// The cwd is stored in Unix-style format.
func NewFS() (*FileSystem, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return &FileSystem{FromNative(dir)}, nil
}

// isDir checks if a native path is a directory.
func (fs *FileSystem) isDir(nativePath string) bool {
	info, err := os.Stat(nativePath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// toNativePath converts a Unix-style input path to a native OS path,
// resolving relative paths against the current working directory.
//
// For robustness, this function also detects unambiguous native paths
// (e.g., "C:\foo" on Windows) and handles them correctly. However,
// the recommended approach is to use FromNative() to convert native
// paths to Unix-style before passing them to osfs functions.
func (fs *FileSystem) toNativePath(name string) string {
	// Handle empty path
	if name == "" {
		return ToNative(fs.cwd)
	}

	// Safety check: detect if this is already a native path (e.g., C:\foo on Windows).
	// This handles the common case where callers pass os.TempDir() or os.MkdirTemp()
	// results directly without converting via FromNative() first.
	if isNativePath(name) {
		return name
	}

	// Check if path is absolute (starts with / or has drive letter)
	if !path.IsAbs(name) {
		// Relative path - join with cwd
		name = path.Join(fs.cwd, name)
	} else {
		// Absolute path - if it has no drive letter on Windows, use current drive
		if GetDrive(name) == "" && GetDrive(fs.cwd) != "" {
			// Path like "/foo" on Windows needs current drive
			name = SetDrive(name, GetDrive(fs.cwd))
		}
	}

	return ToNative(name)
}

// Chdir changes the current working directory.
// The name can be a Unix-style path (e.g., "/c/Users" on Windows).
func (fs *FileSystem) Chdir(name string) error {
	nativePath := fs.toNativePath(name)

	if !fs.isDir(nativePath) {
		return &os.PathError{Op: "chdir", Path: name, Err: errors.New("not a directory")}
	}

	// Store in Unix-style
	fs.cwd = FromNative(nativePath)
	return nil
}

// Getwd returns the current working directory in Unix-style format.
// On Windows, this includes the drive letter (e.g., "/c/Users/foo").
func (fs *FileSystem) Getwd() (dir string, err error) {
	return fs.cwd, nil
}

// TempDir returns the OS temp directory in Unix-style format.
func (fs *FileSystem) TempDir() string {
	return FromNative(os.TempDir())
}

// Open opens the named file for reading.
func (fs *FileSystem) Open(name string) (absfs.File, error) {
	nativePath := fs.toNativePath(name)
	f, err := os.Open(nativePath)
	if err != nil {
		return nil, err
	}
	return &File{fs, f}, nil
}

// Create creates or truncates the named file.
func (fs *FileSystem) Create(name string) (absfs.File, error) {
	nativePath := fs.toNativePath(name)
	f, err := os.Create(nativePath)
	if err != nil {
		return nil, err
	}
	return &File{fs, f}, nil
}

// Truncate changes the size of the named file.
func (fs *FileSystem) Truncate(name string, size int64) error {
	return os.Truncate(fs.toNativePath(name), size)
}

// Mkdir creates a directory with the specified permissions.
func (fs *FileSystem) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(fs.toNativePath(name), perm)
}

// MkdirAll creates a directory and all necessary parents.
func (fs *FileSystem) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(fs.toNativePath(name), perm)
}

// OpenFile opens a file with the specified flags and permissions.
func (fs *FileSystem) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
	nativePath := fs.toNativePath(name)
	f, err := os.OpenFile(nativePath, flag, perm)
	if err != nil {
		return nil, err
	}
	return &File{fs, f}, err
}

// Remove removes the named file or empty directory.
func (fs *FileSystem) Remove(name string) error {
	return os.Remove(fs.toNativePath(name))
}

// Rename renames (moves) a file or directory.
func (fs *FileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(fs.toNativePath(oldpath), fs.toNativePath(newpath))
}

// RemoveAll removes a file or directory and any children.
func (fs *FileSystem) RemoveAll(name string) error {
	return os.RemoveAll(fs.toNativePath(name))
}

// Stat returns file information for the named file.
func (fs *FileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(fs.toNativePath(name))
}

// Chmod changes the mode of the named file.
func (fs *FileSystem) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(fs.toNativePath(name), mode)
}

// Chtimes changes the access and modification times of the named file.
func (fs *FileSystem) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(fs.toNativePath(name), atime, mtime)
}

// Chown changes the owner and group ids of the named file.
func (fs *FileSystem) Chown(name string, uid, gid int) error {
	return os.Chown(fs.toNativePath(name), uid, gid)
}

// Lstat returns file information without following symlinks.
func (fs *FileSystem) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(fs.toNativePath(name))
}

// Lchown changes the owner and group ids of a symlink.
func (fs *FileSystem) Lchown(name string, uid, gid int) error {
	return os.Lchown(fs.toNativePath(name), uid, gid)
}

// Readlink returns the symlink target in Unix-style format.
func (fs *FileSystem) Readlink(name string) (string, error) {
	target, err := os.Readlink(fs.toNativePath(name))
	if err != nil {
		return "", err
	}
	return FromNative(target), nil
}

// Symlink creates a symbolic link.
// The oldname (target) is stored exactly as given - it can be relative or absolute.
// Only the newname (link location) is converted to a native path.
func (fs *FileSystem) Symlink(oldname, newname string) error {
	// Convert only the link location (newname) to native path.
	// The target (oldname) should be stored as-is to preserve relative paths.
	return os.Symlink(ToNative(oldname), fs.toNativePath(newname))
}

// ReadDir reads the named directory and returns a list of directory entries.
// Uses platform-specific optimizations for high-performance directory reading:
// - Linux: syscall.Getdents with 32KB buffer (vs default 8KB)
// - macOS: os.ReadDir (uses getattrlistbulk internally)
// - Windows: FindFirstFileEx with optimizations
// - Other platforms: os.ReadDir fallback
func (fs *FileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	nativePath := fs.toNativePath(name)
	return readDirOptimized(nativePath)
}

// ReadFile reads the named file and returns its contents.
// Uses os.ReadFile for optimized file reading with proper size pre-allocation.
func (fs *FileSystem) ReadFile(name string) ([]byte, error) {
	nativePath := fs.toNativePath(name)
	return os.ReadFile(nativePath)
}

// Sub returns an fs.FS corresponding to the subtree rooted at dir.
// The directory must exist and be a valid directory.
func (fs *FileSystem) Sub(dir string) (fs.FS, error) {
	return absfs.FilerToFS(fs, dir)
}
