package environment

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Mode string

const (
	ModeDirect Mode = "direct"
	ModePR     Mode = "pr"
	ModeTest   Mode = "test"
)

type Action string

const (
	ActionValidate           Action = "validate"
	ActionRunWorkflow        Action = "run-workflow"
	ActionSuggest            Action = "suggest"
	ActionFinalize           Action = "finalize"
	ActionFinalizeSuggestion Action = "finalize-suggestion"
	ActionRelease            Action = "release"
	ActionLog                Action = "log-result"
	ActionPublishEvent       Action = "publish-event"
	ActionTag                Action = "tag"
	ActionTest               Action = "test"
)

const (
	DefaultMaxValidationWarnings = 1000
	DefaultMaxValidationErrors   = 1000
)

var (
	baseDir    = "/"
	invokeTime = time.Now()
)

func init() {
	// Allows us to run this locally
	if os.Getenv("SPEAKEASY_ENVIRONMENT") == "local" {
		baseDir, _ = os.Getwd()
	}
}

func GetBaseDir() string {
	return baseDir
}

func IsDebugMode() bool {
	return os.Getenv("INPUT_DEBUG") == "true" || os.Getenv("RUNNER_DEBUG") == "1"
}

func IsTestMode() bool {
	return GetMode() == ModeTest
}

func SpeakeasyEnvVars() []string {
	rawEnv := os.Getenv("INPUT_CLI_ENVIRONMENT_VARIABLES")
	if len(rawEnv) == 0 {
		return []string{}
	}
	src, err := godotenv.Unmarshal(rawEnv)
	if err != nil {
		fmt.Printf("Error: Failed to parse env vars from %s: %s\n", rawEnv, err)
		return []string{}
	}

	var result []string
	for k, v := range src {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(result)
	return result
}

func ForceGeneration() bool {
	return os.Getenv("INPUT_FORCE") == "true"
}

func PushCodeSamplesOnly() bool {
	return os.Getenv("INPUT_PUSH_CODE_SAMPLES_ONLY") == "true"
}

func SetVersion() string {
	return os.Getenv("INPUT_SET_VERSION")
}

func RegistryTags() string {
	return os.Getenv("INPUT_REGISTRY_TAGS")
}

// Enabled if the INPUT_SKIP_TESTING environment variable is set to "true".
func SkipTesting() bool {
	return os.Getenv("INPUT_SKIP_TESTING") == "true"
}

func SkipCompile() bool {
	return os.Getenv("INPUT_SKIP_COMPILE") == "true"
}

func SpecifiedTarget() string {
	return os.Getenv("INPUT_TARGET")
}

func SpecifiedSources() []string {
	return parseArrayInput(os.Getenv("INPUT_SOURCES"))
}

func SpecifiedCodeSamplesTargets() []string {
	return parseArrayInput(os.Getenv("INPUT_CODE_SAMPLES"))
}

func GetMode() Mode {
	mode := os.Getenv("INPUT_MODE")
	if mode == "" {
		return ModeDirect
	}

	return Mode(mode)
}

func GetAction() Action {
	action := os.Getenv("INPUT_ACTION")
	if action == "" {
		return ActionRunWorkflow
	}

	return Action(action)
}

func GetPinnedSpeakeasyVersion() string {
	return os.Getenv("INPUT_SPEAKEASY_VERSION")
}

func GetMaxSuggestions() string {
	return os.Getenv("INPUT_MAX_SUGGESTIONS")
}

func GetMaxValidationWarnings() (int, error) {
	maxVal := os.Getenv("INPUT_MAX_VALIDATION_WARNINGS")
	if maxVal == "" {
		return DefaultMaxValidationWarnings, nil
	}

	maxWarns, err := strconv.Atoi(maxVal)
	if err != nil {
		return DefaultMaxValidationWarnings, fmt.Errorf("max_validation_warnings must be an integer, falling back to default (%d): %w", DefaultMaxValidationWarnings, err)
	}

	return maxWarns, nil
}

func GetMaxValidationErrors() (int, error) {
	maxVal := os.Getenv("INPUT_MAX_VALIDATION_ERRORS")
	if maxVal == "" {
		return DefaultMaxValidationErrors, nil
	}

	maxErrors, err := strconv.Atoi(maxVal)
	if err != nil {
		return DefaultMaxValidationErrors, fmt.Errorf("max_validaiton_errors must be an integer, falling back to default (%d): %v", DefaultMaxValidationErrors, err)
	}

	return maxErrors, nil
}

func GetOpenAPIDocLocation() string {
	return os.Getenv("INPUT_OPENAPI_DOC_LOCATION")
}

func GetOpenAPIDocs() string {
	return os.Getenv("INPUT_OPENAPI_DOCS")
}

func GetOverlayDocs() string {
	return os.Getenv("INPUT_OVERLAY_DOCS")
}

func GetOpenAPIDocOutput() string {
	return os.Getenv("INPUT_OPENAPI_DOC_OUTPUT")
}

func GetLanguages() string {
	return os.Getenv("INPUT_LANGUAGES")
}

func GetDocsLanguages() string {
	return os.Getenv("INPUT_DOCS_LANGUAGES")
}

func IsDocsGeneration() bool {
	languages := os.Getenv("INPUT_LANGUAGES")
	// Quick check to ensure target is docs, we could parse this further.
	return strings.Contains(languages, "docs")
}

func GetAccessToken() string {
	return os.Getenv("INPUT_GITHUB_ACCESS_TOKEN")
}

func GetGPGFingerprint() string {
	return os.Getenv("INPUT_GPG_FINGERPRINT")
}

func GetInvokeTime() time.Time {
	return invokeTime
}

func GetOpenAPIDocAuthHeader() string {
	return os.Getenv("INPUT_OPENAPI_DOC_AUTH_HEADER")
}

func GetOpenAPIDocAuthToken() string {
	return os.Getenv("INPUT_OPENAPI_DOC_AUTH_TOKEN")
}

func GetPoetryVersion() string {
	return os.Getenv("INPUT_POETRY_VERSION")
}

func GetPnpmVersion() string {
	return os.Getenv("INPUT_PNPM_VERSION")
}

func GetUvVersion() string {
	return os.Getenv("INPUT_UV_VERSION")
}

func GetSDKChangelog() string {
	return os.Getenv("INPUT_ENABLE_SDK_CHANGELOG")
}

func SkipRelease() bool {
	return os.Getenv("INPUT_SKIP_RELEASE") == "true"
}

func SkipVersioning() bool {
	return os.Getenv("INPUT_SKIP_VERSIONING") == "true"
}

// IsPRTriggered returns true if the action was triggered by a PR event
func IsPRTriggered() bool {
	githubRef := os.Getenv("GITHUB_REF")
	return strings.Contains(githubRef, "refs/pull") || strings.Contains(githubRef, "refs/pulls")
}

// ShouldSkipReleasing returns true if we should skip releasing/tagging
// This happens when we're in direct mode and either skip_release is set or triggered by a PR event
func ShouldSkipReleasing() bool {
	return GetMode() == ModeDirect && (SkipRelease() || IsPRTriggered())
}

func GetWorkflowName() string {
	return os.Getenv("GITHUB_WORKFLOW")
}

func GetWorkflowEventPayloadPath() string {
	return os.Getenv("GITHUB_EVENT_PATH")
}

func GetWorkflowEventLabelName() string {
	return os.Getenv("GITHUB_EVENT_LABEL_NAME")
}

func GetSignedCommits() bool {
	return os.Getenv("INPUT_SIGNED_COMMITS") == "true"
}

func GetBranchName() string {
	return os.Getenv("INPUT_BRANCH_NAME")
}

func GetFeatureBranch() string {
	return strings.TrimSpace(os.Getenv("INPUT_FEATURE_BRANCH"))
}

func GetCliOutput() string {
	return os.Getenv("INPUT_CLI_OUTPUT")
}

// check for labeled/unlabeled actions
type gitHubEvent struct {
	Action string `json:"action"`
}

func GetRef() string {
	// handle pr based action triggers
	if strings.Contains(os.Getenv("GITHUB_REF"), "refs/pull") || strings.Contains(os.Getenv("GITHUB_REF"), "refs/pulls") {
		ref := os.Getenv("GITHUB_HEAD_REF")
		data, err := os.ReadFile(os.Getenv("GITHUB_EVENT_PATH"))
		if err != nil {
			return ref
		}

		var event gitHubEvent
		if err := json.Unmarshal(data, &event); err != nil {
			fmt.Println("Error parsing event JSON:", err)
			return ref
		}

		// "labeled" or "unlabeled" PR triggers need to use the base ref for label based versioning
		if event.Action == "labeled" || event.Action == "unlabeled" {
			return os.Getenv("GITHUB_BASE_REF")
		}

		return ref
	}
	return os.Getenv("GITHUB_REF")
}

func GetWorkingDirectory() string {
	return os.Getenv("INPUT_WORKING_DIRECTORY")
}

func GetRepo() string {
	if os.Getenv("INPUT_GITHUB_REPOSITORY") != "" {
		return os.Getenv("INPUT_GITHUB_REPOSITORY")
	}
	return os.Getenv("GITHUB_REPOSITORY")
}

func GetGithubServerURL() string {
	return os.Getenv("GITHUB_SERVER_URL")
}

func GetGithubOIDCRequestURL() string {
	return os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
}

func GetGithubOIDCRequestToken() string {
	return os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
}

func GetWorkspace() string {
	return os.Getenv("GITHUB_WORKSPACE")
}

func ShouldOutputTests() bool {
	return os.Getenv("INPUT_OUTPUT_TESTS") == "true"
}

func SetCLIVersionToUse(version string) error {
	return os.Setenv("PINNED_VERSION", version)
}

func parseArrayInput(input string) []string {
	if input == "" {
		return []string{}
	}

	if strings.Contains(input, "\n") {
		return strings.Split(input, "\n")
	}

	return strings.Split(input, ",")
}

// GetSourceBranch returns the source branch that triggered the generation
func GetSourceBranch() string {
	ref := GetRef()

	// Handle PR-based triggers - use the head ref (source branch)
	if strings.Contains(os.Getenv("GITHUB_REF"), "refs/pull") || strings.Contains(os.Getenv("GITHUB_REF"), "refs/pulls") {
		return os.Getenv("GITHUB_HEAD_REF")
	}

	// For direct branch triggers, extract branch name from ref
	return strings.TrimPrefix(ref, "refs/heads/")
}

// IsMainBranch returns true if the source branch is main or master
func IsMainBranch(branch string) bool {
	return branch == "main" || branch == "master"
}

// GetTargetBaseBranch returns the branch that PRs should target
func GetTargetBaseBranch() string {
	sourceBranch := GetSourceBranch()

	// If triggered from main/master, target the original ref (main/master)
	if IsMainBranch(sourceBranch) {
		return GetRef()
	}

	// For feature branches, target the source branch itself
	return "refs/heads/" + sourceBranch
}

// SanitizeBranchName sanitizes a branch name for use in generated branch names
func SanitizeBranchName(branch string) string {
	// Replace problematic characters with hyphens
	sanitized := strings.ReplaceAll(branch, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "_", "-")
	sanitized = strings.ReplaceAll(sanitized, " ", "-")

	// Remove any leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")

	return sanitized
}
