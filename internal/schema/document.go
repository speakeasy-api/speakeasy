package schema

import (
	"context"
	"errors"
	"fmt"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/registry"
)

func LoadDocument(ctx context.Context, location string) ([]byte, *libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	path, err := ResolveDocument(ctx, workflow.Document{Location: location}, nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	_, schemaContents, _ := GetSchemaContents(ctx, path, "", "")
	doc, err := libopenapi.NewDocumentWithConfiguration(schemaContents, getConfig())
	if err != nil {
		return schemaContents, nil, nil, err
	}

	v3Model, errs := doc.BuildV3Model()
	if errs != nil {
		return schemaContents, &doc, v3Model, errors.Join(errs...)
	}

	return schemaContents, &doc, v3Model, err
}

func getConfig() *datamodel.DocumentConfiguration {
	return &datamodel.DocumentConfiguration{
		AllowRemoteReferences:               true,
		IgnorePolymorphicCircularReferences: true,
		IgnoreArrayCircularReferences:       true,
		ExtractRefsSequentially:             true,
	}
}

func ResolveDocument(ctx context.Context, d workflow.Document, outputLocation *string, step *workflowTracking.WorkflowStep) (string, error) {
	if d.IsSpeakeasyRegistry() {
		step.NewSubstep("Downloading registry bundle")
		if !registry.IsRegistryEnabled(ctx) {
			return "", fmt.Errorf("schema registry is not enabled for this workspace")
		}

		location := d.GetTempRegistryDir(workflow.GetTempDir())
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

	return d.Location, nil
}
