//go:build linux || darwin

package osfs

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestReadDirSpecialFilesMatchOS lists a FIFO and a Unix socket, and the
// device files in /dev, through osfs and os.ReadDir and compares the types.
func TestReadDirSpecialFilesMatchOS(t *testing.T) {
	// A short directory keeps the socket path within sun_path's limit.
	dir, err := os.MkdirTemp("", "osfs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if err := syscall.Mkfifo(filepath.Join(dir, "fifo"), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", filepath.Join(dir, "sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	ofs, err := NewFS()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ofs.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	sameEntries(t, "ReadDir", got, want)

	// /dev changes as terminals come and go, so compare entry by entry.
	devs, err := ofs.ReadDir("/dev")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range devs {
		info, err := os.Lstat(filepath.Join("/dev", d.Name()))
		if err != nil {
			continue
		}
		if d.Type() != info.Mode().Type() {
			t.Errorf("/dev/%s: type %v, os gives %v", d.Name(), d.Type(), info.Mode().Type())
		}
	}
}
