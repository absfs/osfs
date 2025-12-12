package osfs_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/absfs/fstools"
	"github.com/absfs/osfs"
)

// setupTestDir creates a temporary directory with the specified number of files.
// Returns the directory path and a cleanup function.
func setupTestDir(b *testing.B, numFiles int) (string, func()) {
	b.Helper()

	dir, err := os.MkdirTemp("", "osfs-bench-*")
	if err != nil {
		b.Fatal(err)
	}

	// Create files
	for i := 0; i < numFiles; i++ {
		name := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
		f, err := os.Create(name)
		if err != nil {
			os.RemoveAll(dir)
			b.Fatal(err)
		}
		f.Close()
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

// setupTestFile creates a temporary file with the specified size in bytes.
// Returns the file path and a cleanup function.
func setupTestFile(b *testing.B, size int) (string, func()) {
	b.Helper()

	f, err := os.CreateTemp("", "osfs-bench-file-*")
	if err != nil {
		b.Fatal(err)
	}
	path := f.Name()

	// Write data to reach desired size
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 256)
	}

	written := 0
	for written < size {
		toWrite := size - written
		if toWrite > len(data) {
			toWrite = len(data)
		}
		n, err := f.Write(data[:toWrite])
		if err != nil {
			f.Close()
			os.Remove(path)
			b.Fatal(err)
		}
		written += n
	}

	f.Close()

	cleanup := func() {
		os.Remove(path)
	}

	return path, cleanup
}

// setupDeepTree creates a deep directory tree (10 levels, 3 entries per level).
func setupDeepTree(b *testing.B) (string, func()) {
	b.Helper()

	root, err := os.MkdirTemp("", "osfs-bench-deep-*")
	if err != nil {
		b.Fatal(err)
	}

	// Create deep tree: 10 levels with 3 entries each (2 files, 1 dir)
	var createLevel func(path string, depth int)
	createLevel = func(path string, depth int) {
		if depth <= 0 {
			return
		}

		// Create 2 files
		for i := 0; i < 2; i++ {
			f, _ := os.Create(filepath.Join(path, fmt.Sprintf("file%d.txt", i)))
			f.Close()
		}

		// Create 1 subdirectory and recurse
		subdir := filepath.Join(path, "subdir")
		os.Mkdir(subdir, 0755)
		createLevel(subdir, depth-1)
	}

	createLevel(root, 10)

	cleanup := func() {
		os.RemoveAll(root)
	}

	return root, cleanup
}

// setupWideTree creates a wide directory tree (3 levels, 20 entries per level).
func setupWideTree(b *testing.B) (string, func()) {
	b.Helper()

	root, err := os.MkdirTemp("", "osfs-bench-wide-*")
	if err != nil {
		b.Fatal(err)
	}

	// Level 1: 20 files
	for i := 0; i < 20; i++ {
		f, _ := os.Create(filepath.Join(root, fmt.Sprintf("file1-%d.txt", i)))
		f.Close()
	}

	// Level 2: 10 subdirs with 20 files each
	for i := 0; i < 10; i++ {
		subdir := filepath.Join(root, fmt.Sprintf("dir1-%d", i))
		os.Mkdir(subdir, 0755)
		for j := 0; j < 20; j++ {
			f, _ := os.Create(filepath.Join(subdir, fmt.Sprintf("file2-%d.txt", j)))
			f.Close()
		}
	}

	cleanup := func() {
		os.RemoveAll(root)
	}

	return root, cleanup
}

// BenchmarkReadDir compares different ReadDir implementations
func BenchmarkReadDir(b *testing.B) {
	sizes := []struct {
		name  string
		files int
	}{
		{"Small_10", 10},
		{"Medium_100", 100},
		{"Large_1000", 1000},
		{"VeryLarge_10000", 10000},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			dir, cleanup := setupTestDir(b, size.files)
			defer cleanup()

			b.Run("Traditional_OpenReadClose", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					f, err := os.Open(dir)
					if err != nil {
						b.Fatal(err)
					}
					_, err = f.Readdir(-1)
					if err != nil {
						b.Fatal(err)
					}
					f.Close()
				}
			})

			b.Run("os.ReadDir", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, err := os.ReadDir(dir)
					if err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run("osfs.ReadDir", func(b *testing.B) {
				b.ReportAllocs()
				fsu, err := osfs.NewFS()
				if err != nil {
					b.Fatal(err)
				}

				// Convert to Unix-style path
				unixPath := osfs.FromNative(dir)

				for i := 0; i < b.N; i++ {
					_, err := fsu.ReadDir(unixPath)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// BenchmarkReadFile compares different ReadFile implementations
func BenchmarkReadFile(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"Small_1KB", 1024},
		{"Medium_100KB", 100 * 1024},
		{"Large_10MB", 10 * 1024 * 1024},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			path, cleanup := setupTestFile(b, size.size)
			defer cleanup()

			b.Run("Traditional_OpenReadClose", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(size.size))
				for i := 0; i < b.N; i++ {
					f, err := os.Open(path)
					if err != nil {
						b.Fatal(err)
					}
					buf := make([]byte, size.size)
					_, err = f.Read(buf)
					if err != nil {
						b.Fatal(err)
					}
					f.Close()
				}
			})

			b.Run("os.ReadFile", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(size.size))
				for i := 0; i < b.N; i++ {
					_, err := os.ReadFile(path)
					if err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run("osfs.ReadFile", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(size.size))
				fsu, err := osfs.NewFS()
				if err != nil {
					b.Fatal(err)
				}

				// Convert to Unix-style path
				unixPath := osfs.FromNative(path)

				for i := 0; i < b.N; i++ {
					_, err := fsu.ReadFile(unixPath)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// BenchmarkWalk compares different directory walking implementations
func BenchmarkWalk(b *testing.B) {
	b.Run("DeepTree", func(b *testing.B) {
		root, cleanup := setupDeepTree(b)
		defer cleanup()

		b.Run("filepath.Walk", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				count := 0
				err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					count++
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("filepath.WalkDir", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				count := 0
				err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					count++
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("fstools.Walk", func(b *testing.B) {
			b.ReportAllocs()
			fsu, err := osfs.NewFS()
			if err != nil {
				b.Fatal(err)
			}

			// Convert to Unix-style path
			unixPath := osfs.FromNative(root)

			for i := 0; i < b.N; i++ {
				count := 0
				err := fstools.Walk(fsu, unixPath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					count++
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("fstools.FastWalk", func(b *testing.B) {
			b.ReportAllocs()
			fsu, err := osfs.NewFS()
			if err != nil {
				b.Fatal(err)
			}

			// Convert to Unix-style path
			unixPath := osfs.FromNative(root)

			for i := 0; i < b.N; i++ {
				count := 0
				err := fstools.FastWalk(fsu, unixPath, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					count++
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	b.Run("WideTree", func(b *testing.B) {
		root, cleanup := setupWideTree(b)
		defer cleanup()

		b.Run("filepath.Walk", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				count := 0
				err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					count++
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("filepath.WalkDir", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				count := 0
				err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					count++
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("fstools.Walk", func(b *testing.B) {
			b.ReportAllocs()
			fsu, err := osfs.NewFS()
			if err != nil {
				b.Fatal(err)
			}

			// Convert to Unix-style path
			unixPath := osfs.FromNative(root)

			for i := 0; i < b.N; i++ {
				count := 0
				err := fstools.Walk(fsu, unixPath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					count++
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("fstools.FastWalk", func(b *testing.B) {
			b.ReportAllocs()
			fsu, err := osfs.NewFS()
			if err != nil {
				b.Fatal(err)
			}

			// Convert to Unix-style path
			unixPath := osfs.FromNative(root)

			for i := 0; i < b.N; i++ {
				count := 0
				err := fstools.FastWalk(fsu, unixPath, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					count++
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

// BenchmarkFileOperations tests various file operation patterns
func BenchmarkFileOperations(b *testing.B) {
	b.Run("Stat", func(b *testing.B) {
		path, cleanup := setupTestFile(b, 1024)
		defer cleanup()

		b.Run("os.Stat", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := os.Stat(path)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("osfs.Stat", func(b *testing.B) {
			b.ReportAllocs()
			fsu, err := osfs.NewFS()
			if err != nil {
				b.Fatal(err)
			}

			unixPath := osfs.FromNative(path)

			for i := 0; i < b.N; i++ {
				_, err := fsu.Stat(unixPath)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	b.Run("Open", func(b *testing.B) {
		path, cleanup := setupTestFile(b, 1024)
		defer cleanup()

		b.Run("os.Open", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				f, err := os.Open(path)
				if err != nil {
					b.Fatal(err)
				}
				f.Close()
			}
		})

		b.Run("osfs.Open", func(b *testing.B) {
			b.ReportAllocs()
			fsu, err := osfs.NewFS()
			if err != nil {
				b.Fatal(err)
			}

			unixPath := osfs.FromNative(path)

			for i := 0; i < b.N; i++ {
				f, err := fsu.Open(unixPath)
				if err != nil {
					b.Fatal(err)
				}
				f.Close()
			}
		})
	})
}

// BenchmarkPathConversion tests the overhead of path conversion
func BenchmarkPathConversion(b *testing.B) {
	nativePaths := []string{
		"/tmp/test/file.txt",
		"/usr/local/bin/program",
		"/var/log/system.log",
		"/home/user/documents/file.pdf",
	}

	b.Run("FromNative", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, p := range nativePaths {
				_ = osfs.FromNative(p)
			}
		}
	})

	b.Run("ToNative", func(b *testing.B) {
		unixPaths := make([]string, len(nativePaths))
		for i, p := range nativePaths {
			unixPaths[i] = osfs.FromNative(p)
		}

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, p := range unixPaths {
				_ = osfs.ToNative(p)
			}
		}
	})
}
