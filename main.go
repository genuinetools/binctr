package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	aaprofile "github.com/docker/docker/profiles/apparmor"
	"github.com/genuinetools/binctr/version"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/apparmor"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
	"github.com/opencontainers/runc/libcontainer/specconv"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const (
	// BANNER is what is printed for help/info output.
	BANNER = ` _     _            _
| |__ (_)_ __   ___| |_ _ __
| '_ \| | '_ \ / __| __| '__|
| |_) | | | | | (__| |_| |
|_.__/|_|_| |_|\___|\__|_|

 Fully static, self-contained container including the rootfs
 that can be run by an unprivileged user.

 Embedded Image: %s - %s
 Version: %s
 Build: %s

`

	defaultRoot            = "/tmp/binctr"
	defaultRootfsDir       = "rootfs"
	defaultApparmorProfile = "docker-default"
)

var (
	containerID string
	pidFile     string
	root        string

	allocateTty   bool
	consoleSocket string
	detach        bool
	readonly      bool

	hooks     specs.Hooks
	hookflags stringSlice

	debug bool
	vrsn  bool

	// IMAGE is the name of the image that is embedded at compile time.
	IMAGE = "alpine"
	// IMAGESHA is the sha digest of the image that is embedded at compile time.
	IMAGESHA = "sha256:70c557e50ed630deed07cbb0dc4d28aa0f2a485cf7af124cc48f06bce83f784b"
)

// stringSlice is a slice of strings
type stringSlice []string

// implement the flag interface for stringSlice
func (s *stringSlice) String() string {
	return fmt.Sprintf("%s", *s)
}
func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}
func (s stringSlice) ParseHooks() (hooks specs.Hooks, err error) {
	for _, v := range s {
		parts := strings.SplitN(v, ":", 2)
		if len(parts) <= 1 {
			return hooks, fmt.Errorf("parsing %s as hook_name:exec failed", v)
		}
		cmd := strings.Split(parts[1], " ")
		exec, err := exec.LookPath(cmd[0])
		if err != nil {
			return hooks, fmt.Errorf("looking up exec path for %s failed: %v", cmd[0], err)
		}
		hook := specs.Hook{
			Path: exec,
		}
		if len(cmd) > 1 {
			hook.Args = cmd[:1]
		}
		switch parts[0] {
		case "prestart":
			hooks.Prestart = append(hooks.Prestart, hook)
		case "poststart":
			hooks.Poststart = append(hooks.Poststart, hook)
		case "poststop":
			hooks.Poststop = append(hooks.Poststop, hook)
		default:
			return hooks, fmt.Errorf("%s is not a valid hook, try 'prestart', 'poststart', or 'poststop'", parts[0])
		}
	}
	return hooks, nil
}

func init() {
	// Parse flags
	flag.StringVar(&containerID, "id", IMAGE, "container ID")
	flag.StringVar(&pidFile, "pid-file", "", "specify the file to write the process id to")
	flag.StringVar(&root, "root", defaultRoot, "root directory of container state, should be tmpfs")

	flag.Var(&hookflags, "hook", "Hooks to prefill into spec file. (ex. --hook prestart:netns)")

	flag.BoolVar(&allocateTty, "t", true, "allocate a tty for the container")
	flag.StringVar(&consoleSocket, "console-socket", "", "path to an AF_UNIX socket which will receive a file descriptor referencing the master end of the console's pseudoterminal")
	flag.BoolVar(&detach, "d", false, "detach from the container's process")
	flag.BoolVar(&readonly, "read-only", false, "make container filesystem readonly")

	flag.BoolVar(&vrsn, "version", false, "print version and exit")
	flag.BoolVar(&vrsn, "v", false, "print version and exit (shorthand)")
	flag.BoolVar(&debug, "D", false, "run in debug mode")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(BANNER, IMAGE, IMAGESHA, version.VERSION, version.GITCOMMIT))
		flag.PrintDefaults()
	}

	flag.Parse()

	if vrsn {
		fmt.Printf("%s, commit: %s, image: %s, image digest: %s", version.VERSION, version.GITCOMMIT, IMAGE, IMAGESHA)
		os.Exit(0)
	}

	// Set log level.
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Parse the hook flags.
	var err error
	hooks, err = hookflags.ParseHooks()
	if err != nil {
		logrus.Fatal(err)
	}

	// Convert pid-file to an absolute path so we can write to the
	// right file after chdir to bundle.
	if pidFile != "" {
		pidFile, err = filepath.Abs(pidFile)
		if err != nil {
			logrus.Fatal(err)
		}
	}

	// Get the absolute path to the root.
	root, err = filepath.Abs(root)
	if err != nil {
		logrus.Fatal(err)
	}

}

//go:generate go run generate.go
func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runInit()
		return
	}

	// Initialize the spec.
	spec := specconv.Example()

	// Set the spec to be rootless.
	specconv.ToRootless(spec)

	// Setup readonly fs in spec.
	spec.Root.Readonly = readonly

	// Setup tty in spec.
	spec.Process.Terminal = allocateTty

	// Pass in any hooks to the spec.
	spec.Hooks = &hooks

	// Set the default seccomp profile.
	spec.Linux.Seccomp = defaultSeccompProfile

	// Install the default apparmor profile.
	if apparmor.IsEnabled() {
		// Check if we have the docker-default apparmor profile loaded.
		if _, err := aaprofile.IsLoaded(defaultApparmorProfile); err != nil {
			logrus.Warnf("AppArmor enabled on system but the %s profile is not loaded. apparmor_parser needs root to load a profile so we can't do it for you.", defaultApparmorProfile)
		} else {
			spec.Process.ApparmorProfile = defaultApparmorProfile
		}
	}

	// Unpack the rootfs.
	if err := unpackRootfs(spec); err != nil {
		logrus.Fatal(err)
	}

	// Start the container.
	status, err := startContainer(spec, containerID, pidFile, consoleSocket, root, detach)
	if err != nil {
		logrus.Fatal(err)
	}

	// Remove the rootfs after the container has exited.
	if err := os.RemoveAll(defaultRootfsDir); err != nil {
		logrus.Warnf("removing rootfs failed: %v", err)
	}

	// Exit with the container's exit status.
	os.Exit(status)
}

func runInit() {
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()
	factory, _ := libcontainer.New("")
	if err := factory.StartInitialization(); err != nil {
		// as the error is sent back to the parent there is no need to log
		// or write it to stderr because the parent process will handle this
		os.Exit(1)
	}
	panic("libcontainer: container init failed to exec")
}
