package osfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/absfs/fstools"
)

// BenchmarkReadDir benchmarks the optimized ReadDir implementation
func BenchmarkReadDir(b *testing.B) {
	// Create a temporary directory with many files
	tmpDir := b.TempDir()

	// Create 100 files
	for i := 0; i < 100; i++ {
		name := filepath.Join(tmpDir, "file_"+string(rune('a'+i%26))+".txt")
		if err := os.WriteFile(name, []byte("test"), 0644); err != nil {
			b.Fatal(err)
		}
	}

	osFS, err := NewFS()
	if err != nil {
		b.Fatal(err)
	}

	unixTmpDir := FromNative(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := osFS.ReadDir(unixTmpDir)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFastWalk benchmarks the FastWalk implementation from fstools
func BenchmarkFastWalk(b *testing.B) {
	// Create a temporary directory structure
	tmpDir := b.TempDir()

	// Create nested directories with files
	for i := 0; i < 10; i++ {
		dirPath := filepath.Join(tmpDir, "dir"+string(rune('0'+i)))
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			b.Fatal(err)
		}
		for j := 0; j < 10; j++ {
			filePath := filepath.Join(dirPath, "file"+string(rune('0'+j))+".txt")
			if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
				b.Fatal(err)
			}
		}
	}

	osFS, err := NewFS()
	if err != nil {
		b.Fatal(err)
	}

	unixTmpDir := FromNative(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		err := fstools.FastWalk(osFS, unixTmpDir, func(path string, d fs.DirEntry, err error) error {
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
}

// BenchmarkStdlibWalk benchmarks standard library filepath.Walk for comparison
func BenchmarkStdlibWalk(b *testing.B) {
	// Create a temporary directory structure
	tmpDir := b.TempDir()

	// Create nested directories with files
	for i := 0; i < 10; i++ {
		dirPath := filepath.Join(tmpDir, "dir"+string(rune('0'+i)))
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			b.Fatal(err)
		}
		for j := 0; j < 10; j++ {
			filePath := filepath.Join(dirPath, "file"+string(rune('0'+j))+".txt")
			if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		err := filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
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
}
