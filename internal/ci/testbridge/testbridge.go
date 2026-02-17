// Package testbridge provides a bridge between the CI actions and the speakeasy
// test runner, replacing the old subprocess-based cli.Test() call with a direct
// Go function call.
package testbridge

import (
	"context"

	"github.com/speakeasy-api/speakeasy/internal/testcmd"
)

// RunTest runs SDK tests for the specified target, replicating the behavior of
// `speakeasy test -t target`. If target is "all" or empty, all targets are tested.
func RunTest(ctx context.Context, target string) error {
	opts := []testcmd.RunnerOpt{testcmd.WithWorkflowTarget(target)}

	runner := testcmd.NewRunner(ctx, opts...)
	return runner.Run(ctx)
}
