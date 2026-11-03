# Changelog

## Unreleased

### Fixed

- `FileSystem.ReadDir` on Linux opens the directory with `O_CLOEXEC`, as
  `os.Open` does. A process started by another goroutine while a directory
  was being read inherited the descriptor.
- `FileSystem.ReadDir` on Linux reports os's error op: `readdirent` for a
  read failure, where it said `getdents`. For a name that exists but is not
  a directory it follows `os.ReadDir` in the Go release it is built with:
  `open` with `ENOTDIR` from Go 1.24, which opens with `O_DIRECTORY` as
  osfs does, and `readdirent` with `ENOTDIR` with Go 1.23, where it said
  `open`. With Go 1.23 a FIFO also fails that way rather than blocking, as
  `os.ReadDir` would. An open or read interrupted by a signal is retried, as
  os does.
- `FileSystem.ReadDir` on Windows returns the errors `os.ReadDir` returns.
  It reported `ERROR_PATH_NOT_FOUND` for a missing name where `os.ReadDir`
  reports `ERROR_FILE_NOT_FOUND`, and `ERROR_DIRECTORY` for a regular file or
  a dangling symlink, which does not match `fs.ErrNotExist`. Any enumeration
  failure is now answered by `os.ReadDir`, so the op, path, errno and
  `errors.Is` behaviour are os's.
- `FileSystem.ReadDir` on Windows lists a symlink to a directory as
  `fs.ModeSymlink`, as `os.ReadDir` does, instead of `fs.ModeDir`. Reparse
  points are typed by `os.Lstat`, so junctions and other reparse points get
  the type the os package gives them.
- `FileSystem.ReadDir` on Windows lists a bare drive designator (`C:`) as
  the current directory on that drive, as `os.ReadDir` and osfs's own `Stat`
  do, instead of the drive root. Entries' `Info` resolve the same way.
- `FileSystem.ReadDir` on Linux returns a non-nil empty slice for an empty
  directory, as `os.ReadDir` does, instead of nil.
- If a reparse point cannot be `Lstat`ed while it is listed on Windows,
  `ReadDir` types it from its reparse tag (a directory symlink is
  `fs.ModeSymlink`) instead of reporting it as a directory.
- Native absolute paths (`C:\foo`, `C:/foo`, `\\server\share`) are passed to
  the OS unchanged instead of being read as relative absfs paths, so
  `basefs.NewFS(osfs, nativeDir)` works on Windows (commit de085c4, after
  v1.0.0 and not yet in a release).

### Changed

- `FileSystem.ReadDir` on Windows no longer doubles a trailing separator in
  its search pattern (`C:\` searches `C:\*`, not `C:\\*`). Windows accepted
  the doubled form, so listings were already correct.
- CI measures coverage on Linux, macOS and Windows, and tests Go 1.23, 1.24
  and the current stable release with `GOTOOLCHAIN=local`. The matrix said
  1.21 and 1.22, but go.mod's `go 1.23` made those jobs download and run
  go1.23.0.
- README: the fstools section names the traversal constants and operations
  fstools has (`DepthTraversal`, `KeyTraversal`, `ParallelCopy`, `Exists`,
  `Size`, `Count`, `Equal`) instead of ones it does not (`Describe`, `Diff`,
  `Patch`, `Apply`), and the file-operations example no longer redeclares
  `f`. Every README example was compiled against the package.
- README: removed the `WindowsDriveMapper` documentation, which described an
  API the package no longer has, and documented the path rules instead.

## v1.0.0

- absfs 1.0 compatibility.
