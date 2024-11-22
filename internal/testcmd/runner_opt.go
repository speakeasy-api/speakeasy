package testcmd

// RunnerOpt is a function that modifies a Runner.
type RunnerOpt func(*Runner)

// WithDisableMockserver is an option that skips starting the target testing
// mock API server.
func WithDisableMockserver() RunnerOpt {
	return func(r *Runner) {
		r.disableMockserver = true
	}
}

// WithVerboseOutput is an option that enables verbose output.
func WithVerboseOutput() RunnerOpt {
	return func(r *Runner) {
		r.verboseOutput = true
	}
}

// WithWorkflowTarget is an option that specifies a single workflow target to
// run testing against. If passed "all", all targets will be run.
func WithWorkflowTarget(workflowTarget string) RunnerOpt {
	if workflowTarget == "all" {
		workflowTarget = ""
	}

	return func(r *Runner) {
		r.workflowTarget = workflowTarget
	}
}
