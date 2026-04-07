package git

import (
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

// ConfigureURLRewrite avoids persisting access tokens in git config.
func ConfigureURLRewrite(repoDir, host, accessToken string) error {
	return nil
}
