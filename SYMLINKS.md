# Symbolic Links in osfs

This document covers symbolic link support in `osfs`, including platform-specific behavior and Windows configuration requirements.

## Overview

The `osfs` package provides full symbolic link support through the `absfs.SymLinker` interface:

```go
// Create a symlink: /link -> /target
err := fs.Symlink("/target", "/link")

// Read where a symlink points
target, err := fs.Readlink("/link")

// Get symlink info without following it
info, err := fs.Lstat("/link")

// Change symlink ownership (Unix only)
err := fs.Lchown("/link", uid, gid)
```

## Platform Support

| Platform | Symlink Support | Notes |
|----------|-----------------|-------|
| Linux    | Full | No special configuration required |
| macOS    | Full | No special configuration required |
| Windows  | Conditional | Requires elevated privileges or Developer Mode |

## Unix/macOS Behavior

On Unix-like systems, symlinks work as expected with no special configuration:

```go
fs, _ := osfs.NewFS()

// Create a file
f, _ := fs.Create("/tmp/target.txt")
f.Write([]byte("hello"))
f.Close()

// Create a symlink
fs.Symlink("/tmp/target.txt", "/tmp/link.txt")

// Read through the symlink
content, _ := fs.ReadFile("/tmp/link.txt")  // Returns "hello"

// Stat follows symlinks
info, _ := fs.Stat("/tmp/link.txt")  // Returns target file info

// Lstat returns symlink info
info, _ := fs.Lstat("/tmp/link.txt")  // Returns symlink info
fmt.Println(info.Mode() & os.ModeSymlink != 0)  // true
```

## Windows Symlink Support

### Requirements

Windows requires special privileges to create symbolic links. You have three options:

#### Option 1: Enable Developer Mode (Recommended)

Available on Windows 10 build 14972 and later:

1. Open **Settings** (Win + I)
2. Go to **Privacy & Security** > **For developers**
3. Enable **Developer Mode**
4. Restart any open terminals

This allows symlink creation without administrator privileges.

#### Option 2: Run as Administrator

Run your application with elevated privileges:

1. Right-click on Command Prompt or PowerShell
2. Select **Run as administrator**
3. Run your Go application from there

#### Option 3: Grant SeCreateSymbolicLinkPrivilege

For server environments or CI/CD:

1. Open **Local Security Policy** (`secpol.msc`)
2. Navigate to **Local Policies** > **User Rights Assignment**
3. Find **Create symbolic links**
4. Add your user or group
5. Log out and back in

### Error Handling

Without proper privileges, symlink creation fails with a specific error:

```go
err := fs.Symlink("/target", "/link")
if err != nil {
    // On Windows without privileges:
    // "A required privilege is not held by the client."

    // Check if it's a permission error
    if os.IsPermission(err) {
        log.Println("Symlinks require Developer Mode or admin privileges on Windows")
    }
}
```

### Checking Symlink Capability

You can test if symlinks are available at runtime:

```go
func canCreateSymlinks(fs absfs.SymlinkFileSystem) bool {
    testDir := fs.TempDir()
    target := filepath.Join(testDir, ".symlink_test_target")
    link := filepath.Join(testDir, ".symlink_test_link")

    // Create a test file
    f, err := fs.Create(target)
    if err != nil {
        return false
    }
    f.Close()
    defer fs.Remove(target)

    // Try to create a symlink
    err = fs.Symlink(target, link)
    if err != nil {
        return false
    }
    fs.Remove(link)
    return true
}
```

## Differences from Unix Symlinks

### File vs Directory Symlinks

Windows distinguishes between file and directory symlinks internally. Go's `os.Symlink` handles this automatically by checking if the target is a directory, but there are edge cases:

```go
// Creating a symlink to a non-existent target
// Go cannot determine if it should be a file or directory symlink
err := fs.Symlink("/nonexistent", "/link")
// On Windows: creates a file symlink by default
// This may fail if /nonexistent is later created as a directory
```

**Best practice**: Ensure targets exist before creating symlinks on Windows.

### Relative Symlinks

Relative symlinks work on Windows but use forward slashes internally in absfs:

```go
// Both platforms - absfs uses forward slashes
fs.Symlink("../other/file.txt", "/path/link")

// osfs converts to native format:
// - Unix: ../other/file.txt
// - Windows: ..\other\file.txt
```

### Junction Points

Windows junction points are similar to directory symlinks but:
- Don't require elevated privileges
- Only work for directories
- Only work for local paths (not network paths)

Go's `os.Symlink` does **not** create junction points. If you need junction-like behavior without privileges, consider using `os/exec` to call `mklink /J` directly.

### Symlink Targets Across Drives

On Windows, symlinks can span drives:

```go
// Works on Windows (with privileges)
fs.Symlink("/d/data/file.txt", "/c/links/myfile")
// Creates: C:\links\myfile -> D:\data\file.txt
```

## Path Handling

The `osfs` package normalizes symlink targets to Unix-style paths:

```go
// Creating a symlink on Windows
fs.Symlink("/c/target", "/c/link")
// Actual Windows symlink: C:\link -> C:\target

// Reading it back
target, _ := fs.Readlink("/c/link")
// Returns: /c/target (Unix-style, normalized)
```

## Error Reference

| Error | Platform | Cause |
|-------|----------|-------|
| `A required privilege is not held` | Windows | Missing symlink privileges |
| `Access is denied` | Windows | Target path permission issue |
| `The system cannot find the path specified` | Windows | Target doesn't exist (for some operations) |
| `ELOOP` | All | Too many symlink levels (cycle detected) |
| `ENOENT` | All | Symlink target doesn't exist (for Stat) |
| `EINVAL` | All | Invalid symlink operation |

## Virtual Filesystems

For virtual filesystems (memfs, boltfs, etc.), symlinks work without OS privileges since they're implemented entirely in memory or storage:

```go
// memfs - always works, no OS interaction
mfs, _ := memfs.NewFS()
mfs.Symlink("/target", "/link")  // Always succeeds

// boltfs - always works, stored in database
bfs, _ := boltfs.NewFS(db, "")
bfs.Symlink("/target", "/link")  // Always succeeds
```

## Adding Symlink Support to Non-Native Filesystems

For filesystems without native symlink support (like S3), use `absfs.ExtendSymlinkFiler`:

```go
import "github.com/absfs/absfs"

// s3fs doesn't have native symlinks
s3, _ := s3fs.NewFS(bucket)

// Add symlink support via overlay
sfs := absfs.ExtendSymlinkFiler(s3)

// Now symlinks work (stored as special files)
sfs.Symlink("/target", "/link")
```

See the [absfs documentation](https://github.com/absfs/absfs) for details on `ExtendSymlinkFiler` and `SymlinkOverlay`.

## Testing

The `fstesting` package automatically handles platform differences:

```go
import "github.com/absfs/fstesting"

// OSFeatures() returns appropriate settings per platform
// On Windows: Symlinks: false (unless you know privileges are available)
// On Unix: Symlinks: true
features := fstesting.OSFeatures()

suite := &fstesting.Suite{
    FS: myFS,
    Features: features,
}
suite.Run(t)
```

To force symlink testing on Windows (when you have privileges):

```go
features := fstesting.OSFeatures()
features.Symlinks = true  // Override for testing with privileges
```
