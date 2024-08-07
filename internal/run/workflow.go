package run

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
)

type Workflow struct {
	// Opts
	Target           string
	Source           string
	Repo             string
	SetVersion       string
	Debug            bool
	ShouldCompile    bool
	ForceGeneration  bool
	SkipLinting      bool
	SkipChangeReport bool
	SkipSnapshot     bool
	FromQuickstart   bool
	RepoSubDirs      map[string]string
	InstallationURLs map[string]string
	RegistryTags     []string

	// Internal
	workflowName       string
	SDKOverviewURLs    map[string]string
	RootStep           *workflowTracking.WorkflowStep
	workflow           workflow.Workflow
	ProjectDir         string
	validatedDocuments []string
	generationAccess   *sdkgen.GenerationAccess
	OperationsRemoved  []string
	computedChanges    map[string]bool
	sourceResults      map[string]*SourceResult
	lockfile           *workflow.LockFile
	lockfileOld        *workflow.LockFile // the lockfile as it was before the current run
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
		sourceResults:    make(map[string]*SourceResult),
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

func WithSkipLinting() Opt {
	return func(w *Workflow) {
		w.SkipLinting = true
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
