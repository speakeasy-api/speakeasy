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
	AutoYes         bool   `json:"auto-yes"`
	InstallationURL string `json:"installationURL"`
	Published       bool   `json:"published"`
	Repo            string `json:"repo"`
	RepoSubdir      string `json:"repo-subdir"`
	OutputTests     bool   `json:"output-tests"`
}

var genSDKCmd = &model.ExecutableCommand[GenerateFlags]{
	Usage:        "sdk",
	Short:        fmt.Sprintf("Generating Client SDKs from OpenAPI specs (%s)", strings.Join(SDKSupportedLanguageTargets(), ", ")),
	Long:         generateLongDesc,
	Run:          genSDKs,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "lang",
			Shorthand:    "l",
			DefaultValue: "go",
			Description:  fmt.Sprintf("language to generate sdk for (available options: [%s])", strings.Join(SDKSupportedLanguageTargets(), ", ")),
		},
		schemaFlag,
		outFlag,
		headerFlag,
		tokenFlag,
		debugFlag,
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
	},
	NonInteractiveSubcommands: []model.Command{genSDKVersionCmd, genSDKChangelogCmd},
}

func genSDKs(ctx context.Context, flags GenerateFlags) error {
	_, err := sdkgen.Generate(
		ctx,
		config.GetCustomerID(),
		config.GetWorkspaceID(),
		flags.Lang,
		flags.SchemaPath,
		flags.Header,
		flags.Token,
		flags.OutDir,
		events.GetSpeakeasyVersionFromContext(ctx),
		flags.InstallationURL,
		flags.Debug,
		flags.AutoYes,
		flags.Published,
		flags.OutputTests,
		flags.Repo,
		flags.RepoSubdir,
		false,
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
  baseServerUrl: https://api.speakeasyapi.dev 
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
  baseServerUrl: https://api.speakeasyapi.dev 
`+"```"+`

Example gen.yaml file for Typescript SDK:

`+"```"+`
typescript:
  packageName: speakeasy-client-sdk-typescript
  version: 0.1.0
  author: Speakeasy API
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
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
  baseServerUrl: https://api.speakeasyapi.dev 
`+"```"+`

Example gen.yaml file for PHP SDK:

`+"```"+`
php:
  packageName: speakeasy-client-sdk-php
  namespace: "speakeasyapi\\sdk"
  version: 0.1.0
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
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
  baseServerUrl: https://api.speakeasyapi.dev 
`+"```"+`

For additional documentation visit: https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/intro
`, strings.Join(SDKSupportedLanguageTargets(), "\n	- "))
