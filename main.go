package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/genuinetools/binctr/container"
	"github.com/genuinetools/binctr/version"
	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
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

 Version: %s
 Build: %s

`

	defaultRoot      = "/tmp/binctr"
	defaultRootfsDir = "rootfs"
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
	flag.StringVar(&containerID, "id", "binctr", "container ID")
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
		fmt.Fprint(os.Stderr, fmt.Sprintf(BANNER, version.VERSION, version.GITCOMMIT))
		flag.PrintDefaults()
	}

	flag.Parse()

	if vrsn {
		fmt.Printf("%s, commit: %s", version.VERSION, version.GITCOMMIT)
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

}

//go:generate go run generate.go
func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runInit()
		return
	}

	// Create a new container spec with the following options.
	opts := container.SpecOpts{
		Rootless: true,
		Readonly: readonly,
		Terminal: allocateTty,
		Hooks:    &hooks,
	}
	spec := container.Spec(opts)

	// Initialize the container object.
	c := &container.Container{
		ID:            containerID,
		Spec:          spec,
		PIDFile:       pidFile,
		ConsoleSocket: consoleSocket,
		Root:          root,
		Detach:        detach,
		Rootless:      true,
	}

	// Unpack the rootfs.
	if err := c.UnpackRootfs(defaultRootfsDir, Asset); err != nil {
		logrus.Fatal(err)
	}

	// Run the container.
	status, err := c.Run()
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
