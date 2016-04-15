package main

import (
	"bytes"
	"encoding/base64"
	"os"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func unpackRootfs(spec *specs.Spec) error {
	data, err := base64.StdEncoding.DecodeString(DATA)
	if err != nil {
		return err
	}

	if len(spec.Linux.UIDMappings) > 0 && len(spec.Linux.GIDMappings) > 0 {
		if err := idtools.MkdirAs(defaultRootfsDir, 0755, int(spec.Linux.UIDMappings[0].HostID), int(spec.Linux.GIDMappings[0].HostID)); err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(defaultRootfsDir, 0755); err != nil {
			return err
		}
	}

	uidMaps := []idtools.IDMap{}
	gidMaps := []idtools.IDMap{}
	for _, u := range spec.Linux.UIDMappings {
		uidMaps = append(uidMaps, idtools.IDMap{
			ContainerID: int(u.ContainerID),
			HostID:      int(u.HostID),
			Size:        int(u.Size),
		})
	}

	for _, g := range spec.Linux.GIDMappings {
		gidMaps = append(gidMaps, idtools.IDMap{
			ContainerID: int(g.ContainerID),
			HostID:      int(g.HostID),
			Size:        int(g.Size),
		})
	}

	r := bytes.NewReader(data)
	if err := archive.Untar(r, defaultRootfsDir, &archive.TarOptions{
		UIDMaps: uidMaps,
		GIDMaps: gidMaps,
	}); err != nil {
		return err
	}

	return nil
}
