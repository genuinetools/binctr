// +build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	bindata "github.com/jteeuwen/go-bindata"
)

// Reads image.tar and saves the binary data in image/bindata.go.
func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "os.Getwd: %v\n", err)
		os.Exit(1)
	}

	tarPath := filepath.Join(wd, "image.tar")

	// Create the bindata config.
	bc := bindata.NewConfig()
	bc.Input = []bindata.InputConfig{
		{
			Path:      tarPath,
			Recursive: false,
		},
	}
	bc.Output = filepath.Join(wd, "image", "bindata.go")
	bc.Package = "image"
	bc.NoMetadata = true
	bc.Prefix = wd

	if err := bindata.Translate(bc); err != nil {
		fmt.Fprintf(os.Stderr, "bindata: %v\n", err)
		os.Exit(1)
	}
}
