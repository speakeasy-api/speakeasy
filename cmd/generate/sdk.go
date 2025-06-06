package generate

import (
	"context"
	"fmt"
	"strings"

	"github.com/speakeasy-api/speakeasy-core/events"

	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
)

type GenerateFlags struct {
	Lang            string `json:"lang"`
	SchemaPath      string `json:"schema"`
	OutDir          string `json:"out"`
	Header          string `json:"header"`
	Token           string `json:"token"`
	Debug           bool   `json:"debug"`
	Verbose         bool   `json:"verbose"`
	AutoYes         bool   `json:"auto-yes"`
	InstallationURL string `json:"installationURL"`
	Published       bool   `json:"published"`
	Repo            string `json:"repo"`
	RepoSubdir      string `json:"repo-subdir"`
	OutputTests     bool   `json:"output-tests"`
	Force           bool   `json:"force"`
}

var genSDKCmd = &model.ExecutableCommand[GenerateFlags]{
	Usage:        "sdk",
	Short:        fmt.Sprintf("Generating Client SDKs from OpenAPI specs (%s)", strings.Join(GeneratorSupportedTargetNames(), ", ")),
	Long:         generateLongDesc,
	Run:          genSDKs,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.EnumFlag{
			Name:          "lang",
			Shorthand:     "l",
			Required:      true,
			AllowedValues: GeneratorSupportedTargetNames(),
			Description:   fmt.Sprintf("language to generate sdk for (available options: [%s])", strings.Join(GeneratorSupportedTargetNames(), ", ")),
		},
		schemaFlag,
		outFlag,
		headerFlag,
		tokenFlag,
		debugFlag,
		flag.BooleanFlag{
			Name:         "verbose",
			DefaultValue: false,
			Description:  "Verbose output",
		},
		autoYesFlag,
		flag.StringFlag{
			Name:        "installationURL",
			Shorthand:   "i",
			Description: "the language specific installation URL for installation instructions if the SDK is not published to a package manager",
		},
		flag.BooleanFlag{
			Name:        "published",
			Shorthand:   "p",
			Description: "whether the SDK is published to a package manager or not, determines the type of installation instructions to generate",
		},
		repoFlag,
		repoSubdirFlag,
		flag.BooleanFlag{
			Name:        "output-tests",
			Shorthand:   "t",
			Description: "output internal tests for internal speakeasy use cases",
			Hidden:      true,
		},
		flag.BooleanFlag{
			Name:               "force",
			Description:        "Force generation of SDKs even when no changes are present",
			Deprecated:         true,
			DeprecationMessage: "as it is now the default behavior and will be removed in a future version",
		},
	},
	NonInteractiveSubcommands: []model.Command{genSDKVersionCmd, genSDKChangelogCmd},
}

func genSDKs(ctx context.Context, flags GenerateFlags) error {
	_, err := sdkgen.Generate(
		ctx,
		sdkgen.GenerateOptions{
			CustomerID:      config.GetCustomerID(),
			WorkspaceID:     config.GetWorkspaceID(),
			Language:        flags.Lang,
			SchemaPath:      flags.SchemaPath,
			Header:          flags.Header,
			Token:           flags.Token,
			OutDir:          flags.OutDir,
			CLIVersion:      events.GetSpeakeasyVersionFromContext(ctx),
			InstallationURL: flags.InstallationURL,
			Debug:           flags.Debug,
			AutoYes:         flags.AutoYes,
			Published:       flags.Published,
			OutputTests:     flags.OutputTests,
			Repo:            flags.Repo,
			RepoSubDir:      flags.RepoSubdir,
			Verbose:         flags.Verbose,
			Compile:         false,
			TargetName:      "",
			SkipVersioning:  false,
		},
	)

	return err
}

var generateLongDesc = fmt.Sprintf(`Using the "speakeasy generate sdk" command you can generate client SDK packages for various languages
that are ready to use and publish to your favorite package registry.

The following languages are currently supported:
	- %s

By default the command will generate a Go SDK, but you can specify a different language using the --lang flag.
It will also use generic defaults for things such as package name (openapi), etc.

# Configuration

To configure the package of the generated SDKs you can config a "gen.yaml" file in the root of the output directory.

Example gen.yaml file for Go SDK:

`+"```"+`
go:
  packageName: github.com/speakeasy-api/speakeasy-client-sdk-go
  version: 0.1.0
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasy.com 
`+"```"+`

Example gen.yaml file for Python SDK:

`+"```"+`
python:
  packageName: speakeasy-client-sdk-python
  version: 0.1.0
  description: Speakeasy API Client SDK for Python
  author: Speakeasy API
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasy.com 
`+"```"+`

Example gen.yaml file for Typescript SDK:

`+"```"+`
typescript:
  packageName: speakeasy-client-sdk-typescript
  version: 0.1.0
  author: Speakeasy API
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasy.com 
`+"```"+`

Example gen.yaml file for Java SDK:

`+"```"+`
java:
  groupID: dev.speakeasyapi
  artifactID: javasdk
  projectName: speakeasy-client-sdk-java
  version: 0.1.0
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasy.com 
`+"```"+`

Example gen.yaml file for PHP SDK:

`+"```"+`
php:
  packageName: speakeasy-client-sdk-php
  namespace: "speakeasyapi\\sdk"
  version: 0.1.0
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasy.com 
`+"```"+`

Example gen.yaml file for C# SDK:

`+"```"+`
csharp:
  version: 0.1.0
  author: Speakeasy
  maxMethodParams: 0
  packageName: SpeakeasySDK
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document
  baseServerUrl: https://api.speakeasy.com 
`+"```"+`

For additional documentation visit: https://speakeasy.com/docs/using-speakeasy/create-client-sdks/intro
`, strings.Join(GeneratorSupportedTargetNames(), "\n	- "))
