package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func ResolveSpeakeasyRegistryBundle(ctx context.Context, d workflow.Document, outPath string) (string, error) {
	log.From(ctx).Infof("Downloading bundle %s... to %s\n", d.Location, outPath)
	hasSchemaRegistry, _ := auth.HasWorkspaceFeatureFlag(ctx, shared.FeatureFlagsSchemaRegistry)
	if !hasSchemaRegistry {
		return "", fmt.Errorf("schema registry is not enabled for this workspace")
	}

	if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
		return "", err
	}

	registryBreakdown := workflow.ParseSpeakeasyRegistryReference(d.Location)
	if registryBreakdown == nil {
		return "", fmt.Errorf("failed to parse speakeasy registry reference %s", d.Location)
	}

	return download.DownloadRegistryOpenAPIBundle(ctx, registryBreakdown.NamespaceID, registryBreakdown.Reference, outPath)
}
