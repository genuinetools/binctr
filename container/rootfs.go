package container

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
)

const (
	// DefaultTarballPath holds the default path for the embedded tarball.
	DefaultTarballPath = "image.tar"
)

// UnpackRootfs unpacks the embedded tarball to the rootfs.
func (c *Container) UnpackRootfs(rootfsDir string, asset func(string) ([]byte, error)) error {
	// Make the rootfs directory.
	if err := os.MkdirAll(rootfsDir, 0755); err != nil {
		return err
	}

	// Get the embedded tarball.
	data, err := asset(DefaultTarballPath)
	if err != nil {
		return fmt.Errorf("getting bindata asset image.tar failed: %v", err)
	}

	// Unpack the tarball.
	r := bytes.NewReader(data)
	if err := archive.Untar(r, rootfsDir, &archive.TarOptions{NoLchown: true}); err != nil {
		return err
	}

	// Write a resolv.conf.
	if err := ioutil.WriteFile(filepath.Join(rootfsDir, "etc", "resolv.conf"), []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4"), 0755); err != nil {
		return err
	}

	return nil
}
