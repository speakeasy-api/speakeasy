package docs

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
)

var docSiteRoot = "/docs/speakeasy-reference/cli"

// regex to strip any ANSI color codes (for safety, optional)
var ansiEscape = regexp.MustCompile(`\x1b\\[[0-9;]*m`)

func stripAnsi(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

func GenerateDocs(cmd *cobra.Command, outDir string) error {
	cmd.DisableAutoGenTag = true
	return genDocs(cmd, outDir)
}

func genDocs(cmd *cobra.Command, outDir string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := genDocs(c, outDir); err != nil {
			return err
		}
	}

	outFile := filepath.Join(outDir, getPath(cmd))

	doc, err := genDoc(cmd)
	if err != nil {
		return err
	}

	if err := utils.CreateDirectory(outFile); err != nil {
		return err
	}

	if err := os.WriteFile(outFile, []byte(doc), 0o644); err != nil {
		return err
	}

	return nil
}

func genDoc(cmd *cobra.Command) (string, error) {
	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultHelpFlag()

	builder := &strings.Builder{}

	// ✅ Add frontmatter if this is an index.md page
	if strings.HasSuffix(getPath(cmd), "index.md") {
		builder.WriteString("---\nasIndexPage: true\n---\n\n")
	}

	name := cmd.Name()

	builder.WriteString(fmt.Sprintf("# %s  \n", name))
	builder.WriteString(fmt.Sprintf("`%s`  \n\n\n", cmd.CommandPath()))
	builder.WriteString(fmt.Sprintf("%s  \n\n", stripAnsi(cmd.Short)))

	if len(cmd.Long) > 0 {
		builder.WriteString("## Details\n\n")
		builder.WriteString(stripAnsi(cmd.Long) + "\n\n")
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
		link := getDocSiteLink(parent)
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

			link := getDocSiteLink(child)
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
		return docSiteRoot + "/getting-started"
	}

	return fmt.Sprintf("%s/%s", docSiteRoot, strings.ReplaceAll(fullPath, " ", "/"))
}

func getPath(cmd *cobra.Command) string {
	fullPath := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name())

	if cmd.HasAvailableSubCommands() {
		return strings.ReplaceAll(fullPath, " ", "/") + "/index.md"
	}

	return strings.ReplaceAll(fullPath, " ", "/") + ".md"
}
