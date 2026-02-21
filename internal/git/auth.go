package git

import (
	"fmt"

	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// BasicAuth creates a go-git BasicAuth credential from an access token.
// This is the standard auth helper used for HTTPS git operations (clone, fetch, push)
// where a personal access token or GitHub token is used as the password.
// Returns nil if accessToken is empty.
func BasicAuth(accessToken string) *gitHttp.BasicAuth {
	if accessToken == "" {
		return nil
	}
	return &gitHttp.BasicAuth{
		Username: "gen",
		Password: accessToken,
	}
}

// ConfigureURLRewrite sets up a local git config url.<auth>.insteadOf rule
// so that native git commands authenticate using the provided token.
// This rewrites URLs like https://<host>/ to https://gen:<token>@<host>/
// making authentication transparent for subprocesses that shell out to git.
// Does nothing if accessToken is empty.
func ConfigureURLRewrite(repoDir, host, accessToken string) error {
	if accessToken == "" {
		return nil
	}
	authenticatedPrefix := fmt.Sprintf("https://gen:%s@%s/", accessToken, host)
	originalPrefix := fmt.Sprintf("https://%s/", host)
	_, err := RunGitCommand(repoDir, "config", "--local",
		fmt.Sprintf("url.%s.insteadOf", authenticatedPrefix),
		originalPrefix,
	)
	return err
}
