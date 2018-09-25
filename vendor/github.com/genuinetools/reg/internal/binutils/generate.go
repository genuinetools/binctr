// +build ignore

package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/shurcooL/vfsgen"
	"github.com/sirupsen/logrus"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		logrus.Fatal(err)
	}
	// Generate server assets.
	assets := http.Dir(filepath.Join(wd, "server/static"))
	if err := vfsgen.Generate(assets, vfsgen.Options{
		Filename:     filepath.Join(wd, "internal/binutils/static", "static.go"),
		PackageName:  "static",
		VariableName: "Assets",
	}); err != nil {
		logrus.Fatal(err)
	}
	// Generate template assets.
	assets = http.Dir(filepath.Join(wd, "server/templates"))
	if err := vfsgen.Generate(assets, vfsgen.Options{
		Filename:     filepath.Join(wd, "internal/binutils/templates", "templates.go"),
		PackageName:  "templates",
		VariableName: "Assets",
	}); err != nil {
		logrus.Fatal(err)
	}
}
