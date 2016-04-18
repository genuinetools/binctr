package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/term"
	"github.com/jfrazelle/binctr/cryptar"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func unpackRootfs(spec *specs.Spec, keyin string) (err error) {
	fmt.Fprintf(os.Stdout, "Hello.\n")
	fmt.Fprintf(os.Stdout, "Let's play Global Thermonuclear War.\n")

	if keyin == "" {
		keyin, err = promptPasskey()
		if err != nil {
			return err
		}
	}

	key, err := base64.StdEncoding.DecodeString(keyin)
	if err != nil {
		return err
	}

	data, err := cryptar.Decrypt(DATA, key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(defaultRootfsDir, 0755); err != nil {
		return err
	}

	r := bytes.NewReader(data)
	if err := archive.Untar(r, defaultRootfsDir, nil); err != nil {
		return err
	}

	return nil
}

// prompt for passkey
func promptPasskey() (string, error) {
	inFd, _ := term.GetFdInfo(os.Stdin)
	oldState, err := term.SaveState(inFd)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stdout, "Key: ")
	term.DisableEcho(inFd, oldState)

	keyin := readInput(os.Stdin, os.Stdout)
	fmt.Fprint(os.Stdout, "\n")

	term.RestoreTerminal(inFd, oldState)
	return keyin, nil
}

func readInput(in io.Reader, out io.Writer) string {
	reader := bufio.NewReader(in)
	line, _, err := reader.ReadLine()
	if err != nil {
		fmt.Fprintln(out, err.Error())
		os.Exit(1)
	}
	return string(line)
}
