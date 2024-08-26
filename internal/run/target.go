package run

import (
	"context"
	"fmt"
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
	"github.com/speakeasy-api/speakeasy/internal/codesamples"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/git"
	"github.com/speakeasy-api/speakeasy/internal/links"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
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
	targetLock := workflow.TargetLock{Source: t.Source}

	log.From(ctx).Infof("Running target %s (%s)...\n", target, t.Target)

	source, sourcePath, err := w.workflow.GetTargetSource(target)
	if err != nil {
		return nil, nil, err
	}

	var sourceRes *SourceResult

	if source != nil {
		sourcePath, sourceRes, err = w.RunSource(ctx, rootStep, t.Source, target)
		if err != nil {
			if w.FromQuickstart && sourceRes != nil && sourceRes.LintResult != nil && len(sourceRes.LintResult.ValidOperations) > 0 {
				cliEvent := events.GetTelemetryEventFromContext(ctx)
				if cliEvent != nil {
					cliEvent.GenerateNumberOfOperationsIgnored = new(int64)
					*cliEvent.GenerateNumberOfOperationsIgnored = int64(len(sourceRes.LintResult.InvalidOperations))
				}

				retriedPath, retriedRes, retriedErr := w.retryWithMinimumViableSpec(ctx, rootStep, t.Source, target, sourceRes.LintResult.ValidOperations)
				if retriedErr != nil {
					log.From(ctx).Errorf("Failed to retry with minimum viable spec: %s", retriedErr)
					// return the original error
					return nil, nil, err
				}

				w.OperationsRemoved = sourceRes.LintResult.InvalidOperations
				sourcePath = retriedPath
				sourceRes = retriedRes
			} else {
				return nil, nil, err
			}
		}
	} else {
		res, err := w.validateDocument(ctx, rootStep, t.Source, sourcePath, "speakeasy-generation", w.ProjectDir)
		if err != nil {
			return nil, nil, err
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
	targetLock.OutLocation = outDir

	published := t.IsPublished()

	genYamlStep := rootStep.NewSubstep("Validating gen.yaml")

	genConfig, err := sdkGenConfig.Load(outDir)
	if err != nil {
		return nil, nil, err
	}

	if w.SetVersion != "" && genConfig.Config != nil {
		appliedVersion := w.SetVersion
		if appliedVersion[0] == 'v' {
			appliedVersion = appliedVersion[1:]
		}
		if langCfg, ok := genConfig.Config.Languages[t.Target]; ok {
			if _, err := version.NewVersion(appliedVersion); err != nil {
				return nil, nil, fmt.Errorf("failed to parse version %s: %w", w.SetVersion, err)
			}

			langCfg.Version = appliedVersion
			genConfig.Config.Languages[t.Target] = langCfg
			if err := sdkGenConfig.SaveConfig(outDir, genConfig.Config); err != nil {
				return nil, nil, err
			}
		}
	}

	err = validation.ValidateConfigAndPrintErrors(ctx, t.Target, genConfig, published, target)
	if err != nil {
		if errors.Is(err, validation.NoConfigFound) {
			genYamlStep.Skip("gen.yaml not found, assuming new SDK")
		} else {
			return nil, nil, err
		}
	}

	genStep := rootStep.NewSubstep(fmt.Sprintf("Generating %s SDK", utils.CapitalizeFirst(t.Target)))

	logListener := make(chan log.Msg)
	logger := log.From(ctx).WithListener(logListener)
	ctx = log.With(ctx, logger)
	go genStep.ListenForSubsteps(logListener)

	generationAccess, err := sdkgen.Generate(
		ctx,
		config.GetCustomerID(),
		config.GetWorkspaceID(),
		t.Target,
		sourcePath,
		"",
		"",
		outDir,
		events.GetSpeakeasyVersionFromContext(ctx),
		w.InstallationURLs[target],
		w.Debug,
		true,
		published,
		false,
		w.Repo,
		w.RepoSubDirs[target],
		w.Verbose,
		w.ShouldCompile,
		w.ForceGeneration,
		target,
		w.SkipVersioning,
	)
	if err != nil {
		return nil, nil, err
	}
	w.generationAccess = generationAccess

	if t.CodeSamples != nil {
		codeSamplesStep := rootStep.NewSubstep("Generating Code Samples")
		configPath := "."
		outputPath := t.CodeSamples.Output
		if t.Output != nil {
			configPath = *t.Output
			outputPath = filepath.Join(*t.Output, outputPath)
		}

		style := codesamples.Default
		if t.CodeSamples.Style != nil {
			switch *t.CodeSamples.Style {
			case "readme":
				style = codesamples.ReadMe
			}
		}

		overlayString, err := codesamples.GenerateOverlay(ctx, sourcePath, "", "", configPath, outputPath, []string{t.Target}, true, style)
		if err != nil {
			return nil, nil, err
		}

		namespaceName, digest, err := w.snapshotCodeSamples(ctx, codeSamplesStep, overlayString, *t.CodeSamples)
		if err != nil {
			return nil, nil, err
		}
		targetLock.CodeSamplesNamespace = namespaceName
		targetLock.CodeSamplesRevisionDigest = digest
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
		w.SDKOverviewURLs[target] = fmt.Sprintf("https://app.speakeasyapi.dev/org/%s/%s/targets/%s", orgSlug, workspaceSlug, *genLockID)
	}

	w.lockfile.Targets[target] = targetLock

	targetResult := TargetResult{
		OutputPath:  outDir,
		GenYamlPath: genConfig.ConfigPath,
	}

	return sourceRes, &targetResult, nil
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

	orgSlug, workspaceSlug, namespaceName, err := registryLocation.ParseRegistryLocation()
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
		Tags:         tags,
		Reproducible: true,
		Annotations:  annotations,
		MediaType:    ocicommon.MediaTypeOpenAPIOverlayV0,
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
		additionalLines = append(additionalLines, styles.Success.Render(fmt.Sprintf("└─`%s` overview: %s", target, styles.Dimmed.Render(link))))
	}

	msg := fmt.Sprintf("%s\n%s\n", styles.Success.Render(heading), strings.Join(additionalLines, "\n"))
	log.From(ctx).Println(msg)
}
