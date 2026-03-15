package actions

import (
	"context"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/changeset"
	"github.com/speakeasy-api/speakeasy/internal/ci/configuration"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
)

func ChangesetValidate(_ context.Context) error {
	wf, err := configuration.GetWorkflowAndValidateLanguages(true)
	if err != nil {
		return err
	}

	targets, err := collectChangesetReleaseTargets(wf, environment.GetWorkspace())
	if err != nil {
		return err
	}

	for _, target := range targets {
		if err := changeset.ValidateLineageOwnership(nil, target.Changesets); err != nil {
			return err
		}
	}

	logging.Info("Changeset validation succeeded")
	return nil
}
