package schemas

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	coreOpenapi "github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/registry"
)

func LoadDocument(ctx context.Context, schemaLocation string) ([]byte, *openapi.OpenAPI, error) {
	docPath, err := ResolveDocument(ctx, workflow.Document{Location: workflow.LocationString(schemaLocation)}, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	return coreOpenapi.LoadDocument(ctx, docPath)
}

func ResolveDocument(ctx context.Context, d workflow.Document, outputLocation *string, step *workflowTracking.WorkflowStep) (string, error) {
	if d.IsSpeakeasyRegistry() {
		step.NewSubstep("Downloading registry bundle")
		if !registry.IsRegistryEnabled(ctx) {
			return "", fmt.Errorf("schema registry is not enabled for this workspace")
		}

		location := workflow.GetTempDir()
		documentOut, err := registry.ResolveSpeakeasyRegistryBundle(ctx, d, location)
		if err != nil {
			return "", err
		}

		// Note that workflows with inputs from the registry will not work with $refs to other files in the bundle
		if outputLocation != nil {
			// Copy actual document out of bundle over to outputLocation
			if err := utils.CopyFile(documentOut.LocalFilePath, *outputLocation); err != nil {
				return "", err
			}
		}

		return documentOut.LocalFilePath, nil
	} else if d.IsRemote() {
		step.NewSubstep("Downloading remote document")
		location := d.GetTempDownloadPath(workflow.GetTempDir())
		if outputLocation != nil {
			location = *outputLocation
		}

		documentOut, err := download.ResolveRemoteDocument(ctx, d, location)
		if err != nil {
			return "", err
		}

		return documentOut, nil
	}

	return d.Location.Resolve(), nil
}
