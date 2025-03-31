package main

import (
	"runtime"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/speakeasy-api/speakeasy/cmd"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"go.uber.org/automaxprocs/maxprocs"
)

var (
	version      = "0.0.1"
	artifactArch = "linux_amd64"
)

func main() {
	memlimit.SetGoMemLimitWithOpts()
	maxprocs.Set()

	if env.IsLocalDev() {
		if env.GoArch() != "" {
			artifactArch = env.GoArch()
		} else if artifactArch == "" {
			artifactArch = runtime.GOOS + "_" + runtime.GOARCH
		}
	}

	cmd.Execute(version, artifactArch)
}
