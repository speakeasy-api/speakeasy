package ci

import (
	"context"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/ci/actions"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type tagFlags struct {
	RegistryTags string `json:"registry-tags"`
	Sources      string `json:"sources"`
	CodeSamples  string `json:"code-samples"`
	Debug        bool   `json:"debug"`
}

var tagCmd = &model.ExecutableCommand[tagFlags]{
	Usage: "tag",
	Short: "Tag registry images (used by CI/CD)",
	Long:  "Tags source and code sample registry images. Used by CI/CD after generation or release.",
	Run:   runTag,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "registry-tags",
			Description:  "Comma-separated registry tags to apply",
			DefaultValue: os.Getenv("INPUT_REGISTRY_TAGS"),
		},
		flag.StringFlag{
			Name:         "sources",
			Description:  "Comma-separated or newline-separated source names",
			DefaultValue: os.Getenv("INPUT_SOURCES"),
		},
		flag.StringFlag{
			Name:         "code-samples",
			Description:  "Comma-separated or newline-separated code sample target names",
			DefaultValue: os.Getenv("INPUT_CODE_SAMPLES"),
		},
		flag.BooleanFlag{
			Name:         "debug",
			Description:  "Enable debug mode",
			DefaultValue: os.Getenv("INPUT_DEBUG") == "true",
		},
	},
}

func runTag(ctx context.Context, flags tagFlags) error {
	setEnvIfNotEmpty("INPUT_REGISTRY_TAGS", flags.RegistryTags)
	setEnvIfNotEmpty("INPUT_SOURCES", flags.Sources)
	setEnvIfNotEmpty("INPUT_CODE_SAMPLES", flags.CodeSamples)
	setEnvBool("INPUT_DEBUG", flags.Debug)

	return actions.Tag(ctx)
}
