package cmd

import (
	"context"
	"fmt"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/pkg/overlay"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"
	"strings"
)

var overlayFlag = flag.StringFlag{
	Name:        "overlay",
	Shorthand:   "o",
	Description: "the overlay file to use",
	Required:    true,
}

const overlayLong = `# Overlay

Command group for working with OpenAPI Overlays.
`

var overlayCmd = &model.CommandGroup{
	Usage:    "overlay",
	Short:    "Work with OpenAPI Overlays",
	Long:     utils.RenderMarkdown(overlayLong),
	Commands: []model.Command{overlayCompareCmd, overlayValidateCmd, overlayApplyCmd},
}

type overlayValidateFlags struct {
	Overlay string `json:"overlay"`
}

var overlayValidateCmd = &model.ExecutableCommand[overlayValidateFlags]{
	Usage: "validate",
	Short: "Given an overlay, validate it according to the OpenAPI Overlay specification",
	Run:   runValidateOverlay,
	Flags: []flag.Flag{overlayFlag},
}

type overlayCompareFlags struct {
	Before string `json:"before"`
	After  string `json:"after"`
	Out    string `json:"out"`
}

var overlayCompareCmd = &model.ExecutableCommand[overlayCompareFlags]{
	Usage: "compare",
	Short: "Given two specs (before and after), output an overlay that describes the differences between them",
	Run:   runCompare,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:                       "before",
			Shorthand:                  "b",
			Description:                "the before schema to compare",
			Required:                   true,
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		flag.StringFlag{
			Name:                       "after",
			Shorthand:                  "a",
			Description:                "the after schema to compare",
			Required:                   true,
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		flag.StringFlag{
			Name:        "out",
			Description: "write directly to a file instead of stdout",
		},
	},
}

type overlayApplyFlags struct {
	Overlay string `json:"overlay"`
	Schema  string `json:"schema"`
	Strict  bool   `json:"strict"`
	Out     string `json:"out"`
}

var overlayApplyCmd = &model.ExecutableCommand[overlayApplyFlags]{
	Usage: "apply",
	Short: "Given an overlay, construct a new specification by extending a specification and applying the overlay, and output it to stdout.",
	Run:   runApply,
	Flags: []flag.Flag{
		overlayFlag,
		flag.StringFlag{
			Name:                       "schema",
			Shorthand:                  "s",
			Description:                "the schema to extend",
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		flag.BooleanFlag{
			Name:        "strict",
			Description: "fail if the overlay has any action target expressions which match no nodes, and produce warnings if any overlay actions do nothing",
		},
		flag.StringFlag{
			Name:        "out",
			Description: "write directly to a file instead of stdout",
		},
	},
}

func runValidateOverlay(ctx context.Context, flags overlayValidateFlags) error {
	if err := overlay.Validate(flags.Overlay); err != nil {
		return err
	}

	log.From(ctx).Successf("Overlay file %q is valid.", flags.Overlay)
	return nil
}

func runCompare(ctx context.Context, flags overlayCompareFlags) error {
	out := os.Stdout

	if flags.Out != "" {
		file, err := os.Create(flags.Out)
		if err != nil {
			return err
		}
		defer file.Close()
		out = file
	}

	schemas := []string{flags.Before, flags.After}
	summary, err := overlay.Compare(schemas, out)
	if err != nil {
		return err
	}

	// Only print summary information if we aren't writing the overlay to stdout
	if flags.Out != "" {
		printSummary(ctx, summary)

		msg := styles.RenderSuccessMessage(
			"Overlay Generated Successfully",
			fmt.Sprintf("Comparing ^%s^ to ^%s^", flags.Before, flags.After),
			fmt.Sprintf("Differences: `%d`", len(summary.TargetToChangeType)),
			fmt.Sprintf("Overlay written to: `%s`", flags.Out),
		)
		log.From(ctx).Println(msg)
	}

	return nil
}

func runApply(ctx context.Context, flags overlayApplyFlags) error {
	out := os.Stdout
	yamlOut := true

	if flags.Out != "" {
		file, err := os.Create(flags.Out)
		if err != nil {
			return err
		}
		defer file.Close()
		out = file

		yamlOut = utils.HasYAMLExt(flags.Out)
	}

	shouldWarn := len(flags.Out) > 0 && flags.Strict
	summary, err := overlay.Apply(flags.Schema, flags.Overlay, yamlOut, out, flags.Strict, shouldWarn)
	if err != nil {
		return err
	}

	// Only print summary information if we aren't writing the result to stdout
	if flags.Out != "" {
		printSummary(ctx, summary)

		msg := styles.RenderSuccessMessage(
			"Overlay Applied Successfully",
			fmt.Sprintf("Overlay ^%s^ applied to ^%s^", flags.Overlay, flags.Schema),
			fmt.Sprintf("Actions applied: `%d`", len(summary.TargetToChangeType)),
			fmt.Sprintf("Output written to: `%s`", flags.Out),
		)
		log.From(ctx).Println(msg)
	}

	return nil
}

func printSummary(ctx context.Context, summary *overlay.Summary) {
	logger := log.From(ctx)

	maxLines := 10
	var lines []string
	for target, changeType := range summary.TargetToChangeType {
		if len(lines) >= maxLines {
			break
		}

		targetStr := formatTargetPath(target)
		changeTypeStr := "🔀"
		if changeType == overlay.Remove {
			changeTypeStr = "❌"
		}
		action := fmt.Sprintf("%s %s", changeTypeStr, targetStr)
		lines = append(lines, action)
	}

	logger.Println(styles.Info.Underline(true).Bold(true).Render("OpenAPI Paths 🔀Changed and ❌Removed"))
	for _, line := range lines {
		logger.Println(line)
	}

	if len(summary.TargetToChangeType) > maxLines {
		logger.Println(styles.DimmedItalic.Render(fmt.Sprintf("(showing %d of %d)\n", maxLines, len(summary.TargetToChangeType))))
	}
}

func formatTargetPath(target string) string {
	// Remove leading $ if present
	if len(target) > 0 && target[0] == '$' {
		target = target[1:]
	}

	// Remove all [ and ] characters
	target = strings.ReplaceAll(target, "[", ".")
	target = strings.ReplaceAll(target, "]", "")

	// Remove quotes
	target = strings.ReplaceAll(target, "\"", "")

	// Remove leading dot if present
	if len(target) > 0 && target[0] == '.' {
		target = target[1:]
	}

	parts := strings.Split(target, ".")

	for i, part := range parts {
		titleCased := cases.Title(language.English).String(part)
		parts[i] = styles.MakeBold(titleCased)
	}

	return strings.Join(parts, styles.Dimmed.Render(" > "))
}
