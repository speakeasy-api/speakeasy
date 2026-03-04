package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// TargetSubprocessResult holds the outcome of a single parallel target run.
type TargetSubprocessResult struct {
	TargetID string
	Output   string // combined stdout+stderr
	Duration time.Duration
	Err      error
}

// OnTargetProgress is called when a target starts or finishes during parallel execution.
type OnTargetProgress func(targetID string, event TargetEvent)

// TargetEvent describes what happened to a parallel target.
type TargetEvent int

const (
	TargetStarted TargetEvent = iota
	TargetSucceeded
	TargetFailed
)

// RunTargetsParallel spawns a subprocess for each target, runs them concurrently,
// and returns all results. Each subprocess is a complete `speakeasy run -t <target>`
// invocation using the same binary. The optional onProgress callback is invoked as
// targets start and complete, giving the caller real-time feedback.
func RunTargetsParallel(ctx context.Context, targetIDs []string, parentFlagsStr string, onProgress OnTargetProgress) []TargetSubprocessResult {
	execPath, err := os.Executable()
	if err != nil {
		return []TargetSubprocessResult{{
			Err: fmt.Errorf("failed to get executable path: %w", err),
		}}
	}

	if onProgress == nil {
		onProgress = func(string, TargetEvent) {}
	}

	results := make([]TargetSubprocessResult, len(targetIDs))
	var wg sync.WaitGroup

	for i, targetID := range targetIDs {
		wg.Add(1)
		go func(idx int, tid string) {
			defer wg.Done()

			onProgress(tid, TargetStarted)
			start := time.Now()

			args := []string{"run", "--target=" + tid, "--output=console"}
			if parentFlagsStr != "" {
				args = append(args, strings.Fields(parentFlagsStr)...)
			}

			cmd := exec.CommandContext(ctx, execPath, args...)
			out, cmdErr := cmd.CombinedOutput()

			elapsed := time.Since(start)
			results[idx] = TargetSubprocessResult{
				TargetID: tid,
				Output:   string(out),
				Duration: elapsed,
				Err:      cmdErr,
			}

			if cmdErr != nil {
				onProgress(tid, TargetFailed)
			} else {
				onProgress(tid, TargetSucceeded)
			}
		}(i, targetID)
	}

	wg.Wait()
	return results
}
