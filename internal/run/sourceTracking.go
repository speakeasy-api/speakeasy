package run

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/bundler"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy-core/fsextras"
	"github.com/speakeasy-api/speakeasy-core/ocicommon"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/changes"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/git"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/reports"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/registry"
	"go.uber.org/zap"
)

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
			changesStep.Skip("failed to compute document changes")
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

	d := workflow.Document{Location: workflow.LocationString(oldRegistryLocation)}
	oldDocPath, err := registry.ResolveSpeakeasyRegistryBundle(ctx, d, workflow.GetTempDir())
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
			registryStep.Skip("failed to track OpenAPI changes")
		}
	}()

	namespaceName := strcase.ToKebab(sourceID)
	apiKey := config.GetSpeakeasyAPIKey()

	if source.Registry != nil {
		orgSlug, workspaceSlug, name, _, err := source.Registry.ParseRegistryLocation()
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

	if isSingleRegistrySource(source) {
		document, err := registry.ResolveSpeakeasyRegistryBundle(ctx, source.Inputs[0], workflow.GetTempDir())
		if err != nil {
			return err
		}
		w.lockfile.Sources[sourceID] = workflow.SourceLock{
			SourceNamespace:      namespaceName,
			SourceRevisionDigest: document.ManifestDigest,
			SourceBlobDigest:     document.BlobDigest,
			Tags:                 tags,
		}
		return nil
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
	// Always add the openapi document version as a tag
	tags = append(tags, annotations.Version)

	err = pl.BuildOCIImage(ctx, bundler.NewReadWriteFS(memfs, memfs), &bundler.OCIBuildOptions{
		Tags:        tags,
		Annotations: annotations,
		MediaType:   ocicommon.MediaTypeOpenAPIBundleV0,
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
		if os.Getenv("SPEAKEASY_ACTIVE_BRANCH") != "" {
			branch = os.Getenv("SPEAKEASY_ACTIVE_BRANCH")
		} else if strings.Contains(os.Getenv("GITHUB_REF"), "refs/pull/") {
			branch = strings.TrimPrefix(os.Getenv("GITHUB_HEAD_REF"), "refs/heads/")
		} else {
			branch = strings.TrimPrefix(os.Getenv("GITHUB_REF"), "refs/heads/")
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
		// Unclear why this happens but when a flag of type stringSlice is provided from our GitHub runner environment we see these trailing [  ] appear on value read
		// This happens even though the arg set itself is formatted correctly. This is a temporary workaround that will not cause side effects.
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
