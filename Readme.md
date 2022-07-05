# osfs - Abstract File System interface
`osfs` package implements the absfs.FileSystem interface using the `os` standard library file access functions.

## Install

```bash
$ go get github.com/absfs/osfs
```

## Example Usage

```go
package main

import (
	"log"
	"os"

	"github.com/absfs/osfs"
)

func ExitOnError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	fs, err := osfs.NewFS()
	ExitOnError(err)

	f, err := fs.Create("example.txt")
	ExitOnError(err)

	_, err = f.Write([]byte("Hello, world!\n"))
	ExitOnError(err)

	err = f.Close()
	ExitOnError(err)

	// if "keep" is passed as the first argument, don't delete the file
	if len(os.Args) > 1 && os.Args[1] == "keep" {
		return
	}

	// delete the file
	err = fs.Remove("example.txt")
	ExitOnError(err)
}


```

## absfs
Check out the [`absfs`](https://github.com/absfs/absfs) repo for more information about the abstract filesystem interface and features like filesystem composition

## LICENSE

This project is governed by the MIT License. See [LICENSE](https://github.com/absfs/osfs/blob/master/LICENSE)



