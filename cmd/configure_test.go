package cmd

import (
	"testing"

	"github.com/speakeasy-api/speakeasy/prompts"
)

func TestTerraformNamingWarningIntegration(t *testing.T) {
	// Test that the warning is properly detected and handled
	warning := &prompts.TerraformNamingWarning{RepoName: "test-repo"}

	if !prompts.IsTerraformNamingWarning(warning) {
		t.Error("Expected IsTerraformNamingWarning to return true for TerraformNamingWarning")
	}

	// Test that the warning message is properly formatted
	expectedMsg := "Terraform repository naming warning: repository 'test-repo' does not follow the required naming convention"
	if warning.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, warning.Error())
	}
}
