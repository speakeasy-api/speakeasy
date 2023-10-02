package cmd

import (
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/docsgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Use this command to generate content, compile, and publish SDK docs.",
	Long:  "Use this command to generate content, compile, and publish SDK docs. This feature is currently in closed beta, please reach out to speakeasy for more information.",
	RunE:  utils.InteractiveRunFn("Provide a docs sub command"),
}

var genDocsContentCmd = &cobra.Command{
	Use:   "content",
	Short: "Use this command to generate content for the SDK docs directory.",
	Long:  "Use this command to generate content for the SDK docs directory.",
	RunE:  genSDKDocsContent,
}

func docsInit() {
	rootCmd.AddCommand(docsCmd)
	// SDK Docs Content flags.
	genDocsContentCmd.Flags().StringP("out", "o", "", "path to the output directory")
	genDocsContentCmd.MarkFlagRequired("out")
	genDocsContentCmd.Flags().StringP("schema", "s", "./openapi.yaml", "local filepath or URL for the OpenAPI schema")
	genDocsContentCmd.MarkFlagRequired("schema")
	genDocsContentCmd.Flags().StringP("langs", "l", "", "a list of languages to include in SDK Doc generation")
	genDocsContentCmd.Flags().StringP("header", "H", "", "header key to use if authentication is required for downloading schema from remote URL")
	genDocsContentCmd.Flags().String("token", "", "token value to use if authentication is required for downloading schema from remote URL")
	genDocsContentCmd.Flags().BoolP("debug", "d", false, "enable writing debug files with broken code")
	genDocsContentCmd.Flags().BoolP("auto-yes", "y", false, "auto answer yes to all prompts")
	genDocsContentCmd.Flags().StringP("repo", "r", "", "the repository URL for the SDK Docs repo")
	genDocsContentCmd.Flags().StringP("repo-subdir", "b", "", "the subdirectory of the repository where the SDK Docs are located in the repo, helps with documentation generation")
	docsCmd.AddCommand(genDocsContentCmd)
}

func genSDKDocsContent(cmd *cobra.Command, args []string) error {
	languages := make([]string, 0)
	langInput, _ := cmd.Flags().GetString("langs")
	if langInput != "" {
		for _, lang := range strings.Split(langInput, ",") {
			languages = append(languages, strings.TrimSpace(lang))
		}
	}

	schemaPath, err := cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	header, err := cmd.Flags().GetString("header")
	if err != nil {
		return err
	}

	token, err := cmd.Flags().GetString("token")
	if err != nil {
		return err
	}

	outDir, err := cmd.Flags().GetString("out")
	if err != nil {
		return err
	}

	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return err
	}

	autoYes, err := cmd.Flags().GetBool("auto-yes")
	if err != nil {
		return err
	}

	repo, err := cmd.Flags().GetString("repo")
	if err != nil {
		return err
	}

	repoSubdir, err := cmd.Flags().GetString("repo-subdir")
	if err != nil {
		return err
	}

	if err := docsgen.GenerateContent(cmd.Context(), languages, config.GetCustomerID(), schemaPath, header, token, outDir, repo, repoSubdir, debug, autoYes); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	return nil
}
