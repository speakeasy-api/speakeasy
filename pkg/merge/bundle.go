package merge

import (
	"context"
	"fmt"
	"os"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/bundler"
	"github.com/pb33f/libopenapi/datamodel"
)

// MergeByResolvingLocalReferences bundles an OpenAPI document by resolving all local
// and remote $ref references into a single self-contained document.
// This uses libopenapi for bundling functionality.
func MergeByResolvingLocalReferences(ctx context.Context, inFile, outFile, basePath string, defaultRuleset, workingDir string, skipGenerateLintReport bool) error {
	data, err := os.ReadFile(inFile)
	if err != nil {
		panic(fmt.Errorf("error reading file %s: %w", inFile, err))
	}

	config := datamodel.DocumentConfiguration{
		AllowFileReferences:   true,
		AllowRemoteReferences: true,
		BasePath:              basePath,
	}

	doc, err := libopenapi.NewDocumentWithConfiguration(data, &config)
	if err != nil {
		fmt.Printf("Error creating document: %v\n", err)
		os.Exit(1)
	}

	v3Model, errors := doc.BuildV3Model()
	if errors != nil {
		for _, err = range errors {
			fmt.Printf("Error building model: %v\n", err)
		}
		os.Exit(1)
	}

	bytes, e := bundler.BundleDocument(&v3Model.Model)
	if e != nil {
		panic(fmt.Errorf("bundling failed: %w", e))
	}

	err = os.WriteFile(outFile, bytes, 0o644)
	if err != nil {
		panic(fmt.Errorf("failed to write bundled file: %w", err))
	}

	return nil
}
