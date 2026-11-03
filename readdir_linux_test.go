//go:build linux && !appengine

package osfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestDirentType(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		dtype   uint8
		entry   string
		want    fs.FileMode
		wantOK  bool
		wantErr bool
	}{
		{"regular", syscall.DT_REG, "x", 0, true, false},
		{"directory", syscall.DT_DIR, "x", fs.ModeDir, true, false},
		{"symlink", syscall.DT_LNK, "x", fs.ModeSymlink, true, false},
		{"block device", syscall.DT_BLK, "x", fs.ModeDevice, true, false},
		{"char device", syscall.DT_CHR, "x", fs.ModeDevice | fs.ModeCharDevice, true, false},
		{"fifo", syscall.DT_FIFO, "x", fs.ModeNamedPipe, true, false},
		{"socket", syscall.DT_SOCK, "x", fs.ModeSocket, true, false},
		{"unknown d_type, file", syscall.DT_UNKNOWN, "file", 0, true, false},
		{"unknown d_type, directory", syscall.DT_UNKNOWN, "sub", fs.ModeDir, true, false},
		{"unknown d_type, removed", syscall.DT_UNKNOWN, "gone", 0, false, false},
		{"unknown d_type, lstat fails", syscall.DT_UNKNOWN, "bad\x00name", 0, false, true},
		{"unrecognised d_type", 99, "file", 0, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode, ok, err := direntType(tc.dtype, dir, tc.entry)
			if (err != nil) != tc.wantErr || ok != tc.wantOK || mode != tc.want {
				t.Errorf("direntType(%d, %q) = %v, %v, %v; want %v, %v, error %v",
					tc.dtype, tc.entry, mode, ok, err, tc.want, tc.wantOK, tc.wantErr)
			}
		})
	}
}
