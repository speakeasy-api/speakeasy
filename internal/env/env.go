package env

import (
	"os"

	"github.com/speakeasy-api/speakeasy-core/utils"
)

// Returns true if the CLI_RUNTIME environment variable is set to "docs".
// This environment variable is used to determine when website documentation
// is being rendered to prevent unexpected CLI formatting characters.
func IsDocsRuntime() bool {
	return os.Getenv("CLI_RUNTIME") == "docs"
}

func IsGithubAction() bool {
	return os.Getenv("GITHUB_ACTIONS") == "true"
}

func IsGithubDebugMode() bool {
	return os.Getenv("RUNNER_DEBUG") == "true"
}

func PinnedVersion() string {
	return os.Getenv("PINNED_VERSION")
}

func GoArch() string {
	return os.Getenv("GOARCH")
}

func IsLocalDev() bool {
	return os.Getenv("SPEAKEASY_ENVIRONMENT") == "local"
}

func IsCI() bool {
	return os.Getenv("CI") == "true" || IsGithubAction() || utils.IsRunningInCI()
}

// Returns the SPEAKEASY_RUN_LOCATION environment variable value. For example,
// this is set by Speakeasy maintained GitHub Actions to "action".
func SpeakeasyRunLocation() string {
	return os.Getenv("SPEAKEASY_RUN_LOCATION")
}

// speakeasyVersion holds the version of the currently running speakeasy CLI binary,
// set during startup via cmd.Execute -> SetSpeakeasyVersion.
var speakeasyVersion string

// SetSpeakeasyVersion stores the CLI version for access by internal packages.
func SetSpeakeasyVersion(v string) {
	speakeasyVersion = v
}

// SpeakeasyVersion returns the version of the currently running speakeasy CLI binary.
func SpeakeasyVersion() string {
	return speakeasyVersion
}
