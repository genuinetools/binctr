package main

import (
	"bytes"
	"encoding/base64"
	"os"

	"github.com/docker/docker/pkg/archive"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func unpackRootfs(spec *specs.Spec) error {
	data, err := base64.StdEncoding.DecodeString(DATA)
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
