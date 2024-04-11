package env

import "os"

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
