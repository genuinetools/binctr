// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/genuinetools/binctr/container"
)

// Pulls an image and saves the binary data in the container package bindata.go.
func main() {
	if err := container.EmbedImage("r.j3ss.co/clisp"); err != nil {
		fmt.Fprintf(os.Stderr, "embed image failed: %v\n", err)
		os.Exit(1)
	}
}
