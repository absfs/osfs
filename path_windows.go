//go:build windows

package osfs

import (
	"errors"
	"path/filepath"
	"strings"
	"unicode"
)

// toNative converts a Unix-style absfs path to a Windows native path.
func toNative(path string) string {
	if path == "" {
		return ""
	}

	// Handle UNC paths: //server/share → \\server\share
	if isUNC(path) {
		return toNativeUNC(path)
	}

	// Handle drive letter paths: /c/foo → C:\foo
	if drive, rest := splitDrive(path); drive != "" {
		// Convert /c/foo to C:\foo
		nativePath := strings.ToUpper(drive) + ":" + filepath.FromSlash(rest)
		return nativePath
	}

	// No drive letter - just convert slashes
	// /foo/bar → \foo\bar (relative to current drive root)
	return filepath.FromSlash(path)
}

// toNativeUNC converts a UNC-style absfs path to Windows native UNC.
func toNativeUNC(path string) string {
	// //server/share/path → \\server\share\path
	if len(path) < 2 || path[0] != '/' || path[1] != '/' {
		return filepath.FromSlash(path)
	}
	// Replace // with \\ and convert remaining slashes
	return `\\` + filepath.FromSlash(path[2:])
}

// fromNative converts a Windows native path to a Unix-style absfs path.
func fromNative(path string) string {
	if path == "" {
		return ""
	}

	// Handle UNC paths: \\server\share → //server/share
	if len(path) >= 2 && path[0] == '\\' && path[1] == '\\' {
		return fromNativeUNC(path)
	}

	// Handle drive letter paths: C:\foo → /c/foo
	if len(path) >= 2 && path[1] == ':' {
		drive := strings.ToLower(string(path[0]))
		rest := ""
		if len(path) > 2 {
			rest = path[2:]
		}
		// Convert backslashes to forward slashes
		rest = filepath.ToSlash(rest)
		// Ensure rest starts with /
		if rest == "" || rest[0] != '/' {
			rest = "/" + rest
		}
		return "/" + drive + rest
	}

	// No drive letter - just convert slashes
	return filepath.ToSlash(path)
}

// fromNativeUNC converts a Windows native UNC path to Unix-style.
func fromNativeUNC(path string) string {
	// \\server\share\path → //server/share/path
	if len(path) < 2 || path[0] != '\\' || path[1] != '\\' {
		return filepath.ToSlash(path)
	}
	return "//" + filepath.ToSlash(path[2:])
}

// splitDrive extracts the drive letter from a Unix-style path.
func splitDrive(path string) (drive, rest string) {
	if path == "" {
		return "", ""
	}

	// UNC paths don't have a drive letter
	if isUNC(path) {
		return "", path
	}

	// Check for /x/ or /x pattern where x is a single letter
	if len(path) >= 2 && path[0] == '/' {
		// Get potential drive letter
		if len(path) == 2 {
			// "/c" - single letter after slash
			c := rune(path[1])
			if unicode.IsLetter(c) {
				return strings.ToLower(string(c)), "/"
			}
		} else if path[2] == '/' {
			// "/c/..." - letter followed by slash
			c := rune(path[1])
			if unicode.IsLetter(c) {
				return strings.ToLower(string(c)), path[2:]
			}
		}
	}

	return "", path
}

// joinDrive combines a drive letter with a path.
func joinDrive(drive, path string) string {
	if drive == "" {
		return path
	}

	// Normalize drive to lowercase
	drive = strings.ToLower(drive)

	// Ensure path starts with /
	if path == "" || path[0] != '/' {
		path = "/" + path
	}

	return "/" + drive + path
}

// setDrive sets or replaces the drive letter in a path.
func setDrive(path, drive string) string {
	if drive == "" {
		return StripDrive(path)
	}

	_, rest := splitDrive(path)
	return joinDrive(drive, rest)
}

// isUNC returns true if path is a UNC-style path.
func isUNC(path string) bool {
	return len(path) >= 2 && path[0] == '/' && path[1] == '/'
}

// splitUNC splits a UNC path into components.
func splitUNC(path string) (server, share, rest string) {
	if !isUNC(path) {
		return "", "", ""
	}

	// Skip the leading //
	remaining := path[2:]

	// Find server name
	serverEnd := strings.Index(remaining, "/")
	if serverEnd == -1 {
		// Just //server with no share
		return remaining, "", ""
	}
	server = remaining[:serverEnd]
	remaining = remaining[serverEnd+1:]

	// Find share name
	shareEnd := strings.Index(remaining, "/")
	if shareEnd == -1 {
		// //server/share with no additional path
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
		// Ensure path starts with / for joining
		if path[0] != '/' {
			result += "/"
		}
		result += path
	}
	return result
}

// Reserved Windows device names (case-insensitive)
var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true,
	"com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true,
	"lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// Invalid characters in Windows file names
var invalidChars = []rune{'<', '>', ':', '"', '|', '?', '*'}

// validatePath checks if a path is valid for Windows.
func validatePath(path string) error {
	if path == "" {
		return nil
	}

	// Check for null bytes
	if strings.ContainsRune(path, 0) {
		return errors.New("path contains null byte")
	}

	// Split into components and check each
	components := strings.Split(path, "/")
	for _, comp := range components {
		if comp == "" {
			continue
		}

		// Check for reserved names
		if isReservedName(comp) {
			return errors.New("path contains reserved name: " + comp)
		}

		// Check for invalid characters
		for _, c := range comp {
			for _, invalid := range invalidChars {
				if c == invalid {
					return errors.New("path contains invalid character: " + string(c))
				}
			}
			// Check for control characters (0-31)
			if c < 32 {
				return errors.New("path contains control character")
			}
		}

		// Check for trailing space or period
		if len(comp) > 0 {
			last := comp[len(comp)-1]
			if last == ' ' || last == '.' {
				return errors.New("path component has trailing space or period: " + comp)
			}
		}
	}

	return nil
}

// isReservedName checks if a name is a Windows reserved device name.
func isReservedName(name string) bool {
	if name == "" {
		return false
	}

	// Get base name without extension
	baseName := name
	if dot := strings.Index(name, "."); dot != -1 {
		baseName = name[:dot]
	}

	return reservedNames[strings.ToLower(baseName)]
}
