package telemetry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/utils"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
)

func TriggerPublishingEvent(targetDirectory, result, registryName string) (string, error) {
	workspace := environment.GetWorkspace()
	path := filepath.Join(workspace, targetDirectory)

	var packageVersion string
	return packageVersion, Track(context.Background(), shared.InteractionTypePublish, func(ctx context.Context, event *shared.CliEvent) error {
		if registryName != "" {
			event.PublishPackageRegistryName = &registryName
		}

		loadedCfg, err := config.Load(path)
		if err != nil {
			return err
		}

		if loadedCfg.LockFile == nil {
			return fmt.Errorf("empty lock file for python language target in directory %s", path)
		}

		version := processLockFile(*loadedCfg.LockFile, event)
		packageVersion = version

		var processingErr error
		switch registryName {
		case "pypi":
			processingErr = processPyPI(loadedCfg, event, path, version)
		case "npm":
			processingErr = processNPM(loadedCfg, event, path, version)
		case "packagist":
			processingErr = processPackagist(loadedCfg, event, path)
		case "nuget":
			processingErr = processNuget(loadedCfg, event, path, version)
		case "gems":
			processingErr = processGems(loadedCfg, event, path, version)
		case "sonatype":
			processingErr = processSonatype(loadedCfg, event, path, version)
		case "terraform":
			processingErr = processTerraform(loadedCfg, event, path, version)
		case "go":
			processingErr = processGo(loadedCfg, event, path, version)
		}

		if processingErr != nil {
			return processingErr
		}

		event.Success = strings.Contains(strings.ToLower(result), "success")

		return nil
	})
}

func processPyPI(cfg *config.Config, event *shared.CliEvent, path string, version string) error {
	lang := "python"
	if cfg.Config == nil {
		return fmt.Errorf("empty config for %s language target in directory %s", lang, path)
	}

	langCfg, ok := cfg.Config.Languages[lang]
	if !ok {
		return fmt.Errorf("no %s config in directory %s", lang, path)
	}

	event.GenerateTarget = &lang

	packageName := utils.GetPackageName(lang, &langCfg)
	event.PublishPackageName = &packageName

	if packageName != "" && version != "" {
		publishURL := fmt.Sprintf("https://pypi.org/project/%s/%s", packageName, version)
		event.PublishPackageURL = &publishURL
	}

	return nil
}

func processNPM(cfg *config.Config, event *shared.CliEvent, path string, version string) error {
	lang := "typescript"
	if cfg.Config == nil {
		return fmt.Errorf("empty config for %s language target in directory %s", lang, path)
	}

	langCfg, ok := cfg.Config.Languages[lang]
	if !ok {
		return fmt.Errorf("no %s config in directory %s", lang, path)
	}

	event.GenerateTarget = &lang

	packageName := utils.GetPackageName(lang, &langCfg)
	event.PublishPackageName = &packageName

	if packageName != "" && version != "" {
		publishURL := fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", packageName, version)
		event.PublishPackageURL = &publishURL
	}

	return nil
}

func processGo(cfg *config.Config, event *shared.CliEvent, path string, version string) error {
	lang := "go"
	if cfg.Config == nil {
		return fmt.Errorf("empty config for %s language target in directory %s", lang, path)
	}

	langCfg, ok := cfg.Config.Languages[lang]
	if !ok {
		return fmt.Errorf("no %s config in directory %s", lang, path)
	}

	event.GenerateTarget = &lang

	packageName := utils.GetPackageName(lang, &langCfg)
	event.PublishPackageName = &packageName

	if packageName != "" && version != "" {
		relPath, err := filepath.Rel(environment.GetWorkspace(), path)
		if err != nil {
			return err
		}

		tag := fmt.Sprintf("v%s", version)
		if relPath != "" && relPath != "." && relPath != "./" {
			tag = fmt.Sprintf("%s/%s", relPath, tag)
		}

		publishURL := fmt.Sprintf("https://github.com/%s/releases/tag/%s", os.Getenv("GITHUB_REPOSITORY"), tag)
		event.PublishPackageURL = &publishURL
	}

	return nil
}

func processPackagist(cfg *config.Config, event *shared.CliEvent, path string) error {
	lang := "php"
	if cfg.Config == nil {
		return fmt.Errorf("empty config for %s language target in directory %s", lang, path)
	}

	langCfg, ok := cfg.Config.Languages[lang]
	if !ok {
		return fmt.Errorf("no %s config in directory %s", lang, path)
	}

	event.GenerateTarget = &lang

	packageName := utils.GetPackageName(lang, &langCfg)
	event.PublishPackageName = &packageName

	if packageName != "" {
		publishURL := fmt.Sprintf("https://packagist.org/packages/%s", packageName)
		event.PublishPackageURL = &publishURL
	}

	return nil
}

func processNuget(cfg *config.Config, event *shared.CliEvent, path string, version string) error {
	lang := "csharp"
	if cfg.Config == nil {
		return fmt.Errorf("empty config for %s language target in directory %s", lang, path)
	}

	langCfg, ok := cfg.Config.Languages[lang]
	if !ok {
		return fmt.Errorf("no %s config in directory %s", lang, path)
	}

	event.GenerateTarget = &lang

	packageName := utils.GetPackageName(lang, &langCfg)
	event.PublishPackageName = &packageName

	if packageName != "" && version != "" {
		publishURL := fmt.Sprintf("https://www.nuget.org/packages/%s/%s", packageName, version)
		event.PublishPackageURL = &publishURL
	}

	return nil
}

func processGems(cfg *config.Config, event *shared.CliEvent, path string, version string) error {
	lang := "ruby"
	if cfg.Config == nil {
		return fmt.Errorf("empty config for %s language target in directory %s", lang, path)
	}

	langCfg, ok := cfg.Config.Languages[lang]
	if !ok {
		return fmt.Errorf("no %s config in directory %s", lang, path)
	}

	event.GenerateTarget = &lang

	packageName := utils.GetPackageName(lang, &langCfg)
	event.PublishPackageName = &packageName

	if packageName != "" && version != "" {
		publishURL := fmt.Sprintf("https://rubygems.org/gems/%s/%s", packageName, version)
		event.PublishPackageURL = &publishURL
	}

	return nil
}

func processSonatype(cfg *config.Config, event *shared.CliEvent, path string, version string) error {
	lang := "java"
	if cfg.Config == nil {
		return fmt.Errorf("empty config for %s language target in directory %s", lang, path)
	}

	langCfg, ok := cfg.Config.Languages[lang]
	if !ok {
		return fmt.Errorf("no %s config in directory %s", lang, path)
	}

	event.GenerateTarget = &lang

	var groupID string
	if name, ok := langCfg.Cfg["groupID"]; ok {
		if strName, ok := name.(string); ok {
			groupID = strName
		}
	}

	var artifactID string
	if name, ok := langCfg.Cfg["artifactID"]; ok {
		if strName, ok := name.(string); ok {
			artifactID = strName
		}
	}

	packageName := utils.GetPackageName(lang, &langCfg)
	event.PublishPackageName = &packageName

	if groupID != "" && artifactID != "" && version != "" {
		publishURL := fmt.Sprintf("https://central.sonatype.com/artifact/%s/%s/%s", groupID, artifactID, version)
		event.PublishPackageURL = &publishURL
	}

	return nil
}

func processTerraform(cfg *config.Config, event *shared.CliEvent, path string, version string) error {
	lang := "terraform"
	if cfg.Config == nil {
		return fmt.Errorf("empty config for %s language target in directory %s", lang, path)
	}

	langCfg, ok := cfg.Config.Languages[lang]
	if !ok {
		return fmt.Errorf("no %s config in directory %s", lang, path)
	}

	event.GenerateTarget = &lang

	var author string
	if name, ok := langCfg.Cfg["author"]; ok {
		if strName, ok := name.(string); ok {
			author = strName
		}
	}

	packageName := utils.GetPackageName(lang, &langCfg)
	event.PublishPackageName = &packageName

	if packageName != "" && author != "" && version != "" {
		publishURL := fmt.Sprintf("https://registry.terraform.io/providers/%s/%s/%s", author, packageName, version)
		event.PublishPackageURL = &publishURL
	}

	return nil
}

func processLockFile(lockFile config.LockFile, event *shared.CliEvent) string {
	if lockFile.ID != "" {
		event.GenerateGenLockID = &lockFile.ID
	}

	if lockFile.Management.ReleaseVersion != "" {
		event.PublishPackageVersion = &lockFile.Management.ReleaseVersion
	}

	if lockFile.Management.SpeakeasyVersion != "" {
		event.SpeakeasyVersion = lockFile.Management.SpeakeasyVersion
	}

	return lockFile.Management.ReleaseVersion
}
