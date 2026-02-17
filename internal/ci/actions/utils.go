package actions

import (
	"path/filepath"

	"github.com/speakeasy-api/speakeasy/internal/ci/configuration"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
)

func getReleasesDir() (string, error) {
	releasesDir := "."
	if environment.GetWorkingDirectory() != "" {
		releasesDir = environment.GetWorkingDirectory()
	}
	// Find releases file
	wf, err := configuration.GetWorkflowAndValidateLanguages(false)
	if err != nil {
		return "", err
	}

	// Checking for multiple targets ensures backward compatibility with the code below
	if len(wf.Targets) > 1 && environment.SpecifiedTarget() != "" {
		if target, ok := wf.Targets[environment.SpecifiedTarget()]; ok && target.Output != nil {
			if releasesDir != "." {
				releasesDir = filepath.Join(releasesDir, *target.Output)
			} else {
				releasesDir = *target.Output
			}

			return releasesDir, nil
		}
	}

	for _, target := range wf.Targets {
		// If we are only generating one language and its not in the root directory we assume this is a multi-sdk repo
		if len(wf.Targets) == 1 && target.Output != nil && *target.Output != "." {
			if releasesDir != "." {
				releasesDir = filepath.Join(releasesDir, *target.Output)
			} else {
				releasesDir = *target.Output
			}
		}
	}

	return releasesDir, nil
}
