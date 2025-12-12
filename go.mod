module github.com/absfs/osfs

go 1.23

require (
	github.com/absfs/absfs v0.0.0-20251208232938-aa0ca30de832
	github.com/absfs/fstesting v0.0.0-20251207022242-d748a85c4a1e
	github.com/absfs/fstools v0.0.0-00010101000000-000000000000
)

replace (
	github.com/absfs/absfs => ../absfs
	github.com/absfs/fstesting => ../fstesting
	github.com/absfs/fstools => ../fstools
)
