package osfs

import (
	"io/fs"
	"os"
)

type File struct {
	filer *FileSystem
	f     *os.File
}

// Name returns the file name in Unix-style format.
// On Windows, this includes the drive letter (e.g., "/c/Users/foo/file.txt").
func (f *File) Name() string {
	return FromNative(f.f.Name())
}

func (f *File) Read(p []byte) (int, error) {
	return f.f.Read(p)
}

func (f *File) ReadAt(b []byte, off int64) (n int, err error) {
	return f.f.ReadAt(b, off)
}

func (f *File) Write(p []byte) (int, error) {
	return f.f.Write(p)
}

func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	return f.f.WriteAt(b, off)
}

func (f *File) Close() error {
	return f.f.Close()
}

func (f *File) Seek(offset int64, whence int) (ret int64, err error) {
	return f.f.Seek(offset, whence)
}

func (f *File) Stat() (os.FileInfo, error) {
	return f.f.Stat()
}

func (f *File) Sync() error {
	return f.f.Sync()
}

func (f *File) Readdir(n int) ([]os.FileInfo, error) {
	return f.f.Readdir(n)
}

func (f *File) Readdirnames(n int) ([]string, error) {
	return f.f.Readdirnames(n)
}

func (f *File) Truncate(size int64) error {
	return f.f.Truncate(size)
}

func (f *File) WriteString(s string) (n int, err error) {
	return f.f.WriteString(s)
}

// ReadDir reads the contents of the directory and returns a slice of up to n
// DirEntry values in directory order. This delegates to the underlying os.File.ReadDir
// which uses high-performance native OS directory reading APIs.
//
// If n > 0, ReadDir returns at most n entries. In this case, if ReadDir
// returns an empty slice, it will return a non-nil error explaining why.
// At the end of a directory, the error is io.EOF.
//
// If n <= 0, ReadDir returns all entries from the directory in a single slice.
// In this case, if ReadDir succeeds (reads all the way to the end of the
// directory), it returns the slice and a nil error.
func (f *File) ReadDir(n int) ([]fs.DirEntry, error) {
	return f.f.ReadDir(n)
}
