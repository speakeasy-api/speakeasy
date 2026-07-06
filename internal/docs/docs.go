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

// h1Heading matches a top-level markdown heading line ("# Title").
var h1Heading = regexp.MustCompile(`^# `)

// codeFence matches the start/end of a fenced code block (``` or ~~~),
// ignoring any leading indentation.
var codeFence = regexp.MustCompile("^\\s*(```|~~~)")

// demoteHeadings demotes top-level markdown headings ("# ") in a command's Long
// description to second-level ("## "). Astro Starlight renders the frontmatter
// `title` as the page's single H1, so an additional H1 in the body produces a
// competing top-level heading. Headings inside fenced code blocks (e.g. shell
// comments in usage examples) are left untouched.
func demoteHeadings(s string) string {
	lines := strings.Split(s, "\n")
	inFence := false
	for i, line := range lines {
		if codeFence.MatchString(line) {
			inFence = !inFence
			continue
		}
		if !inFence && h1Heading.MatchString(line) {
			lines[i] = "#" + line
		}
	}
	return strings.Join(lines, "\n")
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

	doc := genDoc(cmd)

	if err := utils.CreateDirectory(outFile); err != nil {
		return err
	}

	if err := os.WriteFile(outFile, []byte(doc), 0o644); err != nil {
		return err
	}

	return nil
}

func genDoc(cmd *cobra.Command) string {
	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultHelpFlag()

	builder := &strings.Builder{}

	name := cmd.Name()

	// Astro Starlight's docsSchema requires a `title` in the frontmatter of every
	// page, and renders it as the page's H1 (so we omit a redundant `# name`
	// heading below). Commands with sub commands are emitted as index pages and
	// additionally set `asIndexPage: true`.
	builder.WriteString("---\n")
	fmt.Fprintf(builder, "title: %q\n", name)
	if strings.HasSuffix(getPath(cmd), "index.md") {
		builder.WriteString("asIndexPage: true\n")
	}
	builder.WriteString("---\n\n")

	fmt.Fprintf(builder, "`%s`  \n\n\n", cmd.CommandPath())
	fmt.Fprintf(builder, "%s  \n\n", stripAnsi(cmd.Short))

	if len(cmd.Long) > 0 {
		builder.WriteString("## Details\n\n")
		builder.WriteString(demoteHeadings(stripAnsi(cmd.Long)) + "\n\n")
	}

	if cmd.Runnable() {
		builder.WriteString("## Usage\n\n")
		fmt.Fprintf(builder, "```\n%s\n```\n\n", cmd.UseLine())
	}

	if len(cmd.Example) > 0 {
		builder.WriteString("### Examples\n\n")
		fmt.Fprintf(builder, "```\n%s\n```\n\n", cmd.Example)
	}

	printOptions(builder, cmd)

	if cmd.HasParent() {
		builder.WriteString("### Parent Command\n\n")
		parent := cmd.Parent()
		link := getDocSiteLink(parent)
		fmt.Fprintf(builder, "* [%s](%s)\t - %s\n", parent.CommandPath(), link, parent.Short)
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
			fmt.Fprintf(builder, "* [%s](%s)\t - %s\n", child.CommandPath(), link, child.Short)
		}
	}

	return builder.String()
}

func printOptions(builder *strings.Builder, cmd *cobra.Command) {
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
