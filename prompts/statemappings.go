package prompts

import (
	"context"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
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
	IsUsingBlueprint         bool
	SDKName                  string
}

type Defaults struct {
	SchemaPath *string
	TargetType *string
	Blueprint  *string
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
