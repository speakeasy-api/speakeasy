package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// TargetSubprocessResult holds the outcome of a single parallel target run.
type TargetSubprocessResult struct {
	TargetID string
	Output   string // combined stdout+stderr
	Err      error
}

// RunTargetsParallel spawns a subprocess for each target, runs them concurrently,
// and returns all results. Each subprocess is a complete `speakeasy run -t <target>`
// invocation using the same binary.
func RunTargetsParallel(ctx context.Context, targetIDs []string, parentFlagsStr string) []TargetSubprocessResult {
	execPath, err := os.Executable()
	if err != nil {
		return []TargetSubprocessResult{{
			Err: fmt.Errorf("failed to get executable path: %w", err),
		}}
	}

	results := make([]TargetSubprocessResult, len(targetIDs))
	var wg sync.WaitGroup

	for i, targetID := range targetIDs {
		wg.Add(1)
		go func(idx int, tid string) {
			defer wg.Done()

			args := []string{"run", "--target=" + tid, "--output=console"}
			if parentFlagsStr != "" {
				args = append(args, strings.Fields(parentFlagsStr)...)
			}

			cmd := exec.CommandContext(ctx, execPath, args...)
			out, cmdErr := cmd.CombinedOutput()

			results[idx] = TargetSubprocessResult{
				TargetID: tid,
				Output:   string(out),
				Err:      cmdErr,
			}
		}(i, targetID)
	}

	wg.Wait()
	return results
}
