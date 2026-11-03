//go:build go1.25 && unix

package osfs

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/absfs/absfs"
)

// maxRootLinks bounds the symlinks one name may lead through, as the
// kernel's own limit does.
const maxRootLinks = 40

// RootFS is an absfs.SymlinkFileSystem confined to one host directory, its
// root, by os.Root. It takes the same names osfs does, absolute host paths
// or names relative to its working directory, and refuses a name outside the
// root with an *os.PathError wrapping ErrPathEscapes.
//
// Unlike osfs, which hands each name to the OS, RootFS resolves every name
// through an os.Root, so the host never follows a symlink out of the root,
// whoever made the link and however it got where it is. A relative link that
// leads outside, ".." included, makes the operation fail with
// ErrPathEscapes. An absolute link is followed when its target, as written,
// is the root's path (as given or with symlinks evaluated) or lies below it:
// the target is then taken inside the root. Any other absolute target fails
// with ErrPathEscapes. Symlink stores targets verbatim and Readlink returns
// them verbatim; neither is checked, since neither follows the link.
//
// RootFS requires Go 1.25 on a Unix system. Use FileSystem.OpenRoot for a
// portable entry point that reports errors.ErrUnsupported elsewhere.
//
// A RootFS holds the root directory open; Close releases it. If the root is
// moved on the host, RootFS keeps operating on it in its new place, as
// os.Root does, but names are still judged against the path it was opened
// with.
type RootFS struct {
	root   *os.Root
	dir    string // clean absolute root path, as given
	alias  string // dir with symlinks evaluated, if that differs
	cwd    string // clean absolute path on or below the root
	escape error  // os.Root's own error for a name that leaves the root
}

var _ absfs.SymlinkFileSystem = (*RootFS)(nil)

// NewRootFS opens dir as the root of a RootFS. A relative dir is taken from
// the process's working directory. The working directory of the RootFS
// starts at the root.
func NewRootFS(dir string) (*RootFS, error) {
	if dir == "" {
		return nil, &os.PathError{Op: "openroot", Path: dir, Err: os.ErrInvalid}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	r := &RootFS{root: root, dir: abs, cwd: abs}
	if real, err := filepath.EvalSymlinks(abs); err == nil && real != abs {
		r.alias = real
	}
	// os.Root's escape error is unexported; learn it from a name that
	// certainly leaves the root.
	_, err = root.Lstat("..")
	r.escape = errors.Unwrap(err)
	return r, nil
}

// openRoot is FileSystem.OpenRoot for a native directory.
func openRoot(dir string) (absfs.SymlinkFileSystem, error) {
	r, err := NewRootFS(dir)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Close releases the root directory. Operations after Close fail.
func (r *RootFS) Close() error {
	return r.root.Close()
}

// Root returns the path of the root directory, as given to NewRootFS but
// made absolute and clean.
func (r *RootFS) Root() string {
	return r.dir
}

// within reports whether p, an absolute path, is dir or lies below it on a
// separator boundary, and if so returns it relative to dir. p is not
// cleaned.
func within(dir, p string) (string, bool) {
	if p == dir {
		return ".", true
	}
	if dir != "/" {
		dir += "/"
	}
	if !strings.HasPrefix(p, dir) {
		return "", false
	}
	if rest := p[len(dir):]; rest != "" {
		return rest, true
	}
	return ".", true
}

// inside maps p, an absolute path, to a name relative to the root.
func (r *RootFS) inside(p string) (string, bool) {
	if rel, ok := within(r.dir, p); ok {
		return rel, true
	}
	if r.alias != "" {
		return within(r.alias, p)
	}
	return "", false
}

// rel maps a caller's name to a clean name relative to the root.
func (r *RootFS) rel(op, name string) (string, error) {
	p := name
	if !path.IsAbs(p) {
		p = r.cwd + "/" + p
	}
	if rel, ok := r.inside(path.Clean(p)); ok {
		return rel, nil
	}
	return "", &os.PathError{Op: op, Path: name, Err: ErrPathEscapes}
}

// escaped reports whether err is os.Root refusing a name that leaves the
// root, or one that leads through an absolute symlink.
func (r *RootFS) escaped(err error) bool {
	return err != nil && r.escape != nil && errors.Is(err, r.escape)
}

// resolve replaces each symlink among the directory elements of rel, and its
// final element if follow is set, by what it points to, so that the result
// leads through no symlink. An absolute target that names a place inside the
// root is taken there. ".." is applied to what has been resolved so far, so
// it is physical, as the host would apply it. A target that leaves the root
// gives ErrPathEscapes. Once an element is missing, or cannot be examined,
// the rest is kept as it is for the operation itself to judge.
//
// Every lookup goes through os.Root, and so does the operation given the
// result. A link swapped in meanwhile can make the operation fail, never
// leave the root.
func (r *RootFS) resolve(rel string, follow bool) (string, error) {
	todo := strings.Split(rel, "/")
	var done []string
	links := 0
	for len(todo) > 0 {
		elem := todo[0]
		todo = todo[1:]
		switch elem {
		case "", ".":
			continue
		case "..":
			if len(done) == 0 {
				return "", ErrPathEscapes
			}
			done = done[:len(done)-1]
			continue
		}
		if len(todo) == 0 && !follow {
			done = append(done, elem)
			break
		}
		p := strings.Join(append(slices.Clip(done), elem), "/")
		info, err := r.root.Lstat(p)
		if err != nil {
			done = append(append(done, elem), todo...)
			break
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			done = append(done, elem)
			continue
		}
		if links++; links > maxRootLinks {
			return "", syscall.ELOOP
		}
		target, err := r.root.Readlink(p)
		if err != nil {
			return "", err
		}
		if path.IsAbs(target) {
			in, ok := r.inside(target)
			if !ok {
				return "", ErrPathEscapes
			}
			done = done[:0]
			target = in
		}
		todo = append(strings.Split(target, "/"), todo...)
	}
	if len(done) == 0 {
		return ".", nil
	}
	return strings.Join(done, "/"), nil
}

// fix makes err, from os.Root, name the caller's name rather than the name
// relative to the root, and turns os.Root's escape error into
// ErrPathEscapes.
func (r *RootFS) fix(op, name string, err error) error {
	if err == nil {
		return nil
	}
	if r.escaped(err) || errors.Is(err, ErrPathEscapes) {
		return &os.PathError{Op: op, Path: name, Err: ErrPathEscapes}
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		return &os.PathError{Op: pe.Op, Path: name, Err: pe.Err}
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return &os.PathError{Op: le.Op, Path: name, Err: le.Err}
	}
	return &os.PathError{Op: op, Path: name, Err: err}
}

// do runs f on name made relative to the root. If os.Root refuses the name
// as leaving the root, which it also does for any absolute symlink on the
// way, do resolves the symlinks itself and runs f once more on the result.
// follow says whether f follows a final symlink.
func (r *RootFS) do(op, name string, follow bool, f func(rel string) error) error {
	rel, err := r.rel(op, name)
	if err != nil {
		return err
	}
	err = f(rel)
	if r.escaped(err) {
		if rel, err = r.resolve(rel, follow); err == nil {
			err = f(rel)
		}
	}
	return r.fix(op, name, err)
}

// do2 is do for Rename's two names, neither followed at the end.
func (r *RootFS) do2(op, oldname, newname string, f func(oldrel, newrel string) error) error {
	oldrel, err := r.rel(op, oldname)
	if err != nil {
		return &os.LinkError{Op: op, Old: oldname, New: newname, Err: ErrPathEscapes}
	}
	newrel, err := r.rel(op, newname)
	if err != nil {
		return &os.LinkError{Op: op, Old: oldname, New: newname, Err: ErrPathEscapes}
	}
	err = f(oldrel, newrel)
	if r.escaped(err) {
		oldrel, err = r.resolve(oldrel, false)
		if err == nil {
			newrel, err = r.resolve(newrel, false)
		}
		if err == nil {
			err = f(oldrel, newrel)
		}
	}
	if err == nil {
		return nil
	}
	if r.escaped(err) || errors.Is(err, ErrPathEscapes) {
		err = ErrPathEscapes
	} else if le, ok := err.(*os.LinkError); ok {
		err = le.Err
	}
	return &os.LinkError{Op: op, Old: oldname, New: newname, Err: err}
}

func (r *RootFS) file(f *os.File, err error) (absfs.File, error) {
	if err != nil {
		return nil, err
	}
	return &File{f: f}, nil
}

// Chdir changes the working directory to name, which must be a directory
// inside the root.
func (r *RootFS) Chdir(name string) error {
	info, err := r.Stat(name)
	if err != nil {
		return &os.PathError{Op: "chdir", Path: name, Err: err.(*os.PathError).Err}
	}
	if !info.IsDir() {
		return &os.PathError{Op: "chdir", Path: name, Err: errors.New("not a directory")}
	}
	p := name
	if !path.IsAbs(p) {
		p = r.cwd + "/" + p
	}
	r.cwd = path.Clean(p)
	return nil
}

// Getwd returns the working directory, a host path on or below the root.
func (r *RootFS) Getwd() (string, error) {
	return r.cwd, nil
}

// TempDir returns the OS temporary directory, as osfs does. It is usually
// outside the root.
func (r *RootFS) TempDir() string {
	return os.TempDir()
}

// Open opens the named file for reading.
func (r *RootFS) Open(name string) (absfs.File, error) {
	var f *os.File
	err := r.do("open", name, true, func(rel string) (err error) {
		f, err = r.root.Open(rel)
		return err
	})
	return r.file(f, err)
}

// Create creates or truncates the named file.
func (r *RootFS) Create(name string) (absfs.File, error) {
	var f *os.File
	err := r.do("open", name, true, func(rel string) (err error) {
		f, err = r.root.Create(rel)
		return err
	})
	return r.file(f, err)
}

// OpenFile opens the named file with the given flags and mode. With
// O_NOFOLLOW, a final symlink fails with ELOOP, as it does for os.OpenFile.
func (r *RootFS) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
	var f *os.File
	follow := flag&syscall.O_NOFOLLOW == 0
	if !follow {
		// os.Root follows a final symlink despite O_NOFOLLOW.
		if info, err := r.Lstat(name); err == nil && info.Mode()&fs.ModeSymlink != 0 {
			return nil, &os.PathError{Op: "open", Path: name, Err: syscall.ELOOP}
		}
	}
	err := r.do("open", name, follow, func(rel string) (err error) {
		f, err = r.root.OpenFile(rel, flag, perm)
		return err
	})
	return r.file(f, err)
}

// Truncate changes the size of the named file.
func (r *RootFS) Truncate(name string, size int64) error {
	return r.do("truncate", name, true, func(rel string) error {
		// O_NONBLOCK keeps a FIFO from blocking the open.
		f, err := r.root.OpenFile(rel, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return err
		}
		err = f.Truncate(size)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return err
	})
}

// Mkdir creates a directory.
func (r *RootFS) Mkdir(name string, perm os.FileMode) error {
	return r.do("mkdir", name, false, func(rel string) error {
		return r.root.Mkdir(rel, perm)
	})
}

// MkdirAll creates a directory and any missing parents.
func (r *RootFS) MkdirAll(name string, perm os.FileMode) error {
	return r.do("mkdir", name, true, func(rel string) error {
		return r.root.MkdirAll(rel, perm)
	})
}

// Remove removes the named file or empty directory.
func (r *RootFS) Remove(name string) error {
	return r.do("remove", name, false, func(rel string) error {
		return r.root.Remove(rel)
	})
}

// RemoveAll removes name and any children it contains. os.Root does not
// remove the root itself.
func (r *RootFS) RemoveAll(name string) error {
	return r.do("remove", name, false, func(rel string) error {
		return r.root.RemoveAll(rel)
	})
}

// Rename renames (moves) oldname to newname.
func (r *RootFS) Rename(oldname, newname string) error {
	return r.do2("rename", oldname, newname, r.root.Rename)
}

// Stat returns the FileInfo for name, following symlinks.
func (r *RootFS) Stat(name string) (os.FileInfo, error) {
	var info os.FileInfo
	err := r.do("stat", name, true, func(rel string) (err error) {
		info, err = r.root.Stat(rel)
		return err
	})
	return info, err
}

// Lstat returns the FileInfo for name without following a final symlink.
func (r *RootFS) Lstat(name string) (os.FileInfo, error) {
	var info os.FileInfo
	err := r.do("lstat", name, false, func(rel string) (err error) {
		info, err = r.root.Lstat(rel)
		return err
	})
	return info, err
}

// Chmod changes the mode of the named file.
func (r *RootFS) Chmod(name string, mode os.FileMode) error {
	return r.do("chmod", name, true, func(rel string) error {
		return r.root.Chmod(rel, mode)
	})
}

// Chtimes changes the access and modification times of the named file.
func (r *RootFS) Chtimes(name string, atime, mtime time.Time) error {
	return r.do("chtimes", name, true, func(rel string) error {
		return r.root.Chtimes(rel, atime, mtime)
	})
}

// Chown changes the owner and group of the named file.
func (r *RootFS) Chown(name string, uid, gid int) error {
	return r.do("chown", name, true, func(rel string) error {
		return r.root.Chown(rel, uid, gid)
	})
}

// Lchown changes the owner and group of name without following a final
// symlink.
func (r *RootFS) Lchown(name string, uid, gid int) error {
	return r.do("lchown", name, false, func(rel string) error {
		return r.root.Lchown(rel, uid, gid)
	})
}

// Readlink returns the target of the symlink name, as stored.
func (r *RootFS) Readlink(name string) (string, error) {
	var target string
	err := r.do("readlink", name, false, func(rel string) (err error) {
		target, err = r.root.Readlink(rel)
		return err
	})
	return target, err
}

// Symlink creates newname as a symbolic link to oldname, which is stored as
// given.
func (r *RootFS) Symlink(oldname, newname string) error {
	err := r.do("symlink", newname, false, func(rel string) error {
		return r.root.Symlink(oldname, rel)
	})
	if err != nil {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: err.(*os.PathError).Err}
	}
	return nil
}

// ReadDir reads the named directory and returns its entries sorted by name.
func (r *RootFS) ReadDir(name string) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry
	err := r.do("open", name, true, func(rel string) error {
		f, err := r.root.Open(rel)
		if err != nil {
			return err
		}
		defer f.Close()
		entries, err = f.ReadDir(-1)
		return err
	})
	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return entries, err
}

// ReadFile reads the named file and returns its contents.
func (r *RootFS) ReadFile(name string) ([]byte, error) {
	var data []byte
	err := r.do("open", name, true, func(rel string) (err error) {
		data, err = r.root.ReadFile(rel)
		return err
	})
	return data, err
}

// Sub returns an fs.FS for the subtree rooted at dir.
func (r *RootFS) Sub(dir string) (fs.FS, error) {
	return absfs.FilerToFS(r, dir)
}
