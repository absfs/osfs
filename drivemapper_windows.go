//go:build windows
// +build windows

package osfs

import (
	"os"
	"path/filepath"
	"time"

	"github.com/absfs/absfs"
)

// WindowsDriveMapper wraps an absfs.FileSystem and translates virtual-absolute
// paths to OS-absolute paths on Windows by prepending a drive letter.
//
// This is useful when you want Unix-style absolute paths like "/config/app.json"
// to map to Windows paths like "C:\config\app.json" for OS filesystem operations.
//
// Path translation rules:
//   - Virtual-absolute (starts with / or \) → Prepend drive letter
//   - OS-absolute (has drive letter or UNC) → Pass through unchanged
//   - Relative paths → Pass through unchanged
//
// Example:
//
//	fs, _ := osfs.NewFS()
//	mapped := osfs.NewWindowsDriveMapper(fs, "C:")
//
//	mapped.Create("/config/app.json")      // → C:\config\app.json
//	mapped.Open("C:\\Windows\\file.txt")   // → C:\Windows\file.txt (unchanged)
//	mapped.MkdirAll("/var/log", 0755)      // → C:\var\log
type WindowsDriveMapper struct {
	base  absfs.FileSystem
	drive string
}

// NewWindowsDriveMapper creates a new WindowsDriveMapper that wraps the given
// FileSystem and translates virtual-absolute paths to use the specified drive.
//
// If drive is empty, defaults to "C:". Drive should be in the format "C:" or "D:".
func NewWindowsDriveMapper(base absfs.FileSystem, drive string) absfs.FileSystem {
	if drive == "" {
		drive = "C:"
	}
	return &WindowsDriveMapper{
		base:  base,
		drive: drive,
	}
}

// translatePath converts virtual-absolute paths to OS-absolute paths.
// OS-absolute and relative paths pass through unchanged.
func (w *WindowsDriveMapper) translatePath(path string) string {
	// Already OS-absolute (has drive letter or UNC) - no translation needed
	if filepath.IsAbs(path) {
		return path
	}

	// Virtual-absolute (starts with / or \) - add drive letter
	if len(path) > 0 && (path[0] == '/' || path[0] == '\\') {
		return filepath.Join(w.drive+"\\", path)
	}

	// Relative path - no translation
	return path
}

// Implement all absfs.FileSystem interface methods with path translation

func (w *WindowsDriveMapper) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
	return w.base.OpenFile(w.translatePath(name), flag, perm)
}

func (w *WindowsDriveMapper) Mkdir(name string, perm os.FileMode) error {
	return w.base.Mkdir(w.translatePath(name), perm)
}

func (w *WindowsDriveMapper) Remove(name string) error {
	return w.base.Remove(w.translatePath(name))
}

func (w *WindowsDriveMapper) Rename(oldpath, newpath string) error {
	return w.base.Rename(w.translatePath(oldpath), w.translatePath(newpath))
}

func (w *WindowsDriveMapper) Stat(name string) (os.FileInfo, error) {
	return w.base.Stat(w.translatePath(name))
}

func (w *WindowsDriveMapper) Chmod(name string, mode os.FileMode) error {
	return w.base.Chmod(w.translatePath(name), mode)
}

func (w *WindowsDriveMapper) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return w.base.Chtimes(w.translatePath(name), atime, mtime)
}

func (w *WindowsDriveMapper) Chown(name string, uid, gid int) error {
	return w.base.Chown(w.translatePath(name), uid, gid)
}

// Extended FileSystem methods

func (w *WindowsDriveMapper) Open(name string) (absfs.File, error) {
	return w.base.Open(w.translatePath(name))
}

func (w *WindowsDriveMapper) Create(name string) (absfs.File, error) {
	return w.base.Create(w.translatePath(name))
}

func (w *WindowsDriveMapper) MkdirAll(path string, perm os.FileMode) error {
	return w.base.MkdirAll(w.translatePath(path), perm)
}

func (w *WindowsDriveMapper) RemoveAll(path string) error {
	return w.base.RemoveAll(w.translatePath(path))
}

func (w *WindowsDriveMapper) Truncate(name string, size int64) error {
	return w.base.Truncate(w.translatePath(name), size)
}

func (w *WindowsDriveMapper) Chdir(dir string) error {
	return w.base.Chdir(w.translatePath(dir))
}

// Pass-through methods (no path translation needed)

func (w *WindowsDriveMapper) Separator() uint8 {
	return w.base.Separator()
}

func (w *WindowsDriveMapper) ListSeparator() uint8 {
	return w.base.ListSeparator()
}

func (w *WindowsDriveMapper) Getwd() (dir string, err error) {
	return w.base.Getwd()
}

func (w *WindowsDriveMapper) TempDir() string {
	return w.base.TempDir()
}
