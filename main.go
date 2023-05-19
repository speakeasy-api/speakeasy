package main

import (
	"github.com/speakeasy-api/speakeasy/cmd"
)

var (
	version      = "0.0.1"
	artifactArch = "linux_x86_64"
)

func main() {
	cmd.Execute(version, artifactArch)
}
