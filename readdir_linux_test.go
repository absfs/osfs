//go:build linux && !appengine

package osfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
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

func TestIgnoringEINTR(t *testing.T) {
	calls := 0
	n, err := ignoringEINTR(func() (int, error) {
		calls++
		if calls < 3 {
			return -1, syscall.EINTR
		}
		return 7, nil
	})
	if n != 7 || err != nil || calls != 3 {
		t.Errorf("ignoringEINTR = %d, %v after %d calls; want 7, nil after 3", n, err, calls)
	}

	calls = 0
	_, err = ignoringEINTR(func() (int, error) {
		calls++
		return -1, syscall.EIO
	})
	if err != syscall.EIO || calls != 1 {
		t.Errorf("ignoringEINTR = %v after %d calls; want EIO after 1", err, calls)
	}
}

// childFDEnv names the descriptor TestChildFDHelper reports on.
const childFDEnv = "OSFS_TEST_CHILD_FD"

// TestChildFDHelper is not a test. Run as a child process by childFDTarget,
// it prints what its descriptor $OSFS_TEST_CHILD_FD refers to, as seen in
// /proc/self/fd, or "closed".
func TestChildFDHelper(t *testing.T) {
	n := os.Getenv(childFDEnv)
	if n == "" {
		t.Skip("helper process only")
	}
	target, err := os.Readlink("/proc/self/fd/" + n)
	if err != nil {
		target = "closed"
	}
	fmt.Printf("childfd=%s\n", target)
}

// childFDTarget starts this test binary as a child process and returns what
// descriptor fd refers to in the child, or "closed" if the child did not
// inherit it.
func childFDTarget(t *testing.T, fd int) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestChildFDHelper$", "-test.count=1")
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%d", childFDEnv, fd))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if target, ok := strings.CutPrefix(line, "childfd="); ok {
			return target
		}
	}
	t.Fatalf("child printed no childfd line:\n%s", out)
	return ""
}

// TestOpenDirCloseOnExec checks that a process started while ReadDir has its
// directory open does not inherit the descriptor.
func TestOpenDirCloseOnExec(t *testing.T) {
	if _, err := os.Stat("/proc/self/fd"); err != nil {
		t.Skipf("no /proc/self/fd: %v", err)
	}
	dir := t.TempDir()

	// A descriptor opened without O_CLOEXEC must show up in the child, or
	// the check below proves nothing.
	leak, err := syscall.Open(dir, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := childFDTarget(t, leak)
	syscall.Close(leak)
	if got != dir {
		t.Skipf("child does not see an inherited descriptor (got %q), cannot test", got)
	}

	fd, err := openDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	if got := childFDTarget(t, fd); got == dir {
		t.Errorf("child inherited openDir's descriptor %d for %s", fd, dir)
	}
}

// TestReadDirNotDirectory checks the errors for names that are not
// directories, which the getdents reader opens with O_DIRECTORY. The os
// package does so only from Go 1.24, so the op for a name that exists but is
// not a directory is whichever os.ReadDir gives a regular file: "open" from
// Go 1.24, "readdirent" before.
func TestReadDirNotDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	var osErr *fs.PathError
	if _, err := os.ReadDir(file); !errors.As(err, &osErr) {
		t.Fatalf("os.ReadDir(%q) = %v, want a PathError", file, err)
	}
	notDirOp := osErr.Op

	cases := []struct {
		name string
		path string
		op   string
		err  syscall.Errno
	}{
		{"regular file", file, notDirOp, syscall.ENOTDIR},
		{"child of regular file", filepath.Join(file, "child"), "open", syscall.ENOTDIR},
		// Before Go 1.24 os.ReadDir would block opening a FIFO with no writer.
		{"fifo", fifo, notDirOp, syscall.ENOTDIR},
		{"missing", filepath.Join(dir, "nope"), "open", syscall.ENOENT},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				_, err := readDirOptimized(tc.path)
				done <- err
			}()
			var err error
			select {
			case err = <-done:
			case <-time.After(10 * time.Second):
				t.Fatalf("readDirOptimized(%q) blocked", tc.path)
			}
			var pe *fs.PathError
			if !errors.As(err, &pe) || pe.Op != tc.op || pe.Path != tc.path || pe.Err != tc.err {
				t.Errorf("readDirOptimized(%q) = %#v; want PathError{%q, %q, %v}", tc.path, err, tc.op, tc.path, tc.err)
			}
		})
	}
}

// TestReadDirUnreadable checks that a regular file that cannot be opened
// fails as it does for os.ReadDir: before Go 1.24 at open with EACCES, from
// Go 1.24 at open with ENOTDIR.
func TestReadDirUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can open any file")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o200); err != nil {
		t.Fatal(err)
	}
	got, gotErr := readDirOptimized(file)
	want, wantErr := os.ReadDir(file)
	if wantErr == nil {
		t.Fatalf("os.ReadDir(%q) succeeded", file)
	}
	sameListing(t, "ReadDir", got, want, gotErr, wantErr)
}
