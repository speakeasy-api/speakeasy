package docs

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

var docSiteRoot = "/docs/speakeasy-cli"

func GenerateDocs(cmd *cobra.Command, outDir string, docSiteLinks bool) error {
	docosaurusPositioning := map[string]int{}

	if docSiteLinks {
		docosaurusPositioning = map[string]int{
			filepath.Join(outDir, "README.md"):  2,
			filepath.Join(outDir, "auth"):       3,
			filepath.Join(outDir, "validate"):   4,
			filepath.Join(outDir, "suggest.md"): 5,
			filepath.Join(outDir, "generate"):   6,
			filepath.Join(outDir, "merge.md"):   7,
			filepath.Join(outDir, "api"):        8,
			filepath.Join(outDir, "proxy.md"):   9,
			filepath.Join(outDir, "update.md"):  10,
			filepath.Join(outDir, "usage.md"):   11,
		}
	}

	return genDocs(cmd, outDir, docSiteLinks, docosaurusPositioning)
}

func genDocs(cmd *cobra.Command, outDir string, docSiteLinks bool, docosaurusPositioning map[string]int) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := genDocs(c, outDir, docSiteLinks, docosaurusPositioning); err != nil {
			return err
		}
	}

	outFile := filepath.Join(outDir, getPath(cmd))

	doc, err := genDoc(cmd, docSiteLinks)
	if err != nil {
		return err
	}

	dir := filepath.Dir(outFile)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	if pos, ok := docosaurusPositioning[dir]; ok {
		if err := os.WriteFile(filepath.Join(dir, "_category.json_"), []byte(fmt.Sprintf(`{"position": %02d}`, pos)), 0o644); err != nil {
			return err
		}
	}

	if pos, ok := docosaurusPositioning[outFile]; ok {
		doc = fmt.Sprintf(`---
sidebar_position: %d
---

`, pos) + doc
	}

	if err := os.WriteFile(outFile, []byte(doc), 0o644); err != nil {
		return err
	}

	return nil
}

func genDoc(cmd *cobra.Command, docSiteLinks bool) (string, error) {
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
		slices.SortStableFunc(children, func(i, j *cobra.Command) int {
			return cmp.Compare(i.Name(), j.Name())
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
