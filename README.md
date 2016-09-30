# binctr

Create fully static, including rootfs embedded, binaries that pop you directly
into a container. **Can be run by an unprivileged user.**

Check out the blog post: [blog.jessfraz.com/post/getting-towards-real-sandbox-containers](https://blog.jessfraz.com/post/getting-towards-real-sandbox-containers/).

This is based off a crazy idea from [@crosbymichael](https://github.com/crosbymichael)
who first embedded an image in a binary :D

**NOTE**

You may have noticed you can't file an issue. That's because this is using a crazy
person's (aka my) fork of libcontainer and until I get the patches into upstream
there's no way in hell I'm fielding issues from whoever is crazy enough to try this.

If you are interested, I have started a thread on the
[mailing list](https://groups.google.com/a/opencontainers.org/forum/#!topic/dev/yutVaSLcqWI)
with my proposed steps to make this a reality. Note, adding a `+1` is _not_ of any
value to anyone though.


**Nginx running with my user "jessie".**

![nginx.png](nginx.png)


### Building

This uses the new Golang vendoring so you need go 1.6 or
`GO15VENDOREXPERIMENT=1` in your env.

You will also need `libapparmor-dev` and `libseccomp-dev`.

Most importantly you need userns in your kernel (`CONFIG_USER_NS=y`)
or else this won't even work.

```console
$ make static
Static container created at: ./bin/alpine
Run with ./bin/alpine

# building a different base image
$ make static IMAGE=busybox
Static container created at: ./bin/busybox
Run with ./bin/busybox
```

### Running

```console
$ ./alpine
$ ./busybox --read-only
```

### Running with custom commands & args

```console
# let's make an small web server binary
$ make static IMAGE=r.j3ss.co/hello
Static container created at: ./bin/hello
Run with ./bin/hello

$ ./bin/hello /hello
2016/04/18 04:59:25 Starting server on port:  8080

# But we have no networking! How can we reach it! Don't worry we can fix this
# Let's install my super cool binary for setting up networking in a container
$ go get github.com/jessfraz/netns

# now we can add this as a prestart hook
$ ./bin/hello --hook prestart:netns /hello
2016/04/18 04:59:25 Starting server on port:  8080

# let's get the ip file
$ cat .ip
172.19.0.10

# we can curl it
$ curl -sSL $(cat .ip):8080
Hello World!

Success!
```

### Usage

```console
$ ./bin/alpine -h
 _     _            _
| |__ (_)_ __   ___| |_ _ __
| '_ \| | '_ \ / __| __| '__|
| |_) | | | | | (__| |_| |
|_.__/|_|_| |_|\___|\__|_|

 Fully static, self-contained container including the rootfs
 that can be run by an unprivileged user.

 Embedded Image: alpine - sha256:70c557e50ed630deed07cbb0dc4d28aa0f2a485cf7af124cc48f06bce83f784b
 Version: 0.1.0
 GitCommit: 13fcd27-dirty

  -D	run in debug mode
  -console string
    	the pty slave path for use with the container
  -d	detach from the container's process
  -hook value
    	Hooks to prefill into spec file. (ex. --hook prestart:netns) (default [])
  -id string
    	container ID (default "nginx")
  -pid-file string
    	specify the file to write the process id to
  -read-only
    	make container filesystem readonly
  -root string
    	root directory of container state, should be tmpfs (default "/run/binctr")
  -t	allocate a tty for the container (default true)
  -v	print version and exit (shorthand)
  -version
    	print version and exit
```

## Cool things

The binary spawned does NOT need to oversee the container process if you
run in detached mode with a PID file. You can have it watched by the user mode
systemd so that this binary is really just the launcher :)

## Caveats

- cgroups: coming soon
