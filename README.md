# binctr

[![Build Status](https://travis-ci.org/genuinetools/binctr.svg?branch=master)](https://travis-ci.org/genuinetools/binctr)

Create fully static, including rootfs embedded, binaries that pop you directly
into a container. **Can be run by an unprivileged user.**

Check out the blog post: [blog.jessfraz.com/post/getting-towards-real-sandbox-containers](https://blog.jessfraz.com/post/getting-towards-real-sandbox-containers/).

This is based off a crazy idea from [@crosbymichael](https://github.com/crosbymichael)
who first embedded an image in a binary :D

**HISTORY**

This project used to use a POC fork of libcontainer until [@cyphar](https://github.com/cyphar)
got rootless containers into upstream! Woohoo!
Check out the original thread on the 
[mailing list](https://groups.google.com/a/opencontainers.org/forum/#!topic/dev/yutVaSLcqWI).

**Nginx running with my user "jessie".**

![nginx.png](nginx.png)


### Building

You will need `libapparmor-dev` and `libseccomp-dev`.

Most importantly you need userns in your kernel (`CONFIG_USER_NS=y`)
or else this won't even work.

```console
$ make build
Static container created at: ./bin/alpine
Run with ./bin/alpine

# building a different base image
$ make build IMAGE=busybox
Static container created at: ./bin/busybox
Run with ./bin/busybox
```

### Running

```console
$ ./alpine
$ ./busybox --read-only
```

### Usage

```console
 _     _            _
| |__ (_)_ __   ___| |_ _ __
| '_ \| | '_ \ / __| __| '__|
| |_) | | | | | (__| |_| |
|_.__/|_|_| |_|\___|\__|_|

 Fully static, self-contained container including the rootfs
 that can be run by an unprivileged user.

 Embedded Image: alpine - sha256:3fd9065eaf02feaf94d68376da52541925650b81698c53c6824d92ff63f98353
 Version: 0.1.0
 Build: 91b3ab5-dirty

  -D    run in debug mode
  -console-socket string
        path to an AF_UNIX socket which will receive a file descriptor referencing the master end of the console's pseudoterminal
  -d    detach from the container's process
  -hook value
        Hooks to prefill into spec file. (ex. --hook prestart:netns)
  -id string
        container ID
  -pid-file string
        specify the file to write the process id to
  -read-only
        make container filesystem readonly
  -root string
        root directory of container state, should be tmpfs (default "/tmp/binctr")
  -t    allocate a tty for the container (default true)
  -v    print version and exit (shorthand)
  -version
        print version and exit
```

## Cool things

The binary spawned does NOT need to oversee the container process if you
run in detached mode with a PID file. You can have it watched by the user mode
systemd so that this binary is really just the launcher :)
