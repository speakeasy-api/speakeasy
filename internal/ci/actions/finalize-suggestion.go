package actions

import (
	"errors"

	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/speakeasy/internal/ci/suggestions"
)

func FinalizeSuggestion() error {
	g, err := initAction()
	if err != nil {
		return err
	}

	branchName := environment.GetBranchName()
	if branchName == "" {
		return errors.New("branch name is required")
	}

	success := false

	defer func() {
		if !success {
			if err := g.DeleteBranch(branchName); err != nil {
				logging.Debug("failed to delete branch %s: %v", branchName, err)
			}
		}
	}()

	branchName, err = g.FindAndCheckoutBranch(branchName)
	if err != nil {
		return err
	}

	branchName, _, err = g.FindExistingPR(branchName, environment.ActionFinalize, false)
	if err != nil {
		return err
	}

	prNumber, _, err := g.CreateSuggestionPR(branchName, environment.GetOpenAPIDocOutput())
	if err != nil {
		return err
	}

	out := environment.GetCliOutput()
	if out != "" {
		if err = suggestions.WriteSuggestions(g, *prNumber, out); err != nil {
			return err
		}
	}

	success = true

	return nil
}
