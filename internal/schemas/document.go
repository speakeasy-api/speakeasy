package schemas

import (
	"context"
	"fmt"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/registry"
)

func LoadDocument(ctx context.Context, schemaLocation string) ([]byte, *libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	docPath, err := ResolveDocument(ctx, workflow.Document{Location: workflow.LocationString(schemaLocation)}, nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	return openapi.LoadDocument(ctx, docPath)
}

func ResolveDocument(ctx context.Context, d workflow.Document, outputLocation *string, step *workflowTracking.WorkflowStep) (string, error) {
	if d.IsSpeakeasyRegistry() {
		step.NewSubstep("Downloading registry bundle")
		if !registry.IsRegistryEnabled(ctx) {
			return "", fmt.Errorf("schema registry is not enabled for this workspace")
		}

		location := workflow.GetTempDir()
		if outputLocation != nil {
			location = *outputLocation
		}
		documentOut, err := registry.ResolveSpeakeasyRegistryBundle(ctx, d, location)
		if err != nil {
			return "", err
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
