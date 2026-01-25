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
	NoDiff       bool   `json:"no-diff"`
	FormatToYAML bool   `json:"format-to-yaml"`
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
			Name:        "no-diff",
			Description: "Just download specs, don't compute SDK diff",
		},
		flag.BooleanFlag{
			Name:         "format-to-yaml",
			Description:  "Pre-format specs to YAML before diffing (helps with consistent output)",
			DefaultValue: true,
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
	event      shared.CliEvent
	targetName string
	targetLang string
}

// prIdentifiers contains identifiers extracted from a PR that can be used to find events
type prIdentifiers struct {
	lintReportDigest    string
	changesReportDigest string
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

	return ids, nil
}

// findGenerationEventByPR searches for generation events matching a PR URL
func findGenerationEventByPR(ctx context.Context, pr *parsedPRUrl) (*matchingEventInfo, error) {
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

	// Print the main event details
	logger.Infof("")
	logger.Infof("=== TARGET_GENERATE Event ===")
	printEventDetails(logger, &event)

	// If source spec info is missing, search connected events to find it
	if (event.SourceNamespaceName == nil || event.SourceRevisionDigest == nil) && event.ExecutionID != "" {
		logger.Infof("")
		logger.Infof("=== Connected Events (ExecutionID: %s) ===", event.ExecutionID)
		enrichedEvent := findAndEnrichFromConnectedEvents(ctx, client, &event, logger)
		if enrichedEvent != nil {
			event = *enrichedEvent
		}
	}

	// Get target info from the event itself
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
	if event.LintReportDigest != nil {
		logger.Infof("  LintReportDigest: %s", *event.LintReportDigest)
	}
	if event.GhPullRequest != nil {
		logger.Infof("  GhPullRequest: %s", *event.GhPullRequest)
	}
}

// connectedEventsResult holds the result of searching for connected events
type connectedEventsResult struct {
	interactionType shared.InteractionType
	events          []shared.CliEvent
}

// findAndEnrichFromConnectedEvents searches for connected events with the same ExecutionID,
// prints their details, and enriches the original event with any missing source spec info
func findAndEnrichFromConnectedEvents(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy, event *shared.CliEvent, logger log.Logger) *shared.CliEvent {
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

	// Collect results, print details, and enrich event
	for result := range results {
		for _, connectedEvent := range result.events {
			if connectedEvent.ID == event.ID {
				continue // Skip the original event
			}

			// Print event details
			logger.Infof("")
			logger.Infof("--- %s Event ---", result.interactionType)
			printEventDetails(logger, &connectedEvent)

			// Enrich original event with missing source spec info
			if connectedEvent.SourceNamespaceName != nil && event.SourceNamespaceName == nil {
				event.SourceNamespaceName = connectedEvent.SourceNamespaceName
			}
			if connectedEvent.SourceRevisionDigest != nil && event.SourceRevisionDigest == nil {
				event.SourceRevisionDigest = connectedEvent.SourceRevisionDigest
			}
			if connectedEvent.GenerateGenLockPreRevisionDigest != nil && event.GenerateGenLockPreRevisionDigest == nil {
				event.GenerateGenLockPreRevisionDigest = connectedEvent.GenerateGenLockPreRevisionDigest
			}
		}
	}

	return event
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
	match, err := findGenerationEventByPR(ctx, pr)
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
		NoDiff:       flags.NoDiff,
		FormatToYAML: flags.FormatToYAML,
	})
}
