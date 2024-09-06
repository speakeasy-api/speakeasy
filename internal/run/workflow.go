package run

import (
	"context"
	"fmt"
	"time"

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

	// Internal
	workflowName       string
	SDKOverviewURLs    map[string]string
	RootStep           *workflowTracking.WorkflowStep
	workflow           workflow.Workflow
	ProjectDir         string
	validatedDocuments []string
	generationAccess   *sdkgen.GenerationAccess
	OperationsRemoved  []string
	lockfile           *workflow.LockFile
	lockfileOld        *workflow.LockFile // the lockfile as it was before the current run

	computedChanges map[string]bool
	SourceResults   map[string]*SourceResult
	TargetResults   map[string]*TargetResult
	OnSourceResult  func(*SourceResult)
	Duration        time.Duration
	criticalWarns   []string
	Error           error
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
		ProjectDir:       projectDir,
		ForceGeneration:  false,
		SourceResults:    make(map[string]*SourceResult),
		TargetResults:    make(map[string]*TargetResult),
		OnSourceResult:   func(*SourceResult) {},
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

func WithForceGeneration(forceGeneration bool) Opt {
	return func(w *Workflow) {
		w.ForceGeneration = forceGeneration
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
				WithForceGeneration(w.ForceGeneration),
				WithSkipLinting(),
				WithSkipChangeReport(w.SkipChangeReport),
				WithSkipSnapshot(w.SkipSnapshot),
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
