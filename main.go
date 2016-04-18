package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
	aaprofile "github.com/docker/docker/profiles/apparmor"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/apparmor"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/opencontainers/runtime-spec/specs-go"
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
 GitCommit: %s

`

	defaultRoot            = "/tmp/binctr"
	defaultRootfsDir       = "rootfs"
	defaultApparmorProfile = "docker-default"
)

var (
	console     = os.Getenv("console")
	containerID string
	pidFile     string
	root        string
	passkey     string

	allocateTty      bool
	detach           bool
	readonly         bool
	useSystemdCgroup bool

	hooks     specs.Hooks
	hookflags stringSlice

	debug   bool
	version bool

	// GITCOMMIT is git commit the binary was compiled against.
	GITCOMMIT = ""

	// VERSION is the binary version.
	VERSION = "v0.1.0"

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
	flag.StringVar(&console, "console", console, "the pty slave path for use with the container")
	flag.StringVar(&pidFile, "pid-file", "", "specify the file to write the process id to")
	flag.StringVar(&root, "root", defaultRoot, "root directory of container state, should be tmpfs")
	flag.StringVar(&passkey, "key", "", "key to decrypt the embedded tarball")

	flag.Var(&hookflags, "hook", "Hooks to prefill into spec file. (ex. --hook prestart:netns)")

	flag.BoolVar(&allocateTty, "t", true, "allocate a tty for the container")
	flag.BoolVar(&detach, "d", false, "detach from the container's process")
	// TODO (jess): do not enable this flag, the error is very gross on systemd
	// flag.BoolVar(&useSystemdCgroup, "systemd-cgroup", false, "enable systemd cgroup support")
	flag.BoolVar(&readonly, "read-only", false, "make container filesystem readonly")

	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.BoolVar(&version, "v", false, "print version and exit (shorthand)")
	flag.BoolVar(&debug, "D", false, "run in debug mode")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(BANNER, IMAGE, IMAGESHA, VERSION, GITCOMMIT))
		flag.PrintDefaults()
	}

	flag.Parse()

	if version {
		fmt.Printf("%s, commit: %s, image: %s, image digest: %s", VERSION, GITCOMMIT, IMAGE, IMAGESHA)
		os.Exit(0)
	}

	// Set log level
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// parse the hook flags
	var err error
	hooks, err = hookflags.ParseHooks()
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

	notifySocket := os.Getenv("NOTIFY_SOCKET")
	if notifySocket != "" {
		setupSdNotify(spec, notifySocket)
	}

	// override the cmd in the spec with any args specified
	if len(flag.Args()) > 0 {
		spec.Process.Args = flag.Args()
	}

	// setup readonly fs in spec
	spec.Root.Readonly = readonly

	// setup tty in spec
	spec.Process.Terminal = allocateTty

	// pass in any hooks
	spec.Hooks = hooks

	// install the default apparmor profile
	if apparmor.IsEnabled() {
		// check if we have the docker-default apparmor profile loaded
		if err := aaprofile.IsLoaded(defaultApparmorProfile); err != nil {
			logrus.Warnf("AppArmor enabled on system but the %s profile is not loaded. apparmor_parser needs root to load a profile so we can't do it for you.", defaultApparmorProfile)
		} else {
			spec.Process.ApparmorProfile = defaultApparmorProfile
		}
	}

	// set the CgroupsPath as this user
	u, err := user.CurrentUser()
	if err != nil {
		logrus.Fatal(err)
	}
	spec.Linux.CgroupsPath = sPtr(u.Name)

	// setup UID mappings
	spec.Linux.UIDMappings = []specs.IDMapping{
		{
			HostID:      uint32(u.Uid),
			ContainerID: 0,
			Size:        1,
		},
	}

	// setup GID mappings
	spec.Linux.GIDMappings = []specs.IDMapping{
		{
			HostID:      uint32(u.Gid),
			ContainerID: 0,
			Size:        1,
		},
	}

	if err := unpackRootfs(spec, passkey); err != nil {
		logrus.Fatal(err)
	}

	status, err := startContainer(spec, containerID, pidFile, detach, useSystemdCgroup)
	if err != nil {
		logrus.Fatal(err)
	}

	if err := os.RemoveAll(defaultRootfsDir); err != nil {
		logrus.Warnf("removing rootfs failed: %v", err)
	}

	// exit with the container's exit status
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
