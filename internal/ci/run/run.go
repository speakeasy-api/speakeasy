package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/go-github/v63/github"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/ci/runbridge"
	"github.com/speakeasy-api/speakeasy/internal/ci/utils"
	"github.com/speakeasy-api/speakeasy/internal/ci/versionbumps"
	"github.com/speakeasy-api/speakeasy/internal/ci/versioninfo"
	"github.com/speakeasy-api/versioning-reports/versioning"

	"github.com/speakeasy-api/sdk-gen-config/workflow"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
)

type LanguageGenInfo struct {
	PackageName string
	Version     string
}

type GenerationInfo struct {
	SpeakeasyVersion  string
	GenerationVersion string
	OpenAPIDocVersion string
	Languages         map[string]LanguageGenInfo
	HasTestingEnabled bool
}

type RunResult struct {
	GenInfo              *GenerationInfo
	OpenAPIChangeSummary string
	LintingReportURL     string
	ChangesReportURL     string
	VersioningReport     *versioning.MergedVersionReport
	VersioningInfo       versionbumps.VersioningInfo
	// key is language, value is release notes
	ReleaseNotes map[string]string
}

type Git interface {
	CheckDirDirty(dir string, ignoreMap map[string]string) (bool, string, error)
}

func Run(ctx context.Context, g Git, pr *github.PullRequest, wf *workflow.Workflow) (*RunResult, map[string]string, error) {
	workspace := environment.GetWorkspace()
	outputs := map[string]string{}
	releaseNotes := map[string]string{}

	speakeasyVersion := versioninfo.GetSpeakeasyVersion()
	generationVersion := versioninfo.GetGenerationVersion()

	langGenerated := map[string]bool{}

	globalPreviousGenVersion := ""

	langConfigs := map[string]*config.LanguageConfig{}

	installationURLs := map[string]string{}
	repoURL := getRepoURL()
	fmt.Println("INPUT_ENABLE_SDK_CHANGELOG: ", environment.GetSDKChangelog())
	repoSubdirectories := map[string]string{}
	previousManagementInfos := map[string]config.Management{}

	var manualVersioningBump *versioning.BumpType
	if versionBump := versionbumps.GetLabelBasedVersionBump(pr); versionBump != "" && versionBump != versioning.BumpNone {
		fmt.Println("Using label based version bump: ", versionBump)
		manualVersioningBump = &versionBump
	}

	getDirAndOutputDir := func(target workflow.Target) (string, string) {
		dir := "."
		if target.Output != nil {
			dir = *target.Output
		}

		dir = filepath.Join(environment.GetWorkingDirectory(), dir)
		return dir, path.Join(workspace, dir)
	}

	includesTerraform := false

	// Load initial configs
	for targetID, target := range wf.Targets {
		if environment.SpecifiedTarget() != "" && environment.SpecifiedTarget() != "all" && environment.SpecifiedTarget() != targetID {
			continue
		}

		lang := target.Target
		dir, outputDir := getDirAndOutputDir(target)

		// Load the config so we can get the current version information
		loadedCfg, err := config.Load(outputDir)
		if err != nil {
			return nil, outputs, err
		}
		previousManagementInfos[targetID] = loadedCfg.LockFile.Management

		globalPreviousGenVersion = getPreviousGenVersion(loadedCfg.LockFile, lang, globalPreviousGenVersion)

		fmt.Printf("Generating %s SDK in %s\n", lang, outputDir)

		installationURL := getInstallationURL(lang, dir)

		AddTargetPublishOutputs(target, outputs, &installationURL)

		if installationURL != "" {
			installationURLs[targetID] = installationURL
		}
		if dir != "." {
			repoSubdirectories[targetID] = filepath.Clean(dir)
		} else {
			repoSubdirectories[targetID] = ""
		}
		if lang == "terraform" {
			includesTerraform = true
		}
	}

	// Run the workflow
	var runRes *runbridge.RunResults
	var changereport *versioning.MergedVersionReport
	var err error

	runCtx := events.SetSpeakeasyVersionInContext(ctx, speakeasyVersion)
	changereport, runRes, err = versioning.WithVersionReportCapture(runCtx, func(ctx context.Context) (*runbridge.RunResults, error) {
		return runbridge.Run(ctx, len(wf.Targets) == 0, installationURLs, repoURL, repoSubdirectories, manualVersioningBump)
	})
	if err != nil {
		return nil, outputs, err
	}
	if len(changereport.Reports) == 0 {
		// Assume it's not yet enabled (e.g. CLI version too old)
		changereport = nil
	}
	if changereport != nil && !changereport.MustGenerate() && !environment.ForceGeneration() && pr == nil {
		// no further steps
		fmt.Printf("No changes that imply the need for us to automatically regenerate the SDK.\n  Use \"Force Generation\" if you want to force a new generation.\n  Changes would include:\n-----\n%s", changereport.GetMarkdownSection())
		return &RunResult{
			GenInfo: nil,
			VersioningInfo: versionbumps.VersioningInfo{
				VersionReport: changereport,
				ManualBump:    versionbumps.ManualBumpWasUsed(manualVersioningBump, changereport),
			},
			OpenAPIChangeSummary: runRes.OpenAPIChangeSummary,
			LintingReportURL:     runRes.LintingReportURL,
			ChangesReportURL:     runRes.ChangesReportURL,
		}, outputs, nil
	}

	// For terraform, we also trigger "go generate ./..." to regenerate docs
	if includesTerraform {
		if err = triggerGoGenerate(); err != nil {
			return nil, outputs, err
		}
	}

	hasTestingEnabled := false
	// Legacy logic: check for changes + dirty-check
	for targetID, target := range wf.Targets {
		if environment.SpecifiedTarget() != "" && environment.SpecifiedTarget() != "all" && environment.SpecifiedTarget() != targetID {
			continue
		}

		lang := target.Target
		dir, outputDir := getDirAndOutputDir(target)

		// Load the config again so we can compare the versions
		loadedCfg, err := config.Load(outputDir)
		if err != nil {
			return nil, outputs, err
		}
		currentManagementInfo := loadedCfg.LockFile.Management
		langCfg := loadedCfg.Config.Languages[lang]
		langConfigs[lang] = &langCfg
		if loadedCfg.LockFile.ReleaseNotes != "" {
			releaseNotes[lang] = loadedCfg.LockFile.ReleaseNotes
		}

		outputs[utils.OutputTargetDirectory(lang)] = dir

		previousManagementInfo := previousManagementInfos[targetID]
		dirty, dirtyMsg, err := g.CheckDirDirty(dir, map[string]string{
			previousManagementInfo.ReleaseVersion:    currentManagementInfo.ReleaseVersion,
			previousManagementInfo.GenerationVersion: currentManagementInfo.GenerationVersion,
			previousManagementInfo.ConfigChecksum:    currentManagementInfo.ConfigChecksum,
			previousManagementInfo.DocVersion:        currentManagementInfo.DocVersion,
			previousManagementInfo.DocChecksum:       currentManagementInfo.DocChecksum,
		})
		if err != nil {
			return nil, outputs, err
		}

		if dirty {
			target.IsPublished()
			hasTestingEnabled = true
			langGenerated[lang] = true
			// Set speakeasy version and generation version to what was used by the CLI
			if currentManagementInfo.SpeakeasyVersion != "" {
				speakeasyVersion = currentManagementInfo.SpeakeasyVersion
			}
			if currentManagementInfo.GenerationVersion != "" {
				generationVersion = currentManagementInfo.GenerationVersion
			}

			fmt.Printf("Regenerating %s SDK resulted in significant changes %s\n", lang, dirtyMsg)
		} else {
			fmt.Printf("Regenerating %s SDK did not result in any changes\n", lang)
		}
	}

	outputs["previous_gen_version"] = globalPreviousGenVersion

	regenerated := false

	langGenInfo := map[string]LanguageGenInfo{}

	for lang := range langGenerated {
		outputs[utils.OutputTargetRegenerated(lang)] = "true"

		langCfg := langConfigs[lang]

		langGenInfo[lang] = LanguageGenInfo{
			PackageName: utils.GetPackageName(lang, langCfg),
			Version:     langCfg.Version,
		}

		regenerated = true
	}

	var genInfo *GenerationInfo

	if regenerated {
		genInfo = &GenerationInfo{
			SpeakeasyVersion:  speakeasyVersion,
			GenerationVersion: generationVersion,
			// OpenAPIDocVersion: docVersion, //TODO
			Languages:         langGenInfo,
			HasTestingEnabled: hasTestingEnabled,
		}
	}

	return &RunResult{
		GenInfo: genInfo,
		VersioningInfo: versionbumps.VersioningInfo{
			VersionReport: changereport,
			ManualBump:    versionbumps.ManualBumpWasUsed(manualVersioningBump, changereport),
		},
		OpenAPIChangeSummary: runRes.OpenAPIChangeSummary,
		LintingReportURL:     runRes.LintingReportURL,
		ChangesReportURL:     runRes.ChangesReportURL,
		ReleaseNotes:         releaseNotes,
	}, outputs, nil
}

func getPreviousGenVersion(lockFile *config.LockFile, lang, globalPreviousGenVersion string) string {
	previousFeatureVersions, ok := lockFile.Features[lang]
	if !ok {
		return globalPreviousGenVersion
	}

	if globalPreviousGenVersion != "" {
		globalPreviousGenVersion += ";"
	}

	globalPreviousGenVersion += fmt.Sprintf("%s:", lang)

	previousFeatureParts := []string{}

	for feature, previousVersion := range previousFeatureVersions {
		previousFeatureParts = append(previousFeatureParts, fmt.Sprintf("%s,%s", feature, previousVersion))
	}

	globalPreviousGenVersion += strings.Join(previousFeatureParts, ",")

	return globalPreviousGenVersion
}

func getInstallationURL(lang, subdirectory string) string {
	subdirectory = filepath.Clean(subdirectory)

	switch lang {
	case "go":
		base := fmt.Sprintf("%s/%s", environment.GetGithubServerURL(), environment.GetRepo())

		if subdirectory == "." {
			return base
		}

		return base + "/" + subdirectory
	case "typescript":
		if subdirectory == "." {
			return fmt.Sprintf("%s/%s", environment.GetGithubServerURL(), environment.GetRepo())
		} else {
			return fmt.Sprintf("https://gitpkg.now.sh/%s/%s", environment.GetRepo(), subdirectory)
		}
	case "python":
		base := fmt.Sprintf("%s/%s.git", environment.GetGithubServerURL(), environment.GetRepo())

		if subdirectory == "." {
			return base
		}

		return base + "#subdirectory=" + subdirectory
	case "php":
		// PHP doesn't support subdirectories
		if subdirectory == "." {
			return fmt.Sprintf("%s/%s", environment.GetGithubServerURL(), environment.GetRepo())
		}
	case "ruby":
		base := fmt.Sprintf("%s/%s", environment.GetGithubServerURL(), environment.GetRepo())

		if subdirectory == "." {
			return base
		}

		return base + " -d " + subdirectory
	}

	// Neither Java nor C# support pulling directly from git
	return ""
}

func getRepoURL() string {
	return fmt.Sprintf("%s/%s.git", environment.GetGithubServerURL(), environment.GetRepo())
}

// triggerGoGenerate runs "go mod tidy" and "go generate ./..." for terraform targets.
func triggerGoGenerate() error {
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("error running go mod tidy: %w", err)
	}

	genCmd := exec.Command("go", "generate", "./...")
	genCmd.Stdout = os.Stdout
	genCmd.Stderr = os.Stderr
	if err := genCmd.Run(); err != nil {
		return fmt.Errorf("error running go generate: %w", err)
	}

	return nil
}

func AddTargetPublishOutputs(target workflow.Target, outputs map[string]string, installationURL *string) {
	lang := target.Target
	published := target.IsPublished() || lang == "go"

	// TODO: Temporary check to fix Java. We may remove this entirely, pending conversation
	if installationURL != nil && *installationURL == "" && lang != "java" {
		published = true // Treat as published if we don't have an installation URL
	}

	outputs[utils.OutputTargetPublish(lang)] = fmt.Sprintf("%t", published)

	if published && lang == "java" && target.Publishing != nil && target.Publishing.Java != nil {
		outputs["use_sonatype_legacy"] = strconv.FormatBool(target.Publishing.Java.UseSonatypeLegacy)
	}

	if lang == "python" && target.Publishing != nil && target.Publishing.PyPi != nil &&
		target.Publishing.PyPi.UseTrustedPublishing != nil && *target.Publishing.PyPi.UseTrustedPublishing {
		outputs["use_pypi_trusted_publishing"] = "true"
	}
}
