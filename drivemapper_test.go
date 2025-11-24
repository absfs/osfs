//go:build windows
// +build windows

package osfs

import (
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsDriveMapperTranslatePath(t *testing.T) {
	base, err := NewFS()
	if err != nil {
		t.Fatalf("NewFS failed: %v", err)
	}
	mapper := NewWindowsDriveMapper(base, "C:").(*WindowsDriveMapper)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Virtual-absolute with forward slash",
			input:    "/config/app.json",
			expected: "C:\\config\\app.json",
		},
		{
			name:     "Virtual-absolute with backslash",
			input:    "\\var\\log\\app.log",
			expected: "C:\\var\\log\\app.log",
		},
		{
			name:     "OS-absolute with drive letter",
			input:    "C:\\Windows\\System32\\file.txt",
			expected: "C:\\Windows\\System32\\file.txt",
		},
		{
			name:     "OS-absolute with different drive",
			input:    "D:\\Data\\file.txt",
			expected: "D:\\Data\\file.txt",
		},
		{
			name:     "UNC path",
			input:    "\\\\server\\share\\file.txt",
			expected: "\\\\server\\share\\file.txt",
		},
		{
			name:     "Relative path",
			input:    "relative\\path\\file.txt",
			expected: "relative\\path\\file.txt",
		},
		{
			name:     "Root path",
			input:    "/",
			expected: "C:\\",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.translatePath(tt.input)
			expected := filepath.Clean(tt.expected)
			result = filepath.Clean(result)

			if result != expected {
				t.Errorf("translatePath(%q) = %q, want %q", tt.input, result, expected)
			}
		})
	}
}

func TestWindowsDriveMapperDefaultDrive(t *testing.T) {
	base, err := NewFS()
	if err != nil {
		t.Fatalf("NewFS failed: %v", err)
	}
	mapper := NewWindowsDriveMapper(base, "").(*WindowsDriveMapper)

	if mapper.drive != "C:" {
		t.Errorf("Expected default drive C:, got %s", mapper.drive)
	}
}

func TestWindowsDriveMapperCustomDrive(t *testing.T) {
	base, err := NewFS()
	if err != nil {
		t.Fatalf("NewFS failed: %v", err)
	}
	mapper := NewWindowsDriveMapper(base, "D:").(*WindowsDriveMapper)

	if mapper.drive != "D:" {
		t.Errorf("Expected drive D:, got %s", mapper.drive)
	}

	result := mapper.translatePath("/data/file.txt")
	expected := filepath.Clean("D:\\data\\file.txt")
	result = filepath.Clean(result)

	if result != expected {
		t.Errorf("translatePath with D: drive = %q, want %q", result, expected)
	}
}

func TestWindowsDriveMapperIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	base, err := NewFS()
	if err != nil {
		t.Fatalf("NewFS failed: %v", err)
	}
	mapper := NewWindowsDriveMapper(base, "C:")

	// Test that OS-absolute paths work unchanged
	testFile := filepath.Join(tempDir, "test.txt")
	f, err := mapper.Create(testFile)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	f.Close()

	// Verify file was created
	_, err = mapper.Stat(testFile)
	if err != nil {
		t.Errorf("Stat failed: %v", err)
	}

	// Clean up
	err = mapper.Remove(testFile)
	if err != nil {
		t.Errorf("Remove failed: %v", err)
	}
}

func TestWindowsDriveMapperPassThroughMethods(t *testing.T) {
	base, err := NewFS()
	if err != nil {
		t.Fatalf("NewFS failed: %v", err)
	}
	mapper := NewWindowsDriveMapper(base, "C:")

	// Test methods that should pass through unchanged
	if sep := mapper.Separator(); sep != base.Separator() {
		t.Errorf("Separator() = %c, want %c", sep, base.Separator())
	}

	if listSep := mapper.ListSeparator(); listSep != base.ListSeparator() {
		t.Errorf("ListSeparator() = %c, want %c", listSep, base.ListSeparator())
	}

	if tempDir := mapper.TempDir(); tempDir != base.TempDir() {
		t.Errorf("TempDir() = %s, want %s", tempDir, base.TempDir())
	}
}

func TestWindowsDriveMapperAllMethods(t *testing.T) {
	tempDir := t.TempDir()
	base, err := NewFS()
	if err != nil {
		t.Fatalf("NewFS failed: %v", err)
	}
	mapper := NewWindowsDriveMapper(base, "C:")

	// Test various FileSystem methods with OS-absolute paths
	testPath := filepath.Join(tempDir, "test")

	// Mkdir
	err = mapper.Mkdir(testPath, 0755)
	if err != nil {
		t.Errorf("Mkdir failed: %v", err)
	}

	// Stat
	info, err := mapper.Stat(testPath)
	if err != nil {
		t.Errorf("Stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory")
	}

	// Create file
	filePath := filepath.Join(testPath, "file.txt")
	f, err := mapper.Create(filePath)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	f.Close()

	// Chmod
	err = mapper.Chmod(filePath, 0644)
	if err != nil {
		t.Errorf("Chmod failed: %v", err)
	}

	// Chtimes
	now := time.Now()
	err = mapper.Chtimes(filePath, now, now)
	if err != nil {
		t.Errorf("Chtimes failed: %v", err)
	}

	// Rename
	newPath := filepath.Join(testPath, "renamed.txt")
	err = mapper.Rename(filePath, newPath)
	if err != nil {
		t.Errorf("Rename failed: %v", err)
	}

	// Truncate
	err = mapper.Truncate(newPath, 100)
	if err != nil {
		t.Errorf("Truncate failed: %v", err)
	}

	// RemoveAll
	err = mapper.RemoveAll(testPath)
	if err != nil {
		t.Errorf("RemoveAll failed: %v", err)
	}
}
