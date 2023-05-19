package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

var docSiteRoot = "/docs/speakeasy-cli"

func GenerateDocs(cmd *cobra.Command, outDir string, docSiteLinks bool) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := GenerateDocs(c, outDir, docSiteLinks); err != nil {
			return err
		}
	}

	outFile := filepath.Join(outDir, getPath(cmd))

	doc, err := GenerateDoc(cmd, docSiteLinks)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outFile), os.ModePerm); err != nil {
		return err
	}

	if err := os.WriteFile(outFile, []byte(doc), os.ModePerm); err != nil {
		return err
	}

	return nil
}

func GenerateDoc(cmd *cobra.Command, docSiteLinks bool) (string, error) {
	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultHelpFlag()

	builder := &strings.Builder{}
	name := cmd.Name()

	builder.WriteString(fmt.Sprintf("# %s  \n", name))
	builder.WriteString(fmt.Sprintf("`%s`  \n\n\n", cmd.CommandPath()))
	builder.WriteString(fmt.Sprintf("%s  \n\n", cmd.Short))
	if len(cmd.Long) > 0 {
		builder.WriteString("## Details\n\n")
		builder.WriteString(cmd.Long + "\n\n")
	}

	if cmd.Runnable() {
		builder.WriteString("## Usage\n\n")
		builder.WriteString(fmt.Sprintf("```\n%s\n```\n\n", cmd.UseLine()))
	}

	if len(cmd.Example) > 0 {
		builder.WriteString("### Examples\n\n")
		builder.WriteString(fmt.Sprintf("```\n%s\n```\n\n", cmd.Example))
	}

	if err := printOptions(builder, cmd); err != nil {
		return "", err
	}
	if cmd.HasParent() {
		builder.WriteString("### Parent Command\n\n")
		parent := cmd.Parent()

		link := ""
		if docSiteLinks {
			link = getDocSiteLink(parent)
		} else {
			link = "README.md"
			if cmd.HasAvailableSubCommands() {
				link = "../README.md"
			}
		}

		builder.WriteString(fmt.Sprintf("* [%s](%s)\t - %s\n", parent.CommandPath(), link, parent.Short))
	}

	children := cmd.Commands()

	if len(children) > 0 {
		builder.WriteString("### Sub Commands\n\n")
		slices.SortStableFunc(children, func(i, j *cobra.Command) bool {
			return i.Name() < j.Name()
		})

		for _, child := range children {
			if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
				continue
			}

			link := ""

			if docSiteLinks {
				link = getDocSiteLink(child)
			} else {
				link = fmt.Sprintf("%s.md", child.Name())
				if child.HasAvailableSubCommands() {
					link = fmt.Sprintf("%s/README.md", child.Name())
				}
			}

			builder.WriteString(fmt.Sprintf("* [%s](%s)\t - %s\n", child.CommandPath(), link, child.Short))
		}
	}

	return builder.String(), nil
}

func printOptions(builder *strings.Builder, cmd *cobra.Command) error {
	flags := cmd.NonInheritedFlags()
	flags.SetOutput(builder)
	if flags.HasAvailableFlags() {
		builder.WriteString("### Options\n\n```\n")
		flags.PrintDefaults()
		builder.WriteString("```\n\n")
	}

	parentFlags := cmd.InheritedFlags()
	parentFlags.SetOutput(builder)
	if parentFlags.HasAvailableFlags() {
		builder.WriteString("### Options inherited from parent commands\n\n```\n")
		parentFlags.PrintDefaults()
		builder.WriteString("```\n\n")
	}
	return nil
}

func getDocSiteLink(cmd *cobra.Command) string {
	fullPath := strings.TrimPrefix(strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()), " ")

	if strings.TrimSpace(fullPath) == "" {
		return docSiteRoot
	}

	return fmt.Sprintf("%s/%s", docSiteRoot, strings.ReplaceAll(fullPath, " ", "/"))
}

func getPath(cmd *cobra.Command) string {
	fullPath := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name())

	if cmd.HasAvailableSubCommands() {
		return strings.ReplaceAll(fullPath, " ", "/") + "/README.md"
	}

	return strings.ReplaceAll(fullPath, " ", "/") + ".md"
}
