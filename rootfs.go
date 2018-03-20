package main

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	"github.com/genuinetools/binctr/image"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func unpackRootfs(spec *specs.Spec) error {
	data, err := base64.StdEncoding.DecodeString(image.DATA)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(defaultRootfsDir, 0755); err != nil {
		return err
	}

	r := bytes.NewReader(data)
	if err := archive.Untar(r, defaultRootfsDir, &archive.TarOptions{NoLchown: true}); err != nil {
		return err
	}

	// write a resolv.conf
	if err := ioutil.WriteFile(filepath.Join(defaultRootfsDir, "etc", "resolv.conf"), []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4"), 0755); err != nil {
		return err
	}

	return nil
}
