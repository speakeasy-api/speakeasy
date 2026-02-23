package actions

import (
	"errors"

	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/git"
)

func initAction() (*git.Git, error) {
	accessToken := environment.GetAccessToken()
	if accessToken == "" {
		return nil, errors.New("github access token is required")
	}

	g := git.New(accessToken)
	if err := g.OpenRepo(); err != nil {
		return nil, err
	}

	return g, nil
}
