package cmd

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"golang.org/x/exp/maps"
	"strings"

	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/registry"
)

var tagCmd = &model.CommandGroup{
	Usage:    "tag",
	Short:    "Add tags to a given revision of your API. Specific to a registry namespace",
	Commands: []model.Command{tagPromoteCmd, tagApplyCmd},
}

type tagPromoteFlagsArgs struct {
	Sources     []string `json:"sources"`
	CodeSamples []string `json:"code-samples"`
	Tags        []string `json:"tags"`
}

var tagPromoteCmd = &model.ExecutableCommand[tagPromoteFlagsArgs]{
	Usage:        "promote",
	Short:        "Add tags to a revision in the Registry, based on the most recent workflow run",
	Run:          runTagPromote,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringSliceFlag{
			Name:        "sources",
			Shorthand:   "s",
			Description: "a list of sources whose schema revisions should be tagged",
		},
		flag.StringSliceFlag{
			Name:        "code-samples",
			Shorthand:   "c",
			Description: "a list of targets whose code samples should be tagged",
		},
		flag.StringSliceFlag{
			Name:        "tags",
			Shorthand:   "t",
			Description: "A list of tags to apply",
			Required:    true,
		},
	},
}

type tagApplyFlagsArgs struct {
	NamespaceName  string   `json:"namespace-name"`
	RevisionDigest string   `json:"revision-digest"`
	Tags           []string `json:"tags"`
}

var tagApplyCmd = &model.ExecutableCommand[tagApplyFlagsArgs]{
	Usage:        "apply",
	Short:        "Add tags to a given revision of your API. Specific to a registry namespace",
	Run:          runTagApply,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringFlag{ // TODO: maybe it would be better to just take in a registry URL
			Name:        "namespace-name",
			Shorthand:   "n",
			Description: "the revision to tag",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "revision-digest",
			Shorthand:   "r",
			Description: "the revision ID to tag",
			Required:    true,
		},
		flag.StringSliceFlag{
			Name:        "tags",
			Shorthand:   "t",
			Description: "A list of tags to apply",
			Required:    true,
		},
	},
}

func runTagPromote(ctx context.Context, flags tagPromoteFlagsArgs) error {
	workspaceID, _ := core.GetWorkspaceIDFromContext(ctx)
	if !registry.IsRegistryEnabled(ctx) {
		return fmt.Errorf("API Registry is not enabled for this workspace %s", workspaceID)
	}

	wf, projectDir, err := utils.GetWorkflowAndDir()
	if err != nil || wf == nil {
		return fmt.Errorf("failed to load workflow.yaml: %w", err)
	}

	lockfile, err := workflow.LoadLockfile(projectDir)

	if err = validateSourcesAndTargets(flags.Sources, flags.CodeSamples, wf, lockfile); err != nil {
		return err
	}

	for _, source := range flags.Sources {
		namespace := lockfile.Sources[source].SourceNamespace
		revisionDigest := lockfile.Sources[source].SourceRevisionDigest
		err = registry.AddTags(ctx, namespace, revisionDigest, flags.Tags)
		if err != nil {
			return err
		}
		printSuccessMsg(ctx, namespace, revisionDigest)
	}

	for _, target := range flags.CodeSamples {
		namespace := lockfile.Targets[target].CodeSamplesNamespace
		revisionDigest := lockfile.Targets[target].CodeSamplesRevisionDigest
		err = registry.AddTags(ctx, namespace, revisionDigest, flags.Tags)
		if err != nil {
			return err
		}
		printSuccessMsg(ctx, namespace, revisionDigest)
	}

	return nil
}

func validateSourcesAndTargets(sources, targets []string, wf *workflow.Workflow, lf *workflow.LockFile) error {
	if len(sources) == 0 && len(targets) == 0 {
		return fmt.Errorf("please specify at least one source or target (codeSamples) to tag")
	}

	opts := strings.Join(maps.Keys(wf.Sources), ", ")
	for _, source := range sources {
		if _, ok := wf.Sources[source]; !ok {
			return fmt.Errorf("source %s not found in workflow.yaml. Options: %s", source, opts)
		}
		if _, ok := lf.Sources[source]; !ok {
			return fmt.Errorf("source %s not found in workflow.lock. If it was recently added, execute `speakeasy run` before adding tags. Options: %s", source, opts)
		}
	}

	opts = strings.Join(maps.Keys(wf.Targets), ", ")
	for _, target := range targets {
		if _, ok := wf.Targets[target]; !ok {
			return fmt.Errorf("target %s not found in workflow.yaml. Options: %s", target, opts)
		}
		if _, ok := lf.Targets[target]; !ok {
			return fmt.Errorf("target %s not found in workflow.lock. If it was recently added, execute `speakeasy run` before adding tags. Options: %s", target, opts)
		}
	}

	return nil
}

func runTagApply(ctx context.Context, flags tagApplyFlagsArgs) error {
	workspaceID, _ := core.GetWorkspaceIDFromContext(ctx)
	if !registry.IsRegistryEnabled(ctx) {
		return fmt.Errorf("API Registry is not enabled for this workspace %s", workspaceID)
	}
	revisionDigest := flags.RevisionDigest
	if !strings.HasPrefix(revisionDigest, "sha256:") {
		revisionDigest = "sha256:" + revisionDigest
	}

	if err := registry.AddTags(ctx, flags.NamespaceName, revisionDigest, flags.Tags); err != nil {
		return err
	}

	printSuccessMsg(ctx, flags.NamespaceName, revisionDigest)

	return nil
}

func printSuccessMsg(ctx context.Context, namespaceName, revisionDigest string) {
	org := core.GetOrgSlugFromContext(ctx)
	workspace := core.GetWorkspaceSlugFromContext(ctx)

	url := fmt.Sprintf("https://app.speakeasyapi.dev/org/%s/%s/apis/%s/%s", org, workspace, namespaceName, revisionDigest)

	logger := log.From(ctx)
	logger.Success("Tags successfully added")
	logger.Println(styles.Dimmed.Render(url))
}
