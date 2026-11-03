//go:build !(go1.25 && unix)

package osfs

import (
	"errors"
	"testing"
)

// TestOpenRootUnsupported checks that a build without RootFS says so, so
// that a caller can fall back knowingly.
func TestOpenRootUnsupported(t *testing.T) {
	o, err := NewFS()
	if err != nil {
		t.Fatal(err)
	}
	r, err := o.OpenRoot(o.TempDir())
	if r != nil || !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("OpenRoot = %v, %v; want errors.ErrUnsupported", r, err)
	}
}
