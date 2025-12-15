package osfs

import (
	"os"
	"runtime"
	"testing"

	"github.com/absfs/fstesting"
)

// TestOsFSSuite runs the standard fstesting suite against osfs.
func TestOsFSSuite(t *testing.T) {
	// Create a temporary directory for the test.
	// Note: os.MkdirTemp returns a native path (e.g., "C:\Users\..." on Windows).
	tmpDir, err := os.MkdirTemp("", "osfs-fstesting-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fs, err := NewFS()
	if err != nil {
		t.Fatalf("failed to create osfs: %v", err)
	}

	// Change to temp directory so tests don't pollute the working directory.
	// The recommended approach is to convert native paths to Unix-style using
	// FromNative(). osfs also handles native paths directly for robustness,
	// but explicit conversion makes the intent clear and is the preferred pattern.
	if err := fs.Chdir(FromNative(tmpDir)); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}

	// Use platform-appropriate features
	features := fstesting.OSFeatures()

	// Override case sensitivity based on platform
	if runtime.GOOS == "darwin" {
		// macOS is typically case-insensitive (HFS+/APFS default)
		features.CaseSensitive = false
	}

	suite := &fstesting.Suite{
		FS:       fs,
		Features: features,
	}

	suite.Run(t)
}
