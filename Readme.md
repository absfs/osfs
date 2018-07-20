# osfs - Abstract File System interface
`osfs` package implements the absfs.FileSystem interface using the `os` standard library file access functions.

## Install 

```bash
$ go get github.com/absfs/osfs
```

## Example Usage

```go
package main

import(
    "fmt"
    "os"

    "github.com/absfs/osfs"
)

func main() {
    fs, _ := osfs.NewFs() // remember kids don't ignore errors

    // Opens a file with read/write permissions in the current directory
    f, _ := fs.Create("example.txt")  
    
    f.Write([]byte("Hello, world!"))
    f.Close()

    fs.Remove("example.txt")
}
```

## absfs
Check out the [`absfs`](https://github.com/absfs/absfs) repo for more information about the abstract filesystem interface and features like filesystem composition 

## LICENSE

This project is governed by the MIT License. See [LICENSE](https://github.com/absfs/osfs/blob/master/LICENSE)



