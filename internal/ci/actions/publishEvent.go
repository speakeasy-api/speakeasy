package actions

import (
	"fmt"
	"os"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/ci/telemetry"
)

func PublishEventAction() error {
	g, err := initAction()
	if err != nil {
		return err
	}

	version, err := telemetry.TriggerPublishingEvent(os.Getenv("INPUT_TARGET_DIRECTORY"), os.Getenv("GH_ACTION_RESULT"), os.Getenv("INPUT_REGISTRY_NAME"))
	if version != "" {
		if strings.Contains(os.Getenv("GH_ACTION_RESULT"), "success") {
			if err = g.SetReleaseToPublished(version, os.Getenv("INPUT_TARGET_DIRECTORY")); err != nil {
				fmt.Println("Failed to set release to published %w", err)
			}
		}
	}

	return err
}
