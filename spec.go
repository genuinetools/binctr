package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/opencontainers/runtime-spec/specs-go"
)

var (
	spec = &specs.Spec{
		Version: specs.Version,
		Platform: specs.Platform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		Root: specs.Root{
			Path:     "rootfs",
			Readonly: true,
		},
		Process: specs.Process{
			Terminal: true,
			User:     specs.User{},
			Args: []string{
				"sh",
			},
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM=xterm",
			},
			Cwd:             "/",
			NoNewPrivileges: true,
			Capabilities: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
			Rlimits: []specs.Rlimit{
				{
					Type: "RLIMIT_NOFILE",
					Hard: uint64(1024),
					Soft: uint64(1024),
				},
			},
		},
		Hostname: "ctr",
		Mounts: []specs.Mount{
			{
				Destination: "/proc",
				Type:        "proc",
				Source:      "proc",
				Options:     nil,
			},
			{
				Destination: "/dev",
				Type:        "tmpfs",
				Source:      "tmpfs",
				Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
			},
			{
				Destination: "/dev/pts",
				Type:        "devpts",
				Source:      "devpts",
				Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
			},
			{
				Destination: "/dev/shm",
				Type:        "tmpfs",
				Source:      "shm",
				Options:     []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"},
			},
			{
				Destination: "/dev/mqueue",
				Type:        "mqueue",
				Source:      "mqueue",
				Options:     []string{"nosuid", "noexec", "nodev"},
			},
			{
				Destination: "/sys",
				Type:        "sysfs",
				Source:      "sysfs",
				Options:     []string{"nosuid", "noexec", "nodev", "ro"},
			},
			{
				Destination: "/sys/fs/cgroup",
				Type:        "cgroup",
				Source:      "cgroup",
				Options:     []string{"nosuid", "noexec", "nodev", "relatime"},
			},
		},
		Linux: specs.Linux{
			UIDMappings: []specs.IDMapping{
				{
					HostID:      remappedUID,
					ContainerID: 0,
					Size:        46578392,
				},
			},
			GIDMappings: []specs.IDMapping{
				{
					HostID:      remappedGID,
					ContainerID: 0,
					Size:        46578392,
				},
			},
			MaskedPaths: []string{
				"/proc/kcore",
				"/proc/latency_stats",
				"/proc/timer_stats",
				"/proc/sched_debug",
			},
			ReadonlyPaths: []string{
				"/proc/asound",
				"/proc/bus",
				"/proc/fs",
				"/proc/irq",
				"/proc/sys",
				"/proc/sysrq-trigger",
			},
			Resources: &specs.Resources{
				Devices: []specs.DeviceCgroup{
					{
						Allow:  false,
						Access: sPtr("rwm"),
					},
				},
			},
			Namespaces: []specs.Namespace{
				{
					Type: "pid",
				},
				{
					Type: "ipc",
				},
				{
					Type: "network",
				},
				{
					Type: "user",
				},
				{
					Type: "uts",
				},
				{
					Type: "mount",
				},
			},
			Seccomp: defaultSeccompProfile,
		},
	}
)

// loadSpec loads the specification from the provided path.
// If the path is empty then the default path will be "config.json"
func loadSpec(cPath string) (spec *specs.Spec, err error) {
	cf, err := os.Open(cPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("JSON specification file %s not found", cPath)
		}
		return nil, err
	}
	defer cf.Close()

	if err = json.NewDecoder(cf).Decode(&spec); err != nil {
		return nil, err
	}
	return spec, nil
}
