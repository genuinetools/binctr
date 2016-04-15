package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/archive"
	"github.com/opencontainers/runc/libcontainer"
)

const (
	// BANNER is what is printed for help/info output.
	BANNER = ` _     _            _
| |__ (_)_ __   ___| |_ _ __
| '_ \| | '_ \ / __| __| '__|
| |_) | | | | | (__| |_| |
|_.__/|_|_| |_|\___|\__|_|

 Fully static self-contained container including the rootfs.
 Version: %s
 GitCommit: %s
`

	defaultRoot = "/run/binctr"
)

var (
	console     = os.Getenv("console")
	containerID string
	root        string

	debug   bool
	version bool

	// GITCOMMIT is git commit the binary was compiled against.
	GITCOMMIT = ""

	// VERSION is the binary version.
	VERSION = "v0.1.0"

	// DATA is the rootfs tar that is added at compile time.
	DATA = ""
)

func init() {
	// Parse flags
	flag.StringVar(&containerID, "id", "jessiscool", "container ID")
	flag.StringVar(&console, "console", console, "the pty slave path for use with the container")
	flag.StringVar(&root, "root", defaultRoot, "root directory of container state, should be tmpfs")
	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.BoolVar(&version, "v", false, "print version and exit (shorthand)")
	flag.BoolVar(&debug, "d", false, "run in debug mode")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(BANNER, VERSION, GITCOMMIT))
		flag.PrintDefaults()
	}

	flag.Parse()

	if version {
		fmt.Printf("%s, commit: %s", VERSION, GITCOMMIT)
		os.Exit(0)
	}

	// Set log level
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runInit()
		return
	}

	if err := unpackRootfs(); err != nil {
		logrus.Fatal(err)
	}

	status, err := startContainer(spec, containerID)
	if err != nil {
		logrus.Fatal(err)
	}

	// exit with the container's exit status
	os.Exit(status)
}

func unpackRootfs() error {
	data, err := base64.StdEncoding.DecodeString(DATA)
	if err != nil {
		return err
	}
	r := bytes.NewReader(data)
	if err := os.Mkdir("container", 0755); err != nil {
		return err
	}
	return archive.Untar(r, "container", nil)
}

func runInit() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
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
}
