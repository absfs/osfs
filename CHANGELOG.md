# Changelog

## Unreleased

### Fixed

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
- `FileSystem.ReadDir` on Windows no longer doubles a trailing separator in
  the search pattern (`C:\` lists `C:\*`).
- Native absolute paths (`C:\foo`, `C:/foo`, `\\server\share`) are passed to
  the OS unchanged instead of being read as relative absfs paths, so
  `basefs.NewFS(osfs, nativeDir)` works on Windows (commit de085c4, after
  v1.0.0 and not yet in a release).

### Changed

- CI measures coverage on Linux, macOS and Windows.

## v1.0.0

- absfs 1.0 compatibility.
