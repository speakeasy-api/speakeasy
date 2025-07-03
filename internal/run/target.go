package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-version"
	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/bundler"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy-core/fsextras"
	"github.com/speakeasy-api/speakeasy-core/ocicommon"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/git"
	"github.com/speakeasy-api/speakeasy/internal/links"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/pkg/codesamples"
	"github.com/speakeasy-api/speakeasy/registry"
	"go.uber.org/zap"
)

type TargetResult struct {
	OutputPath  string
	GenYamlPath string
}

func getTarget(target string) (*workflow.Target, error) {
	wf, _, err := utils.GetWorkflowAndDir()
	if err != nil {
		return nil, err
	}
	t := wf.Targets[target]
	return &t, nil
}

func (w *Workflow) runTarget(ctx context.Context, target string) (*SourceResult, *TargetResult, error) {
	rootStep := w.RootStep.NewSubstep(fmt.Sprintf("Target: %s", target))

	t := w.workflow.Targets[target]
	targetLanguage := t.Target
	targetLock := workflow.TargetLock{Source: t.Source}

	log.From(ctx).Infof("Running target %s (%s)...\n", target, t.Target)

	source, sourcePath, err := w.workflow.GetTargetSource(target)
	if err != nil {
		return nil, nil, err
	}

	var sourceRes *SourceResult

	if source != nil {
		sourcePath, sourceRes, err = w.RunSource(ctx, rootStep, t.Source, target, targetLanguage)
		if err != nil {
			if w.FromQuickstart && sourceRes != nil && sourceRes.LintResult != nil && len(sourceRes.LintResult.ValidOperations) > 0 {
				cliEvent := events.GetTelemetryEventFromContext(ctx)
				if cliEvent != nil {
					cliEvent.GenerateNumberOfOperationsIgnored = new(int64)
					*cliEvent.GenerateNumberOfOperationsIgnored = int64(len(sourceRes.LintResult.InvalidOperations))
				}

				retriedPath, retriedRes, retriedErr := w.retryWithMinimumViableSpec(ctx, rootStep, t.Source, target, sourceRes.LintResult.AllErrors)
				if retriedErr != nil {
					log.From(ctx).Errorf("Failed to retry with minimum viable spec: %s", retriedErr)
					// return the original error
					return sourceRes, nil, err
				}

				w.OperationsRemoved = sourceRes.LintResult.InvalidOperations
				sourcePath = retriedPath
				sourceRes = retriedRes
			} else {
				return sourceRes, nil, err
			}
		}
	} else {
		res, err := w.validateDocument(ctx, rootStep, t.Source, sourcePath, "speakeasy-generation", w.ProjectDir, targetLanguage)
		if err != nil {
			return sourceRes, nil, err
		}

		sourceRes = &SourceResult{
			Source:     t.Source,
			LintResult: res,
		}
	}

	var outDir string
	if t.Output != nil {
		outDir = *t.Output
	} else {
		outDir = w.ProjectDir
	}

	published := t.IsPublished()

	genYamlStep := rootStep.NewSubstep("Validating gen.yaml")

	genConfig, err := sdkGenConfig.Load(outDir)
	if err != nil {
		return sourceRes, nil, err
	}

	targetResult := TargetResult{
		OutputPath:  outDir,
		GenYamlPath: genConfig.ConfigPath,
	}
	defer func() {
		w.TargetResults[target] = &targetResult
	}()

	if w.SetVersion != "" && genConfig.Config != nil {
		appliedVersion := w.SetVersion
		if appliedVersion[0] == 'v' {
			appliedVersion = appliedVersion[1:]
		}
		if langCfg, ok := genConfig.Config.Languages[t.Target]; ok {
			if _, err := version.NewVersion(appliedVersion); err != nil {
				return sourceRes, nil, fmt.Errorf("failed to parse version %s: %w", w.SetVersion, err)
			}

			langCfg.Version = appliedVersion
			genConfig.Config.Languages[t.Target] = langCfg
			if err := sdkGenConfig.SaveConfig(outDir, genConfig.Config); err != nil {
				return sourceRes, nil, err
			}
		}
	}

	err = validation.ValidateConfigAndPrintErrors(ctx, t.Target, genConfig, published, target)
	if err != nil {
		if errors.Is(err, validation.NoConfigFound) {
			genYamlStep.Skip("gen.yaml not found, assuming new SDK")
		} else {
			return sourceRes, nil, err
		}
	}

	logListener := make(chan log.Msg)
	logger := log.From(ctx).WithListener(logListener)
	ctx = log.With(ctx, logger)

	if w.StreamableGeneration != nil && w.Debug {
		w.StreamableGeneration.LogListener = logListener
	}

	if w.CancellableGeneration != nil {
		cancelCtx, cancelFunc := context.WithCancel(ctx)
		w.CancellableGeneration.CancellationMutex.Lock()
		w.CancellableGeneration.CancellableContext = cancelCtx
		w.CancellableGeneration.CancelGeneration = cancelFunc
		w.CancellableGeneration.CancellationMutex.Unlock()

		defer func() {
			w.CancellableGeneration.CancellationMutex.Lock()
			w.CancellableGeneration.CancelGeneration = nil
			w.CancellableGeneration.CancellableContext = nil
			w.CancellableGeneration.CancellationMutex.Unlock()
			cancelFunc() // Ensure context is cleaned up
		}()
	}

	genStep := rootStep.NewSubstep(fmt.Sprintf("Generating %s SDK", utils.CapitalizeFirst(t.Target)))
	go genStep.ListenForSubsteps(logListener)

	oldSchema, err := w.fetchOldSchema(ctx, target)
	if err != nil {
		logger.Errorf("An error occurred when downloading old schema: %v", err)
		oldSchema = nil
	}

	generationAccess, err := sdkgen.Generate(
		ctx,
		sdkgen.GenerateOptions{
			CustomerID:            config.GetCustomerID(),
			WorkspaceID:           config.GetWorkspaceID(),
			Language:              t.Target,
			SchemaPath:            sourcePath,
			Header:                "",
			Token:                 "",
			OutDir:                outDir,
			CLIVersion:            events.GetSpeakeasyVersionFromContext(ctx),
			InstallationURL:       w.InstallationURLs[target],
			Debug:                 w.Debug,
			AutoYes:               true,
			Published:             published,
			OutputTests:           false,
			Repo:                  w.Repo,
			RepoSubDir:            w.RepoSubDirs[target],
			Verbose:               w.Verbose,
			Compile:               w.ShouldCompile,
			TargetName:            target,
			SkipVersioning:        w.SkipVersioning,
			CancellableGeneration: w.CancellableGeneration,
			StreamableGeneration:  w.StreamableGeneration,
		},
		oldSchema,
	)

	if err != nil {
		return sourceRes, nil, err
	}
	w.generationAccess = generationAccess

	if t.CodeSamples != nil {
		codeSamplesStep := rootStep.NewSubstep("Generating Code Samples")
		namespaceName, digest, err := w.runCodeSamples(ctx, codeSamplesStep, *t.CodeSamples, t.Target, sourcePath, t.Output)

		if err != nil {
			// Block by default. Only warn if explicitly set to non-blocking
			if t.CodeSamples.Blocking == nil || *t.CodeSamples.Blocking {
				return sourceRes, nil, err
			} else {
				log.From(ctx).Warnf("failed to generate code samples: %s", err.Error())
				codeSamplesStep.Skip("failed, but step set to non-blocking")
			}
		}

		targetLock.CodeSamplesNamespace = namespaceName
		targetLock.CodeSamplesRevisionDigest = digest
	}

	if targetEnablesTesting(ctx, t) {
		testingStep := rootStep.NewSubstep(fmt.Sprintf("Running %s Testing", utils.CapitalizeFirst(t.Target)))

		if w.SkipTesting {
			testingStep.Skip("explicitly disabled")
		} else if err := w.runTesting(ctx, target, t, testingStep, outDir); err != nil {
			return sourceRes, nil, err
		}
	}

	rootStep.SucceedWorkflow()

	if sourceLock, ok := w.lockfile.Sources[t.Source]; ok {
		targetLock.SourceNamespace = sourceLock.SourceNamespace
		targetLock.SourceRevisionDigest = sourceLock.SourceRevisionDigest
		targetLock.SourceBlobDigest = sourceLock.SourceBlobDigest
	}

	orgSlug := auth.GetOrgSlugFromContext(ctx)
	workspaceSlug := auth.GetWorkspaceSlugFromContext(ctx)
	genLockID := sdkgen.GetGenLockID(outDir)
	if orgSlug != "" && workspaceSlug != "" && genLockID != nil && *genLockID != "" && !utils.IsZeroTelemetryOrganization(ctx) {
		w.SDKOverviewURLs[target] = fmt.Sprintf("https://app.speakeasy.com/org/%s/%s/targets/%s", orgSlug, workspaceSlug, *genLockID)
	}

	w.lockfile.Targets[target] = targetLock

	return sourceRes, &targetResult, nil
}

// Returns codeSamples namespace name and digest
func (w *Workflow) runCodeSamples(ctx context.Context, codeSamplesStep *workflowTracking.WorkflowStep, codeSamples workflow.CodeSamples, target, sourcePath string, targetOutputPath *string) (string, string, error) {
	configPath := "."
	writeFileLocation := codeSamples.Output

	if targetOutputPath != nil {
		// configPath should be relative to the target output path for nested SDKs
		configPath = *targetOutputPath
		// If a write file location is specified, make sure it's relative to the target output path
		if writeFileLocation != "" {
			writeFileLocation = filepath.Join(*targetOutputPath, writeFileLocation)
		}
	}

	overlayString, err := codesamples.GenerateOverlay(ctx, sourcePath, "", "", configPath, writeFileLocation, []string{target}, true, false, codeSamples)
	if err != nil {
		return "", "", err
	}

	if !w.FrozenWorkflowLock {
		return w.snapshotCodeSamples(ctx, codeSamplesStep, overlayString, codeSamples)
	}

	return "", "", nil
}

func (w *Workflow) snapshotCodeSamples(ctx context.Context, parentStep *workflowTracking.WorkflowStep, overlayString string, codeSampleConfig workflow.CodeSamples) (namespaceName string, digest string, err error) {
	registryLocation := codeSampleConfig.Registry

	if registryLocation == nil {
		return
	}

	tags, err := w.getRegistryTags(ctx, "")
	if err != nil {
		return
	}

	registryStep := parentStep.NewSubstep("Snapshotting Code Samples")

	if !registry.IsRegistryEnabled(ctx) {
		registryStep.Skip("API Registry not enabled")
		return "", "", ocicommon.ErrAccessGated
	}

	orgSlug, workspaceSlug, namespaceName, _, err := registryLocation.ParseRegistryLocation()
	if err != nil {
		return "", "", fmt.Errorf("error parsing registry location %s: %w", string(registryLocation.Location), err)
	}

	if orgSlug != auth.GetOrgSlugFromContext(ctx) {
		return "", "", fmt.Errorf("current authenticated org %s does not match provided location %s", auth.GetOrgSlugFromContext(ctx), string(registryLocation.Location))
	}

	if workspaceSlug != auth.GetWorkspaceSlugFromContext(ctx) {
		return "", "", fmt.Errorf("current authenticated workspace %s does not match provided location %s", auth.GetWorkspaceSlugFromContext(ctx), string(registryLocation.Location))
	}

	pl := bundler.NewPipeline(&bundler.PipelineOptions{})

	memfs := fsextras.NewMemFS()

	overlayPath := "overlay.yaml"
	err = memfs.WriteBytes(overlayPath, []byte(overlayString), 0644)
	if err != nil {
		return "", "", fmt.Errorf("error writing overlay to memfs: %w", err)
	}

	registryStep.NewSubstep("Snapshotting Code Samples")

	gitRepo, err := git.NewLocalRepository(w.ProjectDir)
	if err != nil {
		log.From(ctx).Debug("error sniffing git repository", zap.Error(err))
	}

	rootDocument, err := memfs.Open(overlayPath)
	if err != nil {
		return "", "", fmt.Errorf("error opening root document: %w", err)
	}

	annotations, err := openapi.NewAnnotationsFromOpenAPI(rootDocument)
	if err != nil {
		return "", "", fmt.Errorf("error extracting annotations from openapi document: %w", err)
	}

	revision := ""
	if gitRepo != nil {
		revision, err = gitRepo.HeadHash()
		if err != nil {
			log.From(ctx).Debug("error sniffing head commit hash", zap.Error(err))
		}
	}
	annotations.Revision = revision
	annotations.BundleRoot = overlayPath

	err = pl.BuildOCIImage(ctx, bundler.NewReadWriteFS(memfs, memfs), &bundler.OCIBuildOptions{
		Tags:        tags,
		Annotations: annotations,
		MediaType:   ocicommon.MediaTypeOpenAPIOverlayV0,
	})

	if err != nil {
		return "", "", fmt.Errorf("error bundling code samples artifact: %w", err)
	}

	serverURL := auth.GetServerURL()

	insecurePublish := false
	if strings.HasPrefix(serverURL, "http://") {
		insecurePublish = true
	}

	reg := strings.TrimPrefix(serverURL, "http://")
	reg = strings.TrimPrefix(reg, "https://")

	substepStore := registryStep.NewSubstep("Uploading Code Samples")
	registryResponse, err := pl.PushOCIImage(ctx, memfs, &bundler.OCIPushOptions{
		Tags:     tags,
		Registry: reg,
		Access: ocicommon.NewRepositoryAccess(config.GetSpeakeasyAPIKey(), namespaceName, ocicommon.RepositoryAccessOptions{
			Insecure: insecurePublish,
		}),
	})

	if err != nil && !errors.Is(err, ocicommon.ErrAccessGated) {
		return "", "", fmt.Errorf("error publishing code samples bundle to registry: %w", err)
	} else if err != nil && errors.Is(err, ocicommon.ErrAccessGated) {
		registryStep.Skip("API Registry not enabled")
		substepStore.Skip("Registry not enabled")
		return "", "", err
	}

	if len(registryResponse.References) > 0 {
		digest = registryResponse.References[0].ManifestDescriptor.Digest.String()
	}

	registryStep.SucceedWorkflow()

	return
}

func (w *Workflow) printTargetSuccessMessage(ctx context.Context) {
	if len(w.SDKOverviewURLs) == 0 {
		return
	}

	heading := styles.Success.Render("SDKs Generated Successfully")
	var additionalLines []string
	for target, url := range w.SDKOverviewURLs {
		link := links.Shorten(ctx, url)
		additionalLines = append(additionalLines, styles.Success.Render(fmt.Sprintf("└─`%s` overview: ", target))+styles.DimmedItalic.Render(link))
	}

	msg := fmt.Sprintf("%s\n%s\n", styles.Success.Render(heading), strings.Join(additionalLines, "\n"))
	log.From(ctx).Println(msg)
}

func (w *Workflow) CancelGeneration() error {
	if w.CancellableGeneration != nil {
		w.CancellableGeneration.CancellationMutex.Lock()
		defer w.CancellableGeneration.CancellationMutex.Unlock()
		if w.CancellableGeneration.CancelGeneration != nil {
			w.CancellableGeneration.CancelGeneration()
			return nil
		}
	}

	return fmt.Errorf("Generation is not cancellable")
}

func (w *Workflow) fetchOldSchema(ctx context.Context, target string) ([]byte, error) {
	log.From(ctx).Info("Fetching old schema for target: ", zap.String("target", target))
	if w.lockfileOld != nil {
		if targetLockOld, ok := w.lockfileOld.Targets[target]; ok && !utils.IsZeroTelemetryOrganization(ctx) {
			log.From(ctx).Info("Starting to fetch old schema")
			orgSlug := auth.GetOrgSlugFromContext(ctx)
			workspaceSlug := auth.GetWorkspaceSlugFromContext(ctx)
			oldRegistryLocation := ""
			if targetLockOld.SourceRevisionDigest != "" && targetLockOld.SourceNamespace != "" {
				log.From(ctx).Info("Found source revision and source namespace")
				oldRegistryLocation = fmt.Sprintf("%s/%s/%s/%s@%s", "registry.speakeasyapi.dev", orgSlug, workspaceSlug,
					targetLockOld.SourceNamespace, targetLockOld.SourceRevisionDigest)
			} else {
				return nil, errors.New("source revision or source namespace was empty. Cant fetch old schema. SourceRevisionDigest: " + targetLockOld.SourceRevisionDigest + " SourceNamespace: " + targetLockOld.SourceNamespace)
			}

			d := workflow.Document{Location: workflow.LocationString(oldRegistryLocation)}
			oldDocPath, err := registry.ResolveSpeakeasyRegistryBundle(ctx, d, workflow.GetTempDir())
			log.From(ctx).Info("fethcing old schema bundle")
			if err != nil {
				log.From(ctx).Info("Error while fetching old schema bundle")
				return nil, fmt.Errorf("failed to resolve old schema. Err: %w", err)
			}
			oldDocBytes, err := GetSchema(ctx, oldDocPath.LocalFilePath)
			log.From(ctx).Info("unbundling old schema")
			if err != nil {
				log.From(ctx).Info("Error while unbundling old schema")
				return nil, fmt.Errorf("Error while unbundling old schema. Err: %w", err)
			}
			log.From(ctx).Info(fmt.Sprintf("oldDocBytes: %d", len(oldDocBytes)))
			return oldDocBytes, nil
		}
	} else {
		log.From(ctx).Info("no previous old schema found")
	}
	return nil, errors.New("no previous revision found")
}

func GetSchema(ctx context.Context, filePath string) ([]byte, error) {
	if filePath == "" {
		log.From(ctx).Info("file path is empty")
		return nil, fmt.Errorf("file path is empty")
	}
	specBytes, err := os.ReadFile(filePath)
	if err != nil {
		log.From(ctx).Info("no previous old schema found")
		return nil, fmt.Errorf("cannot read the spec: %s", err.Error())
	}

	return specBytes, nil
}
