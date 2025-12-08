package osfs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/absfs/absfs"
	"github.com/absfs/fstesting"
	"github.com/absfs/osfs"
)

func TestInterface(t *testing.T) {
	testfs, err := osfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}

	var fs absfs.SymlinkFileSystem
	fs = testfs
	_ = fs
}

func TestWalk(t *testing.T) {
	fs, err := osfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}
	testpath := ".."
	abs, err := filepath.Abs(testpath)
	if err != nil {
		t.Fatal(err)
	}

	// Convert native path to Unix-style for comparison
	testpathUnix := osfs.FromNative(abs)

	t.Run("Walk", func(t *testing.T) {
		list := make(map[string]bool)
		count := 0
		err = filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
			// Convert to Unix-style and trim prefix
			unixPath := osfs.FromNative(path)
			p := strings.TrimPrefix(unixPath, testpathUnix)
			if p == "" {
				p = "/"
			}
			list[p] = true
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("filepath.Walk failed: %v", err)
		}

		count2 := 0
		// fs.Walk expects Unix-style path and returns Unix-style paths
		err = fs.Walk(testpathUnix, func(path string, info os.FileInfo, err error) error {
			p := strings.TrimPrefix(path, testpathUnix)
			if p == "" {
				p = "/"
			}
			if !list[p] {
				return fmt.Errorf("file not found %q", p)
			}
			delete(list, p)
			count2++
			return nil
		})
		if err != nil {
			t.Error(err)
		}
		if count < 10 || count != count2 {
			t.Errorf("incorrect file count: %d, %d", count, count2)
		}
		if len(list) > 0 {
			for k := range list {
				t.Errorf("path not removed %q", k)
			}
		}
	})
}

func TestOSFS(t *testing.T) {
	var ofs absfs.FileSystem

	t.Run("NewFs", func(t *testing.T) {
		fs, err := osfs.NewFS()
		if err != nil {
			t.Fatal(err)
		}
		ofs = fs
	})

	t.Run("Separators", func(t *testing.T) {
		// absfs always uses Unix-style separators regardless of platform
		if ofs.Separator() != '/' {
			t.Errorf("Separator() = %q, want '/'", ofs.Separator())
		}
		if ofs.ListSeparator() != ':' {
			t.Errorf("ListSeparator() = %q, want ':'", ofs.ListSeparator())
		}
	})

	t.Run("Navigation", func(t *testing.T) {
		oswd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		// ofs.Getwd() returns Unix-style path
		expectedWd := osfs.FromNative(oswd)

		fswd, err := ofs.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if fswd != expectedWd {
			t.Fatalf("Getwd() = %q, want %q", fswd, expectedWd)
		}

		// Change to root - on Windows this should stay on current drive
		err = ofs.Chdir("/")
		if err != nil {
			t.Fatal(err)
		}

		fswd, err = ofs.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		// Get expected root: on Unix it's "/", on Windows it's "/c/" (drive from cwd)
		expectedRoot := "/"
		if drive := osfs.GetDrive(expectedWd); drive != "" {
			expectedRoot = "/" + drive + "/"
		}

		if fswd != expectedRoot {
			t.Fatalf("after Chdir(/), Getwd() = %q, want %q", fswd, expectedRoot)
		}
	})

	t.Run("TempDir", func(t *testing.T) {
		ostmp := os.TempDir()
		expectedTmp := osfs.FromNative(ostmp)

		fstmp := ofs.TempDir()
		if fstmp != expectedTmp {
			t.Fatalf("TempDir() = %q, want %q", fstmp, expectedTmp)
		}
	})
}

func TestOSFSSuite(t *testing.T) {
	fs, err := osfs.NewFS()
	if err != nil {
		t.Fatal(err)
	}

	suite := &fstesting.Suite{
		FS:       fs,
		Features: fstesting.OSFeatures(),
	}

	suite.Run(t)
}
