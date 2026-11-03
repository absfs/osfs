# osfs - Operating System Filesystem

[![Go Reference](https://pkg.go.dev/badge/github.com/absfs/osfs.svg)](https://pkg.go.dev/github.com/absfs/osfs)
[![Go Report Card](https://goreportcard.com/badge/github.com/absfs/osfs)](https://goreportcard.com/report/github.com/absfs/osfs)
[![CI](https://github.com/absfs/osfs/actions/workflows/ci.yml/badge.svg)](https://github.com/absfs/osfs/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`osfs` provides an [absfs](https://github.com/absfs/absfs) FileSystem implementation that wraps the Go standard library's `os` package for direct operating system filesystem access.

## Features

- **Native OS Access**: Direct access to your operating system's filesystem
- **Cross-Platform Support**: Works on Unix, macOS, and Windows
- **Unix-Style Paths Everywhere**: `/c/Users/foo` on Windows, with `FromNative`/`ToNative` to convert
- **Full absfs Compatibility**: Implements the complete absfs.FileSystem interface

## Install

```bash
go get github.com/absfs/osfs
```

## Quick Start

### Basic Usage

```go
package main

import (
	"log"
	"github.com/absfs/osfs"
)

func main() {
	fs, err := osfs.NewFS()
	if err != nil {
		log.Fatal(err)
	}

	// Create and write to a file
	f, err := fs.Create("example.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	_, err = f.Write([]byte("Hello, world!\n"))
	if err != nil {
		log.Fatal(err)
	}
}
```

## Paths

absfs paths are Unix-style on every OS, so the same code runs on Unix and
Windows. On Windows a drive is a lowercase first element and a UNC share
starts with two slashes:

| absfs path           | Windows path          | Notes                                           |
| -------------------- | --------------------- | ----------------------------------------------- |
| `/c/config/app.json` | `C:\config\app.json`  | Drive letter as the first element               |
| `/config/app.json`   | `C:\config\app.json`  | No drive: the drive of osfs's working directory |
| `//server/share/f`   | `\\server\share\f`    | UNC path                                        |
| `relative/path`      | `<cwd>\relative\path` | Joined to osfs's working directory              |
| `C:\Windows\file`    | `C:\Windows\file`     | Native absolute path: passed through unchanged  |

On Unix the absfs path is the native path. Convert explicitly with
`osfs.FromNative` and `osfs.ToNative`; `SplitDrive`, `GetDrive`, `SetDrive`,
`IsUNC` and `SplitUNC` take paths apart.

```go
fs, _ := osfs.NewFS()

dir := osfs.FromNative(os.TempDir()) // "/c/Users/me/AppData/Local/Temp" on Windows
fs.MkdirAll(dir+"/app/logs", 0755)
fs.Create(dir + "/app/config.json")
```

### Native Paths

osfs also accepts a native absolute path such as `C:\Users\foo` or
`\\server\share`, and passes it to the OS unchanged; a bare drive designator
(`C:`) means the current directory on that drive, as it does for the os
package. That is what lets [basefs](https://github.com/absfs/basefs) wrap
osfs with a native directory, `basefs.NewFS(osfs, t.TempDir())`: it joins the
native prefix to an absfs suffix (`C:\Users\foo/sub`), which Windows accepts.
Converting with `osfs.FromNative` first is still the clearer choice.

## Symbolic Links

The `osfs` package provides full symbolic link support via the `absfs.SymLinker` interface:

```go
fs, _ := osfs.NewFS()

// Create a symlink
fs.Symlink("/path/to/target", "/path/to/link")

// Read symlink target
target, _ := fs.Readlink("/path/to/link")

// Get symlink info without following
info, _ := fs.Lstat("/path/to/link")
```

**Windows users**: Symlinks require elevated privileges or Developer Mode. See [SYMLINKS.md](SYMLINKS.md) for configuration instructions and platform-specific behavior.

## API Overview

### Creating Filesystems

```go
// Standard OS filesystem
fs, err := osfs.NewFS()
```

### File Operations

```go
// Open files
f, err := fs.Open("/path/to/file.txt")
f, err = fs.Create("/path/to/new.txt")
f, err = fs.OpenFile("/path", os.O_RDWR|os.O_CREATE, 0644)

// File info
info, err := fs.Stat("/path/to/file.txt")

// Remove files
err = fs.Remove("/path/to/file.txt")
```

### Directory Operations

```go
// Create directories
err = fs.Mkdir("/path/to/dir", 0755)
err = fs.MkdirAll("/path/to/nested/dirs", 0755)

// Remove directories
err = fs.Remove("/empty/dir")
err = fs.RemoveAll("/path/to/dir")

// List a directory, sorted by name
entries, err := fs.ReadDir("/path/to/dir")
```

`ReadDir` uses a faster native reader on Linux (`getdents` with a larger
buffer) and Windows (`FindFirstFile`), but its entries and errors are the ones
`os.ReadDir` gives, op and errno included, so `errors.Is(err, fs.ErrNotExist)`
answers the same way.
On Windows that includes the errno: a missing name is `ERROR_FILE_NOT_FOUND`
and a regular file `ERROR_PATH_NOT_FOUND`, never `ERROR_DIRECTORY`, and a
symlink to a directory is listed as a symlink. `File.ReadDir`, `Readdir` and
`Readdirnames` are `os.File`'s own.

### Additional Operations

```go
// Rename/move
err = fs.Rename("/old/path", "/new/path")

// Permissions and attributes
err = fs.Chmod("/path/to/file", 0644)
err = fs.Chtimes("/path/to/file", atime, mtime)
err = fs.Chown("/path/to/file", uid, gid)

// Working directory
err = fs.Chdir("/new/directory")
cwd, err := fs.Getwd()
```

## Advanced Operations

### Directory Walking

For advanced directory traversal operations on absfs filesystems, use the [fstools](https://github.com/absfs/fstools) package:

```go
import (
    "github.com/absfs/fstools"
    "github.com/absfs/osfs"
)

fs, _ := osfs.NewFS()

// Simple walk compatible with filepath.WalkFunc
err := fstools.Walk(fs, "/path", func(path string, info os.FileInfo, err error) error {
    fmt.Println(path)
    return nil
})

// Walk with options for different traversal strategies
options := &fstools.Options{
    Sort:      true,
    Traversal: fstools.BreadthTraversal, // Also: DepthTraversal, PreOrderTraversal, PostOrderTraversal, KeyTraversal
}
err = fstools.WalkWithOptions(fs, options, "/path", func(path string, info os.FileInfo, err error) error {
    fmt.Println(path)
    return err
})
```

The fstools package provides:
- **Multiple traversal strategies**: breadth-first, depth-first, pre-order, post-order, and key order (files only)
- **Sorting options**: Custom sort functions or alphabetical
- **Filesystem operations**: Copy, ParallelCopy, Exists, Size, Count, and Equal
- **Works with any absfs.Filer implementation**

See [github.com/absfs/fstools](https://github.com/absfs/fstools) for complete documentation.

## Documentation

- [SYMLINKS.md](SYMLINKS.md) - Symbolic link support and Windows configuration
- [absfs Documentation](https://github.com/absfs/absfs) - Main abstraction interface
- [PATH_HANDLING.md](https://github.com/absfs/absfs/blob/master/PATH_HANDLING.md) - Cross-platform path handling
- [USER_GUIDE.md](https://github.com/absfs/absfs/blob/master/USER_GUIDE.md) - Complete usage guide
- [GoDoc](https://pkg.go.dev/github.com/absfs/osfs) - API reference

## Related Packages

- [absfs](https://github.com/absfs/absfs) - Core filesystem abstraction
- [fstools](https://github.com/absfs/fstools) - Advanced filesystem operations (Walk, Copy, Diff, Patch)
- [memfs](https://github.com/absfs/memfs) - In-memory filesystem
- [basefs](https://github.com/absfs/basefs) - Chroot filesystem wrapper
- [rofs](https://github.com/absfs/rofs) - Read-only filesystem wrapper

## License

This project is governed by the MIT License. See [LICENSE](https://github.com/absfs/osfs/blob/master/LICENSE)



