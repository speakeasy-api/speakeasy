package prompts

import (
	"context"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
)

type (
	formFunction    func(ctx context.Context, quickstart *Quickstart) (*QuickstartState, error)
	QuickstartState int
)

type Quickstart struct {
	WorkflowFile             *workflow.Workflow
	LanguageConfigs          map[string]*config.Configuration
	Defaults                 Defaults
	IsUsingSampleOpenAPISpec bool
	IsUsingTemplate          bool
	SDKName                  string
	SkipInteractive          bool
}

type Defaults struct {
	SchemaPath *string
	TargetType *string

	// The template to use for quickstart. A template is a pre-configured OAS spec retrieved
	// from the registry schema store.
	// The corresponding CLI flag is --from, e.g:
	// speakeasy quickstart --from wandering-octopus-129129
	Template *string

	TemplateData *shared.SchemaStoreItem

	// SDK name in PascalCase (e.g., "MyCompanySDK")
	// The corresponding CLI flag is --name
	SDKName *string

	// Package name for the generated SDK (e.g., "my-company-sdk" for npm, Go module path for Go)
	// The corresponding CLI flag is --package-name
	PackageName *string

	// Whether to initialize a git repository in the output directory
	// The corresponding CLI flag is --init-git
	InitGit *bool
}

// Define constants using iota
const (
	Complete QuickstartState = iota
	SourceBase
	TargetBase
	ConfigBase
)

// TODO: Add Github Configuration Next
var StateMapping map[QuickstartState]formFunction = map[QuickstartState]formFunction{
	SourceBase: sourceBaseForm,
	TargetBase: targetBaseForm,
	ConfigBase: configBaseForm,
}
