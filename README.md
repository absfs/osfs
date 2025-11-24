# osfs - Operating System Filesystem

`osfs` provides an [absfs](https://github.com/absfs/absfs) FileSystem implementation that wraps the Go standard library's `os` package for direct operating system filesystem access.

## Features

- **Native OS Access**: Direct access to your operating system's filesystem
- **Cross-Platform Support**: Works on Unix, macOS, and Windows
- **Windows Drive Mapping**: `WindowsDriveMapper` for seamless Unix-style paths on Windows
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

### Cross-Platform Applications

For applications that need to work across Unix and Windows, use the build tag pattern with `WindowsDriveMapper`:

**Create `filesystem_windows.go`:**
```go
//go:build windows

package myapp

import "github.com/absfs/osfs"

func NewFS(drive string) absfs.FileSystem {
	if drive == "" {
		drive = "C:"
	}
	fs, _ := osfs.NewFS()
	return osfs.NewWindowsDriveMapper(fs, drive)
}
```

**Create `filesystem_unix.go`:**
```go
//go:build !windows

package myapp

import "github.com/absfs/osfs"

func NewFS(drive string) absfs.FileSystem {
	fs, _ := osfs.NewFS()
	return fs
}
```

**Use it everywhere:**
```go
package main

import "myapp"

func main() {
	fs := myapp.NewFS("")

	// Works identically on all platforms!
	fs.Create("/config/app.json")      // → /config/app.json on Unix, C:\config\app.json on Windows
	fs.MkdirAll("/var/log/app", 0755)  // → /var/log/app on Unix, C:\var\log\app on Windows
}
```

See the [absfs PATH_HANDLING.md](https://github.com/absfs/absfs/blob/master/PATH_HANDLING.md) guide for complete cross-platform patterns.

## WindowsDriveMapper

The `WindowsDriveMapper` enables Unix-style absolute paths on Windows by translating virtual-absolute paths to Windows drive paths:

```go
fs, _ := osfs.NewFS()
mapped := osfs.NewWindowsDriveMapper(fs, "C:")

// Unix-style paths work on Windows
mapped.Create("/tmp/config.json")    // Creates C:\tmp\config.json
mapped.MkdirAll("/var/log", 0755)    // Creates C:\var\log

// OS-absolute paths pass through unchanged
mapped.Open("C:\\Windows\\file.txt") // Opens C:\Windows\file.txt
mapped.Open("\\\\server\\share\\f")  // Opens UNC path \\server\share\f
```

### Path Translation Rules

| Input Path | Windows Result | Notes |
|------------|----------------|-------|
| `/config/app.json` | `C:\config\app.json` | Virtual-absolute → mapped to drive |
| `C:\Windows\file` | `C:\Windows\file` | OS-absolute → pass through |
| `\\server\share\f` | `\\server\share\f` | UNC path → pass through |
| `relative/path` | `relative\path` | Relative → pass through |

## API Overview

### Creating Filesystems

```go
// Standard OS filesystem
fs, err := osfs.NewFS()

// With Windows drive mapping (Windows only)
mapped := osfs.NewWindowsDriveMapper(fs, "D:")
```

### File Operations

```go
// Open files
f, err := fs.Open("/path/to/file.txt")
f, err := fs.Create("/path/to/new.txt")
f, err := fs.OpenFile("/path", os.O_RDWR|os.O_CREATE, 0644)

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
```

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

## Examples

See [example_drivemapper_test.go](example_drivemapper_test.go) for complete Windows drive mapping examples.

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
    Traversal: fstools.BreadthTraversal,  // Also: PreOrder, PostOrder, KeyOrder
}
err = fstools.WalkWithOptions(fs, options, "/path", walkFunc)
```

The fstools package provides:
- **Multiple traversal strategies**: PreOrder, PostOrder, BreadthFirst, and KeyOrder (files only)
- **Sorting options**: Custom sort functions or alphabetical
- **Filesystem operations**: Copy, Describe, Diff, Patch, and Apply
- **Works with any absfs.Filer implementation**

See [github.com/absfs/fstools](https://github.com/absfs/fstools) for complete documentation.

## Documentation

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



