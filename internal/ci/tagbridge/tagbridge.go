// Package tagbridge provides a bridge between the CI actions and the speakeasy
// registry tagging logic, replacing the old subprocess-based cli.Tag() call with
// direct Go function calls.
package tagbridge

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/registry"
)

// TagPromote applies tags to source and code sample revisions in the registry,
// replicating the behavior of `speakeasy tag promote -s sources -c codeSamples -t tags`.
func TagPromote(ctx context.Context, tags, sources, codeSamples []string) error {
	workspaceID, _ := core.GetWorkspaceIDFromContext(ctx)
	if !registry.IsRegistryEnabled(ctx) {
		return fmt.Errorf("API Registry is not enabled for this workspace %s", workspaceID)
	}

	wf, projectDir, err := utils.GetWorkflowAndDir()
	if err != nil || wf == nil {
		return fmt.Errorf("failed to load workflow.yaml: %w", err)
	}

	lockfile, err := workflow.LoadLockfile(projectDir)
	if err != nil || lockfile == nil {
		return fmt.Errorf("failed to load workflow.lock: %w", err)
	}

	revisions, err := getRevisions(sources, codeSamples, wf, lockfile)
	if err != nil {
		return err
	}

	for _, rev := range revisions {
		if err := registry.AddTags(ctx, rev.namespace, rev.revisionDigest, tags); err != nil {
			return err
		}
		logging.Info("Tags successfully added to %s (%s/%s)", strings.Join(rev.usedBy, ", "), rev.namespace, rev.revisionDigest)
	}

	return nil
}

type revision struct {
	namespace      string
	revisionDigest string
	usedBy         []string
}

func getRevisions(sources, targets []string, wf *workflow.Workflow, lf *workflow.LockFile) ([]revision, error) {
	if len(sources) == 0 && len(targets) == 0 {
		return nil, fmt.Errorf("please specify at least one source or target (codeSamples) to tag")
	}

	revisions := make(map[string]revision)

	opts := strings.Join(slices.Collect(maps.Keys(wf.Sources)), ", ")
	for _, source := range sources {
		if _, ok := wf.Sources[source]; !ok {
			return nil, fmt.Errorf("source %s not found in workflow.yaml. Options: %s", source, opts)
		}
		if _, ok := lf.Sources[source]; !ok {
			return nil, fmt.Errorf("source %s not found in workflow.lock. If it was recently added, execute `speakeasy run` before adding tags. Options: %s", source, opts)
		}

		namespace := lf.Sources[source].SourceNamespace
		revisionDigest := lf.Sources[source].SourceRevisionDigest

		addRevision(revisions, source, namespace, revisionDigest)
	}

	opts = strings.Join(slices.Collect(maps.Keys(wf.Targets)), ", ")
	for _, target := range targets {
		if _, ok := wf.Targets[target]; !ok {
			return nil, fmt.Errorf("target %s not found in workflow.yaml. Options: %s", target, opts)
		}
		if _, ok := lf.Targets[target]; !ok {
			return nil, fmt.Errorf("target %s not found in workflow.lock. If it was recently added, execute `speakeasy run` before adding tags. Options: %s", target, opts)
		}

		namespace := lf.Targets[target].CodeSamplesNamespace
		revisionDigest := lf.Targets[target].CodeSamplesRevisionDigest

		addRevision(revisions, target, namespace, revisionDigest)
	}

	return slices.Collect(maps.Values(revisions)), nil
}

func addRevision(revisions map[string]revision, owner, namespace, revisionDigest string) {
	if namespace == "" || revisionDigest == "" {
		logging.Info("%s has no revision information in workflow.lock. Skipping tagging", owner)
	} else if cur, ok := revisions[namespace+revisionDigest]; ok {
		cur.usedBy = append(cur.usedBy, owner)
		revisions[namespace+revisionDigest] = cur
	} else {
		revisions[namespace+revisionDigest] = revision{
			namespace:      namespace,
			revisionDigest: revisionDigest,
			usedBy:         []string{owner},
		}
	}
}
