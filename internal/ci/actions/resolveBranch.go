package actions

import (
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
)

// ResolveBranch finds an existing PR branch or creates a new one.
// It outputs the branch_name for use by downstream matrix jobs via INPUT_BRANCH_NAME.
// This is intended to be called once in a "prep" job before fanning out to parallel targets.
func ResolveBranch() error {
	g, err := initAction()
	if err != nil {
		return err
	}

	sourcesOnly := false // resolve-branch is only used for SDK generation workflows

	// Look for an existing open PR to reuse its branch
	branchName, _, err := g.FindExistingPR(environment.GetFeatureBranch(), environment.ActionRunWorkflow, sourcesOnly)
	if err != nil {
		return err
	}

	// If no existing PR was found, create a new branch
	if branchName == "" {
		branchName, err = g.FindOrCreateBranch("", environment.ActionRunWorkflow)
		if err != nil {
			return err
		}
	} else {
		// Existing PR found — check out its branch
		logging.Info("Reusing existing PR branch: %s", branchName)
		if _, err := g.FindAndCheckoutBranch(branchName); err != nil {
			return err
		}
	}

	outputs := map[string]string{
		"branch_name": branchName,
	}

	return setOutputs(outputs)
}
