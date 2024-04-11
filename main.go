package main

import (
	"github.com/speakeasy-api/speakeasy/cmd"
	"github.com/speakeasy-api/speakeasy/internal/env"
)

var (
	version      = "0.0.1"
	artifactArch = "linux_amd64"
)

func main() {
	if env.GoArch() != "" {
		artifactArch = env.GoArch()
	}
	cmd.Execute(version, artifactArch)
}
