package registry

import (
	"archive/zip"
	"bytes"
	"context"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/loader"
	"github.com/speakeasy-api/speakeasy-core/ocicommon"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"io"
	"strings"
)

func DownloadRegistryBundle(ctx context.Context, document workflow.SpeakeasyRegistryDocument) (*loader.OpenAPIBundleResult, *zip.Reader, error) {
	serverURL := auth.GetServerURL()
	insecurePublish := false
	if strings.HasPrefix(serverURL, "http://") {
		insecurePublish = true
	}
	reg := strings.TrimPrefix(serverURL, "http://")
	reg = strings.TrimPrefix(reg, "https://")

	apiKey := config.GetWorkspaceAPIKey(document.OrganizationSlug, document.WorkspaceSlug)
	if apiKey == "" {
		apiKey = config.GetSpeakeasyAPIKey()
	}

	workspaceID, err := auth.GetWorkspaceIDFromContext(ctx)
	if err != nil {
		return nil, nil, err
	}

	access := ocicommon.NewRepositoryAccess(apiKey, document.NamespaceName, ocicommon.RepositoryAccessOptions{
		Insecure: insecurePublish,
	})
	if (document.WorkspaceSlug != auth.GetWorkspaceSlugFromContext(ctx) || document.OrganizationSlug != auth.GetOrgSlugFromContext(ctx)) && workspaceID == "self" {
		access = ocicommon.NewRepositoryAccessAdmin(apiKey, document.NamespaceID, document.NamespaceName, ocicommon.RepositoryAccessOptions{
			Insecure: insecurePublish,
		})
	}

	bundleLoader := loader.NewLoader(loader.OCILoaderOptions{
		Registry: reg,
		Access:   access,
	})

	bundleResult, err := bundleLoader.LoadOpenAPIBundle(ctx, document.Reference)
	if err != nil {
		return nil, nil, err
	}

	defer bundleResult.Body.Close()

	buf, err := io.ReadAll(bundleResult.Body)
	if err != nil {
		return nil, nil, err
	}

	reader := bytes.NewReader(buf)
	zipReader, err := zip.NewReader(reader, int64(len(buf)))
	if err != nil {
		return nil, nil, err
	}

	return bundleResult, zipReader, nil
}
