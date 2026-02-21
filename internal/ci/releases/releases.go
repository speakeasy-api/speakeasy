package releases

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	version "github.com/hashicorp/go-version"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/speakeasy/internal/ci/utils"
)

type LanguageReleaseInfo struct {
	PackageName     string
	Path            string
	Version         string
	PreviousVersion string
	URL             string
}

type GenerationInfo struct {
	Version string
	Path    string
}

// TargetReleaseNotes maps workflow target name to their specific release content
type TargetReleaseNotes map[string]string

func (t TargetReleaseNotes) GetReleaseNotesForTarget(target string) string {
	if t == nil {
		return ""
	}
	return t[target]
}

func (t TargetReleaseNotes) HasReleaseNotesForTarget(target string) bool {
	if t == nil {
		return false
	}
	notes, exists := t[target]
	return exists && notes != ""
}

type ReleasesInfo struct {
	ReleaseTitle       string
	DocVersion         string
	SpeakeasyVersion   string
	GenerationVersion  string
	DocLocation        string
	Languages          map[string]LanguageReleaseInfo
	LanguagesGenerated map[string]GenerationInfo
}

func (l LanguageReleaseInfo) IsPrerelease() bool {
	logging.Info("version is %v ", l.Version)
	v, err := version.NewVersion(l.Version)
	if err != nil {
		logging.Error("error parsing version when deciding if it is a prerelease. Therefore assuming it is not a prerelease. Version is %v. Error details: %v", l.Version, err)
		return false
	}
	logging.Info("prerelease info from go lib %v", v.Prerelease())
	if v.Prerelease() != "" {
		// If a prerelease info was found it means its a prerelease
		return true
	}
	return false
}

// This representation is used when adding body to Github releases
func (r ReleasesInfo) String() string {
	generationOutput := []string{}
	releasesOutput := []string{}

	for lang, info := range r.LanguagesGenerated {
		generationOutput = append(generationOutput, fmt.Sprintf("- [%s v%s] %s", lang, info.Version, info.Path))
	}

	if len(generationOutput) > 0 {
		generationOutput = append([]string{"\n### Generated"}, generationOutput...)
	}

	for lang, info := range r.Languages {
		pkgID := ""
		pkgURL := ""
		switch lang {
		case "go":
			pkgID = "Go"
			repoPath := os.Getenv("GITHUB_REPOSITORY")
			tag := fmt.Sprintf("v%s", info.Version)
			if info.Path != "." {
				tag = fmt.Sprintf("%s/%s", info.Path, tag)
			}

			pkgURL = fmt.Sprintf("https://github.com/%s/releases/tag/%s", repoPath, tag)
		case "typescript":
			pkgID = "NPM"
			pkgURL = fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", info.PackageName, info.Version)
		case "python":
			pkgID = "PyPI"
			pkgURL = fmt.Sprintf("https://pypi.org/project/%s/%s", info.PackageName, info.Version)
		case "php":
			pkgID = "Composer"
			pkgURL = fmt.Sprintf("https://packagist.org/packages/%s#v%s", info.PackageName, info.Version)
		case "terraform":
			pkgID = "Terraform"
			pkgURL = fmt.Sprintf("https://registry.terraform.io/providers/%s/%s", info.PackageName, info.Version)
		case "java":
			pkgID = "Maven Central"
			lastDotIndex := strings.LastIndex(info.PackageName, ".")
			groupID := info.PackageName[:lastDotIndex]      // everything before last occurrence of '.'
			artifactID := info.PackageName[lastDotIndex+1:] // everything after last occurrence of '.'
			pkgURL = fmt.Sprintf("https://central.sonatype.com/artifact/%s/%s/%s", groupID, artifactID, info.Version)
		case "ruby":
			pkgID = "Ruby Gems"
			pkgURL = fmt.Sprintf("https://rubygems.org/gems/%s/versions/%s", info.PackageName, info.Version)
		case "csharp":
			pkgID = "NuGet"
			pkgURL = fmt.Sprintf("https://www.nuget.org/packages/%s/%s", info.PackageName, info.Version)
		case "swift":
			pkgID = "Swift Package Manager"
			repoPath := os.Getenv("GITHUB_REPOSITORY")

			tag := fmt.Sprintf("v%s", info.Version)
			if info.Path != "." {
				tag = fmt.Sprintf("%s/%s", info.Path, tag)
			}

			pkgURL = fmt.Sprintf("https://github.com/%s/releases/tag/%s", repoPath, tag)
		}

		if pkgID != "" {
			releasesOutput = append(releasesOutput, fmt.Sprintf("- [%s v%s] %s - %s", pkgID, info.Version, pkgURL, info.Path))
		}
	}

	if len(releasesOutput) > 0 {
		releasesOutput = append([]string{"\n### Releases"}, releasesOutput...)
	}

	return fmt.Sprintf(`%s## %s
### Changes
Based on:
- OpenAPI Doc %s %s
- Speakeasy CLI %s (%s) https://github.com/speakeasy-api/speakeasy%s%s`, "\n\n", r.ReleaseTitle, r.DocVersion, r.DocLocation, r.SpeakeasyVersion, r.GenerationVersion, strings.Join(generationOutput, "\n"), strings.Join(releasesOutput, "\n"))
}

func UpdateReleasesFile(releaseInfo ReleasesInfo, dir string) error {
	releasesPath := GetReleasesPath(dir)

	logging.Debug("Updating releases file at %s", releasesPath)

	f, err := os.OpenFile(releasesPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		logging.Error("error while opening file: %s", err.Error())
		return fmt.Errorf("error opening releases file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(releaseInfo.String())
	if err != nil {
		return fmt.Errorf("error writing to releases file: %w", err)
	}

	return nil
}

var (
	releaseInfoRegex        = regexp.MustCompile(`(?s)## (.*?)\n### Changes\nBased on:\n- OpenAPI Doc (.*?) (.*?)\n- Speakeasy CLI (.*?) (\((.*?)\))?.*?`)
	generatedLanguagesRegex = regexp.MustCompile(`- \[([a-z]+) v(\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?)] (.*)`)
	npmReleaseRegex         = regexp.MustCompile(`- \[NPM v(\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?)] (https:\/\/www\.npmjs\.com\/package\/(.*?)\/v\/\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?) - (.*)`)
	pypiReleaseRegex        = regexp.MustCompile(`- \[PyPI v(\d+\.\d+\.\d+(?:-?\w+(?:\.\w+)*)?)] (https:\/\/pypi\.org\/project\/(.*?)\/\d+\.\d+\.\d+(?:-?\w+(?:\.\w+)*)?) - (.*)`)
	goReleaseRegex          = regexp.MustCompile(`- \[Go v(\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?)] (https:\/\/(github.com\/.*?)\/releases\/tag\/.*?\/?v\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?) - (.*)`)
	composerReleaseRegex    = regexp.MustCompile(`- \[Composer v(\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?)] (https:\/\/packagist\.org\/packages\/(.*?)#v\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?) - (.*)`)
	mavenReleaseRegex       = regexp.MustCompile(`- \[Maven Central v(\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?)] (https:\/\/central\.sonatype\.com\/artifact\/(.*?)\/(.*?)\/.*?) - (.*)`)
	terraformReleaseRegex   = regexp.MustCompile(`- \[Terraform v(\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?)] (https:\/\/registry\.terraform\.io\/providers\/(.*?)\/(.*?)\/.*?) - (.*)`)
	rubyGemReleaseRegex     = regexp.MustCompile(`- \[Ruby Gems v(\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?)] (https:\/\/rubygems\.org\/gems\/(.*?)\/versions\/.*?) - (.*)`)
	nugetReleaseRegex       = regexp.MustCompile(`- \[NuGet v(\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?)] (https:\/\/www\.nuget\.org\/packages\/(.*?)\/\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?) - (.*)`)
	swiftReleaseRegex       = regexp.MustCompile(`- \[Swift Package Manager v(\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?)] (https:\/\/(github.com\/.*?)\/releases\/tag\/.*?\/?v\d+\.\d+\.\d+(?:-\w+(?:\.\w+)*)?) - (.*)`)
)

func GetLastReleaseInfo(dir string) (*ReleasesInfo, error) {
	releasesPath := GetReleasesPath(dir)

	logging.Debug("Reading releases file at %s", releasesPath)

	data, err := os.ReadFile(releasesPath)
	if err != nil {
		return nil, fmt.Errorf("error reading releases file: %w", err)
	}

	return ParseReleases(string(data))
}

func GetReleaseInfoFromGenerationFiles(path string) (*ReleasesInfo, error) {

	cfg, err := config.Load(filepath.Join(environment.GetWorkspace(), path))
	if err != nil {
		return nil, err
	}

	cfgFile := cfg.Config
	lockFile := cfg.LockFile
	logging.Info("lockfile release notes: %v", lockFile.ReleaseNotes)
	if cfgFile == nil || lockFile == nil {
		return nil, fmt.Errorf("config or lock file not found")
	}

	releaseInfo := ReleasesInfo{
		ReleaseTitle:       environment.GetInvokeTime().Format("2006-01-02 15:04:05"),
		DocVersion:         lockFile.Management.DocVersion,
		SpeakeasyVersion:   lockFile.Management.SpeakeasyVersion,
		GenerationVersion:  lockFile.Management.GenerationVersion,
		Languages:          map[string]LanguageReleaseInfo{},
		LanguagesGenerated: map[string]GenerationInfo{},
	}

	for lang, info := range cfgFile.Languages {
		releaseInfo.Languages[lang] = LanguageReleaseInfo{
			PackageName: utils.GetPackageName(lang, &info),
			Version:     lockFile.Management.ReleaseVersion,
			Path:        path,
		}

		releaseInfo.LanguagesGenerated[lang] = GenerationInfo{
			Version: lockFile.Management.ReleaseVersion,
			Path:    path,
		}
	}

	return &releaseInfo, nil
}

func GetTargetSpecificReleaseNotes(path string) (TargetReleaseNotes, error) {
	releaseInfoFromLockFile := make(TargetReleaseNotes)
	cfg, err := config.Load(filepath.Join(environment.GetWorkspace(), path))
	if err != nil {
		return nil, err
	}

	cfgFile := cfg.Config
	lockFile := cfg.LockFile
	logging.Info("lockfile release notes: %v", lockFile.ReleaseNotes)

	if cfgFile == nil || lockFile == nil {
		return nil, fmt.Errorf("config or lock file not found")
	}

	speakeasyVersion := lockFile.Management.SpeakeasyVersion
	for lang, info := range cfgFile.Languages {
		packageName := utils.GetPackageName(lang, &info)
		version := lockFile.Management.ReleaseVersion
		notes := ""
		partOfFirstLine := fmt.Sprintf("%s %s", utils.GetPackageName(lang, &info), lockFile.Management.ReleaseVersion)

		pkgURL := ""
		switch lang {
		case "go":
			repoPath := os.Getenv("GITHUB_REPOSITORY")
			tag := fmt.Sprintf("v%s", info.Version)
			if path != "." {
				tag = fmt.Sprintf("%s/%s", path, tag)
			}

			pkgURL = fmt.Sprintf("https://github.com/%s/releases/tag/%s", repoPath, tag)
		case "typescript":
			pkgURL = fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", packageName, version)
		case "python":
			pkgURL = fmt.Sprintf("https://pypi.org/project/%s/%s", packageName, version)
		case "php":
			pkgURL = fmt.Sprintf("https://packagist.org/packages/%s#v%s", packageName, version)
		case "terraform":
			pkgURL = fmt.Sprintf("https://registry.terraform.io/providers/%s/%s", packageName, version)
		case "java":
			lastDotIndex := strings.LastIndex(packageName, ".")
			groupID := packageName[:lastDotIndex]      // everything before last occurrence of '.'
			artifactID := packageName[lastDotIndex+1:] // everything after last occurrence of '.'
			pkgURL = fmt.Sprintf("https://central.sonatype.com/artifact/%s/%s/%s", groupID, artifactID, version)
		case "ruby":
			pkgURL = fmt.Sprintf("https://rubygems.org/gems/%s/versions/%s", packageName, version)
		case "csharp":
			pkgURL = fmt.Sprintf("https://www.nuget.org/packages/%s/%s", packageName, version)
		case "swift":
			repoPath := os.Getenv("GITHUB_REPOSITORY")

			tag := fmt.Sprintf("v%s", info.Version)
			if path != "." {
				tag = fmt.Sprintf("%s/%s", path, tag)
			}

			pkgURL = fmt.Sprintf("https://github.com/%s/releases/tag/%s", repoPath, tag)
		}

		firstLine := fmt.Sprintf("\n[%s](%s)", partOfFirstLine, pkgURL)
		notes += firstLine
		notes += "\n"
		notes += lockFile.ReleaseNotes
		notes += "\n"

		notes += fmt.Sprintf("Generated with [Speakeasy CLI %s](https://github.com/speakeasy-api/speakeasy/releases)\n", speakeasyVersion)

		if lockFile.ReleaseNotes == "" {
			releaseInfoFromLockFile[lang] = ""
		} else {
			releaseInfoFromLockFile[lang] = notes
		}
	}

	return releaseInfoFromLockFile, nil
}

func ParseReleases(data string) (*ReleasesInfo, error) {
	releases := strings.Split(data, "\n\n")

	lastRelease := releases[len(releases)-1]
	var previousRelease *string = nil
	if len(releases) > 1 {
		previousRelease = &releases[len(releases)-2]
	}

	matches := releaseInfoRegex.FindStringSubmatch(lastRelease)

	if len(matches) < 5 {
		return nil, fmt.Errorf("error parsing last release info")
	}

	var genVersion string
	if len(matches) == 7 {
		genVersion = matches[6]
	} else {
		genVersion = matches[4]
	}

	info := &ReleasesInfo{
		ReleaseTitle:       matches[1],
		DocVersion:         matches[2],
		DocLocation:        matches[3],
		SpeakeasyVersion:   matches[4],
		GenerationVersion:  genVersion,
		Languages:          map[string]LanguageReleaseInfo{},
		LanguagesGenerated: map[string]GenerationInfo{},
	}

	generatedMatches := generatedLanguagesRegex.FindAllStringSubmatch(lastRelease, -1)
	for _, subMatch := range generatedMatches {
		if len(subMatch) == 4 {
			info.LanguagesGenerated[subMatch[1]] = GenerationInfo{
				Version: subMatch[2],
				Path:    subMatch[3],
			}
		}
	}

	npmMatches := npmReleaseRegex.FindStringSubmatch(lastRelease)

	if len(npmMatches) == 5 {
		info.Languages["typescript"] = LanguageReleaseInfo{
			Version:     npmMatches[1],
			URL:         npmMatches[2],
			PackageName: npmMatches[3],
			Path:        npmMatches[4],
		}
	}

	pypiMatches := pypiReleaseRegex.FindStringSubmatch(lastRelease)

	if len(pypiMatches) == 5 {
		info.Languages["python"] = LanguageReleaseInfo{
			Version:     pypiMatches[1],
			URL:         pypiMatches[2],
			PackageName: pypiMatches[3],
			Path:        pypiMatches[4],
		}
	}

	goMatches := goReleaseRegex.FindStringSubmatch(lastRelease)

	if len(goMatches) == 5 {
		packageName := goMatches[3]
		path := goMatches[4]

		if path != "." {
			packageName = fmt.Sprintf("%s/%s", packageName, strings.TrimPrefix(path, "./"))
		}

		info.Languages["go"] = LanguageReleaseInfo{
			Version:     goMatches[1],
			URL:         goMatches[2],
			PackageName: packageName,
			Path:        path,
		}
	}

	composerMatches := composerReleaseRegex.FindStringSubmatch(lastRelease)

	if len(composerMatches) == 5 {
		info.Languages["php"] = LanguageReleaseInfo{
			Version:     composerMatches[1],
			URL:         composerMatches[2],
			PackageName: composerMatches[3],
			Path:        composerMatches[4],
		}
	}

	mavenMatches := mavenReleaseRegex.FindStringSubmatch(lastRelease)

	if len(mavenMatches) == 6 {
		groupID := mavenMatches[3]
		artifact := mavenMatches[4]
		info.Languages["java"] = LanguageReleaseInfo{
			Version:     mavenMatches[1],
			URL:         mavenMatches[2],
			PackageName: fmt.Sprintf(`%s.%s`, groupID, artifact),
			Path:        mavenMatches[5],
		}
	}

	terraformMatches := terraformReleaseRegex.FindStringSubmatch(lastRelease)
	if len(terraformMatches) == 6 {
		languageInfo := LanguageReleaseInfo{
			Version:     terraformMatches[1],
			URL:         terraformMatches[2],
			PackageName: fmt.Sprintf("%s/%s", terraformMatches[3], terraformMatches[4]),
			Path:        terraformMatches[5],
		}

		if previousRelease != nil {
			previousReleaseTerraform := terraformReleaseRegex.FindStringSubmatch(*previousRelease)
			if len(previousReleaseTerraform) == 6 {
				languageInfo.PreviousVersion = previousReleaseTerraform[1]
			}
		}
		info.Languages["terraform"] = languageInfo

	}
	rubyGemsMatches := rubyGemReleaseRegex.FindStringSubmatch(lastRelease)

	if len(rubyGemsMatches) == 5 {
		info.Languages["ruby"] = LanguageReleaseInfo{
			Version:     rubyGemsMatches[1],
			URL:         rubyGemsMatches[2],
			PackageName: rubyGemsMatches[3],
			Path:        rubyGemsMatches[4],
		}
	}

	nugetMatches := nugetReleaseRegex.FindStringSubmatch(lastRelease)

	if len(nugetMatches) == 5 {
		info.Languages["csharp"] = LanguageReleaseInfo{
			Version:     nugetMatches[1],
			URL:         nugetMatches[2],
			PackageName: nugetMatches[3],
			Path:        nugetMatches[4],
		}
	}

	swiftMatches := swiftReleaseRegex.FindStringSubmatch(lastRelease)

	if len(swiftMatches) == 5 {
		packageName := swiftMatches[3]
		path := swiftMatches[4]

		if path != "." {
			packageName = fmt.Sprintf("%s/%s", packageName, strings.TrimPrefix(path, "./"))
		}

		info.Languages["swift"] = LanguageReleaseInfo{
			Version:     swiftMatches[1],
			URL:         swiftMatches[2],
			PackageName: packageName,
			Path:        path,
		}
	}

	return info, nil
}

func GetReleasesPath(dir string) string {
	return path.Join(environment.GetWorkspace(), dir, "RELEASES.md")
}
