// +build !windows

package osfs

import "github.com/absfs/absfs"

// NewWindowsDriveMapper on non-Windows platforms simply returns the base FileSystem
// unchanged, since virtual-absolute paths already work correctly on Unix-like systems.
//
// This no-op implementation ensures code using NewWindowsDriveMapper compiles and
// runs correctly on all platforms without conditional compilation in user code.
//
// Example:
//   // This code works on all platforms:
//   fs, _ := osfs.NewFS()
//   mapped := osfs.NewWindowsDriveMapper(fs, "C:")
//   mapped.Create("/config/app.json")
//   // On Unix/macOS: creates /config/app.json
//   // On Windows: creates C:\config\app.json
func NewWindowsDriveMapper(base absfs.FileSystem, drive string) absfs.FileSystem {
	return base
}
