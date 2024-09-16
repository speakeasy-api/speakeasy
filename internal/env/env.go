package env

import "os"

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
