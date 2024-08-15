package run

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/links"
	"github.com/speakeasy-api/versioning-reports/versioning"

	"github.com/iancoleman/strcase"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/bundler"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy-core/fsextras"
	"github.com/speakeasy-api/speakeasy-core/ocicommon"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/changes"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/defaultcodesamples"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/git"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/overlay"
	"github.com/speakeasy-api/speakeasy/internal/reports"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/pkg/merge"
	"github.com/speakeasy-api/speakeasy/registry"
	"go.uber.org/zap"
)

type SourceResult struct {
	Source       string
	InputSpec    string
	LintResult   *validation.ValidationResult
	ChangeReport *reports.ReportResult
	Diagnosis    suggestions.Diagnosis
	OutputPath   string
}

type LintingError struct {
	Err      error
	Document string
}

func (e *LintingError) Error() string {
	return fmt.Sprintf("linting failed: %s - %s", e.Document, e.Err.Error())
}

func (w *Workflow) RunSource(ctx context.Context, parentStep *workflowTracking.WorkflowStep, sourceID, targetID string) (string, *SourceResult, error) {
	rootStep := parentStep.NewSubstep(fmt.Sprintf("Source: %s", sourceID))
	source := w.workflow.Sources[sourceID]
	sourceRes := &SourceResult{
		Source: sourceID,
	}

	rulesetToUse := "speakeasy-generation"
	if source.Ruleset != nil {
		rulesetToUse = *source.Ruleset
	}

	logger := log.From(ctx)
	logger.Infof("Running Source %s...", sourceID)

	outputLocation, err := source.GetOutputLocation()
	if err != nil {
		return "", nil, err
	}

	var currentDocument string
	if len(source.Inputs) == 1 {
		var singleLocation *string
		// The output location should be the resolved location
		if len(source.Overlays) == 0 {
			singleLocation = &outputLocation
		}
		currentDocument, err = schema.ResolveDocument(ctx, source.Inputs[0], singleLocation, rootStep)
		if err != nil {
			return "", nil, err
		}
		// In registry bundles specifically we cannot know the exact file output location before pulling the bundle down
		if len(source.Overlays) == 0 && source.Inputs[0].IsSpeakeasyRegistry() {
			outputLocation = currentDocument
		}
	} else {
		mergeStep := rootStep.NewSubstep("Merge Documents")

		mergeLocation := source.GetTempMergeLocation()
		if len(source.Overlays) == 0 {
			mergeLocation = outputLocation
		}

		logger.Infof("Merging %d schemas into %s...", len(source.Inputs), mergeLocation)

		inSchemas := []string{}
		for _, input := range source.Inputs {
			resolvedPath, err := schema.ResolveDocument(ctx, input, nil, mergeStep)
			if err != nil {
				return "", nil, err
			}
			inSchemas = append(inSchemas, resolvedPath)
		}

		mergeStep.NewSubstep(fmt.Sprintf("Merge %d documents", len(source.Inputs)))

		if err := mergeDocuments(ctx, inSchemas, mergeLocation, rulesetToUse, w.ProjectDir); err != nil {
			return "", nil, err
		}

		currentDocument = mergeLocation
	}

	sourceRes.InputSpec, err = utils.ReadFileToString(currentDocument)
	if err != nil {
		return "", nil, err
	}

	if len(source.Overlays) > 0 {
		overlayStep := rootStep.NewSubstep("Applying Overlays")

		overlayLocation := outputLocation

		logger.Infof("Applying %d overlays into %s...", len(source.Overlays), overlayLocation)

		overlaySchemas := []string{}
		for _, overlay := range source.Overlays {
			overlayFilePath := ""
			if overlay.Document != nil {
				overlayFilePath, err = schema.ResolveDocument(ctx, *overlay.Document, nil, overlayStep)
				if err != nil {
					return "", nil, err
				}
			} else if overlay.FallbackCodeSamples != nil {
				// Make temp file for the overlay output
				overlayFilePath := filepath.Join(workflow.GetTempDir(), fmt.Sprintf("fallback_code_samples_overlay_%s.yaml", randStringBytes(10)))
				if err := os.MkdirAll(filepath.Dir(overlayFilePath), 0o755); err != nil {
					return "", nil, err
				}

				err = defaultcodesamples.DefaultCodeSamples(ctx, defaultcodesamples.DefaultCodeSamplesFlags{
					SchemaPath: currentDocument,
					Language:   overlay.FallbackCodeSamples.FallbackCodeSamplesLanguage,
					Out:        overlayFilePath,
				})
				if err != nil {
					logger.Errorf("failed to generate default code samples: %s", err.Error())
					return "", nil, err
				}
			}
			overlaySchemas = append(overlaySchemas, overlayFilePath)

		}

		overlayStep.NewSubstep(fmt.Sprintf("Apply %d overlay(s)", len(source.Overlays)))

		if err := overlayDocument(ctx, currentDocument, overlaySchemas, overlayLocation); err != nil {
			return "", nil, err
		}

		currentDocument = overlayLocation
	}

	if !isSingleRegistrySource(source) && !w.SkipSnapshot {
		err = w.snapshotSource(ctx, rootStep, sourceID, source, currentDocument)
		if err != nil && !errors.Is(err, ocicommon.ErrAccessGated) {
			logger.Warnf("failed to snapshot source: %s", err.Error())
		}
	}

	sourceRes.Diagnosis, err = suggest.Diagnose(ctx, currentDocument)
	if err != nil {
		return "", sourceRes, err
	}

	// If the source has a previous tracked revision, compute changes against it
	if w.lockfileOld != nil && !w.SkipChangeReport {
		if targetLockOld, ok := w.lockfileOld.Targets[targetID]; ok && !utils.IsZeroTelemetryOrganization(ctx) {
			sourceRes.ChangeReport, err = w.computeChanges(ctx, rootStep, targetLockOld, currentDocument)
			if err != nil {
				// Don't fail the whole workflow if this fails
				logger.Warnf("failed to compute OpenAPI changes: %s", err.Error())
			}
		}
	}
	if sourceRes.ChangeReport == nil {
		// If we failed to compute changes, always generate the SDK
		_ = versioning.AddVersionReport(ctx, versioning.VersionReport{
			MustGenerate: true,
			Key:          "openapi_change_summary",
			Priority:     5,
		})
	}

	if !w.SkipLinting {
		sourceRes.LintResult, err = w.validateDocument(ctx, rootStep, sourceID, currentDocument, rulesetToUse, w.ProjectDir)
		if err != nil {
			return "", sourceRes, &LintingError{Err: err, Document: currentDocument}
		}
	}

	rootStep.SucceedWorkflow()

	sourceRes.OutputPath = currentDocument
	return currentDocument, sourceRes, nil
}

func (w *Workflow) computeChanges(ctx context.Context, rootStep *workflowTracking.WorkflowStep, targetLock workflow.TargetLock, newDocPath string) (r *reports.ReportResult, err error) {
	changesStep := rootStep.NewSubstep("Computing Document Changes")
	if !registry.IsRegistryEnabled(ctx) {
		changesStep.Skip("API Registry not enabled")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("computing document changes panicked: %v", r)
		}

		if err != nil {
			changesStep.Fail()
		}
	}()

	orgSlug := auth.GetOrgSlugFromContext(ctx)
	workspaceSlug := auth.GetWorkspaceSlugFromContext(ctx)

	oldRegistryLocation := ""
	if targetLock.SourceRevisionDigest != "" && targetLock.SourceNamespace != "" {
		oldRegistryLocation = fmt.Sprintf("%s/%s/%s/%s@%s", "registry.speakeasyapi.dev", orgSlug, workspaceSlug,
			targetLock.SourceNamespace, targetLock.SourceRevisionDigest)
	} else {
		changesStep.Skip("no previous revision found")

		return
	}

	changesStep.NewSubstep("Downloading prior revision")

	d := workflow.Document{Location: oldRegistryLocation}
	oldDocPath, err := registry.ResolveSpeakeasyRegistryBundle(ctx, d, d.GetTempRegistryDir(workflow.GetTempDir()))
	if err != nil {
		return
	}

	changesStep.NewSubstep("Computing changes")

	c, err := changes.GetChanges(ctx, oldDocPath.LocalFilePath, newDocPath)
	if err != nil {
		return r, fmt.Errorf("error computing changes: %w", err)
	}

	changesStep.NewSubstep("Uploading changes report")
	report, err := reports.UploadReport(ctx, c.GetHTMLReport(), shared.TypeChanges)
	if err != nil {
		return r, fmt.Errorf("failed to persist report: %w", err)
	}
	r = &report

	log.From(ctx).Info(r.Message)

	summary, err := c.GetSummary()
	if err != nil || summary == nil {
		return r, fmt.Errorf("failed to get report summary: %w", err)
	}

	// Do not write github action changes if we have already processed this source
	// If we don't do this check we will see duplicate openapi changes summaries in the PR
	if _, ok := w.computedChanges[targetLock.Source]; !ok {
		github.GenerateChangesSummary(ctx, r.URL, *summary)
	}

	w.computedChanges[targetLock.Source] = true

	changesStep.SucceedWorkflow()
	return
}

func (w *Workflow) validateDocument(ctx context.Context, parentStep *workflowTracking.WorkflowStep, source, schemaPath, defaultRuleset, projectDir string) (*validation.ValidationResult, error) {
	step := parentStep.NewSubstep("Validating Document")

	if slices.Contains(w.validatedDocuments, schemaPath) {
		step.Skip("already validated")
		return nil, nil
	}

	limits := &validation.OutputLimits{
		MaxErrors: 1000,
		MaxWarns:  1000,
	}

	res, err := validation.ValidateOpenAPI(ctx, source, schemaPath, "", "", limits, defaultRuleset, projectDir, w.FromQuickstart)

	w.validatedDocuments = append(w.validatedDocuments, schemaPath)

	return res, err
}

func (w *Workflow) snapshotSource(ctx context.Context, parentStep *workflowTracking.WorkflowStep, sourceID string, source workflow.Source, documentPath string) (err error) {
	registryStep := parentStep.NewSubstep("Tracking OpenAPI Changes")

	if !registry.IsRegistryEnabled(ctx) {
		registryStep.Skip("API Registry not enabled")
		return ocicommon.ErrAccessGated
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("tracking OpenAPI changes panicked: %v", r)
		}

		if err != nil {
			registryStep.Fail()
		}
	}()

	namespaceName := strcase.ToKebab(sourceID)
	apiKey := config.GetSpeakeasyAPIKey()

	if source.Registry != nil {
		orgSlug, workspaceSlug, name, err := source.Registry.ParseRegistryLocation()
		if err != nil {
			if env.IsGithubAction() {
				return fmt.Errorf("error parsing registry location %s: %w", string(source.Registry.Location), err)
			}

			log.From(ctx).Warnf("error parsing registry location %s: %v", string(source.Registry.Location), err)
		}

		skip, key, err := getAndValidateAPIKey(ctx, orgSlug, workspaceSlug, string(source.Registry.Location))

		if skip {
			registryStep.Skip("you are authenticated with speakeasy-self")
			return nil
		}

		if err != nil {
			return err
		}

		namespaceName = name
		apiKey = key
	}

	tags, err := w.getRegistryTags(ctx, sourceID)
	if err != nil {
		return err
	}

	pl := bundler.NewPipeline(&bundler.PipelineOptions{})
	memfs := fsextras.NewMemFS()

	registryStep.NewSubstep("Snapshotting OpenAPI Revision")

	rootDocumentPath, err := pl.Localize(ctx, memfs, bundler.LocalizeOptions{
		DocumentPath: documentPath,
	})
	if err != nil {
		return fmt.Errorf("error localizing openapi document: %w", err)
	}

	gitRepo, err := git.NewLocalRepository(w.ProjectDir)
	if err != nil {
		log.From(ctx).Debug("error sniffing git repository", zap.Error(err))
	}

	rootDocument, err := memfs.Open(filepath.Join(bundler.BundleRoot.String(), "openapi.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		rootDocument, err = memfs.Open(filepath.Join(bundler.BundleRoot.String(), "openapi.json"))
	}
	if err != nil {
		return fmt.Errorf("error opening root document: %w", err)
	}

	annotations, err := openapi.NewAnnotationsFromOpenAPI(rootDocument)
	if err != nil {
		return fmt.Errorf("error extracting annotations from openapi document: %w", err)
	}

	revision := ""
	if gitRepo != nil {
		revision, err = gitRepo.HeadHash()
		if err != nil {
			log.From(ctx).Debug("error sniffing head commit hash", zap.Error(err))
		}
	}
	annotations.Revision = revision
	annotations.BundleRoot = strings.TrimPrefix(rootDocumentPath, string(os.PathSeparator))

	err = pl.BuildOCIImage(ctx, bundler.NewReadWriteFS(memfs, memfs), &bundler.OCIBuildOptions{
		Tags:         tags,
		Reproducible: true,
		Annotations:  annotations,
		MediaType:    ocicommon.MediaTypeOpenAPIBundleV0,
	})
	if err != nil {
		return fmt.Errorf("error bundling openapi artifact: %w", err)
	}

	serverURL := auth.GetServerURL()
	insecurePublish := false
	if strings.HasPrefix(serverURL, "http://") {
		insecurePublish = true
	}

	reg := strings.TrimPrefix(serverURL, "http://")
	reg = strings.TrimPrefix(reg, "https://")

	substepStore := registryStep.NewSubstep("Storing OpenAPI Revision")
	pushResult, err := pl.PushOCIImage(ctx, memfs, &bundler.OCIPushOptions{
		Tags:     tags,
		Registry: reg,
		Access: ocicommon.NewRepositoryAccess(apiKey, namespaceName, ocicommon.RepositoryAccessOptions{
			Insecure: insecurePublish,
		}),
	})
	if err != nil && !errors.Is(err, ocicommon.ErrAccessGated) {
		return fmt.Errorf("error publishing openapi bundle to registry: %w", err)
	} else if err != nil && errors.Is(err, ocicommon.ErrAccessGated) {
		registryStep.Skip("API Registry not enabled")
		substepStore.Skip("Registry not enabled")
		return err
	}

	registryStep.SucceedWorkflow()

	var manifestDigest *string
	var blobDigest *string
	if pushResult.References != nil && len(pushResult.References) > 0 {
		manifestDigestStr := pushResult.References[0].ManifestDescriptor.Digest.String()
		manifestDigest = &manifestDigestStr
		manifestLayers := pushResult.References[0].Manifest.Layers
		for _, layer := range manifestLayers {
			if layer.MediaType == ocicommon.MediaTypeOpenAPIBundleV0 {
				blobDigestStr := layer.Digest.String()
				blobDigest = &blobDigestStr
				break
			}
		}
	}

	cliEvent := events.GetTelemetryEventFromContext(ctx)
	if cliEvent != nil {
		cliEvent.SourceRevisionDigest = manifestDigest
		cliEvent.SourceNamespaceName = &namespaceName
		cliEvent.SourceBlobDigest = blobDigest
	}

	// Automatically migrate speakeasy registry users to have a source publishing location
	if source.Registry == nil && registry.IsRegistryEnabled(ctx) {
		registryEntry := &workflow.SourceRegistry{}
		if err := registryEntry.SetNamespace(fmt.Sprintf("%s/%s/%s", auth.GetOrgSlugFromContext(ctx), auth.GetWorkspaceSlugFromContext(ctx), namespaceName)); err != nil {
			return err
		}
		source.Registry = registryEntry
		w.workflow.Sources[sourceID] = source
		if err := workflow.Save(w.ProjectDir, &w.workflow); err != nil {
			return err
		}
	} else if source.Registry != nil && !registry.IsRegistryEnabled(ctx) { // Automatically remove source publishing location if registry is disabled
		source.Registry = nil
		w.workflow.Sources[sourceID] = source
		if err := workflow.Save(w.ProjectDir, &w.workflow); err != nil {
			return err
		}
	}

	w.lockfile.Sources[sourceID] = workflow.SourceLock{
		SourceNamespace:      namespaceName,
		SourceRevisionDigest: *manifestDigest,
		SourceBlobDigest:     *blobDigest,
		Tags:                 tags,
	}

	return nil
}

func getAndValidateAPIKey(ctx context.Context, orgSlug, workspaceSlug, registryLocation string) (skip bool, key string, err error) {
	if key = config.GetWorkspaceAPIKey(orgSlug, workspaceSlug); key != "" {
		return
	}

	authenticatedOrg := auth.GetOrgSlugFromContext(ctx)
	if orgSlug != authenticatedOrg {
		// If the user is authenticated with speakeasy-self, just skip snapshotting rather than failing
		if authenticatedOrg == speakeasySelf && !env.IsGithubAction() {
			skip = true
			return
		}

		message := fmt.Sprintf("current authenticated org %s does not match provided location %s", auth.GetOrgSlugFromContext(ctx), registryLocation)
		if !env.IsGithubAction() {
			message += " run `speakeasy auth logout`"
		}
		err = fmt.Errorf(message)
		return
	}

	if workspaceSlug != auth.GetWorkspaceSlugFromContext(ctx) {
		message := fmt.Sprintf("current authenticated workspace %s does not match provided location %s", auth.GetWorkspaceSlugFromContext(ctx), registryLocation)
		if !env.IsGithubAction() {
			message += " run `speakeasy auth logout`"
		}
		err = fmt.Errorf(message)
		return
	}

	key = config.GetSpeakeasyAPIKey()

	return
}

func (w *Workflow) getRegistryTags(ctx context.Context, sourceID string) ([]string, error) {
	tags := []string{"latest"}
	if env.IsGithubAction() {
		// implicitly add branch tag
		var branch string
		if strings.Contains(os.Getenv("GITHUB_REF"), "refs/heads/") {
			branch = strings.TrimPrefix(os.Getenv("GITHUB_REF"), "refs/heads/")
		} else if strings.Contains(os.Getenv("GITHUB_REF"), "refs/pull/") {
			branch = strings.TrimPrefix(os.Getenv("GITHUB_HEAD_REF"), "refs/heads/")
		}

		// trim to fit docker tag format
		branch = strings.TrimSpace(branch)
		branch = strings.ReplaceAll(branch, "/", "-")
		if branch != "" {
			tags = append(tags, branch)
		}
	}
	for _, tag := range w.RegistryTags {
		var parsedTag string
		// Unclear why this happens but when a flag of type stringSlice is provided from our github runner environment we see these trailing [  ] appear on value read
		// This happens even though the arg set itself is formatted correctly. This is a temporary workaround that will not cause side-effects.
		tag = strings.Trim(tag, "[")
		tag = strings.Trim(tag, "]")
		if len(tag) > 0 {
			// TODO: We could add more tag validation here
			if strings.Count(tag, ":") > 1 {
				return tags, fmt.Errorf("invalid tag format: %s", tag)
			}

			if strings.Contains(tag, ":") {
				tagSplit := strings.Split(tag, ":")
				if sourceID == tagSplit[0] {
					parsedTag = strings.Trim(tagSplit[1], " ")
				}
			} else {
				parsedTag = tag
			}

			if len(parsedTag) > 0 && !slices.Contains(tags, parsedTag) {
				tags = append(tags, parsedTag)
			}
		}
	}

	return tags, nil
}

func (w *Workflow) printSourceSuccessMessage(ctx context.Context) {
	if len(w.SourceResults) == 0 {
		return
	}

	logger := log.From(ctx)
	logger.Println("") // Newline for better readability

	for sourceID, sourceRes := range w.SourceResults {
		heading := fmt.Sprintf("Source `%s` Compiled Successfully", sourceID)
		var additionalLines []string

		appendReportLocation := func(report reports.ReportResult) {
			if location := report.Location(); location != "" {
				additionalLines = append(additionalLines, styles.Success.Render(fmt.Sprintf("└─%s: ", report.Title()))+styles.DimmedItalic.Render(location))
			}
		}

		if sourceRes.LintResult != nil && sourceRes.LintResult.Report != nil {
			appendReportLocation(*sourceRes.LintResult.Report)
		}
		if sourceRes.ChangeReport != nil {
			appendReportLocation(*sourceRes.ChangeReport)
		}

		if sourceRes.Diagnosis != nil && suggest.ShouldSuggest(sourceRes.Diagnosis) {
			baseURL := auth.GetWorkspaceBaseURL(ctx)
			link := fmt.Sprintf(`%s/apis/%s/suggest`, baseURL, w.lockfile.Sources[sourceID].SourceNamespace)
			link = links.Shorten(ctx, link)

			msg := fmt.Sprintf("%s %s", styles.Dimmed.Render(sourceRes.Diagnosis.Summarize()+"."), styles.DimmedItalic.Render(link))
			additionalLines = append(additionalLines, fmt.Sprintf("`└─Improve with AI:` %s", msg))
		}

		msg := fmt.Sprintf("%s\n%s\n", styles.Success.Render(heading), strings.Join(additionalLines, "\n"))
		logger.Println(msg)
	}
}

func mergeDocuments(ctx context.Context, inSchemas []string, outFile, defaultRuleset, workingDir string) error {
	if err := os.MkdirAll(filepath.Dir(outFile), os.ModePerm); err != nil {
		return err
	}

	if err := merge.MergeOpenAPIDocuments(ctx, inSchemas, outFile, defaultRuleset, workingDir); err != nil {
		return err
	}

	log.From(ctx).Printf("Successfully merged %d schemas into %s", len(inSchemas), outFile)

	return nil
}

func isSingleRegistrySource(source workflow.Source) bool {
	return len(source.Inputs) == 1 && len(source.Overlays) == 0 && source.Inputs[0].IsSpeakeasyRegistry()
}

func overlayDocument(ctx context.Context, schema string, overlayFiles []string, outFile string) error {
	currentBase := schema
	if err := os.MkdirAll(workflow.GetTempDir(), os.ModePerm); err != nil {
		return err
	}

	for _, overlayFile := range overlayFiles {
		applyPath := getTempApplyPath(outFile)

		tempOutFile, err := os.Create(applyPath)
		if err != nil {
			return err
		}

		// YamlOut param needs to be based on the eventual output file
		if err := overlay.Apply(currentBase, overlayFile, utils.HasYAMLExt(outFile), tempOutFile, false, false); err != nil {
			return err
		}

		if err := tempOutFile.Close(); err != nil {
			return err
		}

		currentBase = applyPath
	}

	if err := os.MkdirAll(filepath.Dir(outFile), os.ModePerm); err != nil {
		return err
	}

	finalTempFile, err := os.Open(currentBase)
	if err != nil {
		return err
	}
	defer finalTempFile.Close()

	outFileWriter, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer outFileWriter.Close()

	if _, err := io.Copy(outFileWriter, finalTempFile); err != nil {
		return err
	}

	log.From(ctx).Successf("Successfully applied %d overlays into %s", len(overlayFiles), outFile)

	return nil
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var randStringBytes = func(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func getTempApplyPath(path string) string {
	return filepath.Join(workflow.GetTempDir(), fmt.Sprintf("applied_%s%s", randStringBytes(10), filepath.Ext(path)))
}
