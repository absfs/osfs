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

	testpath = abs

	t.Run("Walk", func(t *testing.T) {
		list := make(map[string]bool)
		count := 0
		err = filepath.Walk(testpath, func(path string, info os.FileInfo, err error) error {
			p := strings.TrimPrefix(path, testpath)
			if p == "" {
				p = "/"
			}
			list[p] = true
			count++
			return nil
		})

		count2 := 0
		err = fs.Walk(testpath, func(path string, info os.FileInfo, err error) error {
			p := strings.TrimPrefix(path, testpath)
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
		if ofs.Separator() != filepath.Separator {
			t.Errorf("incorrect separator %q", ofs.Separator())
		}
		if ofs.ListSeparator() != filepath.ListSeparator {
			t.Errorf("incorrect list separator %q", ofs.ListSeparator())
		}
	})

	t.Run("Navigation", func(t *testing.T) {

		oswd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		fswd, err := ofs.Getwd()
		if fswd != oswd {
			t.Fatalf("incorrect working directory %q, %q", fswd, oswd)
		}

		cwd := "/"
		err = ofs.Chdir(cwd)
		if err != nil {
			t.Fatal(err)
		}

		fswd, err = ofs.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if fswd != cwd {
			t.Fatalf("incorrect working directory %q, %q", fswd, cwd)
		}

	})

	t.Run("TempDir", func(t *testing.T) {

		ostmp, fstmp := os.TempDir(), ofs.TempDir()
		if fstmp != ostmp {
			t.Fatalf("wrong TempDir output: %q != %q", fstmp, ostmp)
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
		Features: fstesting.DefaultFeatures(),
	}

	suite.Run(t)
}
