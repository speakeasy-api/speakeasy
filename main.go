package main

import (
	"github.com/speakeasy-api/speakeasy/cmd"
)

var (
	version      = "0.0.1"
	artifactArch = "linux_amd64"
)

func main() {
	cmd.Execute(version, artifactArch)
}
