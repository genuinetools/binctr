package main

import (
	"bytes"
	"encoding/base64"
	"os"

	"github.com/docker/docker/pkg/archive"
	"github.com/jfrazelle/binctr/cryptar"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func unpackRootfs(spec *specs.Spec, keyin string) error {
	key, err := base64.StdEncoding.DecodeString(keyin)
	if err != nil {
		return err
	}

	data, err := cryptar.Decrypt(DATA, key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(defaultRootfsDir, 0755); err != nil {
		return err
	}

	r := bytes.NewReader(data)
	if err := archive.Untar(r, defaultRootfsDir, nil); err != nil {
		return err
	}

	return nil
}
