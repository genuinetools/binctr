package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/coreos/go-systemd/activation"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runc/libcontainer/utils"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// startContainer starts the container. Returns the exit status or -1 and an
// error. Signals sent to the current process will be forwarded to container.
func startContainer(spec *specs.Spec, id, pidFile, consoleSocket, root string, detach bool) (int, error) {
	notifySocket := newNotifySocket(id, root)
	if notifySocket != nil {
		// Setup the spec for the notify socket.
		notifySocket.setupSpec(spec)
	}

	// Create the libcontainer config.
	useSystemdCgroup := false
	config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		CgroupName:       id,
		UseSystemdCgroup: useSystemdCgroup,
		NoPivotRoot:      false,
		NoNewKeyring:     false,
		Spec:             spec,
		Rootless:         true,
	})
	if err != nil {
		return -1, err
	}

	// Load the factory.
	factory, err := loadFactory(root, useSystemdCgroup)
	if err != nil {
		return -1, err
	}

	// Create the factory.
	container, err := factory.Create(id, config)
	if err != nil {
		return -1, err
	}

	if notifySocket != nil {
		// Setup the socket for the notify socket.
		err := notifySocket.setupSocket()
		if err != nil {
			return -1, err
		}
	}

	// Support on-demand socket activation by passing file descriptors into
	// the container init process.
	listenFDs := []*os.File{}
	if os.Getenv("LISTEN_FDS") != "" {
		listenFDs = activation.Files(false)
	}

	// Initialize the runner.
	r := &runner{
		enableSubreaper: true,
		shouldDestroy:   true,
		container:       container,
		listenFDs:       listenFDs,
		notifySocket:    notifySocket,
		consoleSocket:   consoleSocket,
		detach:          detach,
		pidFile:         pidFile,
	}
	// Run the process.
	return r.run(spec.Process)
}

// loadFactory returns the configured factory instance for execing containers.
func loadFactory(root string, useSystemdCgroup bool) (libcontainer.Factory, error) {
	// Setup the cgroups manager. Default is cgroupfs.
	cgroupManager := libcontainer.Cgroupfs
	if useSystemdCgroup {
		if systemd.UseSystemd() {
			cgroupManager = libcontainer.SystemdCgroups
		} else {
			return nil, fmt.Errorf("systemd cgroup flag passed, but systemd support for managing cgroups is not available")
		}
	}

	// We resolve the paths for {newuidmap,newgidmap} from the context of runc,
	// to avoid doing a path lookup in the nsexec context. TODO: The binary
	// names are not currently configurable.
	newuidmap, err := exec.LookPath("newuidmap")
	if err != nil {
		newuidmap = ""
	}
	newgidmap, err := exec.LookPath("newgidmap")
	if err != nil {
		newgidmap = ""
	}

	// Create the new libcontainer factory.
	return libcontainer.New(root, cgroupManager, nil, nil,
		libcontainer.NewuidmapPath(newuidmap),
		libcontainer.NewgidmapPath(newgidmap))
}

// newProcess returns a new libcontainer Process with the arguments from the
// spec and stdio from the current process.
func newProcess(p specs.Process) (*libcontainer.Process, error) {
	// Create the libcontainer process.
	lp := &libcontainer.Process{
		Args:            p.Args,
		Env:             p.Env,
		User:            fmt.Sprintf("%d:%d", p.User.UID, p.User.GID),
		Cwd:             p.Cwd,
		Label:           p.SelinuxLabel,
		NoNewPrivileges: &p.NoNewPrivileges,
		AppArmorProfile: p.ApparmorProfile,
	}

	// Setup the console size.
	if p.ConsoleSize != nil {
		lp.ConsoleWidth = uint16(p.ConsoleSize.Width)
		lp.ConsoleHeight = uint16(p.ConsoleSize.Height)
	}

	// Convert the capabilities.
	if p.Capabilities != nil {
		lp.Capabilities = &configs.Capabilities{}
		lp.Capabilities.Bounding = p.Capabilities.Bounding
		lp.Capabilities.Effective = p.Capabilities.Effective
		lp.Capabilities.Inheritable = p.Capabilities.Inheritable
		lp.Capabilities.Permitted = p.Capabilities.Permitted
		lp.Capabilities.Ambient = p.Capabilities.Ambient
	}

	// Setup the additional user groups.
	for _, gid := range p.User.AdditionalGids {
		lp.AdditionalGroups = append(lp.AdditionalGroups, strconv.FormatUint(uint64(gid), 10))
	}

	// Setup the Rlimits.
	for _, rlimit := range p.Rlimits {
		rl, err := createLibContainerRlimit(rlimit)
		if err != nil {
			return nil, err
		}
		lp.Rlimits = append(lp.Rlimits, rl)
	}

	return lp, nil
}

func destroy(container libcontainer.Container) {
	if err := container.Destroy(); err != nil {
		logrus.Error(err)
	}
}

func setupIO(process *libcontainer.Process, rootuid, rootgid int, createTTY, detach bool, sockpath string) (*tty, error) {
	if createTTY {
		process.Stdin = nil
		process.Stdout = nil
		process.Stderr = nil
		t := &tty{}
		if !detach {
			parent, child, err := utils.NewSockPair("console")
			if err != nil {
				return nil, err
			}
			process.ConsoleSocket = child
			t.postStart = append(t.postStart, parent, child)
			t.consoleC = make(chan error, 1)
			go func() {
				if err := t.recvtty(process, parent); err != nil {
					t.consoleC <- err
				}
				t.consoleC <- nil
			}()
		} else {
			// the caller of runc will handle receiving the console master
			conn, err := net.Dial("unix", sockpath)
			if err != nil {
				return nil, err
			}
			uc, ok := conn.(*net.UnixConn)
			if !ok {
				return nil, fmt.Errorf("casting to UnixConn failed")
			}
			t.postStart = append(t.postStart, uc)
			socket, err := uc.File()
			if err != nil {
				return nil, err
			}
			t.postStart = append(t.postStart, socket)
			process.ConsoleSocket = socket
		}
		return t, nil
	}
	// when runc will detach the caller provides the stdio to runc via runc's 0,1,2
	// and the container's process inherits runc's stdio.
	if detach {
		if err := inheritStdio(process); err != nil {
			return nil, err
		}
		return &tty{}, nil
	}
	return setupProcessPipes(process, rootuid, rootgid)
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
	detach          bool
	shouldDestroy   bool
	consoleSocket   string
	pidFile         string
	container       libcontainer.Container
	listenFDs       []*os.File
	notifySocket    *notifySocket
}

func (r *runner) run(config *specs.Process) (int, error) {
	// Check the terminal settings.
	if r.detach && config.Terminal && r.consoleSocket == "" {
		return -1, fmt.Errorf("cannot allocate tty if runc will detach without setting console socket")
	}
	if (!r.detach || !config.Terminal) && r.consoleSocket != "" {
		return -1, fmt.Errorf("cannot use console socket if runc will not detach or allocate tty")
	}

	// Create the process.
	process, err := newProcess(*config)
	if err != nil {
		r.destroy()
		return -1, err
	}

	// Setup the listen file descriptors.
	if len(r.listenFDs) > 0 {
		process.Env = append(process.Env, fmt.Sprintf("LISTEN_FDS=%d", len(r.listenFDs)), "LISTEN_PID=1")
		process.ExtraFiles = append(process.ExtraFiles, r.listenFDs...)
	}

	// Get the rootuid.
	rootuid, err := r.container.Config().HostRootUID()
	if err != nil {
		r.destroy()
		return -1, err
	}

	// Get the rootgid.
	rootgid, err := r.container.Config().HostRootGID()
	if err != nil {
		r.destroy()
		return -1, err
	}

	// Setting up IO is a two stage process. We need to modify process to deal
	// with detaching containers, and then we get a tty after the container has
	// started.
	handler := newSignalHandler(r.enableSubreaper, r.notifySocket)
	tty, err := setupIO(process, rootuid, rootgid, config.Terminal, r.detach, r.consoleSocket)
	if err != nil {
		r.destroy()
		return -1, err
	}
	defer tty.Close()

	// Run the container.
	if err := r.container.Run(process); err != nil {
		r.destroy()
		tty.Close()
		return -1, err
	}

	// Wait for the tty.
	if err := tty.waitConsole(); err != nil {
		r.terminate(process)
		r.destroy()
		tty.Close()
		return -1, err
	}

	// Close after start the tty.
	if err = tty.ClosePostStart(); err != nil {
		r.terminate(process)
		r.destroy()
		tty.Close()
		return -1, err
	}

	// Create the pid file.
	if r.pidFile != "" {
		if err := createPidFile(r.pidFile, process); err != nil {
			r.terminate(process)
			r.destroy()
			tty.Close()
			return -1, err
		}
	}

	// Forward the handler.
	status, err := handler.forward(process, tty, detach)
	if err != nil {
		r.terminate(process)
	}

	// Return early if we are detaching.
	if r.detach {
		return 0, nil
	}

	// Cleanup.
	r.destroy()

	return status, err
}

func (r *runner) destroy() {
	if r.shouldDestroy {
		destroy(r.container)
	}
}

func (r *runner) terminate(p *libcontainer.Process) {
	_ = p.Signal(unix.SIGKILL)
	_, _ = p.Wait()
}

func createLibContainerRlimit(rlimit specs.POSIXRlimit) (configs.Rlimit, error) {
	rl, err := strToRlimit(rlimit.Type)
	if err != nil {
		return configs.Rlimit{}, err
	}
	return configs.Rlimit{
		Type: rl,
		Hard: rlimit.Hard,
		Soft: rlimit.Soft,
	}, nil
}
