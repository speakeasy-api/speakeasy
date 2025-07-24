package prompts_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/charmtest"
	"github.com/speakeasy-api/speakeasy/prompts"
	"github.com/stretchr/testify/assert"
)

func TestTargetSpecificForms_Go_Quickstart(t *testing.T) {
	t.Parallel()

	targetName := "go"
	groups, targetFormFields := setupTargetSpecificFormsQuickstart(t, targetName)

	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}

	if len(targetFormFields) != 2 {
		t.Errorf("expected 2 target form fields, got %d", len(targetFormFields))
	}

	tm := charmtest.ModelFromHuhGroup(t, groups...)
	tm.AssertContains(t,
		"┃ Choose a modulePath",
		"┃ Root module path. To install your SDK, users will execute go get github.com/my-company/company-go-sdk",
		"┃ github.com/my-company/company-go-sdk",
	)
	tm.Type("!")
	tm.SendKeys(tea.KeyEnter)
	tm.AssertContains(t,
		"┃ Root module path. To install your SDK, users will execute go get github.com/my-company/company-go-sdk!",
		"* Letters, numbers, or /.-_~ only. Cannot start or end with slash or dot.",
	)
	tm.SendKeys(
		tea.KeyBackspace,
		tea.KeyEnter,
	)
	tm.AssertContains(t,
		"┃ Choose a sdkPackageName",
		"┃ Root module package name. To instantiate your SDK, users will call company.New()",
		"┃ company",
	)
	tm.Type("!")
	tm.SendKeys(tea.KeyEnter)
	tm.AssertContains(t,
		"┃ Root module package name. To instantiate your SDK, users will call company!.New()",
		"* Letters, numbers, or underscore (_) only. Must start with letter.",
	)
	tm.SendKeys(
		tea.KeyBackspace,
		tea.KeyEnter,
		tea.KeyEnter,
	)
	tm.Quit(t)
	tm.AssertFormStringEqual(t, "modulePath", "github.com/my-company/company-go-sdk")
	tm.AssertFormStringEqual(t, "sdkPackageName", "company")

	// Assert the target form fields used by consuming code matches the expected values.
	assert.Equal(t,
		"github.com/my-company/company-go-sdk",
		targetFormFields.GetField("modulePath").GetValueString(),
	)
	assert.Equal(t,
		"company",
		targetFormFields.GetField("sdkPackageName").GetValueString(),
	)
}

func setupTargetSpecificFormsQuickstart(t *testing.T, targetName string) ([]*huh.Group, prompts.TargetFormFields) {
	t.Helper()

	quickstart := &prompts.Quickstart{}

	target, err := generate.GetTargetFromTargetString(targetName)

	if err != nil {
		t.Fatalf("error getting target for name %s: %s", targetName, err)
	}

	existingConfiguration, err := config.GetDefaultConfig(true, generate.GetLanguageConfigDefaults, map[string]bool{target.Target: true})

	if err != nil {
		t.Fatalf("error getting default config for %s: %s", target.Target, err)
	}

	genConfigFields, err := generate.GetLanguageConfigFields(target, true)

	if err != nil {
		t.Fatalf("error getting generator target configuration fields for %s: %s", target.Target, err)
	}

	groups, targetFormFields, err := prompts.TargetSpecificForms(
		targetName,
		existingConfiguration,
		genConfigFields,
		quickstart,
		"",
	)

	if err != nil {
		t.Fatalf("error getting target specific forms for %s: %s", target.Target, err)
	}

	return groups, targetFormFields
}
