package main

import (
	"flag"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/genuinetools/binctr/container"
	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const (
	defaultRoot      = "/tmp/binctr-cl-k8s"
	defaultRootfsDir = "rootfs"
)

var (
	containerID string
	root        string

	file      string
	dir       string
	shortpath string
)

func init() {
	// Parse flags
	flag.StringVar(&containerID, "id", "cl-k8s", "container ID")
	flag.StringVar(&root, "root", defaultRoot, "root directory of container state, should be tmpfs")

	flag.Usage = func() {
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 1 {
		logrus.Fatal("pass a file to run with cl-k8s")
	}
}

//go:generate go run generate.go
func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runInit()
		return
	}

	// Get the file args passed.
	file = flag.Arg(0)

	// Get the absolute path.
	var err error
	file, err = filepath.Abs(file)
	if err != nil {
		logrus.Fatal(err)
	}

	// Check if its directory.
	fi, err := os.Stat(file)
	if err != nil {
		logrus.Fatal(err)
	}

	dir = file
	if !fi.Mode().IsDir() {
		// Copy the file to a temporary directory.
		file, err = copyFile(file)
		if err != nil {
			logrus.Fatal(err)
		}
		dir = filepath.Dir(file)
	}

	// Create a new container spec with the following options.
	opts := container.SpecOpts{
		Rootless: true,
		Terminal: true,
		Args:     []string{"clisp", filepath.Join("/home/user/scripts", filepath.Base(file))},
		Mounts: []specs.Mount{
			{
				Destination: "/home/user/scripts/",
				Type:        "bind",
				Source:      dir,
				Options:     []string{"bind", "ro"},
			},
		},
	}
	spec := container.Spec(opts)

	// Initialize the container object.
	c := &container.Container{
		ID:       containerID,
		Spec:     spec,
		Root:     root,
		Rootless: true,
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

// copyFile copies the src file to a temporary directory.
func copyFile(src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()

	tmpd, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}

	out, err := os.Create(filepath.Join(tmpd, filepath.Base(src)))
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return "", err
	}
	return out.Name(), out.Close()
}
