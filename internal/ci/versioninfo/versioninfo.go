// Package versioninfo provides access to the speakeasy CLI and generation engine
// versions from within CI code, replacing the old subprocess-based version queries.
package versioninfo

import (
	"strings"

	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/speakeasy/internal/env"
)

// GetSpeakeasyVersion returns the version of the currently running speakeasy CLI.
func GetSpeakeasyVersion() string {
	return env.SpeakeasyVersion()
}

// GetGenerationVersion returns the generation engine version from the openapi-generation module.
func GetGenerationVersion() string {
	return strings.TrimPrefix(changelog.GetLatestVersion(), "v")
}
