package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/v63/github"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/ci/utils"
	"github.com/speakeasy-api/speakeasy/internal/ci/versionbumps"
	"github.com/speakeasy-api/versioning-reports/versioning"

	"github.com/speakeasy-api/speakeasy/internal/ci/configuration"
	"github.com/speakeasy-api/speakeasy/internal/ci/git"
	"github.com/speakeasy-api/speakeasy/internal/ci/run"

	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/speakeasy/internal/ci/releases"
	"github.com/speakeasy-api/speakeasy/internal/ci/tagbridge"
	"github.com/speakeasy-api/speakeasy/internal/ci/versioninfo"
)

func RunWorkflow(ctx context.Context) error {
	g, err := initAction()
	if err != nil {
		return err
	}

	if !environment.SkipCompile() {
		if err := SetupEnvironment(); err != nil {
			return fmt.Errorf("failed to setup environment: %w", err)
		}
	} else {
		logging.Info("Skipping environment setup due to skip_compile input")
	}

	// We ARE the CLI — no download needed. Use the current version.
	resolvedVersion := versioninfo.GetSpeakeasyVersion()

	// This flag is generally deprecated, it will not be provided on new action instances
	pinnedVersion := formatPinnedVersion(environment.GetPinnedSpeakeasyVersion())
	if pinnedVersion != "latest" {
		resolvedVersion = pinnedVersion
		// This environment variable is read by the CLI to determine which version should be used to execute `run`
		if err := environment.SetCLIVersionToUse(pinnedVersion); err != nil {
			return fmt.Errorf("failed to set pinned speakeasy version: %w", err)
		}
	}

	mode := environment.GetMode()

	wf, err := configuration.GetWorkflowAndValidateLanguages(true)
	if err != nil {
		return err
	}

	sourcesOnly := len(wf.Targets) == 0

	branchName := ""
	var pr *github.PullRequest
	if mode == environment.ModePR {
		var err error
		branchName, pr, err = g.FindExistingPR(environment.GetFeatureBranch(), environment.ActionRunWorkflow, sourcesOnly)
		if err != nil {
			return err
		}

		if pr != nil {
			os.Setenv("GH_PULL_REQUEST", *pr.URL)
		}
	}

	// We want to stay on main if we're pushing code samples because we want to tag the code samples with `main`
	if !environment.PushCodeSamplesOnly() && !environment.IsTestMode() {
		branchName, err = g.FindOrCreateBranch(branchName, environment.ActionRunWorkflow)
		if err != nil {
			return err
		}
	}

	success := false
	defer func() {
		if shouldDeleteBranch(success) {
			if err := g.DeleteBranch(branchName); err != nil {
				logging.Debug("failed to delete branch %s: %v", branchName, err)
			}
		}
	}()

	if branchName != "" {
		os.Setenv("SPEAKEASY_ACTIVE_BRANCH", branchName)
	}

	runRes, outputs, err := run.Run(ctx, g, pr, wf)
	if err != nil {
		if err := setOutputs(outputs); err != nil {
			logging.Debug("failed to set outputs: %v", err)
		}

		if environment.GetFeatureBranch() != "" {
			docVersion := ""
			var versionReport *versioning.MergedVersionReport
			if runRes != nil && runRes.GenInfo != nil {
				docVersion = runRes.GenInfo.OpenAPIDocVersion
			}
			if runRes != nil {
				versionReport = runRes.VersioningInfo.VersionReport
			}
			// Doc version and version report will typically be empty here.
			// For feature branches, we always commit to the branch so PRs can be manually resolved.
			// For generation failures, the commit may be empty which is ok - it will be overwritten on future generations.
			// For compilation failures or other failures, the generated code will be available in the failed feature branch.
			if _, err := g.CommitAndPush(docVersion, resolvedVersion, "", environment.ActionRunWorkflow, false, versionReport); err != nil {
				logging.Debug("failed to commit and push: %v", err)
				return err
			}
		}

		return err
	}

	anythingRegenerated := false

	var releaseInfo releases.ReleasesInfo
	runResultInfo, err := json.MarshalIndent(runRes, "", "  ")
	if err != nil {
		logging.Debug("failed to marshal runRes : %s\n", err)
	} else {
		logging.Debug("Result of running the command is: %s\n", runResultInfo)
	}
	if runRes.GenInfo != nil {
		docVersion := runRes.GenInfo.OpenAPIDocVersion
		resolvedVersion = runRes.GenInfo.SpeakeasyVersion

		releaseInfo = releases.ReleasesInfo{
			ReleaseTitle:       environment.GetInvokeTime().Format("2006-01-02 15:04:05"),
			DocVersion:         docVersion,
			SpeakeasyVersion:   resolvedVersion,
			GenerationVersion:  runRes.GenInfo.GenerationVersion,
			DocLocation:        environment.GetOpenAPIDocLocation(),
			Languages:          map[string]releases.LanguageReleaseInfo{},
			LanguagesGenerated: map[string]releases.GenerationInfo{},
		}

		for _, supportedTargetName := range generate.GetSupportedTargetNames() {
			langGenInfo, ok := runRes.GenInfo.Languages[supportedTargetName]
			if ok && outputs[utils.OutputTargetRegenerated(supportedTargetName)] == "true" {
				anythingRegenerated = true

				path := outputs[utils.OutputTargetDirectory(supportedTargetName)]
				path = strings.TrimPrefix(path, "./")

				releaseInfo.LanguagesGenerated[supportedTargetName] = releases.GenerationInfo{
					Version: langGenInfo.Version,
					Path:    path,
				}

				if published, ok := outputs[utils.OutputTargetPublish(supportedTargetName)]; ok && published == "true" {
					releaseInfo.Languages[supportedTargetName] = releases.LanguageReleaseInfo{
						PackageName: langGenInfo.PackageName,
						Version:     langGenInfo.Version,
						Path:        path,
					}
				}
			}
		}

		if environment.PushCodeSamplesOnly() {
			// If we're just pushing code samples we don't want to raise a PR
			return nil
		}

		releasesDir, err := getReleasesDir()
		if err != nil {
			return err
		}

		if err := releases.UpdateReleasesFile(releaseInfo, releasesDir); err != nil {
			logging.Error("error while updating releases file: %v", err.Error())
			return err
		}

		if _, err := g.CommitAndPush(docVersion, resolvedVersion, "", environment.ActionRunWorkflow, false, runRes.VersioningInfo.VersionReport); err != nil {
			return err
		}
	}

	outputs["resolved_speakeasy_version"] = resolvedVersion
	if sourcesOnly {
		if _, err := g.CommitAndPush("", resolvedVersion, "", environment.ActionRunWorkflow, sourcesOnly, nil); err != nil {
			return err
		}
	}

	// If test mode is successful to this point, exit here
	if environment.IsTestMode() {
		success = true
		return nil
	}

	if err := finalize(finalizeInputs{
		ctx:                  ctx,
		Outputs:              outputs,
		BranchName:           branchName,
		AnythingRegenerated:  anythingRegenerated,
		SourcesOnly:          sourcesOnly,
		Git:                  g,
		VersioningInfo:       runRes.VersioningInfo,
		LintingReportURL:     runRes.LintingReportURL,
		ChangesReportURL:     runRes.ChangesReportURL,
		OpenAPIChangeSummary: runRes.OpenAPIChangeSummary,
		GenInfo:              runRes.GenInfo,
		currentRelease:       &releaseInfo,
		releaseNotes:         runRes.ReleaseNotes,
	}); err != nil {
		return err
	}

	success = true

	return nil
}

func shouldDeleteBranch(success bool) bool {
	// Never delete when operating on a user-provided feature branch
	if environment.GetFeatureBranch() != "" {
		return false
	}

	// Keep branches during debug or test runs
	if environment.IsDebugMode() || environment.IsTestMode() {
		return false
	}

	// Delete in direct mode or when the run was unsuccessful
	return environment.GetMode() == environment.ModeDirect || !success
}

type finalizeInputs struct {
	ctx                  context.Context
	Outputs              map[string]string
	BranchName           string
	AnythingRegenerated  bool
	SourcesOnly          bool
	Git                  *git.Git
	LintingReportURL     string
	ChangesReportURL     string
	OpenAPIChangeSummary string
	VersioningInfo       versionbumps.VersioningInfo
	GenInfo              *run.GenerationInfo
	currentRelease       *releases.ReleasesInfo
	// key is language target name, value is release notes
	releaseNotes map[string]string
}

// Sets outputs and creates or adds releases info
func finalize(inputs finalizeInputs) error {
	// If nothing was regenerated, we don't need to do anything
	if !inputs.AnythingRegenerated && !inputs.SourcesOnly {
		return nil
	}

	branchName, err := inputs.Git.FindAndCheckoutBranch(inputs.BranchName)
	if err != nil {
		return err
	}

	defer func() {
		inputs.Outputs["branch_name"] = branchName

		if err := setOutputs(inputs.Outputs); err != nil {
			logging.Debug("failed to set outputs: %v", err)
		}
	}()

	logging.Info("getMode from the environment: %s\n", environment.GetMode())
	logging.Info("INPUT_ENABLE_SDK_CHANGELOG: %s", environment.GetSDKChangelog())
	switch environment.GetMode() {
	case environment.ModePR:
		branchName, pr, err := inputs.Git.FindExistingPR(branchName, environment.ActionFinalize, inputs.SourcesOnly)
		if err != nil {
			return err
		}
		pr, err = inputs.Git.CreateOrUpdatePR(git.PRInfo{
			BranchName:           branchName,
			ReleaseInfo:          inputs.currentRelease,
			PreviousGenVersion:   inputs.Outputs["previous_gen_version"],
			PR:                   pr,
			SourceGeneration:     inputs.SourcesOnly,
			LintingReportURL:     inputs.LintingReportURL,
			ChangesReportURL:     inputs.ChangesReportURL,
			VersioningInfo:       inputs.VersioningInfo,
			OpenAPIChangeSummary: inputs.OpenAPIChangeSummary,
		})

		if err != nil {
			return err
		}

		if pr != nil {
			os.Setenv("GH_PULL_REQUEST", *pr.URL)
		}

		// If we are in PR mode and testing should be triggered by this PR we will attempt to fire an empty commit from our app so trigger github actions checks
		// for more info on why this is necessary see https://github.com/peter-evans/create-pull-request/blob/main/docs/concepts-guidelines.md#workarounds-to-trigger-further-workflow-runs
		// If the customer has manually set up a PR_CREATION_PAT we will not do this
		if inputs.GenInfo != nil && inputs.GenInfo.HasTestingEnabled && os.Getenv("PR_CREATION_PAT") == "" {
			sanitizedBranchName := strings.TrimPrefix(branchName, "refs/heads/")
			if err := fireEmptyCommit(os.Getenv("GITHUB_REPOSITORY_OWNER"), git.GetRepo(), sanitizedBranchName); err != nil {
				fmt.Println("Failed to create empty commit to trigger testing workflow", err)
			}
		}

	case environment.ModeDirect:
		var releaseInfo *releases.ReleasesInfo
		var oldReleaseInfo string
		var languages map[string]releases.LanguageReleaseInfo
		var targetSpecificReleaseNotes releases.TargetReleaseNotes = nil
		if !inputs.SourcesOnly {
			releaseInfo = inputs.currentRelease
			languages = releaseInfo.Languages
			oldReleaseInfo = releaseInfo.String()
			logging.Info("release Notes: %+v", inputs.releaseNotes)
			if environment.GetSDKChangelog() == "true" && inputs.releaseNotes != nil {
				targetSpecificReleaseNotes = inputs.releaseNotes
			}

			// We still read from releases info for terraform generations since they use the goreleaser
			// Read from Releases.md for terraform generations
			if inputs.Outputs[utils.OutputTargetRegenerated("terraform")] == "true" {
				releaseInfo, err = getReleasesInfo()
				oldReleaseInfo = releaseInfo.String()
				targetSpecificReleaseNotes = nil
				languages = releaseInfo.Languages
				if err != nil {
					return err
				}
			}
		}

		commitHash, err := inputs.Git.MergeBranch(branchName)
		if err != nil {
			return err
		}

		// Skip releasing and tagging when configured to do so or when triggered by PR events
		if environment.ShouldSkipReleasing() {
			logging.Info("Skipping release creation and registry tagging - skip_release flag set or triggered by PR event")
			inputs.Outputs["commit_hash"] = commitHash
			return nil
		}

		if !inputs.SourcesOnly {
			if err := inputs.Git.CreateRelease(oldReleaseInfo, languages, inputs.Outputs, targetSpecificReleaseNotes); err != nil {
				return err
			}
		}

		inputs.Outputs["commit_hash"] = commitHash

		// add merging branch registry tag
		if err = addDirectModeBranchTagging(inputs.ctx); err != nil {
			return errors.Wrap(err, "failed to tag registry images")
		}

	}

	return nil
}

func addDirectModeBranchTagging(ctx context.Context) error {
	wf, err := configuration.GetWorkflowAndValidateLanguages(true)
	if err != nil {
		return err
	}

	branch := strings.TrimPrefix(os.Getenv("GITHUB_REF"), "refs/heads/")

	var sources, targets []string
	// a tag that is applied if the target contributing is published
	var isPublished bool
	// the tagging library treats targets synonymously with code samples
	if specificTarget := environment.SpecifiedTarget(); specificTarget != "" {
		if target, ok := wf.Targets[specificTarget]; ok {
			isPublished = target.IsPublished()
			if source, ok := wf.Sources[target.Source]; ok && source.Registry != nil {
				sources = append(sources, target.Source)
			}

			if target.CodeSamples != nil && target.CodeSamples.Registry != nil {
				targets = append(targets, specificTarget)
			}
		}
	} else {
		for name, target := range wf.Targets {
			isPublished = isPublished || target.IsPublished()
			if source, ok := wf.Sources[target.Source]; ok && source.Registry != nil {
				sources = append(sources, target.Source)
			}

			if target.CodeSamples != nil && target.CodeSamples.Registry != nil {
				targets = append(targets, name)
			}
		}
	}
	if (len(sources) > 0 || len(targets) > 0) && branch != "" {
		tags := []string{environment.SanitizeBranchName(branch)}
		if isPublished {
			tags = append(tags, "published")
		}
		return tagbridge.TagPromote(ctx, tags, sources, targets)
	}

	return nil
}

// formatPinnedVersion normalizes a pinned version string, prefixing with "v" if needed.
func formatPinnedVersion(pinnedVersion string) string {
	if pinnedVersion == "" {
		return "latest"
	}
	if pinnedVersion != "latest" && (len(pinnedVersion) == 0 || pinnedVersion[0] != 'v') {
		return "v" + pinnedVersion
	}
	return pinnedVersion
}

// fireEmptyCommit fires an empty commit via the Speakeasy API to trigger workflow checks.
func fireEmptyCommit(org, repo, branch string) error {
	type emptyCommitRequest struct {
		Branch   string `json:"branch"`
		Org      string `json:"org"`
		RepoName string `json:"repo_name"`
	}

	payload := emptyCommitRequest{
		Branch:   branch,
		Org:      org,
		RepoName: repo,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshalling request body: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.speakeasy.com/v1/github/empty_commit", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("error creating the request: %w", err)
	}

	apiKey := os.Getenv("SPEAKEASY_API_KEY")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making the API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	return nil
}

func getReleasesInfo() (*releases.ReleasesInfo, error) {
	releasesDir, err := getReleasesDir()
	if err != nil {
		return nil, err
	}

	releasesInfo, err := releases.GetLastReleaseInfo(releasesDir)
	if err != nil {
		return nil, err
	}

	return releasesInfo, nil
}
