package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	"github.com/genuinetools/binctr/image"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func unpackRootfs(spec *specs.Spec) error {
	// Make the rootfs directory.
	if err := os.MkdirAll(defaultRootfsDir, 0755); err != nil {
		return err
	}

	// Get the embedded tarball.
	data, err := image.Data()
	if err != nil {
		return err
	}

	// Unpack the tarball.
	r := bytes.NewReader(data)
	if err := archive.Untar(r, defaultRootfsDir, &archive.TarOptions{NoLchown: true}); err != nil {
		return err
	}

	// Write a resolv.conf.
	if err := ioutil.WriteFile(filepath.Join(defaultRootfsDir, "etc", "resolv.conf"), []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4"), 0755); err != nil {
		return err
	}

	return nil
}
