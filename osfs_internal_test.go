package osfs

import (
	"bytes"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
)

// TestFileMethods drives each File method against the os.File it wraps.
func TestFileMethods(t *testing.T) {
	dir := t.TempDir()
	ofs, err := NewFS()
	if err != nil {
		t.Fatal(err)
	}
	name := FromNative(filepath.Join(dir, "f.txt"))

	f, err := ofs.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if got := f.Name(); got != name {
		t.Errorf("Name() = %q, want %q", got, name)
	}
	if _, err := f.WriteString("hello world"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("HELLO"), 0); err != nil {
		t.Fatal(err)
	}
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	if _, err := f.ReadAt(buf, 6); err != nil || string(buf) != "world" {
		t.Errorf("ReadAt = %q, %v; want %q", buf, err, "world")
	}
	if off, err := f.Seek(0, io.SeekStart); err != nil || off != 0 {
		t.Errorf("Seek = %d, %v", off, err)
	}
	all, err := io.ReadAll(f)
	if err != nil || string(all) != "HELLO world" {
		t.Errorf("Read = %q, %v; want %q", all, err, "HELLO world")
	}
	if err := f.Truncate(5); err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil || info.Size() != 5 || info.Name() != "f.txt" {
		t.Errorf("Stat = %v, %v; want f.txt of 5 bytes", info, err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := ofs.ReadFile(name)
	if err != nil || !bytes.Equal(data, []byte("HELLO")) {
		t.Errorf("ReadFile = %q, %v; want %q", data, err, "HELLO")
	}

	d, err := ofs.Open(FromNative(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil || len(names) != 1 || names[0] != "f.txt" {
		t.Errorf("Readdirnames = %q, %v; want [f.txt]", names, err)
	}
}

// TestFileSystemErrorsMatchOS checks the FileSystem methods that only
// translate the path pass the os package's answer through.
func TestFileSystemErrorsMatchOS(t *testing.T) {
	dir := t.TempDir()
	ofs, err := NewFS()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing", "child")
	uid, gid := os.Getuid(), os.Getgid()

	cases := []struct {
		name string
		got  func() error
		want func() error
	}{
		{"Create in missing directory",
			func() error { _, err := ofs.Create(FromNative(missing)); return err },
			func() error { _, err := os.Create(missing); return err }},
		{"Readlink of a regular file",
			func() error { _, err := ofs.Readlink(FromNative(file)); return err },
			func() error { _, err := os.Readlink(file); return err }},
		{"Chown",
			func() error { return ofs.Chown(FromNative(file), uid, gid) },
			func() error { return os.Chown(file, uid, gid) }},
		{"Lchown",
			func() error { return ofs.Lchown(FromNative(file), uid, gid) },
			func() error { return os.Lchown(file, uid, gid) }},
		{"Chown missing",
			func() error { return ofs.Chown(FromNative(missing), uid, gid) },
			func() error { return os.Chown(missing, uid, gid) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sameErr(t, tc.name, tc.got(), tc.want())
		})
	}
}

func TestChdir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ofs, err := NewFS()
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{FromNative(filepath.Join(dir, "missing")), FromNative(file)} {
		if err := ofs.Chdir(bad); err == nil {
			t.Errorf("Chdir(%q) succeeded, want an error", bad)
		}
	}

	if err := ofs.Chdir(FromNative(dir)); err != nil {
		t.Fatal(err)
	}
	if wd, _ := ofs.Getwd(); wd != FromNative(dir) {
		t.Errorf("Getwd() = %q, want %q", wd, FromNative(dir))
	}
	// Relative and empty names resolve against the working directory.
	if _, err := ofs.Stat("file"); err != nil {
		t.Errorf("Stat(relative): %v", err)
	}
	entries, err := ofs.ReadDir("")
	if err != nil || len(entries) != 1 || entries[0].Name() != "file" {
		t.Errorf("ReadDir(\"\") = %v, %v; want [file]", entries, err)
	}
}

// TestNativePaths checks osfs accepts a native absolute path, and a native
// prefix joined to an absfs suffix with path.Join, as basefs builds them
// when it wraps osfs with a native directory, and resolves both to the same
// file as the absfs path. On Unix all three spellings are the same string.
func TestNativePaths(t *testing.T) {
	dir := t.TempDir()
	ofs, err := NewFS()
	if err != nil {
		t.Fatal(err)
	}
	if info, err := ofs.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("Stat(%q) = %v, %v; want a directory", dir, info, err)
	}
	forms := map[string]string{
		"absfs":  path.Join(FromNative(dir), "absfs"),
		"native": filepath.Join(dir, "native"),
		"mixed":  path.Join(dir, "mixed"), // C:\...\dir/mixed on Windows
	}
	for form, sub := range forms {
		t.Run(form, func(t *testing.T) {
			if err := ofs.Mkdir(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			f, err := ofs.Create(path.Join(sub, "f"))
			if err != nil {
				t.Fatal(err)
			}
			f.Close()
			if _, err := os.Stat(filepath.Join(dir, form, "f")); err != nil {
				t.Errorf("file not at the native location: %v", err)
			}
			entries, err := ofs.ReadDir(sub)
			if err != nil || len(entries) != 1 || entries[0].Name() != "f" {
				t.Errorf("ReadDir(%q) = %v, %v; want [f]", sub, entries, err)
			}
		})
	}
}

func TestIsNativePath(t *testing.T) {
	windows := runtime.GOOS == "windows"
	cases := []struct {
		path string
		want bool // on Windows; never native on Unix
	}{
		{"", false},
		{"c", false},
		{"C:", true},
		{`C:\`, true},
		{"c:/foo", true},
		{`D:\a\b`, true},
		{"C:foo", false},
		{`\\server\share`, true},
		{`\foo`, false},
		{"/c/foo", false},
		{"//server/share", false},
		{"foo", false},
		{"1:/foo", false},
	}
	for _, tc := range cases {
		want := tc.want && windows
		if got := isNativePath(tc.path); got != want {
			t.Errorf("isNativePath(%q) = %v, want %v", tc.path, got, want)
		}
	}
}

func TestSetDriveUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix has no drive letters")
	}
	for _, p := range []string{"/foo", "foo", "/c/foo"} {
		if got := SetDrive(p, "d"); got != p {
			t.Errorf("SetDrive(%q, d) = %q, want it unchanged", p, got)
		}
	}
}

func TestUNCEdges(t *testing.T) {
	if s, sh, r := SplitUNC("//server/share/"); s != "server" || sh != "share" || r != "/" {
		t.Errorf("SplitUNC(//server/share/) = %q %q %q", s, sh, r)
	}
	if got := JoinUNC("server", "share", "foo"); got != "//server/share/foo" {
		t.Errorf("JoinUNC(server, share, foo) = %q", got)
	}
}
