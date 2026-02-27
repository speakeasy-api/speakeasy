package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/google/go-github/v63/github"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/git"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/speakeasy/internal/prdescription"
	"github.com/speakeasy-api/versioning-reports/versioning"
	"golang.org/x/oauth2"
)

// CreateOrUpdatePR reads an accumulated reports JSON file, builds a merged PR
// description, and creates or updates a GitHub PR on the given branch.
func CreateOrUpdatePR(ctx context.Context, inputFile, branchName string) error {
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read accumulated reports: %w", err)
	}

	var accumulated map[string]TargetGenerationReport
	if err := json.Unmarshal(data, &accumulated); err != nil {
		return fmt.Errorf("failed to parse accumulated reports: %w", err)
	}

	if len(accumulated) == 0 {
		logging.Info("No reports in accumulated file, skipping PR creation")
		return nil
	}

	// Sort target names alphabetically for stable ordering
	targets := make([]string, 0, len(accumulated))
	for k := range accumulated {
		targets = append(targets, k)
	}
	sort.Strings(targets)

	// Merge all per-target version reports into a single MergedVersionReport
	var allReports []versioning.VersionReport
	var speakeasyVersion string
	var lintingReportURL, changesReportURL string
	var manualBump bool

	for _, target := range targets {
		report := accumulated[target]
		if report.VersionReport != nil {
			allReports = append(allReports, report.VersionReport.Reports...)
		}
		if speakeasyVersion == "" && report.SpeakeasyVersion != "" {
			speakeasyVersion = report.SpeakeasyVersion
		}
		if lintingReportURL == "" && report.LintingReportURL != "" {
			lintingReportURL = report.LintingReportURL
		}
		if changesReportURL == "" && report.ChangesReportURL != "" {
			changesReportURL = report.ChangesReportURL
		}
		if report.ManualBump {
			manualBump = true
		}
	}

	mergedReport := &versioning.MergedVersionReport{Reports: allReports}

	// Build PR description
	input := prdescription.Input{
		VersionReport:    mergedReport,
		WorkflowName:     environment.GetWorkflowName(),
		SourceBranch:     environment.GetSourceBranch(),
		SpeakeasyVersion: speakeasyVersion,
		LintingReportURL: lintingReportURL,
		ChangesReportURL: changesReportURL,
		ManualBump:       manualBump,
	}

	output, err := prdescription.Generate(input)
	if err != nil {
		return fmt.Errorf("failed to generate PR description: %w", err)
	}

	// Initialize Git client
	g, err := initAction()
	if err != nil {
		return fmt.Errorf("failed to initialize git: %w", err)
	}

	// Find existing PR for this branch
	owner := os.Getenv("GITHUB_REPOSITORY_OWNER")
	repo := git.GetRepo()

	prClient := g.GetClient()
	if providedPat := os.Getenv("PR_CREATION_PAT"); providedPat != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: providedPat},
		)
		tc := oauth2.NewClient(ctx, ts)
		prClient = github.NewClient(tc)
	}

	// Search for existing PR from this branch
	existingPR := findPRForBranch(ctx, prClient, owner, repo, branchName)

	// Compute labels from version report
	labelTypes := g.UpsertLabelTypes(ctx)
	_, _, labels := git.PRVersionMetadata(mergedReport, labelTypes)

	title := output.Title
	body := output.Body

	const maxBodyLength = 65536
	if len(body) > maxBodyLength {
		body = body[:maxBodyLength-3] + "..."
	}

	if existingPR != nil {
		logging.Info("Updating PR #%d", existingPR.GetNumber())
		existingPR.Title = &title
		existingPR.Body = &body
		_, _, err = prClient.PullRequests.Edit(ctx, owner, repo, existingPR.GetNumber(), existingPR)
		if err != nil {
			return fmt.Errorf("failed to update PR: %w", err)
		}
		g.SetPRLabels(ctx, owner, repo, existingPR.GetNumber(), labelTypes, existingPR.Labels, labels)
		logging.Info("PR updated: %s", existingPR.GetHTMLURL())
	} else {
		logging.Info("Creating PR")
		targetBaseBranch := environment.GetTargetBaseBranch()
		if strings.HasPrefix(targetBaseBranch, "refs/") {
			targetBaseBranch = strings.TrimPrefix(targetBaseBranch, "refs/heads/")
		}

		pr, _, err := prClient.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
			Title:               github.String(title),
			Body:                github.String(body),
			Head:                github.String(branchName),
			Base:                github.String(targetBaseBranch),
			MaintainerCanModify: github.Bool(true),
		})
		if err != nil {
			messageSuffix := ""
			if strings.Contains(err.Error(), "GitHub Actions is not permitted to create or approve pull requests") {
				messageSuffix += "\nNavigate to Settings > Actions > Workflow permissions and ensure that allow GitHub Actions to create and approve pull requests is checked."
			}
			return fmt.Errorf("failed to create PR: %w%s", err, messageSuffix)
		}
		if pr != nil && len(labels) > 0 {
			g.SetPRLabels(ctx, owner, repo, pr.GetNumber(), labelTypes, pr.Labels, labels)
		}
		logging.Info("PR created: %s", pr.GetHTMLURL())
	}

	return nil
}

func findPRForBranch(ctx context.Context, client *github.Client, owner, repo, branch string) *github.PullRequest {
	prs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
		Head:  owner + ":" + branch,
		State: "open",
	})
	if err != nil {
		logging.Debug("failed to list PRs: %v", err)
		return nil
	}
	if len(prs) > 0 {
		return prs[0]
	}
	return nil
}
