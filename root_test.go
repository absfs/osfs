//go:build go1.25 && unix

package osfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/absfs/fstesting"
)

// rootEnv is a host directory parent holding a secret file and the root of
// a RootFS. dir is the root's path with symlinks evaluated, so that
// macOS's /var -> /private/var does not make it an alias.
type rootEnv struct {
	parent, dir, secret string
	r                   *RootFS
}

func newRootEnv(t *testing.T) *rootEnv {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &rootEnv{parent: parent, dir: filepath.Join(parent, "root"), secret: filepath.Join(parent, "secret")}
	if err := os.WriteFile(e.secret, []byte("PARENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(e.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	e.r, err = NewRootFS(e.dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.r.Close() })
	return e
}

// host returns rel's host path below the root.
func (e *rootEnv) host(rel string) string {
	return filepath.Join(e.dir, rel)
}

func (e *rootEnv) write(t *testing.T, rel, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(e.host(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.host(rel), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (e *rootEnv) link(t *testing.T, target, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(e.host(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, e.host(rel)); err != nil {
		t.Fatal(err)
	}
}

// secretIntact fails the test if anything changed the secret outside the
// root.
func (e *rootEnv) secretIntact(t *testing.T) {
	t.Helper()
	info, err := os.Stat(e.secret)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(e.secret)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "PARENT" || info.Mode().Perm() != 0o644 || info.ModTime().Year() < 2020 {
		t.Errorf("secret outside the root changed: %q, %v, %v", data, info.Mode(), info.ModTime())
	}
}

func wantEscape(t *testing.T, what string, err error) {
	t.Helper()
	if !errors.Is(err, ErrPathEscapes) {
		t.Errorf("%s = %v, want ErrPathEscapes", what, err)
	}
}

// TestRootFSHostFollowEscapes reproduces the ways a link inside a directory
// leads the host out of it. Through osfs the host follows each one to the
// secret beside the root; through RootFS every operation that would follow
// it fails with ErrPathEscapes and the secret is untouched.
func TestRootFSHostFollowEscapes(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, e *rootEnv)
		probe func(e *rootEnv) string
	}{
		{"relative link climbing out", func(t *testing.T, e *rootEnv) {
			e.link(t, "../secret", "l")
		}, probe("l")},
		// The basefs case: "../../secret" was inside the root at a/b/l,
		// until a/b moved shallower.
		{"relative link moved shallower", func(t *testing.T, e *rootEnv) {
			e.link(t, "../../secret", "a/b/l")
			if err := os.Rename(e.host("a/b"), e.host("b")); err != nil {
				t.Fatal(err)
			}
		}, probe("b/l")},
		// "../secret" made through a/up, a link to the root, lands in
		// the root itself.
		{"link under a linked directory", func(t *testing.T, e *rootEnv) {
			e.link(t, "..", "a/up")
			e.link(t, "../secret", "a/up/evil")
		}, probe("a/up/evil")},
		{"dotdot after a link in the target", func(t *testing.T, e *rootEnv) {
			e.link(t, "..", "a/top")
			e.link(t, "top/../secret", "a/evil")
		}, probe("a/evil")},
		{"absolute link out of the root", func(t *testing.T, e *rootEnv) {
			e.link(t, e.secret, "abs")
		}, probe("abs")},
		{"absolute link detouring through the root", func(t *testing.T, e *rootEnv) {
			e.link(t, e.dir+"/../secret", "abs")
		}, probe("abs")},
		{"absolute link to a sibling sharing the root's name", func(t *testing.T, e *rootEnv) {
			if err := os.WriteFile(e.dir+"foo", []byte("PARENT"), 0o644); err != nil {
				t.Fatal(err)
			}
			e.link(t, e.dir+"foo", "abs")
		}, probe("abs")},
		{"directory link out of the root", func(t *testing.T, e *rootEnv) {
			e.link(t, "..", "up")
		}, probe("up/secret")},
		{"absolute directory link to /", func(t *testing.T, e *rootEnv) {
			e.link(t, "/", "slash")
		}, func(e *rootEnv) string { return "slash" + e.secret }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newRootEnv(t)
			c.setup(t, e)
			probe := c.probe(e)
			host := e.dir + "/" + probe

			// The host follows the link out: the escape is real.
			if data, err := os.ReadFile(host); err != nil || string(data) != "PARENT" {
				t.Fatalf("os.ReadFile(%q) = %q, %v; the setup no longer escapes", probe, data, err)
			}

			r := e.r
			_, err := r.ReadFile(host)
			wantEscape(t, "ReadFile", err)
			_, err = r.Open(host)
			wantEscape(t, "Open", err)
			_, err = r.Stat(host)
			wantEscape(t, "Stat", err)
			_, err = r.OpenFile(host, os.O_WRONLY|os.O_APPEND, 0)
			wantEscape(t, "OpenFile", err)
			_, err = r.Create(host)
			wantEscape(t, "Create", err)
			wantEscape(t, "Truncate", r.Truncate(host, 0))
			wantEscape(t, "Chmod", r.Chmod(host, 0o600))
			wantEscape(t, "Chtimes", r.Chtimes(host, time.Unix(1, 0), time.Unix(1, 0)))
			wantEscape(t, "Chown", r.Chown(host, os.Getuid(), os.Getgid()))
			e.secretIntact(t)
		})
	}
}

// probe returns a fixed probe name.
func probe(name string) func(*rootEnv) string {
	return func(*rootEnv) string { return name }
}

// TestRootFSDirectorySwapRace swaps a directory inside the root for a
// symlink out of it while reads go on. With osfs some reads land outside;
// with RootFS none may.
func TestRootFSDirectorySwapRace(t *testing.T) {
	e := newRootEnv(t)
	e.write(t, "real/secret", "INSIDE")
	if err := os.Symlink("..", e.host("up")); err != nil {
		t.Fatal(err)
	}
	// d alternates between the directory real (by rename) and up, a link
	// to the root's parent.
	var stop atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			os.Rename(e.host("real"), e.host("d"))
			os.Rename(e.host("d"), e.host("real"))
			os.Rename(e.host("up"), e.host("d"))
			os.Rename(e.host("d"), e.host("up"))
		}
	}()
	defer func() {
		stop.Store(true)
		wg.Wait()
	}()

	host := e.host("d/secret")
	deadline := time.Now().Add(300 * time.Millisecond)
	osOut, rootIn := false, false
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(host); err == nil && string(data) == "PARENT" {
			osOut = true
		}
		data, err := e.r.ReadFile(host)
		if err == nil && string(data) == "PARENT" {
			t.Fatal("RootFS read the secret outside the root")
		}
		if err == nil {
			rootIn = true
		}
	}
	t.Logf("os escaped during the race: %v; RootFS read inside: %v", osOut, rootIn)
}

// TestRootFSAbsoluteLinksInside checks that an absolute link naming a place
// inside the root is followed there, for every operation, including links
// on the way to the name.
func TestRootFSAbsoluteLinksInside(t *testing.T) {
	e := newRootEnv(t)
	r := e.r
	e.write(t, "sub/f", "INSIDE")
	e.link(t, e.dir+"/sub", "d")     // directory link
	e.link(t, e.dir+"/sub/f", "fl")  // file link
	e.link(t, e.dir, "top")          // link to the root itself
	e.link(t, e.dir+"/new", "dl")    // dangling
	e.link(t, "../sub/f", "sub2/rl") // relative, inside

	read := func(name, want string) {
		t.Helper()
		data, err := r.ReadFile(e.host(name))
		if err != nil || string(data) != want {
			t.Errorf("ReadFile(%q) = %q, %v; want %q", name, data, err, want)
		}
	}
	read("fl", "INSIDE")
	read("d/f", "INSIDE")
	read("top/top/d/f", "INSIDE")
	read("top/sub2/rl", "INSIDE")

	if info, err := r.Stat(e.host("d")); err != nil || !info.IsDir() {
		t.Errorf("Stat(d) = %v, %v", info, err)
	}
	if info, err := r.Lstat(e.host("d")); err != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("Lstat(d) = %v, %v", info, err)
	}
	if info, err := r.Lstat(e.host("d/f")); err != nil || !info.Mode().IsRegular() {
		t.Errorf("Lstat(d/f) = %v, %v", info, err)
	}
	f, err := r.Open(e.host("d/f"))
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := io.ReadAll(f); string(data) != "INSIDE" {
		t.Errorf("Open(d/f) read %q", data)
	}
	f.Close()

	// Writes through links land inside.
	f, err = r.Create(e.host("dl"))
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("CREATED")
	f.Close()
	read("new", "CREATED")
	f, err = r.OpenFile(e.host("d/g"), os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := os.Stat(e.host("sub/g")); err != nil {
		t.Errorf("OpenFile(d/g) did not create sub/g: %v", err)
	}
	if err := r.Truncate(e.host("fl"), 2); err != nil {
		t.Error(err)
	}
	read("sub/f", "IN")
	if err := r.Chmod(e.host("fl"), 0o600); err != nil {
		t.Error(err)
	}
	if info, _ := os.Stat(e.host("sub/f")); info.Mode().Perm() != 0o600 {
		t.Errorf("Chmod(fl) left sub/f %v", info.Mode())
	}
	when := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := r.Chtimes(e.host("fl"), when, when); err != nil {
		t.Error(err)
	}
	if info, _ := os.Stat(e.host("sub/f")); !info.ModTime().Equal(when) {
		t.Errorf("Chtimes(fl) left sub/f at %v", info.ModTime())
	}
	if err := r.Chown(e.host("fl"), os.Getuid(), os.Getgid()); err != nil {
		t.Error(err)
	}
	if err := r.Lchown(e.host("d/f"), os.Getuid(), os.Getgid()); err != nil {
		t.Error(err)
	}

	// Directory operations beneath a directory link.
	if err := r.Mkdir(e.host("d/m"), 0o755); err != nil {
		t.Error(err)
	}
	if err := r.MkdirAll(e.host("d/m/n/o"), 0o755); err != nil {
		t.Error(err)
	}
	if err := r.Symlink("f", e.host("d/sl")); err != nil {
		t.Error(err)
	}
	if target, err := r.Readlink(e.host("d/sl")); err != nil || target != "f" {
		t.Errorf("Readlink(d/sl) = %q, %v", target, err)
	}
	if target, err := r.Readlink(e.host("fl")); err != nil || target != e.dir+"/sub/f" {
		t.Errorf("Readlink(fl) = %q, %v; want the target as stored", target, err)
	}
	if err := r.Rename(e.host("d/sl"), e.host("d/sl2")); err != nil {
		t.Error(err)
	}
	if err := r.Remove(e.host("d/sl2")); err != nil {
		t.Error(err)
	}
	if err := r.RemoveAll(e.host("d/m")); err != nil {
		t.Error(err)
	}
	entries, err := r.ReadDir(e.host("d"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, de := range entries {
		names = append(names, de.Name())
	}
	if !slices.Equal(names, []string{"f", "g"}) {
		t.Errorf("ReadDir(d) = %v", names)
	}

	// Missing elements after a link are reported as missing.
	if _, err := r.Stat(e.host("d/missing/x")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(d/missing/x) = %v, want ErrNotExist", err)
	}
	// O_NOFOLLOW refuses a final link without following it.
	if _, err := r.OpenFile(e.host("fl"), os.O_RDONLY|syscall.O_NOFOLLOW, 0); err == nil || errors.Is(err, ErrPathEscapes) {
		t.Errorf("OpenFile(fl, O_NOFOLLOW) = %v, want a refusal that is not an escape", err)
	}
}

func TestRootFSAlias(t *testing.T) {
	e := newRootEnv(t)
	e.write(t, "f", "INSIDE")
	alias := filepath.Join(e.parent, "alias")
	if err := os.Symlink("root", alias); err != nil {
		t.Fatal(err)
	}
	r, err := NewRootFS(alias)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if r.Root() != alias {
		t.Errorf("Root() = %q, want %q", r.Root(), alias)
	}
	// A link written with the evaluated path, and names given either way.
	e.link(t, e.dir+"/f", "real")
	for _, name := range []string{alias + "/real", e.dir + "/real", alias + "/f", e.dir + "/f"} {
		if data, err := r.ReadFile(name); err != nil || string(data) != "INSIDE" {
			t.Errorf("ReadFile(%q) = %q, %v", name, data, err)
		}
	}
}

func TestRootFSNames(t *testing.T) {
	e := newRootEnv(t)
	r := e.r
	e.write(t, "a/f", "A")

	for _, name := range []string{e.secret, "../secret", "/", e.dir + "/../secret", e.dir + "x"} {
		_, err := r.Stat(name)
		wantEscape(t, "Stat("+name+")", err)
		var pe *os.PathError
		if errors.As(err, &pe) && pe.Path != name {
			t.Errorf("Stat(%q) error names %q", name, pe.Path)
		}
	}
	wantEscape(t, "Rename out", r.Rename(e.host("a/f"), e.secret+"2"))
	wantEscape(t, "Rename in", r.Rename(e.secret, e.host("s")))
	err := r.Symlink("x", e.secret+"3")
	wantEscape(t, "Symlink", err)
	var le *os.LinkError
	if !errors.As(err, &le) || le.New != e.secret+"3" {
		t.Errorf("Symlink error = %#v, want an *os.LinkError naming the caller's names", err)
	}

	// Errors name the caller's name, not the root-relative one.
	_, err = r.Stat(e.host("missing"))
	var pe *os.PathError
	if !errors.As(err, &pe) || pe.Path != e.host("missing") || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(missing) = %v", err)
	}
	err = r.Rename(e.host("missing"), e.host("other"))
	if !errors.As(err, &le) || !errors.Is(err, fs.ErrNotExist) || le.Old != e.host("missing") {
		t.Errorf("Rename(missing) = %v", err)
	}

	// The working directory.
	if wd, _ := r.Getwd(); wd != e.dir {
		t.Errorf("Getwd() = %q, want %q", wd, e.dir)
	}
	if err := r.Chdir("a"); err != nil {
		t.Fatal(err)
	}
	if wd, _ := r.Getwd(); wd != e.dir+"/a" {
		t.Errorf("Getwd() = %q after Chdir(a)", wd)
	}
	if data, err := r.ReadFile("f"); err != nil || string(data) != "A" {
		t.Errorf("ReadFile(f) in a = %q, %v", data, err)
	}
	if err := r.Chdir("f"); err == nil {
		t.Error("Chdir(f) succeeded on a file")
	}
	if err := r.Chdir("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Chdir(missing) = %v", err)
	}
	wantEscape(t, "Chdir(../..)", r.Chdir("../.."))
	if err := r.Chdir(e.dir); err != nil {
		t.Error(err)
	}

	if r.TempDir() != os.TempDir() {
		t.Errorf("TempDir() = %q", r.TempDir())
	}
	sub, err := r.Sub(e.host("a"))
	if err != nil {
		t.Fatal(err)
	}
	if data, err := fs.ReadFile(sub, "f"); err != nil || string(data) != "A" {
		t.Errorf("Sub(a): ReadFile(f) = %q, %v", data, err)
	}
	if _, err := r.ReadDir(e.host("missing")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadDir(missing) = %v", err)
	}
	if err := r.Truncate(e.host("missing"), 0); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Truncate(missing) = %v", err)
	}
	if err := r.Truncate(e.host("a"), 0); err == nil {
		t.Error("Truncate(a) succeeded on a directory")
	}
}

func TestRootFSLoops(t *testing.T) {
	e := newRootEnv(t)
	e.link(t, e.dir+"/b", "a")
	e.link(t, e.dir+"/a", "b")
	if _, err := e.r.Stat(e.host("a")); !errors.Is(err, syscall.ELOOP) {
		t.Errorf("Stat(a) = %v, want ELOOP", err)
	}
	// A dangling link after a missing element is left for the operation.
	e.link(t, "missing/../../x", "c")
	if _, err := e.r.Stat(e.host("c")); err == nil {
		t.Error("Stat(c) succeeded")
	}
}

func TestRootFSClosedAndBadRoots(t *testing.T) {
	if _, err := NewRootFS(""); !errors.Is(err, os.ErrInvalid) {
		t.Errorf(`NewRootFS("") = %v`, err)
	}
	if _, err := NewRootFS(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("NewRootFS(missing) = %v", err)
	}
	e := newRootEnv(t)
	e.link(t, e.dir, "top")
	if err := e.r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.r.Stat(e.host("top/x")); err == nil {
		t.Error("Stat after Close succeeded")
	}

	// A relative root is taken from the process's working directory.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRootFS(".")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if r.Root() != wd {
		t.Errorf("Root() = %q, want %q", r.Root(), wd)
	}
}

func TestRootFSAtSlash(t *testing.T) {
	r, err := NewRootFS("/")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	dir := t.TempDir()
	if info, err := r.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("Stat(%q) = %v, %v", dir, info, err)
	}
	if info, err := r.Stat("/"); err != nil || !info.IsDir() {
		t.Errorf("Stat(/) = %v, %v", info, err)
	}
}

func TestOpenRootOsFS(t *testing.T) {
	e := newRootEnv(t)
	e.write(t, "f", "INSIDE")
	o, err := NewFS()
	if err != nil {
		t.Fatal(err)
	}
	if err := o.Chdir(e.parent); err != nil {
		t.Fatal(err)
	}
	sfs, err := o.OpenRoot("root")
	if err != nil {
		t.Fatal(err)
	}
	defer sfs.(*RootFS).Close()
	if data, err := sfs.ReadFile(e.host("f")); err != nil || string(data) != "INSIDE" {
		t.Errorf("ReadFile(f) = %q, %v", data, err)
	}
	if _, err := o.OpenRoot("missing"); err == nil {
		t.Error("OpenRoot(missing) succeeded")
	}
}

// TestRootFSSuite runs the fstesting suite against a RootFS.
func TestRootFSSuite(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRootFS(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	features := fstesting.OSFeatures()
	if runtime.GOOS == "darwin" {
		features.CaseSensitive = false
	}
	suite := &fstesting.Suite{FS: r, Features: features, TestDir: dir}
	suite.Run(t)
}
