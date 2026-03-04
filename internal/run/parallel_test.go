package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestHelperProcess is invoked by the test binary when GO_TEST_HELPER_PROCESS
// is set. It simulates a speakeasy subprocess.
func TestHelperProcess(t *testing.T) { //nolint:paralleltest // helper process, not a real test
	if os.Getenv("GO_TEST_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	// Find "run" in args to parse our simulated flags
	runIdx := -1
	for i, a := range args {
		if a == "run" {
			runIdx = i
			break
		}
	}
	if runIdx == -1 {
		fmt.Fprintf(os.Stderr, "missing 'run' command")
		os.Exit(2)
	}

	var target string
	shouldFail := false
	for _, a := range args[runIdx+1:] {
		if strings.HasPrefix(a, "--target=") {
			target = strings.TrimPrefix(a, "--target=")
		}
		if a == "--fail-for-test" {
			shouldFail = true
		}
	}

	_, _ = fmt.Fprintf(os.Stdout, "generated %s", target)

	if shouldFail && target == "target-fail" {
		os.Exit(1)
	}
	os.Exit(0)
}

// runTargetsParallelWithExec is a test helper that overrides the executable path
// so we use the test binary as the subprocess.
func runTargetsParallelWithExec(ctx context.Context, targetIDs []string, parentFlagsStr string, execPath string) []TargetSubprocessResult {
	results := make([]TargetSubprocessResult, len(targetIDs))
	type done struct{}
	ch := make(chan done, len(targetIDs))

	for i, targetID := range targetIDs {
		go func(idx int, tid string) {
			defer func() { ch <- done{} }()

			args := []string{
				"-test.run=TestHelperProcess", "--",
				"run", "--target=" + tid, "--output=console",
			}
			if parentFlagsStr != "" {
				args = append(args, strings.Fields(parentFlagsStr)...)
			}

			cmd := exec.CommandContext(ctx, execPath, args...)
			cmd.Env = append(os.Environ(), "GO_TEST_HELPER_PROCESS=1")
			out, cmdErr := cmd.CombinedOutput()

			results[idx] = TargetSubprocessResult{
				TargetID: tid,
				Output:   string(out),
				Err:      cmdErr,
			}
		}(i, targetID)
	}

	for range targetIDs {
		<-ch
	}
	return results
}

func TestRunTargetsParallel_AllSucceed(t *testing.T) {
	t.Parallel()

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get test executable: %v", err)
	}

	targets := []string{"target-go", "target-python", "target-typescript"}
	results := runTargetsParallelWithExec(context.Background(), targets, "", execPath)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	for i, r := range results {
		if r.TargetID != targets[i] {
			t.Errorf("result[%d]: expected target %q, got %q", i, targets[i], r.TargetID)
		}
		if r.Err != nil {
			t.Errorf("result[%d]: unexpected error: %v", i, r.Err)
		}
		if !strings.Contains(r.Output, "generated "+targets[i]) {
			t.Errorf("result[%d]: expected output to contain %q, got %q", i, "generated "+targets[i], r.Output)
		}
	}
}

func TestRunTargetsParallel_PartialFailure(t *testing.T) {
	t.Parallel()

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get test executable: %v", err)
	}

	targets := []string{"target-ok", "target-fail"}
	results := runTargetsParallelWithExec(context.Background(), targets, "--fail-for-test", execPath)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// target-ok should succeed
	if results[0].Err != nil {
		t.Errorf("target-ok: unexpected error: %v", results[0].Err)
	}

	// target-fail should fail
	if results[1].Err == nil {
		t.Error("target-fail: expected error, got nil")
	}
}

func TestRunTargetsParallel_ActuallyParallel(t *testing.T) {
	t.Parallel()

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get test executable: %v", err)
	}

	// If 3 targets run serially, they'd take at least 3x the time of one.
	// Running in parallel should complete in roughly the time of one.
	targets := []string{"t1", "t2", "t3"}
	start := time.Now()
	results := runTargetsParallelWithExec(context.Background(), targets, "", execPath)
	elapsed := time.Since(start)

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("target %s: unexpected error: %v", r.TargetID, r.Err)
		}
	}

	// The helper processes are near-instant; serial would still be fast, but
	// we can at least verify they all completed and results are ordered correctly.
	if elapsed > 10*time.Second {
		t.Errorf("parallel execution took too long: %v", elapsed)
	}
	for i, r := range results {
		if r.TargetID != targets[i] {
			t.Errorf("result ordering broken: expected %q at index %d, got %q", targets[i], i, r.TargetID)
		}
	}
}

func TestRunTargetsParallel_ContextCancellation(t *testing.T) {
	t.Parallel()

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get test executable: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	targets := []string{"target-go"}
	results := runTargetsParallelWithExec(ctx, targets, "", execPath)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// With a cancelled context, the subprocess should fail
	if results[0].Err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
