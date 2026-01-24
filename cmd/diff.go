package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	changes "github.com/speakeasy-api/openapi-generation/v2/pkg/changes"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/registry"
	"github.com/spf13/cobra"
)

// DiffFlags for the registry subcommand
type DiffFlags struct {
	Org       string `json:"org"`
	Workspace string `json:"workspace"`
	Namespace string `json:"namespace"`
	OldDigest string `json:"old"`
	NewDigest string `json:"new"`
	OutputDir string `json:"output-dir"`
	Lang      string `json:"lang"`
	NoDiff    bool   `json:"no-diff"`
}

// FilesFlags for the files subcommand
type FilesFlags struct {
	OldSpec   string `json:"old"`
	NewSpec   string `json:"new"`
	OutputDir string `json:"output-dir"`
	Lang      string `json:"lang"`
}

// FromPRFlags for the from-pr subcommand
type FromPRFlags struct {
	PRUrl     string `json:"pr-url"`
	Target    string `json:"target"`
	OutputDir string `json:"output-dir"`
	Lang      string `json:"lang"`
	NoDiff    bool   `json:"no-diff"`
}

const diffLong = `# Diff

Compare OpenAPI spec revisions and show SDK-level changes.

This command supports three modes:

## Files Mode (Local)
Compare two spec files directly from disk:
` + "```bash" + `
speakeasy diff files --old old-spec.yaml --new new-spec.yaml
` + "```" + `

## From PR Mode
Look up specs from a GitHub pull request created by Speakeasy:
` + "```bash" + `
speakeasy diff from-pr https://github.com/org/repo/pull/123
` + "```" + `

## Registry Mode
Compare specs by providing registry namespace and digest values:
` + "```bash" + `
speakeasy diff registry --namespace my-api --old sha256:abc... --new sha256:def...
` + "```"

const diffRegistryLong = `# Diff Registry

Compare two OpenAPI spec revisions from the Speakeasy registry and show SDK-level changes.

This command will:
1. Download the old spec revision from the registry
2. Download the new spec revision from the registry
3. Compute and display SDK-level changes between them

Example usage:
` + "```bash" + `
speakeasy diff registry \
  --namespace my-api \
  --old sha256:abc123... \
  --new sha256:def456...

# Use a specific language for SDK diff context
speakeasy diff registry --namespace myns --old sha256:abc... --new sha256:def... --lang typescript

# Just download specs without showing SDK diff
speakeasy diff registry --namespace myns --old sha256:abc... --new sha256:def... --no-diff
` + "```"

const diffFilesLong = `# Diff Files

Compare two OpenAPI spec files directly from disk and show SDK-level changes.

This is the simplest mode - just provide paths to your old and new spec files.

Example usage:
` + "```bash" + `
speakeasy diff files --old ./old-openapi.yaml --new ./new-openapi.yaml

# Use a specific language for SDK diff context
speakeasy diff files --old v1.yaml --new v2.yaml --lang typescript

# Specify output directory for intermediate files
speakeasy diff files --old old.json --new new.json --output-dir ./diff-output
` + "```"

const diffFromPRLong = `# Diff From PR

Compare spec revisions from a GitHub pull request created by Speakeasy.

This command automatically looks up the spec revisions used in a Speakeasy-generated PR
and shows the SDK-level changes between the previous and new specs.

Example usage:
` + "```bash" + `
speakeasy diff from-pr https://github.com/org/sdk-repo/pull/123

# For monorepos with multiple targets, specify which target
speakeasy diff from-pr https://github.com/org/mono-sdk/pull/456 --target typescript
` + "```"

var diffCmd = &model.CommandGroup{
	Usage:          "diff",
	Short:          "Compare spec revisions and show SDK changes",
	Long:           utils.RenderMarkdown(diffLong),
	InteractiveMsg: "How would you like to look up the diff?",
	Commands:       []model.Command{diffFilesCmd, diffFromPRCmd, diffRegistryCmd},
}

var diffFilesCmd = &model.ExecutableCommand[FilesFlags]{
	Usage: "files",
	Short: "Compare two local spec files",
	Long:  utils.RenderMarkdown(diffFilesLong),
	Run:   runDiffFiles,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "old",
			Description: "Path to the old OpenAPI spec file",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "new",
			Description: "Path to the new OpenAPI spec file",
			Required:    true,
		},
		flag.StringFlag{
			Name:         "output-dir",
			Shorthand:    "o",
			Description:  "Directory for intermediate files",
			DefaultValue: ".speakeasy/diff",
		},
		flag.StringFlag{
			Name:         "lang",
			Shorthand:    "l",
			Description:  "Target language for SDK diff context",
			DefaultValue: "go",
		},
	},
}

var diffRegistryCmd = &model.ExecutableCommand[DiffFlags]{
	Usage:        "registry",
	Short:        "Compare specs by registry namespace and digests",
	Long:         utils.RenderMarkdown(diffRegistryLong),
	Run:          runDiffRegistry,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "org",
			Description: "Organization slug (defaults to current)",
		},
		flag.StringFlag{
			Name:        "workspace",
			Description: "Workspace slug (defaults to current)",
		},
		flag.StringFlag{
			Name:        "namespace",
			Description: "Source namespace",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "old",
			Description: "Old revision digest (e.g., sha256:abc123...)",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "new",
			Description: "New revision digest (e.g., sha256:abc123...)",
			Required:    true,
		},
		flag.StringFlag{
			Name:         "output-dir",
			Shorthand:    "o",
			Description:  "Directory to download specs to",
			DefaultValue: ".speakeasy/diff",
		},
		flag.StringFlag{
			Name:         "lang",
			Shorthand:    "l",
			Description:  "Target language for SDK diff context",
			DefaultValue: "go",
		},
		flag.BooleanFlag{
			Name:        "no-diff",
			Description: "Just download specs, don't compute SDK diff",
		},
	},
}

var diffFromPRCmd = &model.ExecutableCommand[FromPRFlags]{
	Usage:        "from-pr [url]",
	Short:        "Compare specs from a GitHub PR",
	Long:         utils.RenderMarkdown(diffFromPRLong),
	Run:          runDiffFromPR,
	RequiresAuth: true,
	PreRun:       fromPRPreRun,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "target",
			Shorthand:   "t",
			Description: "Filter by target name when multiple targets exist in the PR (for monorepos)",
		},
		flag.StringFlag{
			Name:         "output-dir",
			Shorthand:    "o",
			Description:  "Directory to download specs to",
			DefaultValue: ".speakeasy/diff",
		},
		flag.StringFlag{
			Name:         "lang",
			Shorthand:    "l",
			Description:  "Target language for SDK diff context",
			DefaultValue: "go",
		},
		flag.BooleanFlag{
			Name:        "no-diff",
			Description: "Just download specs, don't compute SDK diff",
		},
	},
}

// fromPRPreRun extracts the PR URL from positional arguments
func fromPRPreRun(cmd *cobra.Command, flags *FromPRFlags) error {
	args := cmd.Flags().Args()
	if len(args) > 0 && flags.PRUrl == "" {
		flags.PRUrl = args[0]
	}
	if flags.PRUrl == "" {
		return fmt.Errorf("PR URL is required - provide it as a positional argument")
	}
	return nil
}

// DiffParams contains the parameters needed to execute a diff from registry
type DiffParams struct {
	Org       string
	Workspace string
	Namespace string
	OldDigest string
	NewDigest string
	OutputDir string
	Lang      string
	NoDiff    bool
}

// LocalDiffParams contains the parameters needed to execute a diff from local files
type LocalDiffParams struct {
	OldSpecPath string
	NewSpecPath string
	OutputDir   string
	Lang        string
}

func runDiffFiles(ctx context.Context, flags FilesFlags) error {
	logger := log.From(ctx)

	// Validate files exist
	if _, err := os.Stat(flags.OldSpec); os.IsNotExist(err) {
		return fmt.Errorf("old spec file not found: %s", flags.OldSpec)
	}
	if _, err := os.Stat(flags.NewSpec); os.IsNotExist(err) {
		return fmt.Errorf("new spec file not found: %s", flags.NewSpec)
	}

	// Get absolute paths
	oldSpecPath, err := filepath.Abs(flags.OldSpec)
	if err != nil {
		return fmt.Errorf("failed to resolve old spec path: %w", err)
	}
	newSpecPath, err := filepath.Abs(flags.NewSpec)
	if err != nil {
		return fmt.Errorf("failed to resolve new spec path: %w", err)
	}

	logger.Infof("Old spec: %s", oldSpecPath)
	logger.Infof("New spec: %s", newSpecPath)
	logger.Infof("")

	return executeLocalDiff(ctx, LocalDiffParams{
		OldSpecPath: oldSpecPath,
		NewSpecPath: newSpecPath,
		OutputDir:   flags.OutputDir,
		Lang:        flags.Lang,
	})
}

func runDiffRegistry(ctx context.Context, flags DiffFlags) error {
	// Use current org/workspace if not provided
	org := flags.Org
	workspace := flags.Workspace
	if org == "" {
		org = core.GetOrgSlugFromContext(ctx)
	}
	if workspace == "" {
		workspace = core.GetWorkspaceSlugFromContext(ctx)
	}
	if org == "" || workspace == "" {
		return fmt.Errorf("org and workspace must be provided via flags or authenticated context")
	}

	return executeDiff(ctx, DiffParams{
		Org:       org,
		Workspace: workspace,
		Namespace: flags.Namespace,
		OldDigest: flags.OldDigest,
		NewDigest: flags.NewDigest,
		OutputDir: flags.OutputDir,
		Lang:      flags.Lang,
		NoDiff:    flags.NoDiff,
	})
}

// parsedPRUrl contains the components of a GitHub PR URL
type parsedPRUrl struct {
	ghOrg    string
	ghRepo   string
	prNumber int
	fullUrl  string // normalized URL without query/fragment
}

// parsePRUrl parses a GitHub PR URL and extracts org, repo, and PR number
// Handles URLs with query strings, fragments, trailing slashes, etc.
func parsePRUrl(rawUrl string) (*parsedPRUrl, error) {
	// Trim whitespace
	rawUrl = strings.TrimSpace(rawUrl)

	// Parse the URL to handle query strings, fragments, etc.
	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Verify it's a GitHub URL
	if parsed.Host != "github.com" && parsed.Host != "www.github.com" {
		return nil, fmt.Errorf("expected a GitHub URL, got host: %s", parsed.Host)
	}

	// Extract path components, removing empty segments and query/fragment
	// Expected format: /{org}/{repo}/pull/{number}
	path := strings.Trim(parsed.Path, "/")
	parts := strings.Split(path, "/")

	// Validate path structure
	if len(parts) < 4 || parts[2] != "pull" {
		return nil, fmt.Errorf("invalid GitHub PR URL format. Expected: https://github.com/{org}/{repo}/pull/{number}")
	}

	// Parse PR number
	prNumber, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, fmt.Errorf("invalid PR number '%s': %w", parts[3], err)
	}

	// Construct normalized URL (without query/fragment)
	normalizedUrl := fmt.Sprintf("https://github.com/%s/%s/pull/%d", parts[0], parts[1], prNumber)

	return &parsedPRUrl{
		ghOrg:    parts[0],
		ghRepo:   parts[1],
		prNumber: prNumber,
		fullUrl:  normalizedUrl,
	}, nil
}

// matchingEventInfo contains the matched event and its target info
type matchingEventInfo struct {
	event      shared.CliEvent
	targetName string
	targetLang string
}

// prIdentifiers contains identifiers extracted from a PR that can be used to find events
type prIdentifiers struct {
	lintReportDigest    string
	changesReportDigest string
}

// extractIdentifiersFromPR extracts lint report and changes report digests from the PR body
func extractIdentifiersFromPR(ctx context.Context, pr *parsedPRUrl) (*prIdentifiers, error) {
	repoArg := fmt.Sprintf("%s/%s", pr.ghOrg, pr.ghRepo)

	// Get the PR body
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", strconv.Itoa(pr.prNumber),
		"--repo", repoArg,
		"--json", "body",
		"--jq", ".body")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get PR body: %w", err)
	}

	body := string(output)
	ids := &prIdentifiers{}

	// Extract lint report digest from URL like:
	// https://app.speakeasy.com/org/dwolla/dwolla/linting-report/36cf12b1bdf5753b49233792aa845428
	lintPattern := regexp.MustCompile(`linting-report/([a-f0-9]+)`)
	if matches := lintPattern.FindStringSubmatch(body); len(matches) > 1 {
		ids.lintReportDigest = matches[1]
	}

	// Extract changes report digest from URL like:
	// https://app.speakeasy.com/org/dwolla/dwolla/changes-report/a94eec74348ccb5e7695a571a07543b2
	changesPattern := regexp.MustCompile(`changes-report/([a-f0-9]+)`)
	if matches := changesPattern.FindStringSubmatch(body); len(matches) > 1 {
		ids.changesReportDigest = matches[1]
	}

	if ids.lintReportDigest == "" && ids.changesReportDigest == "" {
		return nil, fmt.Errorf("could not find Speakeasy report URLs in PR body")
	}

	return ids, nil
}

// findGenerationEventByPR searches for generation events matching a PR URL
func findGenerationEventByPR(ctx context.Context, pr *parsedPRUrl, targetFilter string) (*matchingEventInfo, error) {
	logger := log.From(ctx)

	client, err := core.GetSDKFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get SDK from context: %w", err)
	}

	// Get all targets in the workspace
	logger.Infof("Searching for targets in repository %s/%s...", pr.ghOrg, pr.ghRepo)

	targetsRes, err := client.Events.GetTargets(ctx, operations.GetWorkspaceTargetsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace targets: %w", err)
	}

	if targetsRes.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d when fetching workspace targets", targetsRes.StatusCode)
	}

	// Filter for targets matching the GitHub org/repo
	var matchingTargets []shared.TargetSDK
	for _, target := range targetsRes.TargetSDKList {
		if target.GhActionOrganization != nil && target.GhActionRepository != nil &&
			strings.EqualFold(*target.GhActionOrganization, pr.ghOrg) &&
			strings.EqualFold(*target.GhActionRepository, pr.ghRepo) {
			matchingTargets = append(matchingTargets, target)
		}
	}

	if len(matchingTargets) == 0 {
		return nil, fmt.Errorf("no SDK targets found for repository %s/%s in current workspace", pr.ghOrg, pr.ghRepo)
	}

	logger.Infof("Found %d target(s) for this repository", len(matchingTargets))

	// Try to extract identifiers from PR body
	var prIds *prIdentifiers
	prIds, err = extractIdentifiersFromPR(ctx, pr)
	if err != nil {
		logger.Warnf("Could not extract identifiers from PR: %v", err)
	} else if prIds.lintReportDigest != "" {
		logger.Infof("Found lint report digest: %s", prIds.lintReportDigest)
	}

	// PR URL matching pattern - the stored URL may or may not have trailing content
	prUrlPattern := regexp.MustCompile(fmt.Sprintf(`^%s(/.*)?$`, regexp.QuoteMeta(pr.fullUrl)))

	// If we have a lint report digest, try to search directly by it
	if prIds != nil && prIds.lintReportDigest != "" {
		logger.Infof("Searching for event by lint report digest...")

		eventsRes, err := client.Events.Search(ctx, operations.SearchWorkspaceEventsRequest{
			LintReportDigest: &prIds.lintReportDigest,
			InteractionType:  shared.InteractionTypeTargetGenerate.ToPointer(),
		})
		if err == nil && eventsRes.StatusCode == http.StatusOK && len(eventsRes.CliEventBatch) > 0 {
			event := eventsRes.CliEventBatch[0]
			// Find the matching target
			for _, target := range matchingTargets {
				if event.GenerateGenLockID != nil && *event.GenerateGenLockID == target.GenerateGenLockID {
					targetName := target.GenerateTarget
					if target.GenerateTargetName != nil && *target.GenerateTargetName != "" {
						targetName = *target.GenerateTargetName
					}
					return &matchingEventInfo{
						event:      event,
						targetName: targetName,
						targetLang: target.GenerateTarget,
					}, nil
				}
			}
			// If we found an event but couldn't match to a target, still return it
			targetLang := ""
			if event.GenerateTarget != nil {
				targetLang = *event.GenerateTarget
			}
			return &matchingEventInfo{
				event:      event,
				targetName: targetLang,
				targetLang: targetLang,
			}, nil
		}
	}

	// Fall back to searching events per target
	var matchingEvents []matchingEventInfo
	limit := int64(100) // Check more events

	for _, target := range matchingTargets {
		// Apply target filter if specified (for monorepos)
		if targetFilter != "" {
			targetName := target.GenerateTarget
			if target.GenerateTargetName != nil && *target.GenerateTargetName != "" {
				targetName = *target.GenerateTargetName
			}
			if !strings.EqualFold(targetName, targetFilter) && !strings.EqualFold(target.GenerateTarget, targetFilter) {
				continue
			}
		}

		genLockID := target.GenerateGenLockID

		eventsRes, err := client.Events.Search(ctx, operations.SearchWorkspaceEventsRequest{
			GenerateGenLockID: &genLockID,
			InteractionType:   shared.InteractionTypeTargetGenerate.ToPointer(),
			Limit:             &limit,
		})
		if err != nil {
			logger.Warnf("Failed to search events for target %s: %v", target.GenerateTarget, err)
			continue
		}

		if eventsRes.StatusCode != http.StatusOK {
			logger.Warnf("Unexpected status %d when searching events for target %s", eventsRes.StatusCode, target.GenerateTarget)
			continue
		}

		// Look for events matching the PR URL or lint report digest
		for _, event := range eventsRes.CliEventBatch {
			matched := false

			// Match by PR URL
			if event.GhPullRequest != nil && prUrlPattern.MatchString(*event.GhPullRequest) {
				matched = true
			}

			// Match by lint report digest
			if !matched && prIds != nil && prIds.lintReportDigest != "" &&
				event.LintReportDigest != nil && *event.LintReportDigest == prIds.lintReportDigest {
				matched = true
			}

			if matched {
				targetName := target.GenerateTarget
				if target.GenerateTargetName != nil && *target.GenerateTargetName != "" {
					targetName = *target.GenerateTargetName
				}
				matchingEvents = append(matchingEvents, matchingEventInfo{
					event:      event,
					targetName: targetName,
					targetLang: target.GenerateTarget,
				})
				break // Found match for this target, move to next
			}
		}
	}

	if len(matchingEvents) == 0 {
		if targetFilter != "" {
			return nil, fmt.Errorf("no generation event found for PR %s with target '%s'. The PR may not have been created by Speakeasy, or the target name may be incorrect", pr.fullUrl, targetFilter)
		}
		return nil, fmt.Errorf("no generation event found for PR %s. The PR may not have been created by Speakeasy", pr.fullUrl)
	}

	// If multiple events found (monorepo scenario), need to disambiguate
	if len(matchingEvents) > 1 {
		// List the targets found
		var targetNames []string
		for _, m := range matchingEvents {
			targetNames = append(targetNames, fmt.Sprintf("%s (%s)", m.targetName, m.targetLang))
		}
		return nil, fmt.Errorf("multiple targets found for this PR (monorepo detected). Please specify which target using --target flag.\nAvailable targets: %s", strings.Join(targetNames, ", "))
	}

	return &matchingEvents[0], nil
}

// findPreviousRevisionDigest looks up the previous generation event for this target
// and returns its SourceRevisionDigest (which would have been the "new" spec at that time)
func findPreviousRevisionDigest(ctx context.Context, currentEvent *shared.CliEvent) (string, error) {
	if currentEvent.GenerateGenLockID == nil {
		return "", fmt.Errorf("current event has no GenerateGenLockID")
	}

	client, err := core.GetSDKFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get SDK from context: %w", err)
	}

	limit := int64(50)
	eventsRes, err := client.Events.Search(ctx, operations.SearchWorkspaceEventsRequest{
		GenerateGenLockID: currentEvent.GenerateGenLockID,
		InteractionType:   shared.InteractionTypeTargetGenerate.ToPointer(),
		Limit:             &limit,
	})
	if err != nil {
		return "", fmt.Errorf("failed to search for previous events: %w", err)
	}

	if eventsRes.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d when searching for previous events", eventsRes.StatusCode)
	}

	// Events are typically sorted by creation time (newest first)
	// We want the event that occurred just before the current one
	var foundCurrent bool
	for _, event := range eventsRes.CliEventBatch {
		// Skip until we find the current event
		if event.ID == currentEvent.ID {
			foundCurrent = true
			continue
		}

		// The next event after finding current is the previous one chronologically
		if foundCurrent && event.SourceRevisionDigest != nil {
			return *event.SourceRevisionDigest, nil
		}
	}

	// If we didn't find the current event in the list, just take the second event
	// (assuming the current event is the most recent but wasn't in the batch)
	if !foundCurrent && len(eventsRes.CliEventBatch) > 0 {
		for _, event := range eventsRes.CliEventBatch {
			if event.ID != currentEvent.ID && event.SourceRevisionDigest != nil {
				return *event.SourceRevisionDigest, nil
			}
		}
	}

	return "", fmt.Errorf("no previous generation event found for this target - this may be the first generation")
}

func runDiffFromPR(ctx context.Context, flags FromPRFlags) error {
	logger := log.From(ctx)

	// Parse PR URL
	pr, err := parsePRUrl(flags.PRUrl)
	if err != nil {
		return err
	}

	logger.Infof("Looking up PR: %s", pr.fullUrl)

	// Find matching event
	match, err := findGenerationEventByPR(ctx, pr, flags.Target)
	if err != nil {
		return err
	}

	event := match.event

	logger.Infof("Found generation event for target: %s", match.targetName)

	// Validate required fields
	if event.SourceNamespaceName == nil || event.SourceRevisionDigest == nil {
		return fmt.Errorf("generation event missing source spec information. The generation may have failed before uploading specs")
	}

	// Get the old digest - either from the event or by looking up the previous event
	oldDigest := ""
	if event.GenerateGenLockPreRevisionDigest != nil {
		oldDigest = *event.GenerateGenLockPreRevisionDigest
	} else {
		// Try to find the previous event for this target
		logger.Infof("Looking up previous generation event...")
		prevDigest, err := findPreviousRevisionDigest(ctx, &event)
		if err != nil {
			return fmt.Errorf("no previous spec revision found: %w", err)
		}
		oldDigest = prevDigest
	}

	// Get workspace context
	org := core.GetOrgSlugFromContext(ctx)
	workspace := core.GetWorkspaceSlugFromContext(ctx)
	if org == "" || workspace == "" {
		return fmt.Errorf("could not determine organization or workspace from authenticated context")
	}

	logger.Infof("Namespace: %s", *event.SourceNamespaceName)
	logger.Infof("Old spec: %s", truncateDigest(oldDigest))
	logger.Infof("New spec: %s", truncateDigest(*event.SourceRevisionDigest))
	logger.Infof("")

	// Use the target language from the event if no override specified
	lang := flags.Lang
	if lang == "go" && match.targetLang != "" && match.targetLang != "go" {
		// Default to the target's actual language if user didn't explicitly set one
		lang = match.targetLang
	}

	return executeDiff(ctx, DiffParams{
		Org:       org,
		Workspace: workspace,
		Namespace: *event.SourceNamespaceName,
		OldDigest: oldDigest,
		NewDigest: *event.SourceRevisionDigest,
		OutputDir: flags.OutputDir,
		Lang:      lang,
		NoDiff:    flags.NoDiff,
	})
}

// executeDiff performs the actual diff operation
func executeDiff(ctx context.Context, params DiffParams) error {
	logger := log.From(ctx)

	// Clean up and prepare output directory
	oldDir := filepath.Join(params.OutputDir, "old")
	newDir := filepath.Join(params.OutputDir, "new")

	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf("failed to create old spec directory: %w", err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return fmt.Errorf("failed to create new spec directory: %w", err)
	}

	// Build registry URLs
	oldLocation := fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s",
		params.Org, params.Workspace, params.Namespace, params.OldDigest)
	newLocation := fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s",
		params.Org, params.Workspace, params.Namespace, params.NewDigest)

	// Download old spec
	logger.Infof("Downloading old spec: %s", truncateDigest(params.OldDigest))
	oldDoc := workflow.Document{Location: workflow.LocationString(oldLocation)}
	oldResult, err := registry.ResolveSpeakeasyRegistryBundle(ctx, oldDoc, oldDir)
	if err != nil {
		return fmt.Errorf("failed to download old spec: %w", err)
	}

	// Download new spec
	logger.Infof("Downloading new spec: %s", truncateDigest(params.NewDigest))
	newDoc := workflow.Document{Location: workflow.LocationString(newLocation)}
	newResult, err := registry.ResolveSpeakeasyRegistryBundle(ctx, newDoc, newDir)
	if err != nil {
		return fmt.Errorf("failed to download new spec: %w", err)
	}

	logger.Infof("Specs downloaded to: %s", params.OutputDir)

	if params.NoDiff {
		logger.Infof("Old spec: %s", oldResult.LocalFilePath)
		logger.Infof("New spec: %s", newResult.LocalFilePath)
		return nil
	}

	// Compute SDK diff
	logger.Infof("")
	logger.Infof("Computing SDK changes (%s)...", params.Lang)

	oldConfig, newConfig := changes.CreateConfigsFromSpecPaths(changes.SpecComparison{
		OldSpecPath: oldResult.LocalFilePath,
		NewSpecPath: newResult.LocalFilePath,
		OutputDir:   params.OutputDir,
		Lang:        params.Lang,
		Verbose:     false,
		Logger:      logger,
	})

	diff, err := changes.Changes(ctx, oldConfig, newConfig)
	if err != nil {
		return fmt.Errorf("failed to compute SDK changes: %w", err)
	}

	// Output results
	logger.Infof("")
	printDiffSeparator(logger, params.Namespace)

	if len(diff.Changes) == 0 {
		logger.Infof("No SDK-level changes detected")
	} else {
		markdown := changes.ToMarkdown(diff, changes.DetailLevelFull)
		fmt.Println(markdown)
	}

	printDiffSeparator(logger, "")

	logger.Infof("")
	logger.Infof("Old spec: %s", oldResult.LocalFilePath)
	logger.Infof("New spec: %s", newResult.LocalFilePath)

	return nil
}

func truncateDigest(digest string) string {
	// Show first 12 chars of the hash portion
	if hash, found := strings.CutPrefix(digest, "sha256:"); found {
		if len(hash) > 12 {
			return "sha256:" + hash[:12] + "..."
		}
	}
	if len(digest) > 20 {
		return digest[:20] + "..."
	}
	return digest
}

func printDiffSeparator(logger log.Logger, title string) {
	if title != "" {
		logger.Infof("SDK Changes (%s):", title)
	}
	logger.Infof("────────────────────────────────────────")
}

// executeLocalDiff performs the diff operation using local spec files directly
func executeLocalDiff(ctx context.Context, params LocalDiffParams) error {
	logger := log.From(ctx)

	// Create output directory for intermediate files
	if err := os.MkdirAll(params.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Compute SDK diff
	logger.Infof("Computing SDK changes (%s)...", params.Lang)

	oldConfig, newConfig := changes.CreateConfigsFromSpecPaths(changes.SpecComparison{
		OldSpecPath: params.OldSpecPath,
		NewSpecPath: params.NewSpecPath,
		OutputDir:   params.OutputDir,
		Lang:        params.Lang,
		Verbose:     false,
		Logger:      logger,
	})

	diff, err := changes.Changes(ctx, oldConfig, newConfig)
	if err != nil {
		return fmt.Errorf("failed to compute SDK changes: %w", err)
	}

	// Output results
	logger.Infof("")

	// Use the base name of the new spec as title
	title := filepath.Base(params.NewSpecPath)
	printDiffSeparator(logger, title)

	if len(diff.Changes) == 0 {
		logger.Infof("No SDK-level changes detected")
	} else {
		markdown := changes.ToMarkdown(diff, changes.DetailLevelFull)
		fmt.Println(markdown)
	}

	printDiffSeparator(logger, "")

	return nil
}
