package testutils

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
)

// StartRegistry starts a new registry container.
func StartRegistry(dcli *client.Client, config, username, password string) (string, string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", "", errors.New("No caller information")
	}

	image := "registry:2"

	if err := pullDockerImage(dcli, image); err != nil {
		return "", "", err
	}

	r, err := dcli.ContainerCreate(
		context.Background(),
		&container.Config{
			Image: image,
		},
		&container.HostConfig{
			NetworkMode: "host",
			Binds: []string{
				filepath.Join(filepath.Dir(filename), "configs", config) + ":" + "/etc/docker/registry/config.yml" + ":ro",
				filepath.Join(filepath.Dir(filename), "configs", "htpasswd") + ":" + "/etc/docker/registry/htpasswd" + ":ro",
				filepath.Join(filepath.Dir(filename), "snakeoil") + ":" + "/etc/docker/registry/ssl" + ":ro",
			},
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
		},
		nil, "")
	if err != nil {
		return "", "", err
	}

	// start the container
	if err := dcli.ContainerStart(context.Background(), r.ID, types.ContainerStartOptions{}); err != nil {
		return r.ID, "", err
	}

	port := ":5000"
	addr := "https://localhost" + port

	if err := waitForConn(addr, filepath.Join(filepath.Dir(filename), "snakeoil", "cert.pem"), filepath.Join(filepath.Dir(filename), "snakeoil", "key.pem")); err != nil {
		return r.ID, addr, err
	}

	if err := dockerLogin("localhost"+port, username, password); err != nil {
		return r.ID, addr, err
	}

	// Prefill the images.
	images := []string{"alpine:3.5", "alpine:latest", "busybox:latest", "busybox:musl", "busybox:glibc"}
	for _, image := range images {
		if err := prefillRegistry(image, dcli, "localhost"+port, username, password); err != nil {
			return r.ID, addr, err
		}
	}

	return r.ID, addr, nil
}

func startClairDB(dcli *client.Client) (string, error) {
	image := "postgres:latest"

	if err := pullDockerImage(dcli, image); err != nil {
		return "", err
	}

	c, err := dcli.ContainerCreate(
		context.Background(),
		&container.Config{
			Image: image,
			Env: []string{
				"POSTGRES_PASSWORD=password",
				"POSTGRES_DB=clair",
				"POSTGRES_USER=hacker",
			},
		},
		&container.HostConfig{
			NetworkMode: "host",
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
		},
		nil, "")
	if err != nil {
		return "", err
	}

	// start the container
	return c.ID, dcli.ContainerStart(context.Background(), c.ID, types.ContainerStartOptions{})
}

// StartClair starts a new clair container and accompanying database.
func StartClair(dcli *client.Client) (string, string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", "", errors.New("No caller information")
	}

	// start the database container.
	dbID, err := startClairDB(dcli)
	if err != nil {
		return dbID, "", err
	}

	image := "clair:dev"

	// build the docker image
	// create the tar ball
	ctx := filepath.Dir(filepath.Dir(filename))
	tw, err := tarit(ctx)
	if err != nil {
		return dbID, "", fmt.Errorf("tarit: %v", err)
	}

	// build the image
	resp, err := dcli.ImageBuild(context.Background(), tw, types.ImageBuildOptions{
		Tags:           []string{image},
		Dockerfile:     "Dockerfile.clair",
		ForceRemove:    true,
		Remove:         true,
		SuppressOutput: false,
		PullParent:     true,
	})
	if err != nil {
		return dbID, "", err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return dbID, "", err
	}

	fmt.Printf("body: %s\n", string(b))

	c, err := dcli.ContainerCreate(
		context.Background(),
		&container.Config{
			Image: image,
		},
		&container.HostConfig{
			NetworkMode: "host",
			Binds: []string{
				filepath.Join(filepath.Dir(filename), "configs", "clair.yml") + ":" + "/etc/clair/config.yaml" + ":ro",
			},
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
		},
		nil, "")
	if err != nil {
		return dbID, c.ID, err
	}

	// start the container
	err = dcli.ContainerStart(context.Background(), c.ID, types.ContainerStartOptions{})

	// wait for clair to start
	// TODO: make this not a sleep
	time.Sleep(time.Second * 5)

	return dbID, c.ID, err
}

// RemoveContainer removes with force a container by it's container ID.
func RemoveContainer(dcli *client.Client, ctrs ...string) (err error) {
	for _, c := range ctrs {
		err = dcli.ContainerRemove(context.Background(), c,
			types.ContainerRemoveOptions{
				RemoveVolumes: true,
				Force:         true,
			})
	}

	return err
}

// dockerLogin logins via the command line to a docker registry
func dockerLogin(addr, username, password string) error {
	cmd := exec.Command("docker", "login", "--username", username, "--password", password, addr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker login [%s] failed with output %q and error: %v", strings.Join(cmd.Args, " "), string(out), err)
	}
	return nil
}

// prefillRegistry adds images to a registry.
func prefillRegistry(image string, dcli *client.Client, addr, username, password string) error {
	if err := pullDockerImage(dcli, image); err != nil {
		return err
	}

	if err := dcli.ImageTag(context.Background(), image, addr+"/"+image); err != nil {
		return err
	}

	auth, err := constructRegistryAuth(username, password)
	if err != nil {
		return err
	}

	resp, err := dcli.ImagePush(context.Background(), addr+"/"+image, types.ImagePushOptions{
		RegistryAuth: auth,
	})
	if err != nil {
		return err
	}
	defer resp.Close()

	fd, isTerm := term.GetFdInfo(os.Stdout)

	return jsonmessage.DisplayJSONMessagesStream(resp, os.Stdout, fd, isTerm, nil)
}

func pullDockerImage(dcli *client.Client, image string) error {
	exists, err := imageExists(dcli, image)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	resp, err := dcli.ImagePull(context.Background(), image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer resp.Close()

	fd, isTerm := term.GetFdInfo(os.Stdout)

	return jsonmessage.DisplayJSONMessagesStream(resp, os.Stdout, fd, isTerm, nil)
}

func imageExists(dcli *client.Client, image string) (bool, error) {
	_, _, err := dcli.ImageInspectWithRaw(context.Background(), image)
	if err == nil {
		return true, nil
	}

	if client.IsErrNotFound(err) {
		return false, nil
	}

	return false, err
}

// waitForConn takes a tcp addr and waits until it is reachable
func waitForConn(addr, cert, key string) error {
	tlsCert, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return fmt.Errorf("Could not load X509 key pair: %v. Make sure the key is not encrypted", err)
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("failed to read system certificates: %v", err)
	}
	pem, err := ioutil.ReadFile(cert)
	if err != nil {
		return fmt.Errorf("could not read CA certificate %s: %v", cert, err)
	}
	if !certPool.AppendCertsFromPEM(pem) {
		return fmt.Errorf("failed to append certificates from PEM file: %s", cert)
	}

	c := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
				MinVersion:   tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				},
				RootCAs: certPool,
			},
		},
	}

	n := 0
	max := 10
	for n < max {
		if _, err := c.Get(addr + "/v2/"); err != nil {
			fmt.Printf("try number %d to %s: %v\n", n, addr, err)
			n++
			if n != max {
				fmt.Println("sleeping for 1 second then will try again...")
				time.Sleep(time.Second)
			} else {
				return fmt.Errorf("[WHOOPS]: maximum retries for %s exceeded", addr)
			}
			continue
		} else {
			break
		}
	}

	return nil
}

// constructRegistryAuth serializes the auth configuration as JSON base64 payload.
func constructRegistryAuth(identity, secret string) (string, error) {
	buf, err := json.Marshal(types.AuthConfig{Username: identity, Password: secret})
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(buf), nil
}

func tarit(src string) (io.Reader, error) {
	s := bytes.NewBuffer(nil)
	t := bytes.NewBuffer(nil)
	buf := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(t))
	tarball := tar.NewWriter(s)

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		var link string
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			if link, err = os.Readlink(path); err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}

		header.Name = strings.TrimPrefix(path, src)
		if err := tarball.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !info.Mode().IsRegular() { //nothing more to do for non-regular
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tarball, file)
		return err
	})
	if err != nil {
		return nil, err
	}

	if _, err := s.WriteTo(buf); err != nil {
		return nil, err
	}

	err = buf.Writer.Flush()
	return t, err
}
