//go:build windows

package osfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestFindPatternJoinNative(t *testing.T) {
	tests := []struct {
		dir, pattern, joined string
	}{
		{`C:\dir`, `C:\dir\*`, `C:\dir\x`},
		{`C:\dir\`, `C:\dir\*`, `C:\dir\x`},
		{`C:/dir/`, `C:/dir/*`, `C:/dir/x`},
		{`C:\`, `C:\*`, `C:\x`},
		{`C:`, `C:*`, `C:x`},            // drive-relative: the current directory on C:
		{`C:dir`, `C:dir\*`, `C:dir\x`}, // drive-relative subdirectory
		{`\\server\share`, `\\server\share\*`, `\\server\share\x`},
		{`\\server\share\`, `\\server\share\*`, `\\server\share\x`},
		{`dir`, `dir\*`, `dir\x`},
	}
	for _, tt := range tests {
		if got := findPattern(tt.dir); got != tt.pattern {
			t.Errorf("findPattern(%q) = %q, want %q", tt.dir, got, tt.pattern)
		}
		if got := joinNative(tt.dir, "x"); got != tt.joined {
			t.Errorf("joinNative(%q, x) = %q, want %q", tt.dir, got, tt.joined)
		}
	}
}

// TestReparseType checks the tag fallback entryType uses when Lstat fails
// against the os package's rules for the same attributes and tag.
func TestReparseType(t *testing.T) {
	const (
		dir     = syscall.FILE_ATTRIBUTE_DIRECTORY | syscall.FILE_ATTRIBUTE_REPARSE_POINT
		file    = syscall.FILE_ATTRIBUTE_REPARSE_POINT
		cloud   = 0x9000001a // IO_REPARSE_TAG_CLOUD_A, not a name surrogate
		symlink = syscall.IO_REPARSE_TAG_SYMLINK
		mount   = 0xa0000003 // IO_REPARSE_TAG_MOUNT_POINT
	)
	tests := []struct {
		name  string
		attrs uint32
		tag   uint32
		want  fs.FileMode
	}{
		{"file symlink", file, symlink, fs.ModeSymlink},
		{"directory symlink", dir, symlink, fs.ModeSymlink},
		{"junction", dir, mount, fs.ModeIrregular},
		{"socket", file, reparseTagAFUnix, fs.ModeSocket},
		{"dedup file", file, reparseTagDedup, 0},
		{"dedup directory", dir, reparseTagDedup, fs.ModeDir},
		{"cloud file", file, cloud, fs.ModeIrregular},
		{"cloud directory", dir, cloud, fs.ModeDir | fs.ModeIrregular},
	}
	for _, tt := range tests {
		if got := reparseType(tt.attrs, tt.tag); got != tt.want {
			t.Errorf("%s: reparseType(%#x, %#x) = %v, want %v", tt.name, tt.attrs, tt.tag, got, tt.want)
		}
	}
}

// TestEntryTypeVanished types reparse points that are gone by the time
// entryType looks at them, which is when it falls back to the tag.
func TestEntryTypeVanished(t *testing.T) {
	dir := t.TempDir()
	const attrs = syscall.FILE_ATTRIBUTE_DIRECTORY | syscall.FILE_ATTRIBUTE_REPARSE_POINT
	if got := entryType(dir, "gone", attrs, syscall.IO_REPARSE_TAG_SYMLINK); got != fs.ModeSymlink {
		t.Errorf("vanished directory symlink: type %v, want %v", got, fs.ModeSymlink)
	}
	if got := entryType(dir, "gone", syscall.FILE_ATTRIBUTE_DIRECTORY, 0); got != fs.ModeDir {
		t.Errorf("plain directory: type %v, want %v", got, fs.ModeDir)
	}
	if got := entryType(dir, "gone", 0, 0); got != 0 {
		t.Errorf("plain file: type %v, want 0", got)
	}
}

// TestReadDirDriveRelative lists a bare drive designator ("D:"), which the
// os package resolves to the current directory on that drive, not its root.
func TestReadDirDriveRelative(t *testing.T) {
	root, _ := readDirTree(t)
	dir := filepath.Join(root, "dir")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	})

	ofs, err := NewFS()
	if err != nil {
		t.Fatal(err)
	}
	vol := filepath.VolumeName(dir)

	want, wantErr := os.ReadDir(vol)
	if wantErr != nil {
		t.Fatalf("os.ReadDir(%q): %v", vol, wantErr)
	}
	// Make sure the case means something: os lists dir, not the drive root.
	inDir, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	sameNames(t, "os.ReadDir "+vol+" vs cwd", entryNames(want), entryNames(inDir))

	got, gotErr := ofs.ReadDir(vol)
	sameListing(t, "ReadDir "+vol, got, want, gotErr, wantErr)

	gi, gotErr := ofs.Stat(vol)
	wi, wantErr := os.Stat(vol)
	sameErr(t, "Stat "+vol, gotErr, wantErr)
	if gotErr == nil && wantErr == nil && !os.SameFile(gi, wi) {
		t.Errorf("Stat(%q) is not the file os.Stat gives", vol)
	}
}

// TestReadDirDriveRoot lists the root of the temp directory's drive, the
// one input whose pattern once doubled a separator.
func TestReadDirDriveRoot(t *testing.T) {
	ofs, err := NewFS()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.VolumeName(t.TempDir()) + `\`
	for _, name := range []string{root, FromNative(root)} {
		got, gotErr := ofs.ReadDir(name)
		want, wantErr := os.ReadDir(root)
		sameListing(t, "ReadDir "+name, got, want, gotErr, wantErr)
	}
}
