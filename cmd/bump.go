package cmd

import (
	"fmt"
	"os"
	"slices"

	"github.com/hashicorp/go-version"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/spf13/cobra"
)

type bumpType string

const (
	bumpPatch bumpType = "patch"
	bumpMinor bumpType = "minor"
	bumpMajor bumpType = "major"
)

var bumpCommand = &cobra.Command{
	Use:   "bump [patch|minor|major]",
	Short: "Bumps the version of a Speakeasy Generation Target",
	Long: `Bumps the version of a Speakeasy Generation Target, run within the target's directory. Allows the bumping of patch, minor, and major versions or setting to a specific version.

Examples:

- speakeasy bump patch - Bumps the target's version by one patch version
- speakeasy bump -v 1.2.3 - Sets the target's version to 1.2.3
- speakeasy bump major -t typescript - Bumps the typescript target's version by one major version
`,
	Args: cobra.RangeArgs(0, 1),
}

func bumpInit() {
	bumpCommand.Flags().StringP("target", "t", "", "The target to bump the version of, if more than one target is found in the gen.yaml")
	bumpCommand.Flags().StringP("version", "v", "", "The version to bump to, if you want to specify a specific version.")

	bumpCommand.RunE = bumpExec
	rootCmd.AddCommand(bumpCommand)
}

func bumpExec(cmd *cobra.Command, args []string) error {
	target, err := cmd.Flags().GetString("target")
	if err != nil {
		return err
	}

	specificVersion, err := cmd.Flags().GetString("version")
	if err != nil {
		return err
	}

	bumpTyp := ""

	if specificVersion == "" && len(args) > 0 {
		bumpTyp = args[0]
	}

	if (bumpTyp != "" || specificVersion == "") && !slices.Contains([]string{"patch", "minor", "major"}, bumpTyp) {
		return fmt.Errorf("bump type must be one of patch, minor, or major")
	} else if specificVersion != "" {
		if _, err := version.NewVersion(specificVersion); err != nil {
			return fmt.Errorf("specified version %s is not a valid semantic version: %w", specificVersion, err)
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(wd)
	if err != nil {
		return err
	}

	targets := []string{}
	for lang, langCfg := range cfg.Config.Languages {
		if langCfg.Version != "" {
			targets = append(targets, lang)
		}
	}

	slices.Sort(targets)

	if len(targets) == 0 {
		return fmt.Errorf("no targets found in the gen.yaml")
	}

	if target != "" && !slices.Contains(targets, target) {
		return fmt.Errorf("target %s not found in the gen.yaml", target)
	}

	if target == "" {
		target = targets[0]
	}

	if len(targets) > 1 {
		target, err = askForTarget("Select the target you want to bump", "We will bump the version of the selected target", "Lets bump your target's version", targets, false)
		if err != nil {
			return err
		}
	}

	langCfg := cfg.Config.Languages[target]

	if specificVersion != "" {
		langCfg.Version = specificVersion
	} else {
		currentVersionString := langCfg.Version

		v, err := version.NewVersion(currentVersionString)
		if err != nil {
			return fmt.Errorf("failed to parse version %s: %w", currentVersionString, err)
		}

		langCfg.Version = bump(v, bumpType(bumpTyp))
	}

	cfg.Config.Languages[target] = langCfg

	if err := config.SaveConfig(wd, cfg.Config); err != nil {
		return err
	}

	if bumpTyp != "" {
		bumpTyp = " " + bumpTyp
	}

	fmt.Printf("Bumped target %s's%s version to %s\n", target, bumpTyp, langCfg.Version)

	return nil
}

func bump(v *version.Version, bumpType bumpType) string {
	major := v.Segments()[0]
	minor := v.Segments()[1]
	patch := v.Segments()[2]

	switch bumpType {
	case bumpMajor:
		major++
		minor = 0
		patch = 0
	case bumpMinor:
		minor++
		patch = 0
	case bumpPatch:
		patch++
	}

	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}
