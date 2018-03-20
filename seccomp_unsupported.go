// +build !seccomp

package main

import (
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// defaultProfile defines the whitelist for the default seccomp profile.
var defaultSeccompProfile = &specs.LinuxSeccomp{}
