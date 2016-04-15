package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-systemd/activation"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var container libcontainer.Container

func createContainer(id string, spec *specs.Spec) (libcontainer.Container, error) {
	// create the libcontainer config
	config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		CgroupName:       id,
		UseSystemdCgroup: false,
		NoPivotRoot:      false,
		Spec:             spec,
	})
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(config.Rootfs); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("rootfs (%q) does not exist", config.Rootfs)
		}
		return nil, err
	}

	logrus.Debugf("loading factory")
	factory, err := loadFactory()
	if err != nil {
		return nil, err
	}

	logrus.Debugf("creating factory")
	return factory.Create(id, config)
}

// startContainer starts the container. Returns the exit status or -1 and an
// error. Signals sent to the current process will be forwarded to container.
func startContainer(spec *specs.Spec, id, pidFile string, detach bool) (int, error) {
	container, err := createContainer(id, spec)
	if err != nil {
		return -1, err
	}

	// Support on-demand socket activation by passing file descriptors into the container init process.
	listenFDs := []*os.File{}
	if os.Getenv("LISTEN_FDS") != "" {
		listenFDs = activation.Files(false)
	}

	r := &runner{
		enableSubreaper: true,
		shouldDestroy:   true,
		container:       container,
		console:         console,
		detach:          detach,
		pidFile:         pidFile,
		listenFDs:       listenFDs,
	}
	logrus.Debugf("running %#v", *r)
	return r.run(&spec.Process)
}

// loadFactory returns the configured factory instance for execing containers.
func loadFactory() (libcontainer.Factory, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	cgroupManager := libcontainer.Cgroupfs
	return libcontainer.New(abs, cgroupManager, func(l *libcontainer.LinuxFactory) error {
		return nil
	})
}

// newProcess returns a new libcontainer Process with the arguments from the
// spec and stdio from the current process.
func newProcess(p specs.Process) (*libcontainer.Process, error) {
	lp := &libcontainer.Process{
		Args: p.Args,
		Env:  p.Env,
		// TODO: fix libcontainer's API to better support uid/gid in a typesafe way.
		User:            fmt.Sprintf("%d:%d", p.User.UID, p.User.GID),
		Cwd:             p.Cwd,
		Capabilities:    p.Capabilities,
		Label:           p.SelinuxLabel,
		NoNewPrivileges: &p.NoNewPrivileges,
		AppArmorProfile: p.ApparmorProfile,
	}
	for _, rlimit := range p.Rlimits {
		rl, err := createLibContainerRlimit(rlimit)
		if err != nil {
			return nil, err
		}
		lp.Rlimits = append(lp.Rlimits, rl)
	}
	return lp, nil
}

func dupStdio(process *libcontainer.Process, rootuid int) error {
	process.Stdin = os.Stdin
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	for _, fd := range []uintptr{
		os.Stdin.Fd(),
		os.Stdout.Fd(),
		os.Stderr.Fd(),
	} {
		if err := syscall.Fchown(int(fd), rootuid, rootuid); err != nil {
			return err
		}
	}
	return nil
}

func destroy(container libcontainer.Container) {
	if err := container.Destroy(); err != nil {
		logrus.Error(err)
	}
}

// setupIO sets the proper IO on the process depending on the configuration
// If there is a nil error then there must be a non nil tty returned
func setupIO(process *libcontainer.Process, rootuid int, console string, createTTY, detach bool) (*tty, error) {
	// detach and createTty will not work unless a console path is passed
	// so error out here before changing any terminal settings
	if createTTY && detach && console == "" {
		return nil, fmt.Errorf("cannot allocate tty if runc will detach")
	}
	if createTTY {
		return createTty(process, rootuid, console)
	}
	if detach {
		if err := dupStdio(process, rootuid); err != nil {
			return nil, err
		}
		return &tty{}, nil
	}
	return createStdioPipes(process, rootuid)
}

// createPidFile creates a file with the processes pid inside it atomically
// it creates a temp file with the paths filename + '.' infront of it
// then renames the file
func createPidFile(path string, process *libcontainer.Process) error {
	pid, err := process.Pid()
	if err != nil {
		return err
	}
	var (
		tmpDir  = filepath.Dir(path)
		tmpName = filepath.Join(tmpDir, fmt.Sprintf(".%s", filepath.Base(path)))
	)
	f, err := os.OpenFile(tmpName, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0666)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d", pid)
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

type runner struct {
	enableSubreaper bool
	shouldDestroy   bool
	detach          bool
	listenFDs       []*os.File
	pidFile         string
	console         string
	container       libcontainer.Container
}

func (r *runner) run(config *specs.Process) (int, error) {
	logrus.Debugf("runner new process")
	process, err := newProcess(*config)
	if err != nil {
		r.destroy()
		return -1, err
	}
	if len(r.listenFDs) > 0 {
		process.Env = append(process.Env, fmt.Sprintf("LISTEN_FDS=%d", len(r.listenFDs)), "LISTEN_PID=1")
		process.ExtraFiles = append(process.ExtraFiles, r.listenFDs...)
	}
	logrus.Debugf("runner hostuid")
	rootuid, err := r.container.Config().HostUID()
	if err != nil {
		r.destroy()
		return -1, err
	}
	logrus.Debugf("runner setupio")
	tty, err := setupIO(process, rootuid, r.console, config.Terminal, r.detach)
	if err != nil {
		r.destroy()
		return -1, err
	}
	handler := newSignalHandler(tty, r.enableSubreaper)
	logrus.Debugf("container start, %#v", r.container)
	if err := r.container.Start(process); err != nil {
		r.destroy()
		tty.Close()
		return -1, err
	}
	logrus.Debugf("close post start")
	if err := tty.ClosePostStart(); err != nil {
		r.terminate(process)
		r.destroy()
		tty.Close()
		return -1, err
	}
	if r.pidFile != "" {
		if err := createPidFile(r.pidFile, process); err != nil {
			r.terminate(process)
			r.destroy()
			tty.Close()
			return -1, err
		}
	}
	if r.detach {
		tty.Close()
		return 0, nil
	}
	logrus.Debugf("forward handler")
	status, err := handler.forward(process)
	if err != nil {
		r.terminate(process)
	}
	r.destroy()
	tty.Close()
	return status, err
}

func (r *runner) destroy() {
	if r.shouldDestroy {
		destroy(r.container)
	}
}

func (r *runner) terminate(p *libcontainer.Process) {
	p.Signal(syscall.SIGKILL)
	p.Wait()
}

func sPtr(s string) *string { return &s }

func createLibContainerRlimit(rlimit specs.Rlimit) (configs.Rlimit, error) {
	rl, err := strToRlimit(rlimit.Type)
	if err != nil {
		return configs.Rlimit{}, err
	}
	return configs.Rlimit{
		Type: rl,
		Hard: uint64(rlimit.Hard),
		Soft: uint64(rlimit.Soft),
	}, nil
}

// If systemd is supporting sd_notify protocol, this function will add support
// for sd_notify protocol from within the container.
func setupSdNotify(spec *specs.Spec, notifySocket string) {
	spec.Mounts = append(spec.Mounts, specs.Mount{Destination: notifySocket, Type: "bind", Source: notifySocket, Options: []string{"bind"}})
	spec.Process.Env = append(spec.Process.Env, fmt.Sprintf("NOTIFY_SOCKET=%s", notifySocket))
}
