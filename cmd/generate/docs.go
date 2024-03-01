package generate

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/docsgen"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"strings"
)

type GenerateSDKDocsFlags struct {
	Langs      string `json:"langs"`
	SchemaPath string `json:"schema"`
	OutDir     string `json:"out"`
	Header     string `json:"header"`
	Token      string `json:"token"`
	Debug      bool   `json:"debug"`
	AutoYes    bool   `json:"auto-yes"`
	Compile    bool   `json:"compile"`
	Repo       string `json:"repo"`
	RepoSubdir string `json:"repo-subdir"`
}

var genSDKDocsCmd = &model.ExecutableCommand[GenerateSDKDocsFlags]{
	Usage: "docs",
	Short: "Use this command to generate content for the SDK docs directory.",
	Long:  "Use this command to generate content for the SDK docs directory.",
	Run:   genSDKDocsContent,
	Flags: []flag.Flag{
		outFlag,
		schemaFlag,
		flag.StringFlag{
			Name:        "langs",
			Shorthand:   "l",
			Description: "a list of languages to include in SDK Docs generation. Example usage -l go,python,typescript",
		},
		headerFlag,
		tokenFlag,
		debugFlag,
		autoYesFlag,
		flag.BooleanFlag{
			Name:        "compile",
			Shorthand:   "c",
			Description: "automatically compile SDK docs content for a single page doc site",
		},
		repoFlag,
		repoSubdirFlag,
	},
}

func genSDKDocsContent(ctx context.Context, flags GenerateSDKDocsFlags) error {
	languages := make([]string, 0)
	if flags.Langs != "" {
		for _, lang := range strings.Split(flags.Langs, ",") {
			languages = append(languages, strings.TrimSpace(lang))
		}
	}

	return docsgen.GenerateContent(
		ctx,
		languages,
		config.GetCustomerID(),
		flags.SchemaPath,
		flags.Header,
		flags.Token,
		flags.OutDir,
		flags.Repo,
		flags.RepoSubdir,
		flags.Debug,
		flags.AutoYes,
		flags.Compile,
	)
}
