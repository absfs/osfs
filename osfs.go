package osfs

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/absfs/absfs"
)

type FileSystem struct {
	cwd string
}

func NewFS() (*FileSystem, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return &FileSystem{dir}, nil
}

// Separator returns the virtual path separator, which is always '/'.
// This maintains consistency with the absfs contract that all virtual
// filesystems use Unix-style paths regardless of the host OS.
func (fs *FileSystem) Separator() uint8 {
	return '/'
}

// ListSeparator returns the virtual path list separator, which is always ':'.
// This maintains consistency with the absfs contract that all virtual
// filesystems use Unix-style paths regardless of the host OS.
func (fs *FileSystem) ListSeparator() uint8 {
	return ':'
}

func (fs *FileSystem) isDir(name string) bool {
	info, err := os.Stat(name)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func (fs *FileSystem) fixPath(name string) string {
	// Handle "/" specially on Windows - it refers to the root of the current drive
	if name == "/" && filepath.Separator == '\\' {
		// Extract volume from current working directory and use as root
		vol := filepath.VolumeName(fs.cwd)
		if vol != "" {
			return vol + string(filepath.Separator)
		}
	}

	if !filepath.IsAbs(name) {
		name = filepath.Join(fs.cwd, name)
	}
	return name
}

func (fs *FileSystem) Chdir(name string) error {
	name = fs.fixPath(name)
	if !fs.isDir(name) {
		return &os.PathError{Op: "chdir", Path: name, Err: errors.New("not a directory")}
	}
	fs.cwd = name
	return nil
}

// Getwd returns the current working directory with Unix-style path separators.
func (fs *FileSystem) Getwd() (dir string, err error) {
	return filepath.ToSlash(fs.cwd), nil
}

// TempDir returns the OS temp directory with Unix-style path separators.
func (fs *FileSystem) TempDir() string {
	return filepath.ToSlash(os.TempDir())
}

func (fs *FileSystem) Open(name string) (absfs.File, error) {

	f, err := os.Open(fs.fixPath(name))
	if err != nil {
		return nil, err
	}

	return &File{fs, f}, nil
}

func (fs *FileSystem) Create(name string) (absfs.File, error) {
	f, err := os.Create(fs.fixPath(name))
	if err != nil {
		return nil, err
	}

	return &File{fs, f}, nil
}

// func (fs *FileSystem) MkdirAll(name string, perm os.FileMode) error {
// 	return os.MkdirAll(fs.fixPath(name), perm)
// }

// func (fs *FileSystem) RemoveAll(name string) (err error) {
// 	return os.RemoveAll(fs.fixPath(name))
// }

func (fs *FileSystem) Truncate(name string, size int64) error {
	return os.Truncate(fs.fixPath(name), size)
}

func (fs *FileSystem) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(fs.fixPath(name), perm)
}

func (fs *FileSystem) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(fs.fixPath(name), perm)
}

func (fs *FileSystem) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
	f, err := os.OpenFile(fs.fixPath(name), flag, perm)
	if err != nil {
		return nil, err
	}

	return absfs.File(&File{fs, f}), err
}

// func (fs *FileSystem) Lstat(name string) (os.FileInfo, error) {
// 	return os.Lstat(fs.fixPath(name))
// }

func (fs *FileSystem) Remove(name string) error {
	return os.Remove(fs.fixPath(name))
}

func (fs *FileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(fs.fixPath(oldpath), fs.fixPath(newpath))
}

func (fs *FileSystem) RemoveAll(name string) error {
	return os.RemoveAll(fs.fixPath(name))
}

func (fs *FileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(fs.fixPath(name))
}

// Chmod changes the mode of the named file to mode.
func (fs *FileSystem) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(fs.fixPath(name), mode)
}

// Chtimes changes the access and modification times of the named file
func (fs *FileSystem) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(fs.fixPath(name), atime, mtime)
}

// Chown changes the owner and group ids of the named file
func (fs *FileSystem) Chown(name string, uid, gid int) error {
	return os.Chown(fs.fixPath(name), uid, gid)
}

func (fs *FileSystem) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(fs.fixPath(name))
}

// ess

func (fs *FileSystem) Lchown(name string, uid, gid int) error {
	return os.Lchown(fs.fixPath(name), uid, gid)
}

// Readlink returns the symlink target with Unix-style path separators.
func (fs *FileSystem) Readlink(name string) (string, error) {
	target, err := os.Readlink(fs.fixPath(name))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(target), nil
}

func (fs *FileSystem) Symlink(oldname, newname string) error {
	return os.Symlink(fs.fixPath(oldname), fs.fixPath(newname))
}

// Walk traverses the directory tree, calling fn with Unix-style paths.
func (fs *FileSystem) Walk(root string, fn func(string, os.FileInfo, error) error) error {
	return filepath.Walk(fs.fixPath(root), func(path string, info os.FileInfo, err error) error {
		return fn(filepath.ToSlash(path), info, err)
	})
}
