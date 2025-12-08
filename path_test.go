package osfs

import (
	"runtime"
	"testing"
)

func TestSplitDrive(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantDrive string
		wantRest  string
		windows   bool // only run on Windows
	}{
		// Cross-platform tests (drive letter extraction works everywhere)
		{"empty", "", "", "", false},
		{"root", "/", "", "/", false},
		{"simple path", "/foo/bar", "", "/foo/bar", false},
		{"relative", "foo/bar", "", "foo/bar", false},

		// Windows-style paths (only meaningful on Windows, but parsing works everywhere)
		{"drive root", "/c/", "c", "/", true},
		{"drive with path", "/c/foo/bar", "c", "/foo/bar", true},
		{"uppercase drive", "/C/foo", "c", "/foo", true},
		{"just drive", "/c", "c", "/", true},
		{"drive d", "/d/Users/test", "d", "/Users/test", true},

		// UNC paths (no drive)
		{"UNC path", "//server/share/path", "", "//server/share/path", false},
		{"UNC root", "//server/share", "", "//server/share", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.windows && runtime.GOOS != "windows" {
				t.Skip("Windows-only test")
			}
			drive, rest := SplitDrive(tt.path)
			if drive != tt.wantDrive {
				t.Errorf("SplitDrive(%q) drive = %q, want %q", tt.path, drive, tt.wantDrive)
			}
			if rest != tt.wantRest {
				t.Errorf("SplitDrive(%q) rest = %q, want %q", tt.path, rest, tt.wantRest)
			}
		})
	}
}

func TestJoinDrive(t *testing.T) {
	tests := []struct {
		name    string
		drive   string
		path    string
		want    string
		windows bool
	}{
		{"empty drive", "", "/foo", "/foo", false},
		{"empty both", "", "", "", false},
		{"drive with path", "c", "/foo", "/c/foo", true},
		{"uppercase drive", "C", "/foo", "/c/foo", true},
		{"no leading slash", "c", "foo", "/c/foo", true},
		{"root path", "c", "/", "/c/", true},
		{"empty path", "c", "", "/c/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.windows && runtime.GOOS != "windows" {
				t.Skip("Windows-only test")
			}
			got := JoinDrive(tt.drive, tt.path)
			if got != tt.want {
				t.Errorf("JoinDrive(%q, %q) = %q, want %q", tt.drive, tt.path, got, tt.want)
			}
		})
	}
}

func TestGetDrive(t *testing.T) {
	tests := []struct {
		path    string
		want    string
		windows bool
	}{
		{"/c/foo", "c", true},
		{"/C/foo", "c", true},
		{"/foo", "", false},
		{"//server/share", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if tt.windows && runtime.GOOS != "windows" {
				t.Skip("Windows-only test")
			}
			got := GetDrive(tt.path)
			if got != tt.want {
				t.Errorf("GetDrive(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSetDrive(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		drive   string
		want    string
		windows bool
	}{
		{"change drive", "/c/foo", "d", "/d/foo", true},
		{"add drive", "/foo", "c", "/c/foo", true},
		{"remove drive", "/c/foo", "", "/foo", true},
		{"no drive to change", "/foo", "c", "/c/foo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.windows && runtime.GOOS != "windows" {
				t.Skip("Windows-only test")
			}
			got := SetDrive(tt.path, tt.drive)
			if got != tt.want {
				t.Errorf("SetDrive(%q, %q) = %q, want %q", tt.path, tt.drive, got, tt.want)
			}
		})
	}
}

func TestStripDrive(t *testing.T) {
	tests := []struct {
		path    string
		want    string
		windows bool
	}{
		{"/c/foo", "/foo", true},
		{"/foo", "/foo", false},
		{"//server/share", "//server/share", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if tt.windows && runtime.GOOS != "windows" {
				t.Skip("Windows-only test")
			}
			got := StripDrive(tt.path)
			if got != tt.want {
				t.Errorf("StripDrive(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsUNC(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"//server/share", true},
		{"//server/share/path", true},
		{"//a/b", true},
		{"/c/foo", false},
		{"/foo", false},
		{"foo", false},
		{"", false},
		{"/", false},
		{"//", true}, // technically malformed but matches pattern
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsUNC(tt.path)
			if got != tt.want {
				t.Errorf("IsUNC(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSplitUNC(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantServer string
		wantShare  string
		wantRest   string
	}{
		{"full UNC", "//server/share/foo/bar", "server", "share", "/foo/bar"},
		{"UNC root", "//server/share", "server", "share", "/"},
		{"UNC with trailing slash", "//server/share/", "server", "share", "/"},
		{"just server", "//server", "server", "", ""},
		{"not UNC", "/foo/bar", "", "", ""},
		{"drive path", "/c/foo", "", "", ""},
		{"empty", "", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, share, rest := SplitUNC(tt.path)
			if server != tt.wantServer {
				t.Errorf("SplitUNC(%q) server = %q, want %q", tt.path, server, tt.wantServer)
			}
			if share != tt.wantShare {
				t.Errorf("SplitUNC(%q) share = %q, want %q", tt.path, share, tt.wantShare)
			}
			if rest != tt.wantRest {
				t.Errorf("SplitUNC(%q) rest = %q, want %q", tt.path, rest, tt.wantRest)
			}
		})
	}
}

func TestJoinUNC(t *testing.T) {
	tests := []struct {
		name   string
		server string
		share  string
		path   string
		want   string
	}{
		{"full UNC", "server", "share", "/foo/bar", "//server/share/foo/bar"},
		{"UNC root", "server", "share", "/", "//server/share"},
		{"empty path", "server", "share", "", "//server/share"},
		{"no share", "server", "", "/foo", "//server/foo"},
		{"empty server", "", "share", "/foo", "/foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinUNC(tt.server, tt.share, tt.path)
			if got != tt.want {
				t.Errorf("JoinUNC(%q, %q, %q) = %q, want %q",
					tt.server, tt.share, tt.path, got, tt.want)
			}
		})
	}
}

func TestToNative(t *testing.T) {
	if runtime.GOOS != "windows" {
		// On Unix, ToNative is a no-op
		tests := []struct {
			path string
			want string
		}{
			{"/foo/bar", "/foo/bar"},
			{"//server/share", "//server/share"},
			{"/c/foo", "/c/foo"}, // drive letters pass through on Unix
			{"", ""},
		}
		for _, tt := range tests {
			got := ToNative(tt.path)
			if got != tt.want {
				t.Errorf("ToNative(%q) = %q, want %q", tt.path, got, tt.want)
			}
		}
		return
	}

	// Windows tests
	tests := []struct {
		name string
		path string
		want string
	}{
		{"drive path", "/c/foo/bar", `C:\foo\bar`},
		{"drive root", "/c/", `C:\`},
		{"uppercase drive", "/C/foo", `C:\foo`},
		{"UNC path", "//server/share/path", `\\server\share\path`},
		{"UNC root", "//server/share", `\\server\share`},
		{"no drive", "/foo/bar", `\foo\bar`},
		{"relative", "foo/bar", `foo\bar`},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToNative(tt.path)
			if got != tt.want {
				t.Errorf("ToNative(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestFromNative(t *testing.T) {
	if runtime.GOOS != "windows" {
		// On Unix, FromNative is a no-op
		tests := []struct {
			path string
			want string
		}{
			{"/foo/bar", "/foo/bar"},
			{"", ""},
		}
		for _, tt := range tests {
			got := FromNative(tt.path)
			if got != tt.want {
				t.Errorf("FromNative(%q) = %q, want %q", tt.path, got, tt.want)
			}
		}
		return
	}

	// Windows tests
	tests := []struct {
		name string
		path string
		want string
	}{
		{"drive path", `C:\foo\bar`, "/c/foo/bar"},
		{"drive root", `C:\`, "/c/"},
		{"lowercase drive", `c:\foo`, "/c/foo"},
		{"UNC path", `\\server\share\path`, "//server/share/path"},
		{"UNC root", `\\server\share`, "//server/share"},
		{"no drive", `\foo\bar`, "/foo/bar"},
		{"relative", `foo\bar`, "foo/bar"},
		{"forward slashes", "C:/foo/bar", "/c/foo/bar"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromNative(tt.path)
			if got != tt.want {
				t.Errorf("FromNative(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Round-trip test only meaningful on Windows")
	}

	// Unix-style → Native → Unix-style
	unixPaths := []string{
		"/c/Users/test/file.txt",
		"/d/Data/folder",
		"//server/share/path",
		"/c/",
	}

	for _, path := range unixPaths {
		native := ToNative(path)
		back := FromNative(native)
		if back != path {
			t.Errorf("Round-trip failed: %q → %q → %q", path, native, back)
		}
	}

	// Native → Unix-style → Native
	nativePaths := []string{
		`C:\Users\test\file.txt`,
		`D:\Data\folder`,
		`\\server\share\path`,
		`C:\`,
	}

	for _, path := range nativePaths {
		unix := FromNative(path)
		back := ToNative(unix)
		if back != path {
			t.Errorf("Round-trip failed: %q → %q → %q", path, unix, back)
		}
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		windows bool
	}{
		// Valid on all platforms
		{"simple path", "/foo/bar", false, false},
		{"empty", "", false, false},
		{"deep path", "/a/b/c/d/e/f", false, false},

		// Invalid on all platforms
		{"null byte", "/foo\x00bar", true, false},

		// Invalid on Windows only
		{"reserved CON", "/c/CON", true, true},
		{"reserved con lowercase", "/c/con", true, true},
		{"reserved with extension", "/c/CON.txt", true, true},
		{"reserved NUL", "/c/NUL", true, true},
		{"reserved COM1", "/c/COM1", true, true},
		{"reserved LPT1", "/c/LPT1", true, true},
		{"invalid char <", "/c/foo<bar", true, true},
		{"invalid char >", "/c/foo>bar", true, true},
		{"invalid char |", "/c/foo|bar", true, true},
		{"invalid char ?", "/c/foo?bar", true, true},
		{"invalid char *", "/c/foo*bar", true, true},
		{"trailing space", "/c/foo ", true, true},
		{"trailing period", "/c/foo.", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.windows && runtime.GOOS != "windows" {
				// These should NOT error on Unix
				err := ValidatePath(tt.path)
				if err != nil {
					t.Errorf("ValidatePath(%q) unexpected error on Unix: %v", tt.path, err)
				}
				return
			}

			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestIsReservedName(t *testing.T) {
	if runtime.GOOS != "windows" {
		// On Unix, nothing is reserved
		reserved := []string{"CON", "PRN", "AUX", "NUL", "COM1", "LPT1"}
		for _, name := range reserved {
			if IsReservedName(name) {
				t.Errorf("IsReservedName(%q) = true on Unix, want false", name)
			}
		}
		return
	}

	// Windows tests
	tests := []struct {
		name string
		want bool
	}{
		{"CON", true},
		{"con", true},
		{"Con", true},
		{"PRN", true},
		{"AUX", true},
		{"NUL", true},
		{"COM1", true},
		{"COM9", true},
		{"LPT1", true},
		{"LPT9", true},
		{"CON.txt", true},  // reserved even with extension
		{"con.exe", true},  // reserved even with extension
		{"config", false},  // not reserved
		{"CONSOLE", false}, // not reserved
		{"COM10", false},   // only 1-9 are reserved
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsReservedName(tt.name)
			if got != tt.want {
				t.Errorf("IsReservedName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
