// Package osfs provides helpers for converting between Unix-style absfs paths
// and native OS paths. All absfs filesystems use Unix-style forward slash paths
// for consistency and composability. The osfs package provides conversion helpers
// for interacting with native filesystem APIs.
//
// # Path Conventions
//
// absfs paths use Unix-style conventions regardless of the host OS:
//   - Forward slashes: /foo/bar
//   - Drive letters (Windows): /c/Users/foo (lowercase)
//   - UNC paths (Windows): //server/share/path
//   - No drive = current drive: /foo resolves using cwd's drive
//
// # Windows Examples
//
//	FromNative("C:\\Users\\foo") → "/c/Users/foo"
//	FromNative("\\\\server\\share") → "//server/share"
//	ToNative("/c/Users/foo") → "C:\\Users\\foo"
//	ToNative("//server/share") → "\\\\server\\share"
//
// # Unix Examples
//
// On Unix systems, ToNative and FromNative are effectively no-ops since
// Unix already uses forward slashes. Drive letter functions return empty
// strings and pass through paths unchanged.
package osfs

// ToNative converts a Unix-style absfs path to an OS-native path.
//
// On Windows:
//   - "/c/foo/bar" → "C:\foo\bar"
//   - "//server/share/path" → "\\server\share\path"
//   - "/foo/bar" → "\foo\bar" (no drive, relative to current drive root)
//
// On Unix: returns the path unchanged (no-op).
func ToNative(path string) string {
	return toNative(path)
}

// FromNative converts an OS-native path to a Unix-style absfs path.
//
// On Windows:
//   - "C:\foo\bar" → "/c/foo/bar"
//   - "\\server\share\path" → "//server/share/path"
//   - "\foo\bar" → "/foo/bar"
//
// On Unix: returns the path unchanged (no-op).
func FromNative(path string) string {
	return fromNative(path)
}

// SplitDrive extracts the drive letter from a Unix-style absfs path.
// Returns the drive letter (lowercase, without colon) and the remaining path.
//
// Examples:
//
//	SplitDrive("/c/foo") → ("c", "/foo")
//	SplitDrive("/C/foo") → ("c", "/foo")  // normalized to lowercase
//	SplitDrive("/foo") → ("", "/foo")      // no drive
//	SplitDrive("//server/share") → ("", "//server/share")  // UNC, no drive
//	SplitDrive("foo/bar") → ("", "foo/bar")  // relative path
func SplitDrive(path string) (drive, rest string) {
	return splitDrive(path)
}

// JoinDrive combines a drive letter with a path to create a Unix-style absfs path.
//
// Examples:
//
//	JoinDrive("c", "/foo") → "/c/foo"
//	JoinDrive("C", "/foo") → "/c/foo"  // normalized to lowercase
//	JoinDrive("", "/foo") → "/foo"      // no drive
//	JoinDrive("c", "foo") → "/c/foo"    // ensures leading slash
func JoinDrive(drive, path string) string {
	return joinDrive(drive, path)
}

// GetDrive returns just the drive letter from a Unix-style absfs path.
// Returns empty string if path has no drive letter.
//
// Examples:
//
//	GetDrive("/c/foo") → "c"
//	GetDrive("/foo") → ""
//	GetDrive("//server/share") → ""
func GetDrive(path string) string {
	drive, _ := splitDrive(path)
	return drive
}

// SetDrive returns the path with its drive letter changed (or added).
// If the path already has a drive, it is replaced. If not, one is added.
//
// Examples:
//
//	SetDrive("/c/foo", "d") → "/d/foo"
//	SetDrive("/foo", "c") → "/c/foo"
//	SetDrive("foo", "c") → "/c/foo"
func SetDrive(path, drive string) string {
	return setDrive(path, drive)
}

// StripDrive removes the drive prefix from a Unix-style absfs path.
//
// Examples:
//
//	StripDrive("/c/foo") → "/foo"
//	StripDrive("/foo") → "/foo"  // no change
//	StripDrive("//server/share/path") → "//server/share/path"  // UNC unchanged
func StripDrive(path string) string {
	_, rest := splitDrive(path)
	return rest
}

// IsUNC returns true if the path is a UNC-style path (//server/share).
//
// Examples:
//
//	IsUNC("//server/share") → true
//	IsUNC("//server/share/path") → true
//	IsUNC("/c/foo") → false
//	IsUNC("/foo") → false
func IsUNC(path string) bool {
	return isUNC(path)
}

// SplitUNC splits a UNC path into server, share, and remaining path components.
// Returns empty strings if the path is not a valid UNC path.
//
// Examples:
//
//	SplitUNC("//server/share/foo/bar") → ("server", "share", "/foo/bar")
//	SplitUNC("//server/share") → ("server", "share", "/")
//	SplitUNC("/c/foo") → ("", "", "")  // not UNC
func SplitUNC(path string) (server, share, rest string) {
	return splitUNC(path)
}

// JoinUNC creates a UNC path from server, share, and path components.
//
// Examples:
//
//	JoinUNC("server", "share", "/foo") → "//server/share/foo"
//	JoinUNC("server", "share", "/") → "//server/share"
//	JoinUNC("server", "share", "") → "//server/share"
func JoinUNC(server, share, path string) string {
	return joinUNC(server, share, path)
}

// ValidatePath checks if a path is valid for the current OS.
// Returns nil if valid, or an error describing the issue.
//
// On Windows, checks for:
//   - Reserved device names (CON, PRN, NUL, etc.)
//   - Invalid characters (< > : " | ? *)
//   - Trailing spaces or periods in path components
//
// On Unix: most paths are valid, only checks for null bytes.
func ValidatePath(path string) error {
	return validatePath(path)
}

// IsReservedName returns true if name is a reserved device name on Windows.
// Always returns false on Unix.
//
// Reserved names (case-insensitive): CON, PRN, AUX, NUL, COM1-COM9, LPT1-LPT9
//
// Examples:
//
//	IsReservedName("CON") → true (Windows), false (Unix)
//	IsReservedName("con.txt") → true (Windows), false (Unix)
//	IsReservedName("config") → false
func IsReservedName(name string) bool {
	return isReservedName(name)
}
