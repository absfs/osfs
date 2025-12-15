//go:build !windows

package osfs

import (
	"errors"
	"strings"
)

// toNative converts a Unix-style absfs path to a native path.
// On Unix, this is a no-op since Unix already uses forward slashes.
func toNative(path string) string {
	return path
}

// fromNative converts a native path to a Unix-style absfs path.
// On Unix, this is a no-op since Unix already uses forward slashes.
func fromNative(path string) string {
	return path
}

// splitDrive extracts the drive letter from a Unix-style path.
// On Unix, paths never have drive letters, so this always returns empty.
func splitDrive(path string) (drive, rest string) {
	return "", path
}

// joinDrive combines a drive letter with a path.
// On Unix, drive letters are ignored.
func joinDrive(drive, path string) string {
	// Ignore drive letter on Unix
	return path
}

// setDrive sets or replaces the drive letter in a path.
// On Unix, drive letters are ignored, so this just returns the path.
func setDrive(path, drive string) string {
	return path
}

// isUNC returns true if path is a UNC-style path.
// On Unix, UNC paths are not native but we still recognize the pattern
// for cross-platform path handling.
func isUNC(path string) bool {
	return len(path) >= 2 && path[0] == '/' && path[1] == '/'
}

// splitUNC splits a UNC path into components.
// On Unix, this still parses the //server/share pattern for compatibility.
func splitUNC(path string) (server, share, rest string) {
	if !isUNC(path) {
		return "", "", ""
	}

	// Skip the leading //
	remaining := path[2:]

	// Find server name
	serverEnd := strings.Index(remaining, "/")
	if serverEnd == -1 {
		return remaining, "", ""
	}
	server = remaining[:serverEnd]
	remaining = remaining[serverEnd+1:]

	// Find share name
	shareEnd := strings.Index(remaining, "/")
	if shareEnd == -1 {
		return server, remaining, "/"
	}
	share = remaining[:shareEnd]
	rest = remaining[shareEnd:]

	if rest == "" {
		rest = "/"
	}

	return server, share, rest
}

// joinUNC creates a UNC path from components.
func joinUNC(server, share, path string) string {
	if server == "" {
		return path
	}

	result := "//" + server
	if share != "" {
		result += "/" + share
	}
	if path != "" && path != "/" {
		if path[0] != '/' {
			result += "/"
		}
		result += path
	}
	return result
}

// validatePath checks if a path is valid for Unix.
// Unix is very permissive - only null bytes are invalid.
func validatePath(path string) error {
	if strings.ContainsRune(path, 0) {
		return errors.New("path contains null byte")
	}
	return nil
}

// isReservedName checks if a name is reserved.
// On Unix, no names are reserved.
func isReservedName(name string) bool {
	return false
}

// isNativePath returns true if path appears to be a native OS path
// rather than a Unix-style absfs path.
// On Unix, native paths ARE Unix-style, so this always returns false.
func isNativePath(path string) bool {
	return false
}
