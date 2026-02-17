package merge

import (
	"context"
	"os"

	"github.com/speakeasy-api/openapi/openapi"
)

// MergeByResolvingLocalReferences bundles an OpenAPI document by resolving all local
// and remote $ref references into a single self-contained document.
func MergeByResolvingLocalReferences(ctx context.Context, inFile, outFile, basePath string, defaultRuleset, workingDir string, skipGenerateLintReport bool) error {
	f, err := os.Open(inFile)
	if err != nil {
		return err
	}
	defer f.Close()

	doc, _, err := openapi.Unmarshal(ctx, f, openapi.WithSkipValidation())
	if err != nil {
		return err
	}

	// Inline resolves all external references and replaces them with their content
	err = openapi.Inline(ctx, doc, openapi.InlineOptions{
		ResolveOptions: openapi.ResolveOptions{
			RootDocument:   doc,
			TargetLocation: inFile,
		},
		RemoveUnusedComponents: true,
	})
	if err != nil {
		return err
	}

	// Write the inlined document
	out, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer out.Close()

	return openapi.Marshal(ctx, doc, out)
}
