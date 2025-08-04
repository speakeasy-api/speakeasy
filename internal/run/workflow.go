package run

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/speakeasy-api/speakeasy/registry"
	"gopkg.in/yaml.v3"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
)

type Workflow struct {
	// Opts
	Target                 string
	Source                 string
	Repo                   string
	SetVersion             string
	Debug                  bool
	ShouldCompile          bool
	Verbose                bool
	ForceGeneration        bool
	FrozenWorkflowLock     bool
	SkipVersioning         bool
	SkipLinting            bool
	SkipChangeReport       bool
	SkipSnapshot           bool
	SkipCleanup            bool
	FromQuickstart         bool
	SkipGenerateLintReport bool
	RepoSubDirs            map[string]string
	InstallationURLs       map[string]string
	RegistryTags           []string

	// Enable if target testing should be explicitly disabled, regardless of the
	// workflow configuration enabling testing.
	SkipTesting   bool
	BoostrapTests bool

	// Internal
	workflowName       string
	SDKOverviewURLs    map[string]string
	RootStep           *workflowTracking.WorkflowStep
	workflow           workflow.Workflow
	workflowRaw        string // the raw workflow YAML content
	ProjectDir         string
	validatedDocuments []string
	generationAccess   *sdkgen.GenerationAccess
	OperationsRemoved  []string
	lockfile           *workflow.LockFile
	lockfileOld        *workflow.LockFile // the lockfile as it was before the current run

	computedChanges map[string]bool
	SourceResults   map[string]*SourceResult
	TargetResults   map[string]*TargetResult
	OnSourceResult  SourceResultCallback
	Duration        time.Duration
	criticalWarns   []string
	Error           error

	// Studio
	CancellableGeneration *sdkgen.CancellableGeneration
	StreamableGeneration  *sdkgen.StreamableGeneration
}

type Opt func(w *Workflow)

func NewWorkflow(
	ctx context.Context,
	opts ...Opt,
) (*Workflow, error) {
	wf, projectDir, err := utils.GetWorkflowAndDir()
	if err != nil || wf == nil {
		return nil, fmt.Errorf("failed to load workflow.yaml: %w", err)
	}

	// Marshal the workflow to get the YAML content
	workflowRawBytes, err := yaml.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow: %w", err)
	}
	workflowRaw := string(workflowRawBytes)

	// Load the current lockfile so that we don't overwrite all targets
	lockfile, err := workflow.LoadLockfile(projectDir)
	lockfileOld := lockfile

	if err != nil || lockfile == nil {
		lockfile = &workflow.LockFile{
			Sources: make(map[string]workflow.SourceLock),
			Targets: make(map[string]workflow.TargetLock),
		}
	}
	lockfile.SpeakeasyVersion = events.GetSpeakeasyVersionFromContext(ctx)
	lockfile.Workflow = *wf

	// Default values
	w := &Workflow{
		workflowName:     "Workflow",
		RepoSubDirs:      make(map[string]string),
		InstallationURLs: make(map[string]string),
		SDKOverviewURLs:  make(map[string]string),
		Debug:            false,
		ShouldCompile:    true,
		workflow:         *wf,
		workflowRaw:      workflowRaw,
		ProjectDir:       projectDir,
		ForceGeneration:  false,
		SourceResults:    make(map[string]*SourceResult),
		TargetResults:    make(map[string]*TargetResult),
		OnSourceResult:   func(*SourceResult, SourceStepID) error { return nil },
		computedChanges:  make(map[string]bool),
		lockfile:         lockfile,
		lockfileOld:      lockfileOld,
	}

	for _, opt := range opts {
		opt(w)
	}

	w.RootStep = workflowTracking.NewWorkflowStep(w.workflowName, log.From(ctx), nil)

	return w, nil
}

func WithWorkflowName(name string) Opt {
	return func(w *Workflow) {
		w.workflowName = name
	}
}

func WithSource(source string) Opt {
	return func(w *Workflow) {
		w.Source = source
	}
}

func WithFrozenWorkflowLock(frozen bool) Opt {
	return func(w *Workflow) {
		w.FrozenWorkflowLock = frozen
		if frozen {
			// Implies force generation -- workflow.lock being unchanged should mean we skip generation
			w.ForceGeneration = true
			// Implies No auto versioning -- workflow.lock being unchanged should mean we skip versioning
			w.SkipVersioning = true
			// Implies no snapshot
			w.SkipSnapshot = true
			// Implies no change report
			w.SkipChangeReport = true
		}
	}
}

func WithSkipVersioning(skipVersioning bool) Opt {
	return func(w *Workflow) {
		w.SkipVersioning = skipVersioning
	}
}

func WithTarget(target string) Opt {
	return func(w *Workflow) {
		w.Target = target
	}
}

func WithBoostrapTests() Opt {
	return func(w *Workflow) {
		w.BoostrapTests = true
		w.ShouldCompile = true
		w.SkipTesting = true
		w.SkipVersioning = true
	}
}

func WithSetVersion(version string) Opt {
	return func(w *Workflow) {
		w.SetVersion = version
	}
}

func WithDebug(debug bool) Opt {
	return func(w *Workflow) {
		w.Debug = debug
	}
}

func WithSkipChangeReport(skip bool) Opt {
	return func(w *Workflow) {
		w.SkipChangeReport = skip
	}
}

func WithSkipSnapshot(skip bool) Opt {
	return func(w *Workflow) {
		w.SkipSnapshot = skip
	}
}

func WithRepo(repo string) Opt {
	return func(w *Workflow) {
		w.Repo = repo
	}
}

func WithShouldCompile(shouldCompile bool) Opt {
	return func(w *Workflow) {
		w.ShouldCompile = shouldCompile
	}
}

func WithVerbose(verbose bool) Opt {
	return func(w *Workflow) {
		w.Verbose = verbose
	}
}

func WithSkipLinting() Opt {
	return func(w *Workflow) {
		w.SkipLinting = true
	}
}

func WithLinting() Opt {
	return func(w *Workflow) {
		w.SkipLinting = false
	}
}

func WithSkipGenerateLintReport() Opt {
	return func(w *Workflow) {
		w.SkipGenerateLintReport = true
	}
}

func WithSkipCleanup() Opt {
	return func(w *Workflow) {
		w.SkipCleanup = true
	}
}

// Prevents target testing from running even if the workflow configuration
// enables it.
func WithSkipTesting(skipTesting bool) Opt {
	return func(w *Workflow) {
		w.SkipTesting = skipTesting
	}
}

func WithFromQuickstart(fromQuickstart bool) Opt {
	return func(w *Workflow) {
		w.FromQuickstart = fromQuickstart
	}
}

func WithRepoSubDirs(repoSubDirs map[string]string) Opt {
	return func(w *Workflow) {
		w.RepoSubDirs = repoSubDirs
	}
}

func WithInstallationURLs(installationURLs map[string]string) Opt {
	return func(w *Workflow) {
		w.InstallationURLs = installationURLs
	}
}

func WithRegistryTags(registryTags []string) Opt {
	return func(w *Workflow) {
		w.RegistryTags = registryTags
	}
}

func WithSourceUpdates(onSourceResult SourceResultCallback) Opt {
	if onSourceResult != nil {
		return func(w *Workflow) {
			w.OnSourceResult = onSourceResult
		}
	}

	return func(w *Workflow) {
		w.OnSourceResult = func(sourceRes *SourceResult, sourceStep SourceStepID) error { return nil }
	}
}

func WithCancellableGeneration(cancellable bool) Opt {
	return func(w *Workflow) {
		if cancellable {
			w.CancellableGeneration = &sdkgen.CancellableGeneration{
				CancellationMutex: sync.Mutex{},
				// CancelCtx and CancelFunc fields depend on the runTarget context
				// and will be set right before generation starts.
			}
		} else {
			w.CancellableGeneration = nil
		}
	}
}

func WithStreamableGeneration(onProgressUpdate func(generate.ProgressUpdate), genSteps, fileStatus bool) Opt {
	return func(w *Workflow) {
		w.StreamableGeneration = &sdkgen.StreamableGeneration{
			OnProgressUpdate: onProgressUpdate,
			GenSteps:         genSteps,
			FileStatus:       fileStatus,
		}
	}
}

func (w *Workflow) CountDiagnostics() int {
	count := 0
	for _, sourceResult := range w.SourceResults {
		for _, d := range sourceResult.Diagnosis {
			count += len(d)
		}
	}
	return count
}

func (w *Workflow) Clone(ctx context.Context, opts ...Opt) (*Workflow, error) {
	return NewWorkflow(
		ctx,
		append(
			[]Opt{
				WithWorkflowName(w.workflowName),
				WithSource(w.Source),
				WithTarget(w.Target),
				WithSetVersion(w.SetVersion),
				WithDebug(w.Debug),
				WithShouldCompile(w.ShouldCompile),
				WithSkipLinting(),
				WithSkipChangeReport(w.SkipChangeReport),
				WithSkipSnapshot(w.SkipSnapshot),
				WithSkipTesting(w.SkipTesting),
				WithFromQuickstart(w.FromQuickstart),
				WithRepo(w.Repo),
				WithRepoSubDirs(w.RepoSubDirs),
				WithInstallationURLs(w.InstallationURLs),
				WithRegistryTags(w.RegistryTags),
			},
			opts...,
		)...,
	)
}

func Migrate(ctx context.Context, wf *workflow.Workflow) {
	if registry.IsRegistryEnabled(ctx) {
		*wf = wf.Migrate()
	} else {
		*wf = wf.MigrateNoTelemetry()
	}
}
