package run

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With --frozen-workflow-lockfile the source pipeline never writes
// Source.GetOutputLocation() (.speakeasy/temp/output_<hash>.yaml), so
// SourceResult.OutputPath points at a file that does not exist. The cached
// result served to the second and later targets sharing the source must hand
// back the document the pipeline actually produced, otherwise the generator
// stats the missing temp file, falls through to the remote-download branch and
// fails with `unsupported protocol scheme ""`.
func TestCachedSource_FrozenLockfileReturnsResolvedDocument(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	resolved := filepath.Join(tempDir, "merge_abc123.yaml")
	require.NoError(t, os.WriteFile(resolved, []byte("openapi: 3.0.0\ninfo:\n  title: t\n  version: 1.0.0\npaths: {}\n"), 0o644))
	neverWritten := filepath.Join(tempDir, "output_32fac1.yaml")

	w := &Workflow{
		FrozenWorkflowLock: true,
		SourceResults: map[string]*SourceResult{
			"my-source": {
				Source:       "my-source",
				OutputPath:   neverWritten,
				documentPath: resolved,
			},
		},
	}

	path, res, ok := w.cachedSource("my-source")
	require.True(t, ok)
	require.NotNil(t, res)
	assert.Equal(t, resolved, path)

	// The path handed to the generator must be an existing local file so that
	// it is never classified as a remote document.
	_, err := os.Stat(path)
	require.NoError(t, err)
	isRemote, contents, err := openapi.GetSchemaContents(t.Context(), path, "", "")
	require.NoError(t, err)
	assert.False(t, isRemote)
	assert.Contains(t, string(contents), "openapi: 3.0.0")
}

// A SourceResult that was recorded for a run that did not complete (for
// example a linting failure) carries an OutputPath but no resolved document.
// It must not be served from the cache: the next caller (another target, or
// the minimum-viable-spec retry) has to run the source again.
func TestCachedSource_IncompleteResultIsNotCached(t *testing.T) {
	t.Parallel()

	w := &Workflow{
		SourceResults: map[string]*SourceResult{
			"my-source": {
				Source:     "my-source",
				OutputPath: filepath.Join(t.TempDir(), "output_32fac1.yaml"),
			},
		},
	}

	path, res, ok := w.cachedSource("my-source")
	assert.False(t, ok)
	assert.Nil(t, res)
	assert.Empty(t, path)

	_, _, ok = w.cachedSource("unknown-source")
	assert.False(t, ok)
}

// testWorkflow is a Workflow wired up just enough to run RunSource end to end
// against local fixtures, without linting, snapshotting or change reports.
// runs counts how many times a source pipeline was actually started.
type testWorkflow struct {
	*Workflow
	runs atomic.Int32
}

// newTestWorkflow builds a frozen-lockfile Workflow over the given sources. It
// changes the working directory to a fresh temp dir so that the source
// pipeline's temp files (workflow.GetTempDir()) never land in the source tree,
// which is why the tests using it cannot run in parallel.
func newTestWorkflow(t *testing.T, sources map[string]workflow.Source) *testWorkflow {
	t.Helper()
	t.Chdir(t.TempDir())

	tw := &testWorkflow{}
	tw.Workflow = &Workflow{
		FrozenWorkflowLock: true,
		SkipLinting:        true,
		SkipSnapshot:       true,
		SkipChangeReport:   true,
		workflow: workflow.Workflow{
			Version: workflow.WorkflowVersion,
			Sources: sources,
		},
		RootStep:       workflowTracking.NewWorkflowStep("test", log.From(t.Context()), nil),
		SourceResults:  make(map[string]*SourceResult),
		sourceInflight: make(map[string]*sourceInflight),
		OnSourceResult: func(_ *SourceResult, step SourceStepID) error {
			if step == SourceStepFetch {
				tw.runs.Add(1)
			}
			return nil
		},
	}
	return tw
}

func (tw *testWorkflow) inflightCount() int {
	tw.sourceInflightMu.Lock()
	defer tw.sourceInflightMu.Unlock()
	return len(tw.sourceInflight)
}

// testdataDir is resolved once, before any test changes the working
// directory, so fixture paths stay valid inside newTestWorkflow's temp dir.
var testdataDir = func() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(wd, "testdata")
}()

// fixture returns the absolute path of a file under testdata.
func fixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(testdataDir, name)
	require.FileExists(t, path)
	return path
}

// overlaidSource is the workflow shape of the bug report: one input plus an
// overlay, so the source is not IsSingleInput() and its output location is a
// .speakeasy/temp/output_<hash>.yaml file that a frozen run never writes.
func overlaidSource(t *testing.T) workflow.Source {
	t.Helper()
	return workflow.Source{
		Inputs:   []workflow.Document{{Location: workflow.LocationString(fixture(t, "openapi.yaml"))}},
		Overlays: []workflow.Overlay{{Document: &workflow.Document{Location: workflow.LocationString(fixture(t, "overlay.yaml"))}}},
	}
}

func TestRunSource_FrozenLockfileSharedSourceServesResolvedDocument(t *testing.T) { //nolint:paralleltest // changes the working directory
	tw := newTestWorkflow(t, map[string]workflow.Source{"my-source": overlaidSource(t)})

	path, res, err := tw.RunSource(t.Context(), tw.RootStep, "my-source", "first-target", "go")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, int32(1), tw.runs.Load())

	// The frozen shape: the output location was never written, the pipeline's
	// resolved document (the applied overlay) is what exists on disk.
	_, err = os.Stat(res.OutputPath)
	require.ErrorIs(t, err, os.ErrNotExist, "frozen runs must not write the output location")
	require.FileExists(t, path)
	assert.NotEqual(t, res.OutputPath, path)
	assert.Equal(t, path, res.documentPath)

	// The second target sharing the source must get the existing resolved
	// document, and must not run the source again.
	path2, res2, err := tw.RunSource(t.Context(), tw.RootStep, "my-source", "second-target", "typescript")
	require.NoError(t, err)
	assert.Equal(t, path, path2)
	assert.Same(t, res, res2)
	assert.Equal(t, int32(1), tw.runs.Load())
	require.FileExists(t, path2)
	assert.NotEqual(t, res2.OutputPath, path2)

	isRemote, contents, err := openapi.GetSchemaContents(t.Context(), path2, "", "")
	require.NoError(t, err)
	assert.False(t, isRemote)
	assert.Contains(t, string(contents), "x-speakeasy-name-override: TestAPI", "overlay must have been applied")

	assert.Equal(t, []string{"my-source"}, tw.sourceOrder)
	assert.Equal(t, 0, tw.inflightCount())
}

// Concurrent callers for the same source (targets resolved in parallel, or a
// diamond of source refs) must share a single run.
func TestRunSource_ConcurrentCallersShareOneRun(t *testing.T) { //nolint:paralleltest // changes the working directory
	tw := newTestWorkflow(t, map[string]workflow.Source{"my-source": overlaidSource(t)})

	const callers = 8
	paths := make([]string, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			paths[i], _, errs[i] = tw.RunSource(t.Context(), tw.RootStep, "my-source", "target", "go")
		}()
	}
	wg.Wait()

	for i := range callers {
		require.NoError(t, errs[i])
		require.FileExists(t, paths[i])
		assert.Equal(t, paths[0], paths[i])
	}
	assert.Equal(t, int32(1), tw.runs.Load())
	assert.Equal(t, []string{"my-source"}, tw.sourceOrder)
	assert.Equal(t, 0, tw.inflightCount())
}

// A caller that missed the cache just before the first run published its
// result, and only reaches the in-flight registration after that run removed
// its in-flight entry, must be served the completed run rather than starting
// a second one. runSourceOnce is RunSource minus the cache fast path, which is
// exactly the state such a caller is in.
func TestRunSource_LateCallerAfterCompletionDoesNotRerun(t *testing.T) { //nolint:paralleltest // changes the working directory
	tw := newTestWorkflow(t, map[string]workflow.Source{"my-source": overlaidSource(t)})

	path, res, err := tw.RunSource(t.Context(), tw.RootStep, "my-source", "first-target", "go")
	require.NoError(t, err)
	require.Equal(t, 0, tw.inflightCount(), "the in-flight entry is dropped once the run is over")

	latePath, lateRes, err := tw.runSourceOnce(t.Context(), tw.RootStep, "my-source", "second-target", "go")
	require.NoError(t, err)
	assert.Equal(t, path, latePath)
	assert.Same(t, res, lateRes)
	assert.Equal(t, int32(1), tw.runs.Load(), "source must not run a second time")
	assert.Equal(t, 0, tw.inflightCount())
}

// A source that fails part way through its pipeline records a SourceResult
// without a resolved document. It must not be served from the cache: the next
// caller runs it again, and sourceOrder records it once.
func TestRunSource_FailedSourceIsRerun(t *testing.T) { //nolint:paralleltest // changes the working directory
	malformedOverlay := filepath.Join(t.TempDir(), "malformed_overlay.yaml")
	require.NoError(t, os.WriteFile(malformedOverlay, []byte("overlay: 1.0.0\nactions: [\n"), 0o644))
	broken := overlaidSource(t)
	broken.Overlays = []workflow.Overlay{{Document: &workflow.Document{Location: workflow.LocationString(malformedOverlay)}}}
	tw := newTestWorkflow(t, map[string]workflow.Source{"broken": broken})

	_, _, err := tw.RunSource(t.Context(), tw.RootStep, "broken", "target", "go")
	require.Error(t, err)
	assert.Equal(t, int32(1), tw.runs.Load())
	assert.Equal(t, 0, tw.inflightCount(), "a failed run must not stay in flight")

	_, _, err = tw.RunSource(t.Context(), tw.RootStep, "broken", "target", "go")
	require.Error(t, err)
	assert.Equal(t, int32(2), tw.runs.Load(), "a failed source must be run again, not served from the cache")
	assert.Equal(t, []string{"broken"}, tw.sourceOrder, "a re-run source is recorded once")
	assert.Equal(t, 0, tw.inflightCount())
}

// A source that fails before its pipeline starts (an input referencing an
// unknown source) leaves nothing behind. Fixing the workflow and calling
// RunSource again, as the minimum-viable-spec retry does, runs the source
// instead of returning the first call's error.
func TestRunSource_EarlyFailureIsRerunAfterWorkflowFix(t *testing.T) { //nolint:paralleltest // changes the working directory
	consumer := workflow.Source{
		Inputs:   []workflow.Document{{Location: "source:missing"}},
		Overlays: []workflow.Overlay{{Document: &workflow.Document{Location: workflow.LocationString(fixture(t, "overlay.yaml"))}}},
	}
	tw := newTestWorkflow(t, map[string]workflow.Source{"consumer": consumer})

	_, _, err := tw.RunSource(t.Context(), tw.RootStep, "consumer", "target", "go")
	require.ErrorContains(t, err, `references unknown source "missing"`)
	assert.Equal(t, int32(0), tw.runs.Load())
	assert.Empty(t, tw.sourceOrder)
	assert.Equal(t, 0, tw.inflightCount(), "a failed run must not stay in flight")

	tw.workflow.Sources["missing"] = workflow.Source{
		Inputs: []workflow.Document{{Location: workflow.LocationString(fixture(t, "openapi.yaml"))}},
	}

	path, res, err := tw.RunSource(t.Context(), tw.RootStep, "consumer", "target", "go")
	require.NoError(t, err, "the second call must re-run the source, not return the cached error")
	require.FileExists(t, path)
	assert.Equal(t, path, res.documentPath)
	assert.Equal(t, int32(2), tw.runs.Load(), "both the referenced source and the consumer ran")
	assert.Equal(t, []string{"missing", "consumer"}, tw.sourceOrder)
	assert.Equal(t, 0, tw.inflightCount())
}
