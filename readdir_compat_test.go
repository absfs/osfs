package osfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
)

// readDirTree builds the tree the ReadDir comparison tests list, under a new
// temp directory, and reports whether symlinks could be made.
//
//	dir/a.txt, dir/b/, dir/flink -> a.txt, dir/dlink -> b, dir/dangling -> nope
//	empty/
//	file.txt
//	dangling -> nope
//	dirlink -> dir
//	filelink -> file.txt
func readDirTree(t *testing.T) (root string, symlinks bool) {
	t.Helper()
	root = t.TempDir()
	mkdir := func(p string) {
		if err := os.Mkdir(filepath.Join(root, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p string) {
		if err := os.WriteFile(filepath.Join(root, p), []byte(p), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkdir("dir")
	mkdir(filepath.Join("dir", "b"))
	mkdir("empty")
	write(filepath.Join("dir", "a.txt"))
	write("file.txt")

	links := []struct{ target, link string }{
		{"a.txt", filepath.Join("dir", "flink")},
		{"b", filepath.Join("dir", "dlink")},
		{"nope", filepath.Join("dir", "dangling")},
		{filepath.Join(root, "nope"), "dangling"},
		{filepath.Join(root, "dir"), "dirlink"},
		{filepath.Join(root, "file.txt"), "filelink"},
	}
	for _, l := range links {
		if err := os.Symlink(l.target, filepath.Join(root, l.link)); err != nil {
			t.Logf("symlinks unavailable, skipping symlink cases: %v", err)
			// Leave dir's listing the same whether or not links exist.
			for _, l := range links {
				os.Remove(filepath.Join(root, l.link))
			}
			return root, false
		}
	}
	return root, true
}

// readDirCases are the names the comparison tests list, relative to the
// readDirTree root. A trailing "/" means a trailing native separator.
var readDirCases = []struct {
	name    string
	rel     string
	symlink bool
}{
	{"directory", "dir", false},
	{"directory trailing separator", "dir/", false},
	{"empty directory", "empty", false},
	{"missing", "nope", false},
	{"missing trailing separator", "nope/", false},
	{"missing parent", "nope/child", false},
	{"regular file", "file.txt", false},
	{"regular file trailing separator", "file.txt/", false},
	{"child of regular file", "file.txt/child", false},
	{"NUL byte", "bad\x00name", false},
	{"dangling symlink", "dangling", true},
	{"dangling symlink trailing separator", "dangling/", true},
	{"symlink to directory", "dirlink", true},
	{"symlink to directory trailing separator", "dirlink/", true},
	{"symlink to file", "filelink", true},
}

// nativeCase returns the native path of a readDirCases entry under root.
func nativeCase(root, rel string) string {
	trailing := strings.HasSuffix(rel, "/")
	p := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/")))
	if trailing {
		p += string(filepath.Separator)
	}
	return p
}

// pathForm is one way of naming a case to osfs, and the native path osfs
// should resolve it to, which is what the os package is given.
type pathForm struct {
	name   string // passed to osfs
	native string // passed to os
}

// pathForms returns the ways a caller can name native path p to osfs: as an
// absfs path, and (on Windows, where they differ) as the native path itself
// and as basefs builds it, a native prefix joined to an absfs suffix with
// path.Join, which osfs passes through unchanged.
func pathForms(root, rel, p string) map[string]pathForm {
	forms := map[string]pathForm{"absfs": {FromNative(p), p}}
	if FromNative(p) != p {
		mixed := root + "/" + rel
		forms["native"] = pathForm{p, p}
		forms["mixed"] = pathForm{mixed, mixed}
	}
	return forms
}

// TestReadDirMatchesOS lists each case through osfs and through the os
// package and requires the same entries and equivalent errors: the same
// errno, the same path, and the same answer from errors.Is for the fs error
// kinds. It covers FileSystem.ReadDir and, through Open, File.ReadDir,
// File.Readdir and File.Readdirnames.
func TestReadDirMatchesOS(t *testing.T) {
	root, symlinks := readDirTree(t)
	ofs, err := NewFS()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range readDirCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.symlink && !symlinks {
				t.Skip("symlinks unavailable")
			}
			p := nativeCase(root, tc.rel)
			for form, pf := range pathForms(root, tc.rel, p) {
				t.Run(form, func(t *testing.T) {
					got, gotErr := ofs.ReadDir(pf.name)
					want, wantErr := os.ReadDir(pf.native)
					sameErr(t, "ReadDir", gotErr, wantErr)
					sameEntries(t, "ReadDir", got, want)

					// Each File method gets fresh handles, so a failed read
					// on one cannot leave the next reading from mid-listing.
					open := func(what string) (*File, *os.File, bool) {
						t.Helper()
						gf, gotErr := ofs.Open(pf.name)
						wf, wantErr := os.Open(pf.native)
						sameErr(t, what+" Open", gotErr, wantErr)
						if gotErr != nil || wantErr != nil {
							if gotErr == nil {
								gf.Close()
							}
							if wantErr == nil {
								wf.Close()
							}
							return nil, nil, false
						}
						t.Cleanup(func() { gf.Close(); wf.Close() })
						return gf.(*File), wf, true
					}

					if gf, wf, ok := open("File.ReadDir"); ok {
						got, gotErr := gf.ReadDir(-1)
						want, wantErr := wf.ReadDir(-1)
						sameErr(t, "File.ReadDir", gotErr, wantErr)
						sameEntries(t, "File.ReadDir", got, want)
					}
					if gf, wf, ok := open("File.Readdir"); ok {
						got, gotErr := gf.Readdir(-1)
						want, wantErr := wf.Readdir(-1)
						sameErr(t, "File.Readdir", gotErr, wantErr)
						sameNames(t, "File.Readdir", infoNames(got), infoNames(want))
					}
					if gf, wf, ok := open("File.Readdirnames"); ok {
						got, gotErr := gf.Readdirnames(-1)
						want, wantErr := wf.Readdirnames(-1)
						sameErr(t, "File.Readdirnames", gotErr, wantErr)
						sameNames(t, "File.Readdirnames", got, want)
					}
					if gf, wf, ok := open("File.ReadDir(1)"); ok {
						got, gotErr := pagedNames(gf)
						want, wantErr := pagedNames(wf)
						sameErr(t, "File.ReadDir(1)", gotErr, wantErr)
						sameNames(t, "File.ReadDir(1)", got, want)
					}
				})
			}
		})
	}
}

// TestReadDirSorted checks a directory big enough for sortDirEntries'
// quicksort path comes back in os.ReadDir's order.
func TestReadDirSorted(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 64; i++ {
		name := fmt.Sprintf("f%02d", (i*37)%64)
		if i%5 == 0 {
			if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ofs, err := NewFS()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ofs.ReadDir(FromNative(dir))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	sameEntries(t, "ReadDir", got, want)
}

// errKinds are the fs error kinds a caller tests with errors.Is.
var errKinds = []error{fs.ErrNotExist, fs.ErrExist, fs.ErrPermission, fs.ErrInvalid, fs.ErrClosed}

// sameErr fails t unless got is equivalent to want, the os package's error:
// both nil, or both *fs.PathError naming the same path, with the same errno
// and the same errors.Is answer for each of errKinds. The Op may differ;
// Linux's getdents reader fails a regular file at open, os at readdirent.
func sameErr(t *testing.T, what string, got, want error) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Errorf("%s: error %v, os gives %v", what, got, want)
		return
	}
	if want == nil {
		return
	}
	var gpe, wpe *fs.PathError
	if errors.As(got, &gpe) != errors.As(want, &wpe) {
		t.Errorf("%s: error %#v, os gives %#v", what, got, want)
	} else if wpe != nil && gpe.Path != wpe.Path {
		t.Errorf("%s: error path %q, os gives %q", what, gpe.Path, wpe.Path)
	}
	var gno, wno syscall.Errno
	if errors.As(got, &gno) != errors.As(want, &wno) || gno != wno {
		t.Errorf("%s: errno %d (%v), os gives %d (%v)", what, uintptr(gno), got, uintptr(wno), want)
	}
	for _, kind := range errKinds {
		if g, w := errors.Is(got, kind), errors.Is(want, kind); g != w {
			t.Errorf("%s: errors.Is(%v, %v) = %v, os gives %v", what, got, kind, g, w)
		}
	}
}

// sameEntries fails t unless got lists the same entries as want, in the same
// order, with the same type bits and the same Lstat answer from Info.
func sameEntries(t *testing.T, what string, got, want []fs.DirEntry) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: %d entries %v, os gives %d %v", what, len(got), entryNames(got), len(want), entryNames(want))
		return
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Name() != w.Name() || g.IsDir() != w.IsDir() || g.Type() != w.Type() {
			t.Errorf("%s: entry %d is %q dir=%v type=%v, os gives %q dir=%v type=%v",
				what, i, g.Name(), g.IsDir(), g.Type(), w.Name(), w.IsDir(), w.Type())
			continue
		}
		gi, gerr := g.Info()
		wi, werr := w.Info()
		sameErr(t, what+" Info "+w.Name(), gerr, werr)
		if gerr == nil && werr == nil && (gi.Name() != wi.Name() || gi.Mode() != wi.Mode()) {
			t.Errorf("%s: Info %q mode %v, os gives %q mode %v", what, gi.Name(), gi.Mode(), wi.Name(), wi.Mode())
		}
	}
}

func sameNames(t *testing.T, what string, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("%s: names %q, os gives %q", what, got, want)
	}
}

func entryNames(entries []fs.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func infoNames(infos []fs.FileInfo) []string {
	names := make([]string, len(infos))
	for i, fi := range infos {
		names[i] = fi.Name()
	}
	return names
}

// pagedNames reads f one entry at a time to io.EOF, which it reports as nil.
func pagedNames(f interface {
	ReadDir(int) ([]fs.DirEntry, error)
}) ([]string, error) {
	var names []string
	for {
		entries, err := f.ReadDir(1)
		if err == io.EOF {
			return names, nil
		}
		if err != nil {
			return names, err
		}
		names = append(names, entryNames(entries)...)
	}
}
