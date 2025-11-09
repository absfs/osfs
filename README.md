# osfs - OS Filesystem Implementation for absfs

[![Go Reference](https://pkg.go.dev/badge/github.com/absfs/osfs.svg)](https://pkg.go.dev/github.com/absfs/osfs)

Package `osfs` provides an implementation of the `absfs.FileSystem` interface that wraps the standard Go `os` package, allowing direct interaction with the operating system's filesystem.

## Installation

```bash
go get github.com/absfs/osfs
```

## Basic Usage

```go
import "github.com/absfs/osfs"

// Create a new OS filesystem
fs, err := osfs.NewFS()
if err != nil {
    log.Fatal(err)
}

// Use it like any absfs.FileSystem
file, err := fs.Create("/path/to/file.txt")
if err != nil {
    log.Fatal(err)
}
defer file.Close()
```

## Features

- **Full absfs.FileSystem implementation**: Implements all methods of the absfs interface
- **Direct OS access**: All operations map directly to Go's `os` package
- **Working directory management**: Maintains an internal working directory
- **Cross-platform**: Works on Unix, Linux, macOS, and Windows
- **Fast filesystem walking**: Includes optimized `FastWalk` implementation

## Windows Drive Mapping

On Windows, the `WindowsDriveMapper` wrapper provides intuitive path translation for cross-platform code:

```go
import "github.com/absfs/osfs"

// Create a mapper that translates virtual-absolute paths
fs, err := osfs.NewFS()
if err != nil {
    log.Fatal(err)
}
mapped := osfs.NewWindowsDriveMapper(fs, "C:")

// Unix-style paths automatically map to Windows paths
mapped.Create("/config/app.json")      // → C:\config\app.json
mapped.MkdirAll("/var/log/app", 0755)  // → C:\var\log\app

// OS-absolute paths pass through unchanged
mapped.Open("C:\\Windows\\file.txt")   // → C:\Windows\file.txt
mapped.Open("D:\\Data\\file.txt")      // → D:\Data\file.txt

// UNC paths work correctly
mapped.Open("\\\\server\\share\\file") // → \\server\share\file

// On Unix/macOS, the mapper is a no-op
mapped.Create("/config/app.json")      // → /config/app.json
```

### When to use WindowsDriveMapper

**Use it when:**
- Writing cross-platform CLI tools that work with OS filesystems
- Porting Unix-based tools to Windows
- You want `/path` semantics to map to `C:\path` on Windows

**Don't use it when:**
- Working with virtual/in-memory filesystems (use base absfs)
- You need full control over Windows drive letters in your code
- Testing with mock filesystems

See [absfs PATH_HANDLING.md](https://github.com/absfs/absfs/blob/main/PATH_HANDLING.md) for more details on cross-platform path handling.

### Custom Drive Letters

You can specify a different drive letter:

```go
// Use D: drive instead of C:
mapped := osfs.NewWindowsDriveMapper(fs, "D:")
mapped.Create("/data/file.txt")  // → D:\data\file.txt
```

If you don't specify a drive, it defaults to `C:`.

## API Overview

The `FileSystem` type implements the following methods:

### File Operations
- `Open(name string) (absfs.File, error)`
- `Create(name string) (absfs.File, error)`
- `OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error)`
- `Truncate(name string, size int64) error`

### Directory Operations
- `Mkdir(name string, perm os.FileMode) error`
- `MkdirAll(path string, perm os.FileMode) error`
- `Remove(name string) error`
- `RemoveAll(path string) error`
- `Chdir(dir string) error`
- `Getwd() (dir string, err error)`

### File Information
- `Stat(name string) (os.FileInfo, error)`
- `Lstat(name string) (os.FileInfo, error)`

### File Attributes
- `Chmod(name string, mode os.FileMode) error`
- `Chtimes(name string, atime time.Time, mtime time.Time) error`
- `Chown(name string, uid, gid int) error`
- `Lchown(name string, uid, gid int) error`

### Path Operations
- `Rename(oldpath, newpath string) error`
- `Symlink(oldname, newname string) error`
- `Readlink(name string) (string, error)`

### Filesystem Walking
- `Walk(path string, fn func(string, os.FileInfo, error) error) error`
- `FastWalk(path string, fn func(string, os.FileMode) error) error`

### System Information
- `Separator() uint8` - Returns the OS-specific path separator
- `ListSeparator() uint8` - Returns the OS-specific list separator
- `TempDir() string` - Returns the temporary directory path

## FastWalk

The `FastWalk` method provides optimized filesystem traversal:

```go
fs, _ := osfs.NewFS()
err := fs.FastWalk("/path/to/dir", func(path string, mode os.FileMode) error {
    fmt.Println(path, mode)
    return nil
})
```

FastWalk is significantly faster than `Walk` for large directory trees because it avoids unnecessary stat calls.

## License

See the main [absfs repository](https://github.com/absfs/absfs) for license information.

## Contributing

Contributions are welcome! Please see the main [absfs repository](https://github.com/absfs/absfs) for contribution guidelines.
