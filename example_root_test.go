package osfs_test

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/absfs/osfs"
)

// ExampleFileSystem_OpenRoot builds on every platform and Go release osfs
// supports: it names no build-constrained identifier and closes the root
// through io.Closer.
func ExampleFileSystem_OpenRoot() {
	o, err := osfs.NewFS()
	if err != nil {
		log.Fatal(err)
	}
	sfs, err := o.OpenRoot(os.TempDir())
	if errors.Is(err, errors.ErrUnsupported) {
		return // before Go 1.25 or off Unix: fall back to o knowingly
	}
	if err != nil {
		log.Fatal(err)
	}
	if c, ok := sfs.(io.Closer); ok {
		defer c.Close()
	}

	_, err = sfs.Stat("../outside")
	fmt.Println(errors.Is(err, osfs.ErrPathEscapes))
}
