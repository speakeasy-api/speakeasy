package actions

import (
	"context"

	"github.com/speakeasy-api/speakeasy/internal/ci/configuration"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/speakeasy/internal/ci/registry"
	"github.com/speakeasy-api/speakeasy/internal/ci/tagbridge"
	"golang.org/x/exp/maps"
)

func Tag(ctx context.Context) error {
	tags := registry.ProcessRegistryTags()

	sources := environment.SpecifiedSources()
	targets := environment.SpecifiedCodeSamplesTargets()

	if len(sources) == 0 && len(targets) == 0 {
		wf, err := configuration.GetWorkflowAndValidateLanguages(false)
		if err != nil {
			return err
		}

		sources = maps.Keys(wf.Sources)
		targets = maps.Keys(wf.Targets)

		logging.Info("No sources or targets specified, using all sources and targets from workflow")
	}

	return tagbridge.TagPromote(ctx, tags, sources, targets)
}
