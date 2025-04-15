package generate

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/usagegen"
	"strings"
)

type GenerateUsageSnippetFlags struct {
	Lang       string `json:"lang"`
	SchemaPath string `json:"schema"`
	Header     string `json:"header"`
	Token      string `json:"token"`
	Operation  string `json:"operation-id"`
	Namespace  string `json:"namespace"`
	Out        string `json:"out"`
	ConfigPath string `json:"config-path"`
	All        bool   `json:"all"`
}

var genUsageSnippetCmd = &model.ExecutableCommand[GenerateUsageSnippetFlags]{
	Usage: "usage",
	Short: fmt.Sprintf("Generate standalone usage snippets for SDKs in (%s)", strings.Join(workflow.SupportedLanguagesUsageSnippets, ", ")),
	Long: fmt.Sprintf(`Using the "speakeasy generate usage" command you can generate usage snippets for various SDKs.

The following languages are currently supported:
	- %s

You can generate usage snippets by AffectedOperationIDs or by Namespace. By default this command will write to stdout.

You can also select to write to a file or write to a formatted output directory.
`, strings.Join(workflow.SupportedLanguagesUsageSnippets, "\n	- ")),
	Run: genUsageSnippets,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "lang",
			Shorthand:    "l",
			Description:  fmt.Sprintf("language to generate sdk for (available options: [%s])", strings.Join(workflow.SupportedLanguagesUsageSnippets, ", ")),
			DefaultValue: "go",
		},
		schemaFlag,
		headerFlag,
		tokenFlag,
		flag.StringFlag{
			Name:        "operation-id",
			Shorthand:   "i",
			Description: "The AffectedOperationIDs to generate usage snippet for",
		},
		flag.StringFlag{
			Name:        "namespace",
			Shorthand:   "n",
			Description: "The namespace to generate multiple usage snippets for. This could correspond to a tag or a x-speakeasy-group-name in your OpenAPI spec.",
		},
		flag.BooleanFlag{
			Name:        "all",
			Shorthand:   "a",
			Description: "Generate usage snippets for all operations. Overrides operation-id and namespace flags.",
		},
		flag.StringFlag{
			Name:      "out",
			Shorthand: "o",
			Description: `By default this command will write to stdout. If a filepath is provided results will be written into that file.
	If the path to an existing directory is provided, all results will be formatted into that directory with each operation getting its own sub folder.`,
		},
		flag.StringFlag{
			Name:         "config-path",
			Shorthand:    "c",
			DefaultValue: ".",
			Description:  "An optional argument to pass in the path to a directory that holds the gen.yaml configuration file.",
		},
	},
}

func genUsageSnippets(ctx context.Context, flags GenerateUsageSnippetFlags) error {
	return usagegen.Generate(
		ctx,
		config.GetCustomerID(),
		flags.Lang,
		flags.SchemaPath,
		flags.Header,
		flags.Token,
		flags.Out,
		flags.Operation,
		flags.Namespace,
		flags.ConfigPath,
		flags.All,
		nil,
		nil,
		nil,
	)
}
