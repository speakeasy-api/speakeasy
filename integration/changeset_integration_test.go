//go:build integration

package integration_tests

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/v63/github"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/changeset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangesetE2E_FeaturePRs_UpdateReleasePRAndRelease(t *testing.T) {
	requireAcceptanceTest(t)

	token := getAcceptanceToken(t)
	apiKey := getAcceptanceAPIKey(t)
	client := newAcceptanceGitHubClient(token)

	baseBranch := "test-changeset-e2e-" + strings.ToLower(randStringBytes(8))
	seedVersion, tagPrefix := uniqueAcceptanceVersionPrefix(t)

	t.Cleanup(func() { cleanupAcceptanceBranches(t, token, baseBranch) })
	t.Cleanup(func() { cleanupAcceptanceReleases(t, token, tagPrefix) })

	setupAcceptanceChangesetBase(t, token, apiKey, baseBranch, seedVersion)

	featureOne := prepareFeatureChangesetPR(
		t,
		client,
		token,
		apiKey,
		baseBranch,
		baseBranch+"-feature-status",
		writeAcceptanceSpecWithStatusFeature,
		func(t *testing.T, dir string) {
			insertGoCustomCodeRegionCode(t, filepath.Join(dir, "models", "components", "statusresponse.go"), `
func (s *StatusResponse) String() string {
	return "status-response-custom"
}
`)
		},
		"feat: add status feature",
		"Adds the `status` subSDK, a new response model, and custom code in that model.",
	)
	t.Logf("feature PR body #1:\n%s", featureOne.PR.GetBody())
	featureOneChangesetRef := filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(featureOne.ChangesetPath)))
	featureOneChangeset := getAcceptanceChangesetFromRef(t, client, featureOne.Branch, featureOneChangesetRef)
	featureOneStatusEntry, ok := featureOneChangeset.CustomFiles["models/components/statusresponse.go"]
	require.True(t, ok, "expected first feature changeset to capture custom model code")
	assert.NotEmpty(t, changeset.ClaimID("models/components/statusresponse.go", featureOneStatusEntry))
	assert.NotEmpty(t, featureOneStatusEntry.LineageGitObject)
	assert.NotEmpty(t, featureOneStatusEntry.LineageRef)
	mergeAcceptancePR(t, client, featureOne.PR)
	waitForAcceptanceFileOnRef(t, client, baseBranch, featureOneChangesetRef)

	runChangesetReleaseOnBranch(t, token, apiKey, baseBranch)

	releaseBranch := "speakeasy-changeset-release-" + baseBranch
	releasePR := findAcceptancePRByHeadRef(t, client, releaseBranch)
	require.NotNil(t, releasePR, "expected a changeset release PR after the first feature PR merged")
	assert.Contains(t, releasePR.GetBody(), "feat: add status feature")
	t.Logf("changeset release PR body after first merge:\n%s", releasePR.GetBody())

	featureTwo := prepareFeatureChangesetPR(
		t,
		client,
		token,
		apiKey,
		baseBranch,
		baseBranch+"-feature-history",
		writeAcceptanceSpecWithStatusHistoryFeature,
		func(t *testing.T, dir string) {
			insertGoCustomCodeRegionCode(t, filepath.Join(dir, "models", "components", "statushistoryresponse.go"), `
func (s *StatusHistoryResponse) HistoryLabel() string {
	return "status-history-custom"
}
`)
			insertGoCustomCodeRegionCode(t, filepath.Join(dir, "status.go"), `
func (s *Status) HistoryLabel() string {
	return "status-subsdk-history-custom"
}
`)
		},
		"feat: extend status subSDK",
		"Adds another operation and model to the same `status` subSDK and custom code in both the model and `status.go`.",
	)
	t.Logf("feature PR body #2:\n%s", featureTwo.PR.GetBody())
	featureTwoChangesetRef := filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(featureTwo.ChangesetPath)))
	featureTwoChangeset := getAcceptanceChangesetFromRef(t, client, featureTwo.Branch, featureTwoChangesetRef)
	featureTwoStatusEntry, ok := featureTwoChangeset.CustomFiles["status.go"]
	require.True(t, ok, "expected second feature changeset to capture status.go customization")
	assert.NotEmpty(t, changeset.ClaimID("status.go", featureTwoStatusEntry))
	mergeAcceptancePR(t, client, featureTwo.PR)
	waitForAcceptanceFileOnRef(t, client, baseBranch, featureTwoChangesetRef)

	runChangesetReleaseOnBranch(t, token, apiKey, baseBranch)

	updatedReleasePR := findAcceptancePRByHeadRef(t, client, releaseBranch)
	require.NotNil(t, updatedReleasePR, "expected the same changeset release PR to remain open")
	require.Equal(t, releasePR.GetNumber(), updatedReleasePR.GetNumber(), "changeset release should update the existing PR")
	assert.Contains(t, updatedReleasePR.GetBody(), "feat: add status feature")
	assert.Contains(t, updatedReleasePR.GetBody(), "feat: extend status subSDK")
	t.Logf("changeset release PR body after second merge:\n%s", updatedReleasePR.GetBody())

	mergeAcceptancePR(t, client, updatedReleasePR)

	releaseWorkspace := cloneAcceptanceBranch(t, token, baseBranch)
	setupAcceptanceEnvironment(t, releaseWorkspace, token, baseBranch)
	t.Setenv("SPEAKEASY_API_KEY", apiKey)

	err := executeCIWithEnv(t, releaseWorkspace, nil, "release", "--github-access-token", token, "--working-directory", ".", "--enable-sdk-changelog", "true").Run()
	require.NoError(t, err, "ci release should succeed after merging the changeset release PR")

	releases := listAcceptanceReleasesByTagPrefix(t, client, tagPrefix)
	require.NotEmpty(t, releases, "expected a GitHub release with prefix %s", tagPrefix)

	releaseBody := releases[0].GetBody()
	require.NotEmpty(t, releaseBody, "release body should not be empty")
	assert.Contains(t, releaseBody, "Generated by Speakeasy CLI")
	assert.Contains(t, releaseBody, "### Changes")
	t.Logf("release body (%s):\n%s", releases[0].GetTagName(), releaseBody)
}

func TestChangesetE2E_ValidateRejectsSameFileCustomLease(t *testing.T) {
	requireAcceptanceTest(t)

	token := getAcceptanceToken(t)
	apiKey := getAcceptanceAPIKey(t)
	client := newAcceptanceGitHubClient(token)

	baseBranch := "test-changeset-lease-" + strings.ToLower(randStringBytes(8))
	seedVersion, _ := uniqueAcceptanceVersionPrefix(t)

	t.Cleanup(func() { cleanupAcceptanceBranches(t, token, baseBranch) })

	setupAcceptanceChangesetBase(t, token, apiKey, baseBranch, seedVersion)

	featureOne := prepareFeatureChangesetPR(
		t,
		client,
		token,
		apiKey,
		baseBranch,
		baseBranch+"-feature-status",
		writeAcceptanceSpecWithStatusFeature,
		func(t *testing.T, dir string) {
			insertGoCustomCodeRegionCode(t, filepath.Join(dir, "models", "components", "statusresponse.go"), `
func (s *StatusResponse) String() string {
	return "lease-status-response-custom"
}
`)
		},
		"feat: add status feature",
		"Adds the initial `status` subSDK and custom model code.",
	)
	mergeAcceptancePR(t, client, featureOne.PR)
	waitForAcceptanceFileOnRef(t, client, baseBranch, filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(featureOne.ChangesetPath))))

	featureTwo := prepareFeatureChangesetPR(
		t,
		client,
		token,
		apiKey,
		baseBranch,
		baseBranch+"-feature-history",
		writeAcceptanceSpecWithStatusHistoryFeature,
		func(t *testing.T, dir string) {
			insertGoCustomCodeRegionCode(t, filepath.Join(dir, "models", "components", "statushistoryresponse.go"), `
func (s *StatusHistoryResponse) HistoryLabel() string {
	return "lease-status-history-custom"
}
`)
			insertGoCustomCodeRegionCode(t, filepath.Join(dir, "status.go"), `
func (s *Status) HistoryLabel() string {
	return "lease-status-subsdk-history"
}
`)
		},
		"feat: extend status subSDK",
		"Adds a second `status` operation and custom code in `status.go`.",
	)
	featureTwoChangesetRef := filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(featureTwo.ChangesetPath)))
	featureTwoChangeset := getAcceptanceChangesetFromRef(t, client, featureTwo.Branch, featureTwoChangesetRef)
	featureTwoStatusEntry, ok := featureTwoChangeset.CustomFiles["status.go"]
	require.True(t, ok, "expected second feature changeset to capture status.go customization")
	featureTwoStatusClaimID := changeset.ClaimID("status.go", featureTwoStatusEntry)
	assert.NotEmpty(t, featureTwoStatusClaimID)

	featureThree := prepareFeatureChangesetPR(
		t,
		client,
		token,
		apiKey,
		baseBranch,
		baseBranch+"-feature-summary",
		writeAcceptanceSpecWithStatusSummaryFeature,
		func(t *testing.T, dir string) {
			insertGoCustomCodeRegionCode(t, filepath.Join(dir, "models", "components", "statussummaryresponse.go"), `
func (s *StatusSummaryResponse) SummaryLabel() string {
	return "lease-status-summary-custom"
}
`)
			insertGoCustomCodeRegionCode(t, filepath.Join(dir, "status.go"), `
func (s *Status) SummaryLabel() string {
	return "lease-status-subsdk-summary"
}
`)
		},
		"feat: add another status customization",
		"Adds another `status` operation and customizes the same `status.go` file from a stale branch.",
	)

	mergeAcceptancePR(t, client, featureTwo.PR)
	waitForAcceptanceFileOnRef(t, client, baseBranch, filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(featureTwo.ChangesetPath))))

	validationWorkspace := cloneAcceptanceBranch(t, token, featureTwo.Branch)
	copyAbsoluteFile(
		t,
		featureThree.ChangesetPath,
		filepath.Join(validationWorkspace, ".speakeasy", "changesets", filepath.Base(featureThree.ChangesetPath)),
	)
	setupAcceptanceEnvironment(t, validationWorkspace, token, featureTwo.Branch)

	output, err := runCIWithOutput(
		t,
		validationWorkspace,
		nil,
		"changeset-validate",
		"--working-directory", ".",
	)
	require.Error(t, err, "changeset validation should fail when two visible changesets customize the same sdk file")
	assert.Contains(t, output, "Custom lineage conflict for status.go")

	rebasedFeatureThree := rebaseAndRerunFeatureBranch(
		t,
		token,
		apiKey,
		baseBranch,
		featureThree.Branch,
		writeAcceptanceSpecWithStatusHistoryAndSummaryFeature,
		func(t *testing.T, dir string) {
			insertGoCustomCodeRegionCode(t, filepath.Join(dir, "models", "components", "statussummaryresponse.go"), `
func (s *StatusSummaryResponse) SummaryLabel() string {
	return "lease-status-summary-custom"
}
`)
			insertGoCustomCodeRegionCode(t, filepath.Join(dir, "status.go"), `
func (s *Status) SummaryLabel() string {
	return "lease-status-subsdk-summary"
}
`)
		},
	)
	rebasedFeatureThreeChangesetRef := filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(rebasedFeatureThree.ChangesetPath)))
	rebasedFeatureThreeChangeset := getAcceptanceChangesetFromRef(t, client, rebasedFeatureThree.Branch, rebasedFeatureThreeChangesetRef)
	rebasedStatusEntry, ok := rebasedFeatureThreeChangeset.CustomFiles["status.go"]
	require.True(t, ok, "expected rebased feature changeset to capture status.go customization")
	assert.Equal(t, featureTwoStatusClaimID, rebasedStatusEntry.ParentClaimID)
	assert.Equal(t, featureTwoStatusClaimID, rebasedStatusEntry.ParentLineageID)

	validationWorkspace = cloneAcceptanceBranch(t, token, rebasedFeatureThree.Branch)
	setupAcceptanceEnvironment(t, validationWorkspace, token, rebasedFeatureThree.Branch)

	err = executeCIWithEnv(
		t,
		validationWorkspace,
		nil,
		"changeset-validate",
		"--working-directory", ".",
	).Run()
	require.NoError(t, err, "changeset validation should succeed after rebasing and rerunning generation")
}

func TestChangesetE2E_ReleaseWritesPatchFilesAndSummarizesThem(t *testing.T) {
	requireAcceptanceTest(t)

	token := getAcceptanceToken(t)
	apiKey := getAcceptanceAPIKey(t)
	client := newAcceptanceGitHubClient(token)

	baseBranch := "test-changeset-patchfiles-" + strings.ToLower(randStringBytes(8))

	t.Cleanup(func() { cleanupAcceptanceBranches(t, token, baseBranch) })

	setupAcceptanceChangesetBaseWithOptions(t, token, apiKey, baseBranch, "0.1.0", false)

	baseWorkspace := cloneAcceptanceBranch(t, token, baseBranch)
	enableAcceptancePatchFilesInRootGenYAML(t, baseWorkspace)
	runAcceptanceGit(t, baseWorkspace, "add", "gen.yaml")
	runAcceptanceGit(t, baseWorkspace, "commit", "-m", "ci: enable patch files")
	runAcceptanceGit(t, baseWorkspace, "push", "--force", "origin", baseBranch)

	feature := prepareFeatureChangesetPR(
		t,
		client,
		token,
		apiKey,
		baseBranch,
		baseBranch+"-feature-status-patch",
		writeAcceptanceSpecWithStatusFeature,
		func(t *testing.T, dir string) {
			appendGoDeclaration(t, filepath.Join(dir, "status.go"), `
func (s *Status) PatchFilesLabel() string {
	return "patch-files-live"
}
`)
		},
		"feat: add status patch-file customization",
		"Adds the `status` subSDK and a raw generated-file customization that should collapse into a release patch file.",
	)
	mergeAcceptancePR(t, client, feature.PR)
	featureChangesetRef := filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(feature.ChangesetPath)))
	statusEntry := requireAcceptanceChangesetCustomFile(t, client, feature.Branch, featureChangesetRef, "status.go")
	assert.Contains(t, statusEntry.ClaimID, "patch:")
	assert.NotEmpty(t, statusEntry.Patch)
	assert.NotEmpty(t, statusEntry.LineageGitObject)
	assert.NotEmpty(t, statusEntry.LineageRef)
	rawFeatureChangeset := getAcceptanceFileContentFromRef(t, client, feature.Branch, featureChangesetRef)
	assert.Contains(t, rawFeatureChangeset, "patch: |")
	assert.Contains(t, rawFeatureChangeset, "\n      diff --git a/status.go b/status.go\n")

	mergeAcceptancePR(t, client, feature.PR)
	waitForAcceptanceFileOnRef(t, client, baseBranch, featureChangesetRef)

	runChangesetReleaseOnBranch(t, token, apiKey, baseBranch)

	releaseBranch := "speakeasy-changeset-release-" + baseBranch
	releasePR := findAcceptancePRByHeadRef(t, client, releaseBranch)
	require.NotNil(t, releasePR, "expected a changeset release PR")
	assert.Contains(t, releasePR.GetBody(), "| File | Delta |")
	assert.Contains(t, releasePR.GetBody(), "| `status.go.patch` |")
	assert.Contains(t, releasePR.GetBody(), "| Changeset | Branch | Bump | Summary |")

	prFiles := listAcceptancePRFiles(t, client, releasePR.GetNumber())
	patchPRFiles := filterAcceptancePatchFiles(prFiles)
	assert.Equal(t, []string{".speakeasy/patches/status.go.patch"}, patchPRFiles)

	patchContent := getAcceptanceFileContentFromRef(t, client, releaseBranch, ".speakeasy/patches/status.go.patch")
	assert.Contains(t, patchContent, "PatchFilesLabel")
	assert.Contains(t, patchContent, "patch-files-live")
	assert.NotContains(t, patchContent, "@generated-id:")

	genLockContent := getAcceptanceFileContentFromRef(t, client, releaseBranch, ".speakeasy/gen.lock")
	assert.NotContains(t, genLockContent, "customCodeRegions")
	t.Logf("changeset release PR body with patch files:\n%s", releasePR.GetBody())

	mergeAcceptancePR(t, client, releasePR)

	replayWorkspace := cloneAcceptanceBranch(t, token, baseBranch)
	scrubAcceptanceLockPersistentState(t, replayWorkspace)

	writeAcceptanceSpecWithStatusHistoryFeature(t, replayWorkspace)

	runAcceptanceSpeakeasy(t, replayWorkspace, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console")

	statusPath := filepath.Join(replayWorkspace, "status.go")
	replayedStatus, err := os.ReadFile(statusPath)
	require.NoError(t, err)
	assert.Contains(t, string(replayedStatus), `func (s *Status) PatchFilesLabel() string {`)
	assert.Contains(t, string(replayedStatus), `return "patch-files-live"`)
	assert.Contains(t, string(replayedStatus), "GetStatusHistory")
}

func TestChangesetE2E_PatchFiles_SerializesSameFileSuccessorClaims(t *testing.T) {
	requireAcceptanceTest(t)

	token := getAcceptanceToken(t)
	apiKey := getAcceptanceAPIKey(t)
	client := newAcceptanceGitHubClient(token)

	baseBranch := "test-changeset-patch-chain-" + strings.ToLower(randStringBytes(8))

	t.Cleanup(func() { cleanupAcceptanceBranches(t, token, baseBranch) })

	setupAcceptanceChangesetBaseWithOptions(t, token, apiKey, baseBranch, "0.1.0", false)

	baseWorkspace := cloneAcceptanceBranch(t, token, baseBranch)
	enableAcceptancePatchFilesInRootGenYAML(t, baseWorkspace)
	runAcceptanceGit(t, baseWorkspace, "add", "gen.yaml")
	runAcceptanceGit(t, baseWorkspace, "commit", "-m", "ci: enable patch files")
	runAcceptanceGit(t, baseWorkspace, "push", "--force", "origin", baseBranch)

	featureOne := prepareFeatureChangesetPR(
		t,
		client,
		token,
		apiKey,
		baseBranch,
		baseBranch+"-feature-status-first",
		writeAcceptanceSpecWithStatusFeature,
		func(t *testing.T, dir string) {
			appendGoDeclaration(t, filepath.Join(dir, "status.go"), `
func (s *Status) FirstPatchChainLabel() string {
	return "patch-chain-first"
}
`)
		},
		"feat: first patch-chain customization",
		"Adds the `status` subSDK and the first raw generated-file customization in `status.go`.",
	)

	featureOneChangesetRef := filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(featureOne.ChangesetPath)))
	featureOneStatusEntry := requireAcceptanceChangesetCustomFile(t, client, featureOne.Branch, featureOneChangesetRef, "status.go")
	featureOneClaimID := changeset.ClaimID("status.go", featureOneStatusEntry)
	require.NotEmpty(t, featureOneClaimID)
	assert.Contains(t, featureOneStatusEntry.ClaimID, "patch:")
	assert.Contains(t, featureOneStatusEntry.Patch, "FirstPatchChainLabel")

	featureTwo := prepareFeatureChangesetPR(
		t,
		client,
		token,
		apiKey,
		baseBranch,
		baseBranch+"-feature-status-second",
		writeAcceptanceSpecWithStatusHistoryFeature,
		func(t *testing.T, dir string) {
			appendGoDeclaration(t, filepath.Join(dir, "status.go"), `
func (s *Status) SecondPatchChainLabel() string {
	return "patch-chain-second"
}
`)
		},
		"feat: second patch-chain customization",
		"Starts from a stale branch, adds another `status.go` customization plus a new status operation.",
	)

	mergeAcceptancePR(t, client, featureOne.PR)
	waitForAcceptanceFileOnRef(t, client, baseBranch, featureOneChangesetRef)

	rebasedFeatureTwo := rebaseAndRerunFeatureBranch(
		t,
		token,
		apiKey,
		baseBranch,
		featureTwo.Branch,
		writeAcceptanceSpecWithStatusHistoryFeature,
		func(t *testing.T, dir string) {
			appendGoDeclaration(t, filepath.Join(dir, "status.go"), `
func (s *Status) SecondPatchChainLabel() string {
	return "patch-chain-second"
}
`)
		},
	)

	rebasedChangesetRef := filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(rebasedFeatureTwo.ChangesetPath)))
	rebasedStatusEntry := requireAcceptanceChangesetCustomFile(t, client, rebasedFeatureTwo.Branch, rebasedChangesetRef, "status.go")
	assert.Equal(t, featureOneClaimID, rebasedStatusEntry.ParentClaimID)
	assert.Equal(t, featureOneClaimID, rebasedStatusEntry.ParentLineageID)
	assert.Contains(t, rebasedStatusEntry.ClaimID, "patch:")
	assert.Contains(t, rebasedStatusEntry.Patch, "SecondPatchChainLabel")
	assert.NotContains(t, rebasedStatusEntry.Patch, "FirstPatchChainLabel", "successor patch should be incremental relative to the parent claim")

	rebasedPR := findAcceptancePRForBranch(t, client, rebasedFeatureTwo.Branch)
	require.NotNil(t, rebasedPR, "expected rebased feature PR to remain open")
	mergeAcceptancePR(t, client, rebasedPR)
	waitForAcceptanceFileOnRef(t, client, baseBranch, rebasedChangesetRef)

	runChangesetReleaseOnBranch(t, token, apiKey, baseBranch)

	releaseBranch := "speakeasy-changeset-release-" + baseBranch
	releasePR := findAcceptancePRByHeadRef(t, client, releaseBranch)
	require.NotNil(t, releasePR, "expected a changeset release PR")

	patchContent := getAcceptanceFileContentFromRef(t, client, releaseBranch, ".speakeasy/patches/status.go.patch")
	assert.Contains(t, patchContent, "FirstPatchChainLabel")
	assert.Contains(t, patchContent, "SecondPatchChainLabel")
	assert.Contains(t, patchContent, "GetStatusHistory")
}

func TestChangesetE2E_ReleaseCapturesUncapturedMainlineEdit(t *testing.T) {
	requireAcceptanceTest(t)

	token := getAcceptanceToken(t)
	apiKey := getAcceptanceAPIKey(t)
	client := newAcceptanceGitHubClient(token)

	baseBranch := "test-changeset-uncaptured-main-" + strings.ToLower(randStringBytes(8))

	t.Cleanup(func() { cleanupAcceptanceBranches(t, token, baseBranch) })

	setupAcceptanceChangesetBaseWithOptions(t, token, apiKey, baseBranch, "0.1.0", false)

	baseWorkspace := cloneAcceptanceBranch(t, token, baseBranch)
	enableAcceptancePatchFilesInRootGenYAML(t, baseWorkspace)
	runAcceptanceGit(t, baseWorkspace, "add", "gen.yaml")
	runAcceptanceGit(t, baseWorkspace, "commit", "-m", "ci: enable patch files")
	runAcceptanceGit(t, baseWorkspace, "push", "--force", "origin", baseBranch)

	feature := prepareFeatureChangesetPR(
		t,
		client,
		token,
		apiKey,
		baseBranch,
		baseBranch+"-feature-status",
		writeAcceptanceSpecWithStatusFeature,
		func(t *testing.T, dir string) {},
		"feat: add status feature",
		"Adds the `status` subSDK without any captured custom code.",
	)
	mergeAcceptancePR(t, client, feature.PR)
	featureChangesetRef := filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(feature.ChangesetPath)))
	waitForAcceptanceFileOnRef(t, client, baseBranch, featureChangesetRef)

	mainWorkspace := cloneAcceptanceBranch(t, token, baseBranch)
	appendGoDeclaration(t, filepath.Join(mainWorkspace, "status.go"), `
func (s *Status) MainlineUncapturedLabel() string {
	return "mainline-uncaptured"
}
`)
	runAcceptanceGit(t, mainWorkspace, "add", "status.go")
	runAcceptanceGit(t, mainWorkspace, "commit", "-m", "ci: add uncaptured mainline edit")
	runAcceptanceGit(t, mainWorkspace, "push", "--force", "origin", baseBranch)

	runChangesetReleaseOnBranch(t, token, apiKey, baseBranch)

	releaseBranch := "speakeasy-changeset-release-" + baseBranch
	releasePR := findAcceptancePRByHeadRef(t, client, releaseBranch)
	require.NotNil(t, releasePR, "expected changeset release PR to capture uncaptured mainline edit")
	assert.Contains(t, releasePR.GetBody(), "| `status.go.patch` |")

	patchContent := getAcceptanceFileContentFromRef(t, client, releaseBranch, ".speakeasy/patches/status.go.patch")
	assert.Contains(t, patchContent, "MainlineUncapturedLabel")
	assert.Contains(t, patchContent, "mainline-uncaptured")
}

func TestChangesetE2E_ReleaseFailsClosedOnAmbiguousMainAndSucceedsAfterRerun(t *testing.T) {
	requireAcceptanceTest(t)

	token := getAcceptanceToken(t)
	apiKey := getAcceptanceAPIKey(t)
	client := newAcceptanceGitHubClient(token)

	baseBranch := "test-changeset-ambiguous-main-" + strings.ToLower(randStringBytes(8))

	t.Cleanup(func() { cleanupAcceptanceBranches(t, token, baseBranch) })

	setupAcceptanceChangesetBaseWithOptions(t, token, apiKey, baseBranch, "0.1.0", false)

	baseWorkspace := cloneAcceptanceBranch(t, token, baseBranch)
	enableAcceptancePatchFilesInRootGenYAML(t, baseWorkspace)
	runAcceptanceGit(t, baseWorkspace, "add", "gen.yaml")
	runAcceptanceGit(t, baseWorkspace, "commit", "-m", "ci: enable patch files")
	runAcceptanceGit(t, baseWorkspace, "push", "--force", "origin", baseBranch)

	featureOne := prepareFeatureChangesetPR(
		t,
		client,
		token,
		apiKey,
		baseBranch,
		baseBranch+"-feature-status-first",
		writeAcceptanceSpecWithStatusFeature,
		func(t *testing.T, dir string) {
			appendGoDeclaration(t, filepath.Join(dir, "status.go"), `
func (s *Status) FirstAmbiguousLabel() string {
	return "ambiguous-first"
}
`)
		},
		"feat: first ambiguous customization",
		"Adds the first `status.go` customization.",
	)
	featureOneChangesetRef := filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(featureOne.ChangesetPath)))
	featureOneStatusEntry := requireAcceptanceChangesetCustomFile(t, client, featureOne.Branch, featureOneChangesetRef, "status.go")
	featureOneClaimID := changeset.ClaimID("status.go", featureOneStatusEntry)
	require.NotEmpty(t, featureOneClaimID)

	featureTwo := prepareFeatureChangesetPR(
		t,
		client,
		token,
		apiKey,
		baseBranch,
		baseBranch+"-feature-status-second",
		writeAcceptanceSpecWithStatusHistoryFeature,
		func(t *testing.T, dir string) {
			appendGoDeclaration(t, filepath.Join(dir, "status.go"), `
func (s *Status) SecondAmbiguousLabel() string {
	return "ambiguous-second"
}
`)
		},
		"feat: second ambiguous customization",
		"Starts from a stale branch and customizes the same `status.go` file.",
	)

	mergeAcceptancePR(t, client, featureOne.PR)
	waitForAcceptanceFileOnRef(t, client, baseBranch, featureOneChangesetRef)

	ambiguousWorkspace := cloneAcceptanceBranch(t, token, baseBranch)
	rogueChangesetRef := filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(featureTwo.ChangesetPath)))
	rogueChangesetPath := filepath.Join(ambiguousWorkspace, ".speakeasy", "changesets", filepath.Base(featureTwo.ChangesetPath))
	copyAbsoluteFile(t, featureTwo.ChangesetPath, rogueChangesetPath)
	runAcceptanceGit(t, ambiguousWorkspace, "add", filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(featureTwo.ChangesetPath))))
	runAcceptanceGit(t, ambiguousWorkspace, "commit", "-m", "ci: introduce ambiguous visible changeset")
	runAcceptanceGit(t, ambiguousWorkspace, "push", "--force", "origin", baseBranch)
	waitForAcceptanceFileOnRef(t, client, baseBranch, rogueChangesetRef)

	releaseOutput, err := runChangesetReleaseWithOutputOnBranch(t, token, apiKey, baseBranch)
	require.Error(t, err, "changeset release should fail closed when main contains ambiguous visible claims")
	assert.Contains(t, releaseOutput, "Custom lineage conflict for status.go")

	releaseBranch := "speakeasy-changeset-release-" + baseBranch
	assert.Nil(t, findAcceptancePRByHeadRef(t, client, releaseBranch), "release PR should not be created on ambiguous main")

	repairBranch := baseBranch + "-repair-status"
	repairWorkspace := cloneAcceptanceBranchAs(t, token, baseBranch, repairBranch)
	require.NoError(t, os.Remove(filepath.Join(repairWorkspace, ".speakeasy", "changesets", filepath.Base(featureTwo.ChangesetPath))))

	writeAcceptanceSpecWithStatusHistoryFeature(t, repairWorkspace)
	runAcceptanceSpeakeasy(t, repairWorkspace, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--auto-yes", "--output", "console")
	appendGoDeclaration(t, filepath.Join(repairWorkspace, "status.go"), `
func (s *Status) SecondAmbiguousLabel() string {
	return "ambiguous-second"
}
`)
	runAcceptanceSpeakeasy(t, repairWorkspace, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--auto-yes", "--output", "console")

	runAcceptanceGit(t, repairWorkspace, "add", "-A")
	runAcceptanceGit(t, repairWorkspace, "commit", "-m", "ci: rerun and recapture status history customization")
	runAcceptanceGit(t, repairWorkspace, "push", "--force", "origin", repairBranch)

	repairPR := createAcceptancePullRequest(
		t,
		client,
		repairBranch,
		baseBranch,
		"feat: remediate ambiguous status customization",
		"Deletes the stale copied changeset, reruns generation from the latest base, and recaptures the second `status.go` customization.",
	)
	repairChangesetRef := filepath.ToSlash(filepath.Join(".speakeasy", "changesets", filepath.Base(mustFindBranchChangesetPath(t, repairWorkspace, repairBranch))))
	repairStatusEntry := requireAcceptanceChangesetCustomFile(t, client, repairBranch, repairChangesetRef, "status.go")
	assert.Equal(t, featureOneClaimID, repairStatusEntry.ParentClaimID)
	assert.Equal(t, featureOneClaimID, repairStatusEntry.ParentLineageID)
	assert.Contains(t, repairStatusEntry.Patch, "SecondAmbiguousLabel")
	assert.NotContains(t, repairStatusEntry.Patch, "FirstAmbiguousLabel")

	mergeAcceptancePR(t, client, repairPR)
	waitForAcceptanceFileOnRef(t, client, baseBranch, repairChangesetRef)

	runChangesetReleaseOnBranch(t, token, apiKey, baseBranch)

	releasePR := findAcceptancePRByHeadRef(t, client, releaseBranch)
	require.NotNil(t, releasePR, "expected changeset release PR after remediation")

	patchContent := getAcceptanceFileContentFromRef(t, client, releaseBranch, ".speakeasy/patches/status.go.patch")
	assert.Contains(t, patchContent, "FirstAmbiguousLabel")
	assert.Contains(t, patchContent, "SecondAmbiguousLabel")
	assert.Contains(t, patchContent, "GetStatusHistory")
}

type preparedFeatureChangeset struct {
	Branch        string
	PR            *github.PullRequest
	ChangesetPath string
}

func requireAcceptanceChangesetCustomFile(t *testing.T, client *github.Client, branch, changesetRef, customFile string) changeset.CustomFileEntry {
	t.Helper()

	cs := getAcceptanceChangesetFromRef(t, client, branch, changesetRef)
	entry, ok := cs.CustomFiles[customFile]
	require.True(t, ok, "expected changeset %s on %s to capture %s", changesetRef, branch, customFile)
	return entry
}

func setupAcceptanceChangesetBase(t *testing.T, token, apiKey, baseBranch, seedVersion string) {
	setupAcceptanceChangesetBaseWithOptions(t, token, apiKey, baseBranch, seedVersion, true)
}

func setupAcceptanceChangesetBaseWithOptions(t *testing.T, token, apiKey, baseBranch, seedVersion string, enableCustomCodeRegions bool) {
	t.Helper()

	dir := pushAcceptanceBranchWithSDK(t, token, baseBranch, apiKey, writeAcceptanceProjectFiles)

	setAcceptanceGenVersion(t, dir, seedVersion)
	runAcceptanceSpeakeasy(t, dir, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console")
	runAcceptanceGit(t, dir, "add", "-A")
	runAcceptanceGit(t, dir, "commit", "-m", "ci: seed unique release version")
	runAcceptanceGit(t, dir, "push", "--force", "origin", baseBranch)

	enablePersistentEditsInRootGenYAML(t, dir)
	if enableCustomCodeRegions {
		enableGoCustomCodeRegionsInRootGenYAML(t, dir)
	}
	runAcceptanceSpeakeasy(t, dir, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console")
	runAcceptanceGit(t, dir, "add", "-A")
	commitMessage := "ci: enable persistent edits"
	if enableCustomCodeRegions {
		commitMessage += " and code regions"
	}
	runAcceptanceGit(t, dir, "commit", "-m", commitMessage)
	runAcceptanceGit(t, dir, "push", "--force", "origin", baseBranch)

	enableAcceptanceChangesetVersionStrategy(t, dir)
	runAcceptanceGit(t, dir, "add", "gen.yaml")
	runAcceptanceGit(t, dir, "commit", "-m", "ci: enable changeset mode")
	runAcceptanceGit(t, dir, "push", "--force", "origin", baseBranch)
}

func prepareFeatureChangesetPR(
	t *testing.T,
	client *github.Client,
	token string,
	apiKey string,
	baseBranch string,
	featureBranch string,
	writeSpec func(*testing.T, string),
	customize func(*testing.T, string),
	title string,
	body string,
) preparedFeatureChangeset {
	t.Helper()

	dir := cloneAcceptanceBranchAs(t, token, baseBranch, featureBranch)
	writeSpec(t, dir)
	runAcceptanceSpeakeasy(t, dir, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--auto-yes", "--output", "console")
	customize(t, dir)
	runAcceptanceSpeakeasy(t, dir, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--auto-yes", "--output", "console")

	runAcceptanceGit(t, dir, "add", "-A")
	runAcceptanceGit(t, dir, "commit", "-m", title)
	runAcceptanceGit(t, dir, "push", "--force", "origin", featureBranch)

	return preparedFeatureChangeset{
		Branch:        featureBranch,
		PR:            createAcceptancePullRequest(t, client, featureBranch, baseBranch, title, body),
		ChangesetPath: mustFindBranchChangesetPath(t, dir, featureBranch),
	}
}

func rebaseAndRerunFeatureBranch(
	t *testing.T,
	token string,
	apiKey string,
	baseBranch string,
	featureBranch string,
	writeSpec func(*testing.T, string),
	customize func(*testing.T, string),
) preparedFeatureChangeset {
	t.Helper()

	dir := cloneAcceptanceBranch(t, token, baseBranch)
	runAcceptanceGit(t, dir, "checkout", "-B", featureBranch)
	writeSpec(t, dir)
	runAcceptanceSpeakeasy(t, dir, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--auto-yes", "--output", "console")
	customize(t, dir)
	runAcceptanceSpeakeasy(t, dir, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--auto-yes", "--output", "console")

	runAcceptanceGit(t, dir, "add", "-A")
	runAcceptanceGit(t, dir, "commit", "-m", "ci: rerun after rebase")
	runAcceptanceGit(t, dir, "push", "--force", "origin", featureBranch)

	return preparedFeatureChangeset{
		Branch:        featureBranch,
		ChangesetPath: mustFindBranchChangesetPath(t, dir, featureBranch),
	}
}

func runChangesetReleaseOnBranch(t *testing.T, token, apiKey, branchName string) {
	t.Helper()

	workspace := cloneAcceptanceBranch(t, token, branchName)
	setupAcceptanceEnvironment(t, workspace, token, branchName)
	t.Setenv("SPEAKEASY_API_KEY", apiKey)

	err := executeCIWithEnv(
		t,
		workspace,
		nil,
		"changeset-release",
		"--github-access-token", token,
		"--working-directory", ".",
		"--skip-compile",
		"--skip-testing",
		"--enable-sdk-changelog", "true",
	).Run()
	require.NoError(t, err, "ci changeset-release should succeed")
}

func runChangesetReleaseWithOutputOnBranch(t *testing.T, token, apiKey, branchName string) (string, error) {
	t.Helper()

	workspace := cloneAcceptanceBranch(t, token, branchName)
	setupAcceptanceEnvironment(t, workspace, token, branchName)
	t.Setenv("SPEAKEASY_API_KEY", apiKey)

	return runCIWithOutput(
		t,
		workspace,
		nil,
		"changeset-release",
		"--github-access-token", token,
		"--working-directory", ".",
		"--skip-compile",
		"--skip-testing",
		"--enable-sdk-changelog", "true",
	)
}

func writeAcceptanceSpecWithStatusFeature(t *testing.T, dir string) {
	t.Helper()
	writeAcceptanceSpecFile(t, dir, statusFeatureSpec(false, false))
}

func writeAcceptanceSpecWithStatusHistoryFeature(t *testing.T, dir string) {
	t.Helper()
	writeAcceptanceSpecFile(t, dir, statusFeatureSpec(true, false))
}

func writeAcceptanceSpecWithStatusSummaryFeature(t *testing.T, dir string) {
	t.Helper()
	writeAcceptanceSpecFile(t, dir, statusFeatureSpec(false, true))
}

func writeAcceptanceSpecWithStatusHistoryAndSummaryFeature(t *testing.T, dir string) {
	t.Helper()
	writeAcceptanceSpecFile(t, dir, statusFeatureSpec(true, true))
}

func writeAcceptanceSpecFile(t *testing.T, dir, specContent string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "openapi.yaml"), []byte(specContent), 0o644))
}

func statusFeatureSpec(includeHistory, includeSummary bool) string {
	historyPath := ""
	historySchema := ""
	if includeHistory {
		historyPath = `
  /status/history:
    get:
      tags: [status]
      operationId: getStatusHistory
      summary: Get status history
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/StatusHistoryResponse'
`
		historySchema = `
    StatusHistoryResponse:
      type: object
      properties:
        history:
          type: array
          items:
            type: string
`
	}

	summaryPath := ""
	summarySchema := ""
	if includeSummary {
		summaryPath = `
  /status/summary:
    get:
      tags: [status]
      operationId: getStatusSummary
      summary: Get status summary
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/StatusSummaryResponse'
`
		summarySchema = `
    StatusSummaryResponse:
      type: object
      properties:
        summary:
          type: string
`
	}

	return `openapi: "3.0.0"
info:
  title: Test API
  version: "1.0.0"
paths:
  /health:
    get:
      operationId: getHealth
      responses:
        "200":
          description: OK
  /status:
    get:
      tags: [status]
      operationId: getStatus
      summary: Get service status
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/StatusResponse'
` + historyPath + summaryPath + `
components:
  schemas:
    StatusResponse:
      type: object
      properties:
        status:
          type: string
` + historySchema + summarySchema
}

func insertGoCustomCodeRegionCode(t *testing.T, filePath, declaration string) {
	t.Helper()

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	updated := string(content)
	if strings.Contains(updated, declaration) {
		return
	}

	regionStart := strings.Index(updated, "// #region sdk-class-body")
	regionEndMarker := "// #endregion sdk-class-body"
	if regionStart == -1 {
		regionStart = strings.Index(updated, "// #region class-body-")
		if regionStart != -1 {
			lineEnd := strings.Index(updated[regionStart:], "\n")
			require.NotEqual(t, -1, lineEnd, "class-body region start should end with newline in %s", filePath)
			regionName := strings.TrimSpace(strings.TrimPrefix(updated[regionStart:regionStart+lineEnd], "// #region "))
			regionEndMarker = "// #endregion " + regionName
		}
	}
	if regionStart != -1 {
		regionEnd := strings.Index(updated, regionEndMarker)
		require.NotEqual(t, -1, regionEnd, "expected matching custom code region end in %s", filePath)
		code := strings.TrimSpace(declaration)
		updated = updated[:regionEnd] + code + "\n" + updated[regionEnd:]
	} else {
		updated = strings.TrimRight(updated, "\n") + "\n\n" + strings.TrimSpace(declaration) + "\n"
	}
	require.NoError(t, os.WriteFile(filePath, []byte(updated), 0o644))
}

func copyAbsoluteFile(t *testing.T, src, dst string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))

	in, err := os.Open(src)
	require.NoError(t, err)
	defer in.Close()

	out, err := os.Create(dst)
	require.NoError(t, err)
	defer out.Close()

	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Close())
}

func mustFindBranchChangesetPath(t *testing.T, dir, branchName string) string {
	t.Helper()

	loaded, err := changeset.LoadAll(dir)
	require.NoError(t, err)
	require.NotEmpty(t, loaded, "expected at least one changeset in %s", dir)

	for i := len(loaded) - 1; i >= 0; i-- {
		if loaded[i].Branch == branchName {
			return filepath.Join(dir, ".speakeasy", "changesets", loaded[i].Filename())
		}
	}

	t.Fatalf("expected a changeset for branch %s in %s", branchName, dir)
	return ""
}

func runCIWithOutput(t *testing.T, wd string, envOverrides map[string]string, args ...string) (string, error) {
	t.Helper()

	binaryPath, err := ensureBinary()
	require.NoError(t, err, "failed to build speakeasy binary")

	baseEnv := map[string]string{
		"GITHUB_WORKSPACE":        wd,
		"GITHUB_OUTPUT":           filepath.Join(wd, "github-output.txt"),
		"GITHUB_SERVER_URL":       defaultString(os.Getenv("GITHUB_SERVER_URL"), "https://github.com"),
		"GITHUB_REPOSITORY":       defaultString(os.Getenv("GITHUB_REPOSITORY"), "test-org/test-repo"),
		"GITHUB_REPOSITORY_OWNER": defaultString(os.Getenv("GITHUB_REPOSITORY_OWNER"), "test-org"),
	}
	for key, value := range envOverrides {
		baseEnv[key] = value
	}

	cmd := exec.Command(binaryPath, append([]string{"ci"}, args...)...)
	cmd.Dir = wd
	cmd.Env = mergeEnv(os.Environ(), baseEnv)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err = cmd.Run()
	return output.String(), err
}
