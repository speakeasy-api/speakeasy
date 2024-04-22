package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func ResolveSpeakeasyRegistryBundle(ctx context.Context, d workflow.Document, outPath string) (*download.DownloadedRegistryOpenAPIBundle, error) {
	log.From(ctx).Infof("Downloading bundle %s... to %s\n", d.Location, outPath)

	if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
		return nil, err
	}

	registryBreakdown := workflow.ParseSpeakeasyRegistryReference(d.Location)
	if registryBreakdown == nil {
		return nil, fmt.Errorf("failed to parse speakeasy registry reference %s", d.Location)
	}

	return download.DownloadRegistryOpenAPIBundle(ctx, registryBreakdown.NamespaceID, registryBreakdown.Reference, outPath)
}
