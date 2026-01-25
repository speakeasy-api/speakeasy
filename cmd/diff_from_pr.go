package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	speakeasyclientsdkgo "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
)

// FromPRFlags for the from-pr subcommand
type FromPRFlags struct {
	PRUrl        string `json:"pr-url"`
	OutputDir    string `json:"output-dir"`
	FormatToYAML bool   `json:"format-to-yaml"`
	Verbose      bool   `json:"verbose"`
}

const diffFromPRLong = `# Diff From PR

Compare spec revisions from a GitHub pull request created by Speakeasy.

This command automatically looks up the spec revisions used in a Speakeasy-generated PR
and shows the SDK-level changes between the previous and new specs.

Example usage:
` + "```bash" + `
speakeasy diff from-pr https://github.com/org/sdk-repo/pull/123
` + "```"

var diffFromPRCmd = &model.ExecutableCommand[FromPRFlags]{
	Usage:        "from-pr [url]",
	Short:        "Compare specs from a GitHub PR",
	Long:         utils.RenderMarkdown(diffFromPRLong),
	Run:          runDiffFromPR,
	RequiresAuth: true,
	PreRun:       fromPRPreRun,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "output-dir",
			Shorthand:    "o",
			Description:  "Directory to download specs to",
			DefaultValue: "/tmp/speakeasy-diff",
		},
		flag.BooleanFlag{
			Name:         "format-to-yaml",
			Description:  "Pre-format specs to YAML before diffing (helps with consistent output)",
			DefaultValue: true,
		},
		flag.BooleanFlag{
			Name:        "verbose",
			Shorthand:   "v",
			Description: "Show detailed event information during lookup",
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
	event                shared.CliEvent
	targetName           string
	targetLang           string
	previousSourceDigest string // from PR's workflow.lock diff (most reliable)
}

// prIdentifiers contains identifiers extracted from a PR that can be used to find events
type prIdentifiers struct {
	lintReportDigest       string
	changesReportDigest    string
	previousSourceDigest   string // extracted from workflow.lock diff
}

// checkGHCLIAvailable checks if the GitHub CLI is installed
func checkGHCLIAvailable() error {
	_, err := exec.LookPath("gh")
	if err != nil {
		return fmt.Errorf("GitHub CLI (gh) is required for this command but was not found.\n\n" +
			"Install it from: https://cli.github.com/\n\n" +
			"Alternatively, use 'speakeasy diff registry' with explicit namespace and digests")
	}
	return nil
}

// extractIdentifiersFromPR extracts lint report and changes report digests from the PR body
func extractIdentifiersFromPR(ctx context.Context, pr *parsedPRUrl) (*prIdentifiers, error) {
	repoArg := fmt.Sprintf("%s/%s", pr.ghOrg, pr.ghRepo)

	// Get the PR body using gh CLI
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", strconv.Itoa(pr.prNumber),
		"--repo", repoArg,
		"--json", "body",
		"--jq", ".body")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get PR body (ensure you are authenticated with 'gh auth login'): %w", err)
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

	// Try to extract the previous sourceRevisionDigest from the PR's workflow.lock diff
	// This is the most reliable source of truth for what the PR is changing FROM
	// (reflects actual repo state, not intermediate CI runs)
	ids.previousSourceDigest = extractPreviousDigestFromPRDiff(ctx, pr)

	return ids, nil
}

// extractPreviousDigestFromPRDiff gets the previous sourceRevisionDigest from the PR's workflow.lock diff
func extractPreviousDigestFromPRDiff(ctx context.Context, pr *parsedPRUrl) string {
	repoArg := fmt.Sprintf("%s/%s", pr.ghOrg, pr.ghRepo)

	// Get the PR diff
	cmd := exec.CommandContext(ctx, "gh", "pr", "diff", strconv.Itoa(pr.prNumber), "--repo", repoArg)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Look for removed lines containing sourceRevisionDigest
	// Format: -        sourceRevisionDigest: sha256:abc123...
	pattern := regexp.MustCompile(`(?m)^-\s+sourceRevisionDigest:\s+(sha256:[a-f0-9]+)`)
	if matches := pattern.FindStringSubmatch(string(output)); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// findGenerationEventByPR searches for generation events matching a PR URL
func findGenerationEventByPR(ctx context.Context, pr *parsedPRUrl, verbose bool) (*matchingEventInfo, error) {
	logger := log.From(ctx)

	// Extract lint report digest from PR body
	prIds, err := extractIdentifiersFromPR(ctx, pr)
	if err != nil {
		return nil, fmt.Errorf("could not extract Speakeasy identifiers from PR: %w", err)
	}

	if prIds.lintReportDigest == "" {
		return nil, fmt.Errorf("no lint report digest found in PR body - this PR may not have been created by Speakeasy")
	}

	logger.Infof("Found lint report digest: %s", prIds.lintReportDigest)

	// Search directly by lint report digest
	client, err := core.GetSDKFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get SDK from context: %w", err)
	}

	logger.Infof("Searching for generation event...")

	eventsRes, err := client.Events.Search(ctx, operations.SearchWorkspaceEventsRequest{
		LintReportDigest: &prIds.lintReportDigest,
		InteractionType:  shared.InteractionTypeTargetGenerate.ToPointer(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search events: %w", err)
	}

	if eventsRes.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d when searching events", eventsRes.StatusCode)
	}

	if len(eventsRes.CliEventBatch) == 0 {
		return nil, fmt.Errorf("no generation event found for lint report digest %s", prIds.lintReportDigest)
	}

	event := eventsRes.CliEventBatch[0]

	// Get target info from the original event BEFORE enrichment
	targetLang := ""
	if event.GenerateTarget != nil {
		targetLang = *event.GenerateTarget
	}

	// Print the main event details in verbose mode
	if verbose {
		logger.Infof("")
		logger.Infof("=== TARGET_GENERATE Event ===")
		printEventDetails(logger, &event)
	}

	// If source spec info or prev digest is missing, search connected events to find it
	missingSourceInfo := event.SourceNamespaceName == nil || event.SourceRevisionDigest == nil
	missingPrevDigest := event.OpenapiDiffBaseSourceRevisionDigest == nil && event.GenerateGenLockPreRevisionDigest == nil
	if (missingSourceInfo || missingPrevDigest) && event.ExecutionID != "" {
		if verbose {
			logger.Infof("")
			logger.Infof("=== Connected Events (ExecutionID: %s) ===", event.ExecutionID)
		}
		enrichedEvent := findAndEnrichFromConnectedEvents(ctx, client, &event, logger, verbose)
		if enrichedEvent != nil {
			event = *enrichedEvent
		}
	}

	return &matchingEventInfo{
		event:                event,
		targetName:           targetLang,
		targetLang:           targetLang,
		previousSourceDigest: prIds.previousSourceDigest,
	}, nil
}

// printEventDetails prints relevant fields from a CLI event
func printEventDetails(logger log.Logger, event *shared.CliEvent) {
	logger.Infof("  ID: %s", event.ID)
	logger.Infof("  InteractionType: %s", event.InteractionType)
	logger.Infof("  ExecutionID: %s", event.ExecutionID)
	if event.GenerateTarget != nil {
		logger.Infof("  GenerateTarget: %s", *event.GenerateTarget)
	}
	if event.GenerateGenLockID != nil {
		logger.Infof("  GenerateGenLockID: %s", *event.GenerateGenLockID)
	}
	if event.SourceNamespaceName != nil {
		logger.Infof("  SourceNamespaceName: %s", *event.SourceNamespaceName)
	} else {
		logger.Infof("  SourceNamespaceName: <nil>")
	}
	if event.SourceRevisionDigest != nil {
		logger.Infof("  SourceRevisionDigest: %s", *event.SourceRevisionDigest)
	} else {
		logger.Infof("  SourceRevisionDigest: <nil>")
	}
	if event.GenerateGenLockPreRevisionDigest != nil {
		logger.Infof("  GenerateGenLockPreRevisionDigest: %s", *event.GenerateGenLockPreRevisionDigest)
	} else {
		logger.Infof("  GenerateGenLockPreRevisionDigest: <nil>")
	}
	if event.OpenapiDiffBaseSourceRevisionDigest != nil {
		logger.Infof("  OpenapiDiffBaseSourceRevisionDigest: %s", *event.OpenapiDiffBaseSourceRevisionDigest)
	} else {
		logger.Infof("  OpenapiDiffBaseSourceRevisionDigest: <nil>")
	}

	// Check if WorkflowLockPreRaw and WorkflowLockPostRaw are the same (indicates bug)
	if event.WorkflowLockPreRaw != nil && event.WorkflowLockPostRaw != nil {
		if *event.WorkflowLockPreRaw == *event.WorkflowLockPostRaw {
			logger.Infof("  WorkflowLockPreRaw: <SAME AS POST - BUG!>")
		} else {
			logger.Infof("  WorkflowLockPreRaw: (differs from post)")
		}
	} else if event.WorkflowLockPreRaw != nil {
		logger.Infof("  WorkflowLockPreRaw: <set>")
	} else {
		logger.Infof("  WorkflowLockPreRaw: <nil>")
	}

	// Check if GenerateConfigPreRaw and GenerateConfigPostRaw are the same (indicates bug)
	if event.GenerateConfigPreRaw != nil && event.GenerateConfigPostRaw != nil {
		if *event.GenerateConfigPreRaw == *event.GenerateConfigPostRaw {
			logger.Infof("  GenerateConfigPreRaw: <SAME AS POST - BUG!>")
		} else {
			logger.Infof("  GenerateConfigPreRaw: (differs from post)")
		}
	} else if event.GenerateConfigPreRaw != nil {
		logger.Infof("  GenerateConfigPreRaw: <set>")
	} else {
		logger.Infof("  GenerateConfigPreRaw: <nil>")
	}

	if event.LintReportDigest != nil {
		logger.Infof("  LintReportDigest: %s", *event.LintReportDigest)
	}

	// GitHub action fields
	if event.GhActionOrganization != nil {
		logger.Infof("  GhActionOrganization: %s", *event.GhActionOrganization)
	}
	if event.GhActionRepository != nil {
		logger.Infof("  GhActionRepository: %s", *event.GhActionRepository)
	}
	if event.GhActionRunLink != nil {
		logger.Infof("  GhActionRunLink: %s", *event.GhActionRunLink)
	}
	if event.GhActionRef != nil {
		logger.Infof("  GhActionRef: %s", *event.GhActionRef)
	}
	if event.GhActionVersion != nil {
		logger.Infof("  GhActionVersion: %s", *event.GhActionVersion)
	}
	if event.GhChangesCommitted != nil {
		logger.Infof("  GhChangesCommitted: %v", *event.GhChangesCommitted)
	}
	if event.GhPullRequest != nil {
		logger.Infof("  GhPullRequest: %s", *event.GhPullRequest)
	}

	// Git fields
	if event.GitRemoteDefaultOwner != nil {
		logger.Infof("  GitRemoteDefaultOwner: %s", *event.GitRemoteDefaultOwner)
	}
	if event.GitRemoteDefaultRepo != nil {
		logger.Infof("  GitRemoteDefaultRepo: %s", *event.GitRemoteDefaultRepo)
	}
	if event.GitRelativeCwd != nil {
		logger.Infof("  GitRelativeCwd: %s", *event.GitRelativeCwd)
	}
	if event.GitUserName != nil {
		logger.Infof("  GitUserName: %s", *event.GitUserName)
	}
	if event.GitUserEmail != nil {
		logger.Infof("  GitUserEmail: %s", *event.GitUserEmail)
	}
}

// connectedEventsResult holds the result of searching for connected events
type connectedEventsResult struct {
	interactionType shared.InteractionType
	events          []shared.CliEvent
}

// findAndEnrichFromConnectedEvents searches for connected events with the same ExecutionID,
// prints their details (if verbose), and enriches the original event with any missing source spec info
func findAndEnrichFromConnectedEvents(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy, event *shared.CliEvent, logger log.Logger, verbose bool) *shared.CliEvent {
	if event.ExecutionID == "" {
		return nil
	}

	execID := event.ExecutionID

	// Search for all interaction types in parallel
	interactionTypes := []shared.InteractionType{
		shared.InteractionTypeCiExec,
		shared.InteractionTypeCliExec,
		shared.InteractionTypeTargetGenerate,
	}

	results := make(chan connectedEventsResult, len(interactionTypes))
	var wg sync.WaitGroup

	for _, interactionType := range interactionTypes {
		wg.Add(1)
		go func(it shared.InteractionType) {
			defer wg.Done()
			eventsRes, err := client.Events.Search(ctx, operations.SearchWorkspaceEventsRequest{
				ExecutionID:     &execID,
				InteractionType: it.ToPointer(),
			})
			if err != nil || eventsRes.StatusCode != http.StatusOK {
				return
			}
			results <- connectedEventsResult{interactionType: it, events: eventsRes.CliEventBatch}
		}(interactionType)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results, print details (if verbose), and enrich event
	for result := range results {
		for _, connectedEvent := range result.events {
			if connectedEvent.ID == event.ID {
				continue // Skip the original event
			}

			// Print event details in verbose mode
			if verbose {
				logger.Infof("")
				logger.Infof("--- %s Event ---", result.interactionType)
				printEventDetails(logger, &connectedEvent)
			}

			// Enrich original event with missing source spec info
			if connectedEvent.SourceNamespaceName != nil && event.SourceNamespaceName == nil {
				event.SourceNamespaceName = connectedEvent.SourceNamespaceName
			}
			if connectedEvent.SourceRevisionDigest != nil && event.SourceRevisionDigest == nil {
				event.SourceRevisionDigest = connectedEvent.SourceRevisionDigest
			}
			if connectedEvent.OpenapiDiffBaseSourceRevisionDigest != nil && event.OpenapiDiffBaseSourceRevisionDigest == nil {
				event.OpenapiDiffBaseSourceRevisionDigest = connectedEvent.OpenapiDiffBaseSourceRevisionDigest
			}
			if connectedEvent.GenerateGenLockPreRevisionDigest != nil && event.GenerateGenLockPreRevisionDigest == nil {
				event.GenerateGenLockPreRevisionDigest = connectedEvent.GenerateGenLockPreRevisionDigest
			}
			// If connected event has WorkflowLockPreRaw and we still need prev digest, try to extract it
			if connectedEvent.WorkflowLockPreRaw != nil && event.WorkflowLockPreRaw == nil {
				event.WorkflowLockPreRaw = connectedEvent.WorkflowLockPreRaw
			}
			// Also enrich WorkflowLockPostRaw so we can detect the Pre==Post bug
			if connectedEvent.WorkflowLockPostRaw != nil && event.WorkflowLockPostRaw == nil {
				event.WorkflowLockPostRaw = connectedEvent.WorkflowLockPostRaw
			}
		}
	}

	return event
}

func runDiffFromPR(ctx context.Context, flags FromPRFlags) error {
	logger := log.From(ctx)

	// Check for gh CLI early
	if err := checkGHCLIAvailable(); err != nil {
		return err
	}

	// Parse PR URL
	pr, err := parsePRUrl(flags.PRUrl)
	if err != nil {
		return err
	}

	logger.Infof("Looking up PR: %s", pr.fullUrl)

	// Find matching event
	match, err := findGenerationEventByPR(ctx, pr, flags.Verbose)
	if err != nil {
		return err
	}

	event := match.event

	logger.Infof("Found generation event for target: %s", match.targetName)

	// Validate required fields
	if event.SourceNamespaceName == nil || event.SourceRevisionDigest == nil {
		return fmt.Errorf("generation event missing source spec information. The generation may have failed before uploading specs")
	}

	// Get the old digest:
	// 1. Check event for previous registry digest
	// 2. Fallback to PR's workflow.lock diff
	oldDigest := ""
	if event.OpenapiDiffBaseSourceRevisionDigest != nil {
		oldDigest = *event.OpenapiDiffBaseSourceRevisionDigest
		if flags.Verbose {
			logger.Infof("Using OpenapiDiffBaseSourceRevisionDigest from event")
		}
	} else if event.GenerateGenLockPreRevisionDigest != nil {
		oldDigest = *event.GenerateGenLockPreRevisionDigest
		if flags.Verbose {
			logger.Infof("Using GenerateGenLockPreRevisionDigest from event")
		}
	} else if match.previousSourceDigest != "" {
		oldDigest = match.previousSourceDigest
		if flags.Verbose {
			logger.Infof("Using previous digest from PR's workflow.lock diff")
		}
	}

	if oldDigest == "" {
		return fmt.Errorf("no previous spec revision found - event missing digest fields and PR diff unavailable")
	}

	// Get workspace context
	org := core.GetOrgSlugFromContext(ctx)
	workspace := core.GetWorkspaceSlugFromContext(ctx)
	if org == "" || workspace == "" {
		return fmt.Errorf("could not determine organization or workspace from authenticated context")
	}

	// Use the target language from the event
	lang := match.targetLang
	if lang == "" {
		lang = "go" // fallback if not available
	}

	logger.Infof("Namespace: %s", *event.SourceNamespaceName)
	logger.Infof("Old spec: %s", truncateDigest(oldDigest))
	logger.Infof("New spec: %s", truncateDigest(*event.SourceRevisionDigest))
	logger.Infof("Language: %s", lang)
	logger.Infof("")

	return executeDiff(ctx, DiffParams{
		Org:          org,
		Workspace:    workspace,
		Namespace:    *event.SourceNamespaceName,
		OldDigest:    oldDigest,
		NewDigest:    *event.SourceRevisionDigest,
		OutputDir:    flags.OutputDir,
		Lang:         lang,
		FormatToYAML: flags.FormatToYAML,
	})
}
