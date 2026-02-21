package actions

import (
	"context"

	"github.com/speakeasy-api/speakeasy/internal/ci/document"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/speakeasy/internal/ci/suggestions"
)

func Suggest(ctx context.Context) error {
	g, err := initAction()
	if err != nil {
		return err
	}

	docPath, _, err := document.GetOpenAPIFileInfo(ctx)
	if err != nil {
		return err
	}

	outputs := make(map[string]string)

	branchName := ""

	branchName, _, err = g.FindExistingPR("", environment.ActionSuggest, false)
	if err != nil {
		return err
	}

	branchName, err = g.FindOrCreateBranch(branchName, environment.ActionSuggest)
	if err != nil {
		return err
	}

	success := false
	defer func() {
		if !success && !environment.IsDebugMode() {
			if err := g.DeleteBranch(branchName); err != nil {
				logging.Debug("failed to delete branch %s: %v", branchName, err)
			}
		}
	}()

	out, err := suggestions.Suggest(ctx, docPath, environment.GetMaxSuggestions())
	if err != nil {
		return err
	}

	outputs["cli_output"] = out

	if _, err := g.CommitAndPush("", "", environment.GetOpenAPIDocOutput(), environment.ActionSuggest, false, nil); err != nil {
		return err
	}

	outputs["branch_name"] = branchName

	if err := setOutputs(outputs); err != nil {
		return err
	}

	success = true

	return nil
}
