package quickstart

import (
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
)

type (
	formFunction func(quickstart *Quickstart) (*State, error)
	State        int
)

type Quickstart struct {
	WorkflowFile    *workflow.Workflow
	LanguageConfigs map[string]*config.Configuration
	GithubWorkflow  *config.GenerateWorkflow
}

// Define constants using iota
const (
	Complete State = iota
	SourceBase
	TargetBase
	ConfigBase
	GithubWorkflowBase
)

// TODO: Add Github Configuration Next
var StateMapping map[State]formFunction = map[State]formFunction{
	SourceBase:         sourceBaseForm,
	TargetBase:         targetBaseForm,
	ConfigBase:         configBaseForm,
	GithubWorkflowBase: githubWorkflowBaseForm,
}
