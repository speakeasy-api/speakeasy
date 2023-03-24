package env

import "os"

func IsGithubAction() bool {
	return os.Getenv("GITHUB_ACTIONS") == "true"
}
