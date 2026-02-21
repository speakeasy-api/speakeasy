package utils

import (
	"fmt"

	config "github.com/speakeasy-api/sdk-gen-config"
)

func GetPackageName(lang string, cfg *config.LanguageConfig) string {
	var packageName string
	switch lang {
	case "java":
		packageName = fmt.Sprintf("%s.%s", cfg.Cfg["groupID"], cfg.Cfg["artifactID"])
	case "terraform":
		packageName = fmt.Sprintf("%s/%s", cfg.Cfg["author"], cfg.Cfg["packageName"])
	default:
		packageName = fmt.Sprintf("%s", cfg.Cfg["packageName"])
	}

	return packageName
}

func GetRegistryName(lang string) string {
	var registryName string
	switch lang {
	case "python":
		registryName = "pypi"
	case "typescript":
		registryName = "npm"
	case "php":
		registryName = "packagist"
	case "csharp":
		registryName = "nuget"
	case "ruby":
		registryName = "gems"
	case "java":
		registryName = "sonatype"
	default:
		registryName = lang
	}
	return registryName
}
