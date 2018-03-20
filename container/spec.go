package container

import (
	aaprofile "github.com/docker/docker/profiles/apparmor"
	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runc/libcontainer/specconv"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	// DefaultApparmorProfile is the default apparmor profile for the containers.
	DefaultApparmorProfile = "docker-default"
)

// SpecOpts defines the options available for a spec.
type SpecOpts struct {
	Rootless bool
	Readonly bool
	Terminal bool
	Hooks    *specs.Hooks
}

// Spec returns a default oci spec with some options being passed.
func Spec(opts SpecOpts) *specs.Spec {
	// Initialize the spec.
	spec := specconv.Example()

	// Set the spec to be rootless.
	if opts.Rootless {
		specconv.ToRootless(spec)
	}

	// Setup readonly fs in spec.
	spec.Root.Readonly = opts.Readonly

	// Setup tty in spec.
	spec.Process.Terminal = opts.Terminal

	// Pass in any hooks to the spec.
	spec.Hooks = opts.Hooks

	// Set the default seccomp profile.
	spec.Linux.Seccomp = DefaultSeccompProfile

	// Install the default apparmor profile.
	if apparmor.IsEnabled() {
		// Check if we have the docker-default apparmor profile loaded.
		if _, err := aaprofile.IsLoaded(DefaultApparmorProfile); err == nil {
			spec.Process.ApparmorProfile = DefaultApparmorProfile
		}
	}

	return spec
}
