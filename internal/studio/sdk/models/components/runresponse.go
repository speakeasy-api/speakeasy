// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package components

// RunResponse - Map of target run summaries
type RunResponse struct {
	// Link to the linting report
	LintingReportLink *string        `json:"lintingReportLink,omitempty"`
	SourceResult      SourceResponse `json:"sourceResult"`
	// Map of target results
	TargetResults map[string]TargetRunSummary `json:"targetResults"`
	Workflow      Workflow                    `json:"workflow"`
	// Working directory
	WorkingDirectory string `json:"workingDirectory"`
	// Time taken to run the workflow in milliseconds
	Took int64 `json:"took"`
}

func (o *RunResponse) GetLintingReportLink() *string {
	if o == nil {
		return nil
	}
	return o.LintingReportLink
}

func (o *RunResponse) GetSourceResult() SourceResponse {
	if o == nil {
		return SourceResponse{}
	}
	return o.SourceResult
}

func (o *RunResponse) GetTargetResults() map[string]TargetRunSummary {
	if o == nil {
		return map[string]TargetRunSummary{}
	}
	return o.TargetResults
}

func (o *RunResponse) GetWorkflow() Workflow {
	if o == nil {
		return Workflow{}
	}
	return o.Workflow
}

func (o *RunResponse) GetWorkingDirectory() string {
	if o == nil {
		return ""
	}
	return o.WorkingDirectory
}

func (o *RunResponse) GetTook() int64 {
	if o == nil {
		return 0
	}
	return o.Took
}
