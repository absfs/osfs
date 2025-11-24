//go:build windows
// +build windows

package osfs_test

import (
	"fmt"
	"log"

	"github.com/absfs/osfs"
)

func ExampleNewWindowsDriveMapper() {
	// Create an OS filesystem with drive mapping
	fs, err := osfs.NewFS()
	if err != nil {
		log.Fatal(err)
	}
	mapped := osfs.NewWindowsDriveMapper(fs, "C:")

	// Unix-style paths work intuitively on Windows
	f, err := mapped.Create("/tmp/config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// This created C:\tmp\config.json on Windows
	// and /tmp/config.json on Unix/macOS

	fmt.Println("Config file created")
	// Output: Config file created
}

func ExampleNewWindowsDriveMapper_customDrive() {
	// Use a different drive letter
	fs, err := osfs.NewFS()
	if err != nil {
		log.Fatal(err)
	}
	mapped := osfs.NewWindowsDriveMapper(fs, "D:")

	// Paths map to D: drive instead
	err = mapped.MkdirAll("/data/logs", 0755)
	if err != nil {
		log.Fatal(err)
	}

	// This created D:\data\logs on Windows
	// and /data/logs on Unix/macOS

	fmt.Println("Directory created")
	// Output: Directory created
}
