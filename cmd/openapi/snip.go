package openapi

import (
	"context"
	"fmt"

	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/pkg/transform"
)

const snipLong = `Remove operations from an OpenAPI specification and clean up unused components.

This command can operate in two modes:

**Remove Mode (default):**
Removes the specified operations from the document.

**Keep Mode (--keep flag):**
Keeps only the specified operations and removes everything else.

## Specifying Operations

Operations can be specified in two ways:

1. **By Operation ID** (using --operationId):
   The operationId field from your OpenAPI spec

2. **By Path and Method** (using --operation):
   Format: path:METHOD
   Example: /users/{id}:DELETE

You can specify multiple operations by:
- Using the flag multiple times: --operation /users:GET --operation /users:POST
- Using comma-separated values: --operation /users:GET,/users:POST

## Examples

Remove specific operations by operation ID:
` + "```" + `
speakeasy openapi snip --schema ./spec.yaml --operationId deleteUser --operationId adminDebug
` + "```" + `

Remove operations by path and method:
` + "```" + `
speakeasy openapi snip --schema ./spec.yaml --operation /users/{id}:DELETE --operation /admin:GET
` + "```" + `

Keep only specified operations (remove everything else):
` + "```" + `
speakeasy openapi snip --schema ./spec.yaml --keep --operation /users:GET --operation /users:POST
` + "```" + `

Write to a file instead of stdout:
` + "```" + `
speakeasy openapi snip --schema ./spec.yaml --out ./public-spec.yaml --operation /internal:GET
` + "```" + `

Pipe to other commands:
` + "```" + `
speakeasy openapi snip --schema ./spec.yaml --operation /debug:GET | speakeasy openapi lint
` + "```"

type snipFlags struct {
	Schema       string   `json:"schema"`
	Out          string   `json:"out"`
	OperationIDs []string `json:"operationId"`
	Operations   []string `json:"operation"`
	Keep         bool     `json:"keep"`
}

var snipCmd = &model.ExecutableCommand[snipFlags]{
	Usage: "snip",
	Short: "Remove operations from an OpenAPI specification",
	Long:  utils.RenderMarkdown(snipLong),
	Run:   runSnip,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:                       "schema",
			Shorthand:                  "s",
			Description:                "the OpenAPI schema to process",
			Required:                   true,
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		flag.StringFlag{
			Name:        "out",
			Shorthand:   "o",
			Description: "write to a file instead of stdout",
		},
		flag.StringSliceFlag{
			Name:        "operationId",
			Description: "operation ID to process (can be comma-separated or repeated)",
		},
		flag.StringSliceFlag{
			Name:        "operation",
			Description: "operation as path:method to process (can be comma-separated or repeated)",
		},
		flag.BooleanFlag{
			Name:         "keep",
			Shorthand:    "k",
			Description:  "keep mode: keep specified operations and remove all others",
			DefaultValue: false,
		},
	},
}

func runSnip(ctx context.Context, flags snipFlags) error {
	// Validate that at least one operation is specified
	if len(flags.OperationIDs) == 0 && len(flags.Operations) == 0 {
		return fmt.Errorf("no operations specified; use --operationId or --operation flags")
	}

	// Combine all operation specifications into a single list
	var allOperations []string
	allOperations = append(allOperations, flags.OperationIDs...)
	allOperations = append(allOperations, flags.Operations...)

	// Setup output
	out, yamlOut, err := setupOutput(ctx, flags.Out)
	if err != nil {
		return err
	}
	defer out.Close()

	// Run the snip transform
	return transform.Snip(ctx, flags.Schema, allOperations, flags.Keep, yamlOut, out)
}
