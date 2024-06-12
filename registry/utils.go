package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func ResolveSpeakeasyRegistryBundle(ctx context.Context, d workflow.Document, outPath string) (*download.DownloadedRegistryOpenAPIBundle, error) {
	log.From(ctx).Infof("Downloading bundle %s... to %s\n", d.Location, outPath)

	if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
		return nil, err
	}

	workspaceSlug := core.GetWorkspaceSlugFromContext(ctx)
	organizationSlug := core.GetOrgSlugFromContext(ctx)
	if workspaceSlug == "" || organizationSlug == "" {
		return nil, fmt.Errorf("unable to use speakeasy registry reference without authenticating")
	}

	registryBreakdown := workflow.ParseSpeakeasyRegistryReference(d.Location)
	if registryBreakdown == nil {
		return nil, fmt.Errorf("failed to parse speakeasy registry reference %s", d.Location)
	}

	if registryBreakdown.OrganizationSlug != organizationSlug {
		return nil, fmt.Errorf("organization mismatch: %s != %s", registryBreakdown.OrganizationSlug, organizationSlug)
	}

	if registryBreakdown.WorkspaceSlug != workspaceSlug {
		return nil, fmt.Errorf("workspace mismatch: %s != %s", registryBreakdown.WorkspaceSlug, workspaceSlug)
	}

	return download.DownloadRegistryOpenAPIBundle(ctx, registryBreakdown.NamespaceName, registryBreakdown.Reference, outPath)
}

func IsRegistryEnabled(ctx context.Context) bool {
	hasSkipSchemaRegistry, _ := core.HasWorkspaceFeatureFlag(ctx, "skip_schema_registry")
	telemetryDisabled := core.IsTelemetryDisabled(ctx)
	return !hasSkipSchemaRegistry && !telemetryDisabled
}
