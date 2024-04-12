package main

import (
	"github.com/speakeasy-api/speakeasy/cmd"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"runtime"
)

var (
	version      = "0.0.1"
	artifactArch = ""
)

func main() {
	if env.GoArch() != "" {
		artifactArch = env.GoArch()
	} else if artifactArch == "" {
		artifactArch = runtime.GOOS + "_" +runtime.GOARCH
	}
	cmd.Execute(version, artifactArch)
}
