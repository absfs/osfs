# Changelog

## Unreleased

### Security

- `RootFS`, from `NewRootFS(dir)` or `FileSystem.OpenRoot(dir)`, is a
  `SymlinkFileSystem` confined to `dir` by `os.Root`: the host never follows
  a symlink out of it. Through `FileSystem`, a link inside a directory leads
  the host wherever it points, so a relative link moved shallower, a link to
  `..`, an absolute link, or a directory swapped for a link mid-walk can make
  an operation read, write, truncate or chmod a file outside. Through
  `RootFS` each of those fails with `ErrPathEscapes`. An absolute link whose
  target lies inside the root is still followed, inside the root.
  `RootFS` needs Go 1.25 on Unix; elsewhere `OpenRoot` returns an error
  wrapping `errors.ErrUnsupported`, and osfs still builds with Go 1.23.
- `RootFS.OpenFile` with `O_NOFOLLOW` opens the final element with
  `openat(2)` and `O_NOFOLLOW` itself, since `os.Root` follows a final link
  whatever the flags. A link swapped in while it runs is refused, as
  `os.OpenFile` refuses it, not followed to another file in the root.

### Added

- `RootFS`, `NewRootFS`, `FileSystem.OpenRoot` and `ErrPathEscapes`.
  `RootFS` and `NewRootFS` are defined only in `go1.25 && unix` builds;
  `OpenRoot` and `ErrPathEscapes` everywhere. `OpenRoot`'s result is an
  `io.Closer`.
- A dependency on `golang.org/x/sys`, for `openat(2)`.

### Fixed

- `FileSystem.ReadDir` on Linux opens the directory with `O_CLOEXEC`, as
  `os.Open` does. A process started by another goroutine while a directory
  was being read inherited the descriptor.
- `FileSystem.ReadDir` on Linux reports os's error op: `readdirent` for a
  read failure, where it said `getdents`. For a name that exists but is not
  a directory it follows `os.ReadDir` in the Go release it is built with:
  `open` with `ENOTDIR` from Go 1.24, which opens with `O_DIRECTORY` as
  osfs does, and `readdirent` with `ENOTDIR` with Go 1.23, where it said
  `open`. With Go 1.23 a name that is not a directory and cannot be opened
  reports os's errno at open, where it reported `ENOTDIR`: a file without
  read permission gives `EACCES`, so `errors.Is(err, fs.ErrPermission)` now
  holds, and a socket gives `ENXIO`. ReadDir works these out with `stat`
  and `access` and never opens a name that is not a directory, so with Go
  1.23 a FIFO fails with `readdirent` and `ENOTDIR` rather than blocking as
  `os.ReadDir` would, and a device whose open would fail gives the same
  error rather than the device's. An open or read interrupted by a signal
  is retried, as os does.
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
