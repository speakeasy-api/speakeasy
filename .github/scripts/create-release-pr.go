package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

type PR struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	URL    string   `json:"url"`
	Body   string   `json:"body"`
	Labels []Label  `json:"labels"`
}

type Label struct {
	Name string `json:"name"`
}

type PRFile struct {
	Path string `json:"path"`
}

func main() {
	// Get repo root (script is in .github/scripts/, repo root is 2 levels up)
	// First try to get the directory of the source file if available
	_, filename, _, ok := runtime.Caller(0)
	var scriptDir string
	if ok {
		scriptDir = filepath.Dir(filename)
	} else {
		// Fallback to working directory
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
			os.Exit(1)
		}
		scriptDir = wd
	}

	// If we're in .github/scripts/, go up 2 levels
	if strings.Contains(scriptDir, ".github/scripts") || filepath.Base(scriptDir) == "scripts" {
		scriptDir = filepath.Join(scriptDir, "../..")
	}

	repoRoot, err := filepath.Abs(scriptDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting repo root: %v\n", err)
		os.Exit(1)
	}

	if err := os.Chdir(repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error changing directory: %v\n", err)
		os.Exit(1)
	}

	// Set up environment
	os.Setenv("GOPRIVATE", "github.com/speakeasy-api/*")
	if os.Getenv("GH_TOKEN") == "" {
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			os.Setenv("GH_TOKEN", token)
		}
	}

	// Ensure we're on main branch
	runCmdIgnoreError("git", "checkout", "main")
	runCmdIgnoreError("git", "pull", "origin", "main")

	// Get current version
	currentVersion := getCurrentVersion()
	fmt.Printf("Current version: %s\n", currentVersion)

	// Get current openapi-generation version from go.mod
	currentOpenAPIVersion := getCurrentOpenAPIVersion()
	fmt.Printf("Current openapi-generation version: %s\n", currentOpenAPIVersion)

	// Get start date from the release
	startDate := execCmd("gh", "release", "view", fmt.Sprintf("v%s", currentOpenAPIVersion),
		"--repo", "speakeasy-api/openapi-generation",
		"--json", "createdAt", "-q", ".createdAt")
	if startDate == "" {
		fmt.Printf("Could not find release v%s, exiting\n", currentOpenAPIVersion)
		return
	}

	// Get latest openapi-generation version
	latestTag := execCmd("gh", "release", "list", "--limit", "1",
		"--repo", "speakeasy-api/openapi-generation",
		"--json", "tagName", "-q", ".[0].tagName")
	latestOpenAPIVersion := strings.TrimPrefix(latestTag, "v")
	fmt.Printf("Latest openapi-generation version: %s\n", latestOpenAPIVersion)

	// Check if there's a version difference using semver.bash
	semverChange := execCmd(filepath.Join(repoRoot, "scripts", "semver.bash"),
		"diff", currentOpenAPIVersion, latestOpenAPIVersion)
	if semverChange == "" || semverChange == "none" {
		fmt.Println("No semver change detected, exiting")
		return
	}

	fmt.Printf("Semver change detected: %s\n", semverChange)

	// Get merged PRs since START_DATE
	fmt.Printf("Fetching merged PRs since %s...\n", startDate)
	prsJSON := execCmd("gh", "pr", "list",
		"--repo", "speakeasy-api/openapi-generation",
		"--state", "merged",
		"--search", fmt.Sprintf("merged:>%s", startDate),
		"--json", "number,title,url,body,labels",
		"--limit", "100")

	var prs []PR
	if err := json.Unmarshal([]byte(prsJSON), &prs); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing PRs JSON: %v\n", err)
		os.Exit(1)
	}

	if len(prs) == 0 {
		fmt.Println("No merged PRs found, exiting")
		return
	}

	fmt.Printf("Found %d merged PRs\n", len(prs))

	// Filter out internal PRs and group by language
	langPRs := make(map[string][]string)
	corePRs := []string{}

	for _, pr := range prs {
		// Check if internal
		if isInternalPR(repoRoot, pr.Number, pr.Title) {
			fmt.Printf("Skipping internal PR: #%d - %s\n", pr.Number, pr.Title)
			continue
		}

		// Extract language from labels or title
		lang := extractLanguage(pr)

		// Check files for language hints if lang still unknown
		if lang == "" {
			lang = extractLanguageFromFiles(repoRoot, pr.Number)
		}

		// Create user-facing summary
		summary := pr.Title
		if pr.Body != "" && pr.Body != "null" {
			firstLine := extractFirstLine(pr.Body)
			if len(firstLine) < 200 {
				summary = firstLine
			}
		}

		prEntry := fmt.Sprintf("- [%s](%s)", summary, pr.URL)

		if lang != "" {
			langPRs[lang] = append(langPRs[lang], prEntry)
		} else {
			corePRs = append(corePRs, prEntry)
		}
	}

	// Calculate new version
	bumpedVersion := execCmd(filepath.Join(repoRoot, "scripts", "semver.bash"),
		"bump", semverChange, currentVersion)
	fmt.Printf("Bumped version: %s\n", bumpedVersion)

	// Check if a release PR already exists
	branchName := fmt.Sprintf("release/v%s", bumpedVersion)
	existingPR := execCmdIgnoreError("gh", "pr", "list",
		"--head", branchName,
		"--state", "open",
		"--json", "number", "-q", ".[0].number")

	if existingPR != "" {
		fmt.Printf("Release PR #%s already exists for %s, exiting\n", existingPR, branchName)
		return
	}

	// Create branch
	runCmdIgnoreError("git", "checkout", "-b", branchName)
	runCmdIgnoreError("git", "checkout", branchName)

	// Update go.mod
	runCmd("go", "get", "-v", fmt.Sprintf("github.com/speakeasy-api/openapi-generation/v2@v%s", latestOpenAPIVersion))
	runCmd("go", "mod", "tidy")

	// Check if there are changes
	runCmdIgnoreError("git", "diff", "--quiet", "go.mod", "go.sum")
	if exitCode := getLastExitCode(); exitCode == 0 {
		fmt.Println("No changes to go.mod/go.sum, exiting")
		return
	}

	// Build PR title
	prTitle := buildPRTitle(langPRs, corePRs)

	// Build PR description
	prBody := buildPRBody(langPRs, corePRs, latestOpenAPIVersion)

	// Commit changes
	runCmd("git", "add", "go.mod", "go.sum")
	runCmdIgnoreError("git", "-c", "user.name=speakeasybot",
		"-c", "user.email=bot@speakeasyapi.dev",
		"commit", "-m", fmt.Sprintf("chore: bump openapi-generation to v%s", latestOpenAPIVersion))

	// Push branch
	runCmdIgnoreError("git", "push", "origin", branchName)
	if exitCode := getLastExitCode(); exitCode != 0 {
		runCmd("git", "push", "-f", "origin", branchName)
	}

	// Create PR
	fmt.Println("Creating PR...")
	repoName := execCmd("gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner")
	runCmd("gh", "pr", "create",
		"--title", prTitle,
		"--body", prBody,
		"--head", branchName,
		"--base", "main",
		"--repo", repoName)

	fmt.Println("Release PR created successfully!")
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running %s: %v\n", name, err)
		os.Exit(1)
	}
}

var lastExitCode int

func runCmdIgnoreError(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			lastExitCode = exitError.ExitCode()
		} else {
			lastExitCode = 1
		}
	} else {
		lastExitCode = 0
	}
}

func getLastExitCode() int {
	return lastExitCode
}

func execCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func execCmdIgnoreError(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output))
}

func getCurrentVersion() string {
	output := execCmd("git", "describe", "--tags")
	if output == "" {
		return "0.0.0"
	}
	// Remove 'v' prefix if present
	if strings.HasPrefix(output, "v") {
		output = output[1:]
	}
	// Extract version (handle tags like v1.2.3-5-gabc1234)
	parts := strings.Split(output, "-")
	return parts[0]
}

func getCurrentOpenAPIVersion() string {
	goModPath := "go.mod"
	data, err := os.ReadFile(goModPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading go.mod: %v\n", err)
		os.Exit(1)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "github.com/speakeasy-api/openapi-generation/v2") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				version := fields[1]
				return strings.TrimPrefix(version, "v")
			}
		}
	}
	return ""
}

func normalizeLang(lang string) string {
	lang = strings.ToLower(lang)
	lang = regexp.MustCompile(`v2$`).ReplaceAllString(lang, "")
	return lang
}

func isInternalPR(repoRoot string, prNum int, title string) bool {
	// Filter out chore: PRs
	if matched, _ := regexp.MatchString(`^[cC]hore:`, title); matched {
		return true
	}

	// Check if PR has changelog files
	filesJSON := execCmdIgnoreError("gh", "pr", "view", fmt.Sprintf("%d", prNum),
		"--repo", "speakeasy-api/openapi-generation",
		"--json", "files")
	if filesJSON != "" {
		var result struct {
			Files []PRFile `json:"files"`
		}
		if err := json.Unmarshal([]byte(filesJSON), &result); err == nil {
			for _, file := range result.Files {
				if strings.Contains(file.Path, "changelogs/") {
					return false // Has changelogs, not internal
				}
			}
		}
	}

	return true // No changelogs found, consider it internal
}

func extractLanguage(pr PR) string {
	// Check labels
	for _, label := range pr.Labels {
		labelLower := strings.ToLower(label.Name)
		if matched, _ := regexp.MatchString(`(python|typescript|java|go|csharp|php|ruby|terraform)`, labelLower); matched {
			return normalizeLang(label.Name)
		}
	}

	// Try to extract from title
	titleLower := strings.ToLower(pr.Title)
	langRegex := regexp.MustCompile(`(?i)(pythonv2|typescriptv2|python|typescript|java|go|csharp|php|ruby|terraform)`)
	matches := langRegex.FindStringSubmatch(pr.Title)
	if len(matches) > 0 {
		return normalizeLang(matches[0])
	}

	return ""
}

func extractLanguageFromFiles(repoRoot string, prNum int) string {
	// Get files as JSON array
	filesJSON := execCmdIgnoreError("gh", "pr", "view", fmt.Sprintf("%d", prNum),
		"--repo", "speakeasy-api/openapi-generation",
		"--json", "files")
	if filesJSON == "" {
		return ""
	}

	var result struct {
		Files []PRFile `json:"files"`
	}
	if err := json.Unmarshal([]byte(filesJSON), &result); err != nil {
		return ""
	}

	langRegex := regexp.MustCompile(`(?i)(pythonv2|typescriptv2|python|typescript|java|go|csharp|php|ruby|terraform)`)
	for _, file := range result.Files {
		if langRegex.MatchString(file.Path) {
			matches := langRegex.FindStringSubmatch(file.Path)
			if len(matches) > 0 {
				return normalizeLang(matches[0])
			}
		}
	}

	return ""
}

func extractFirstLine(body string) string {
	lines := strings.Split(body, "\n")
	if len(lines) == 0 {
		return ""
	}
	firstLine := lines[0]
	// Remove markdown headers and formatting
	firstLine = regexp.MustCompile(`^#*\s*`).ReplaceAllString(firstLine, "")
	firstLine = regexp.MustCompile(`\*\*`).ReplaceAllString(firstLine, "")
	return strings.TrimSpace(firstLine)
}

func buildPRTitle(langPRs map[string][]string, corePRs []string) string {
	if len(langPRs) == 0 && len(corePRs) > 0 {
		return "chore: update dependencies"
	}

	var titleParts []string
	languages := make([]string, 0, len(langPRs))
	for lang := range langPRs {
		languages = append(languages, lang)
	}
	sort.Strings(languages)

	for _, lang := range languages {
		prs := strings.Join(langPRs[lang], "\n")
		prsLower := strings.ToLower(prs)

		hasFixes := regexp.MustCompile(`(?i)\bfix`).MatchString(prsLower)
		hasFeatures := regexp.MustCompile(`(?i)(feat|add|new|support)`).MatchString(prsLower)

		if hasFeatures && hasFixes {
			titleParts = append(titleParts, fmt.Sprintf("feat(%s): updates; fix(%s): fixes", lang, lang))
		} else if hasFeatures {
			titleParts = append(titleParts, fmt.Sprintf("feat(%s): updates", lang))
		} else if hasFixes {
			titleParts = append(titleParts, fmt.Sprintf("fix(%s): fixes", lang))
		} else {
			titleParts = append(titleParts, fmt.Sprintf("chore(%s): updates", lang))
		}
	}

	if len(titleParts) > 0 {
		return strings.Join(titleParts, "; ")
	}
	return "chore: update dependencies"
}

func buildPRBody(langPRs map[string][]string, corePRs []string, latestVersion string) string {
	var body strings.Builder

	body.WriteString("## Core\n")
	if len(corePRs) > 0 {
		for _, pr := range corePRs {
			body.WriteString(pr)
			body.WriteString("\n")
		}
	} else {
		body.WriteString("- Dependency updates\n")
	}
	body.WriteString("\n")

	// Add language sections in order
	langOrder := []string{"python", "typescript", "java", "go", "csharp", "php", "ruby", "terraform"}
	for _, lang := range langOrder {
		if prs, ok := langPRs[lang]; ok && len(prs) > 0 {
			body.WriteString(fmt.Sprintf("## %s\n", capitalize(lang)))
			for _, pr := range prs {
				body.WriteString(pr)
				body.WriteString("\n")
			}
			body.WriteString("\n")
		}
	}

	return body.String()
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

