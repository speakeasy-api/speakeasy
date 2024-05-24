package main

import (
	"github.com/speakeasy-api/speakeasy/cmd"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"runtime"
)

var (
	version      = "0.0.1"
	artifactArch = "linux_amd64"
)

func main() {
	if env.IsLocalDev() {
		if env.GoArch() != "" {
			artifactArch = env.GoArch()
		} else if artifactArch == "" {
			artifactArch = runtime.GOOS + "_" + runtime.GOARCH
		}
	}

	cmd.Execute(version, artifactArch)
}
