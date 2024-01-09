package quickstart

import "github.com/speakeasy-api/sdk-gen-config/workflow"

type (
	formFunction func(workflow *workflow.Workflow) (*State, error)
	State        int
)

// Define constants using iota
const (
	Complete State = iota
	SourceBase
	TargetBase
)

var StateMapping map[State]formFunction = map[State]formFunction{
	SourceBase: sourceBaseForm,
	TargetBase: targetBaseForm,
}
