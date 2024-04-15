package main

import (
	"github.com/speakeasy-api/speakeasy/cmd"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"runtime"
)

const defaultVersion = "0.0.1"

func main() {
	artifactArch := ""
	if env.GoArch() != "" {
		artifactArch = env.GoArch()
	} else if artifactArch == "" {
		artifactArch = runtime.GOOS + "_" + runtime.GOARCH
	}
	cmd.Execute(defaultVersion, artifactArch)
}
