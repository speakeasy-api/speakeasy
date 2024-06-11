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

	revisions, err := getRevisions(ctx, flags.Sources, flags.CodeSamples, wf, lockfile)
	if err != nil {
		return err
	}

	for _, revision := range revisions {
		err = registry.AddTags(ctx, revision.namespace, revision.revisionDigest, flags.Tags)
		if err != nil {
			return err
		}
		printSuccessMsg(ctx, revision.namespace, revision.revisionDigest, revision.usedBy...)
	}

	return nil
}

type revision struct {
	namespace      string
	revisionDigest string
	usedBy         []string
}

func getRevisions(ctx context.Context, sources, targets []string, wf *workflow.Workflow, lf *workflow.LockFile) ([]revision, error) {
	if len(sources) == 0 && len(targets) == 0 {
		return nil, fmt.Errorf("please specify at least one source or target (codeSamples) to tag")
	}

	// Dedup revisions
	revisions := make(map[string]revision)

	opts := strings.Join(maps.Keys(wf.Sources), ", ")
	for _, source := range sources {
		if _, ok := wf.Sources[source]; !ok {
			return nil, fmt.Errorf("source %s not found in workflow.yaml. Options: %s", source, opts)
		}
		if _, ok := lf.Sources[source]; !ok {
			return nil, fmt.Errorf("source %s not found in workflow.lock. If it was recently added, execute `speakeasy run` before adding tags. Options: %s", source, opts)
		}

		namespace := lf.Sources[source].SourceNamespace
		revisionDigest := lf.Sources[source].SourceRevisionDigest

		addRevision(ctx, revisions, source, namespace, revisionDigest)
	}

	opts = strings.Join(maps.Keys(wf.Targets), ", ")
	for _, target := range targets {
		if _, ok := wf.Targets[target]; !ok {
			return nil, fmt.Errorf("target %s not found in workflow.yaml. Options: %s", target, opts)
		}
		if _, ok := lf.Targets[target]; !ok {
			return nil, fmt.Errorf("target %s not found in workflow.lock. If it was recently added, execute `speakeasy run` before adding tags. Options: %s", target, opts)
		}

		namespace := lf.Targets[target].CodeSamplesNamespace
		revisionDigest := lf.Targets[target].CodeSamplesRevisionDigest

		addRevision(ctx, revisions, target, namespace, revisionDigest)
	}

	return maps.Values(revisions), nil
}

func addRevision(ctx context.Context, revisions map[string]revision, owner, namespace, revisionDigest string) {
	if namespace == "" || revisionDigest == "" {
		log.From(ctx).Println(styles.DimmedItalic.Render(fmt.Sprintf("%s has no revision information in workflow.lock. Skipping tagging\n", owner)))
	} else if cur, ok := revisions[namespace+revisionDigest]; ok {
		cur.usedBy = append(cur.usedBy, owner)
	} else {
		revisions[namespace+revisionDigest] = revision{
			namespace:      namespace,
			revisionDigest: revisionDigest,
			usedBy:         []string{owner},
		}
	}
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

func printSuccessMsg(ctx context.Context, namespaceName, revisionDigest string, usedBy ...string) {
	org := core.GetOrgSlugFromContext(ctx)
	workspace := core.GetWorkspaceSlugFromContext(ctx)

	msg := "Tags successfully added"
	if len(usedBy) > 0 {
		msg += fmt.Sprintf(" to %s", styles.HeavilyEmphasized.Render(strings.Join(usedBy, ", ")))
	}

	url := fmt.Sprintf("https://app.speakeasyapi.dev/org/%s/%s/apis/%s/%s", org, workspace, namespaceName, revisionDigest)

	logger := log.From(ctx)
	logger.Success(msg)
	logger.Println(styles.Dimmed.Render(url))
}
