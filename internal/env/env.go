package env

import "os"

func IsGithubAction() bool {
	return os.Getenv("GITHUB_ACTIONS") == "true"
}

func IsGithubDebugMode() bool {
	return os.Getenv("RUNNER_DEBUG") == "true"
}

func GoArch() string {
	return os.Getenv("GOARCH")
}
