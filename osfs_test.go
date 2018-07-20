package osfs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/absfs/absfs"
	"github.com/absfs/fstesting"
	"github.com/absfs/osfs"
)

const O_ACCESS = 0x3

func TestOSFS(t *testing.T) {

	var testfs absfs.FileSystem

	t.Run("NewFs", func(t *testing.T) {
		fs, err := osfs.NewFs()
		if err != nil {
			t.Fatal(err)
		}

		testfs = fs
	})

	t.Run("Separators", func(t *testing.T) {
		if testfs.Separator() != filepath.Separator {
			t.Errorf("incorrect separator %q", testfs.Separator())
		}
		if testfs.ListSeparator() != filepath.ListSeparator {
			t.Errorf("incorrect list separator %q", testfs.ListSeparator())
		}
	})

	t.Run("Navigation", func(t *testing.T) {

		oswd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		fswd, err := testfs.Getwd()
		if fswd != oswd {
			t.Fatalf("incorrect working directory %q, %q", fswd, oswd)
		}

		cwd := "/"
		err = testfs.Chdir(cwd)
		if err != nil {
			t.Fatal(err)
		}

		fswd, err = testfs.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if fswd != cwd {
			t.Fatalf("incorrect working directory %q, %q", fswd, cwd)
		}

	})

	t.Run("TempDir", func(t *testing.T) {

		ostmp, fstmp := os.TempDir(), testfs.TempDir()
		if fstmp != ostmp {
			t.Fatalf("wrong TempDir output: %q != %q", fstmp, ostmp)
		}

	})

	// Move to the test directory for all further tests
	testdir, cleanup, err := fstesting.OsTestDir(os.TempDir())

	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	testcases, err := fstesting.GenerateTestcases(testdir, func(testcase *fstesting.Testcase) error {
		// Testcase counter
		// if testcase.TestNo > 10 {
		// 	return errors.New("stop")
		// }
		// fmt.Printf("  %10d %s%*s\r", testcase.TestNo, testcase.Path, len(testcase.Path)-100, " ")
		fmt.Printf("  %10d Generated Testcases\r", testcase.TestNo)
		return nil
	})
	if err != nil && err.Error() != "stop" {
		t.Fatal(err)
	}

	fmt.Printf("\n")
	if len(testcases) < 10 {
		t.Fatalf("Number of test cases too small: %d", len(testcases))
	}
	t.Logf("Number of test cases: %d", len(testcases))
	cleanup()

	tmp := filepath.Join(testfs.TempDir(), "osfs_test")
	if _, err := testfs.Stat(tmp); os.IsNotExist(err) {
		err = os.Mkdir(tmp, 0777)
		if err != nil {
			t.Fatal(err)
		}
	}
	testdir, cleanup, err = fstesting.FsTestDir(testfs, tmp)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}

	maxerrors := 10
	for _, testcase := range testcases {

		result, err := fstesting.FsTest(testfs, testdir, testcase)

		if testcase.OpenErr != nil {
			if result.OpenErr == nil {
				t.Fatalf("%d: %s, expected error %v", testcase.TestNo, testcase.Path, testcase.OpenErr)
			} else {
				err := fstesting.CompareErrors(result.OpenErr, testcase.OpenErr)
				if err != nil {
					t.Fatal(err)
				}
			}
		}

		if testcase.WriteErr != nil {
			if result.WriteErr == nil {
				t.Fatalf("%d: %s, expected error %v", testcase.TestNo, testcase.Path, testcase.WriteErr)
			} else {
				err := fstesting.CompareErrors(result.WriteErr, testcase.WriteErr)
				if err != nil {
					t.Fatal(err)
				}
			}
		}

		if testcase.ReadErr != nil {
			if result.ReadErr == nil {
				t.Fatalf("%d: %s, expected error %v", testcase.TestNo, testcase.Path, testcase.ReadErr)
			} else {
				err := fstesting.CompareErrors(result.ReadErr, testcase.ReadErr)
				if err != nil {
					t.Fatal(err)
				}
			}
		}

		if testcase.CloseErr != nil {
			if result.CloseErr == nil {
				t.Fatalf("%d: %s, expected error %v", testcase.TestNo, testcase.Path, testcase.CloseErr)
			} else {
				err := fstesting.CompareErrors(result.CloseErr, testcase.CloseErr)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		fmt.Printf("  %10d File System Tests \r", testcase.TestNo)
		if err == nil {
			continue
		}

		if maxerrors <= 0 {
			t.Fatal(err)
		}
		t.Error(err)
		maxerrors--
	}
}
