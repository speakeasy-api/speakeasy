package actions

import (
	"context"
	"maps"
	"slices"

	"github.com/speakeasy-api/speakeasy/internal/ci/configuration"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/speakeasy/internal/ci/registry"
	"github.com/speakeasy-api/speakeasy/internal/ci/tagbridge"
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

		sources = slices.Collect(maps.Keys(wf.Sources))
		targets = slices.Collect(maps.Keys(wf.Targets))

		logging.Info("No sources or targets specified, using all sources and targets from workflow")
	}

	return tagbridge.TagPromote(ctx, tags, sources, targets)
}
