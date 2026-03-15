//go:build integration

package integration_tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCIWorkflow_PRMode_E2E(t *testing.T) {
	requireAcceptanceTest(t)

	token := getAcceptanceToken(t)
	apiKey := getAcceptanceAPIKey(t)

	branchName := "test-integration-" + strings.ToLower(randStringBytes(8))
	t.Cleanup(func() { cleanupAcceptanceBranches(t, token, branchName) })

	pushAcceptanceOrphanBranch(t, token, branchName, writeAcceptanceProjectFiles)
	workspace := cloneAcceptanceBranch(t, token, branchName)

	setupAcceptanceEnvironment(t, workspace, token, branchName)
	t.Setenv("SPEAKEASY_API_KEY", apiKey)

	err := executeCIWithEnv(t, workspace, nil, "generate", "--mode", "pr", "--github-access-token", token, "--working-directory", ".", "--force", "--skip-compile", "--skip-testing", "--skip-release").Run()
	require.NoError(t, err, "ci generate should succeed")

	client := newAcceptanceGitHubClient(token)
	pr := findAcceptancePRForBranch(t, client, branchName)
	require.NotNil(t, pr, "expected a PR to be created")
	assert.Equal(t, branchName, pr.GetBase().GetRef(), "PR base branch should be the test branch")

	output := runAcceptanceGit(t, workspace, "config", "--local", "--get-regexp", `url\..*\.insteadOf`)
	assert.Contains(t, output, "https://github.com/", "git config should include an insteadOf rule for GitHub auth")
}

func TestCIWorkflow_PRMode_WithChangelog(t *testing.T) {
	requireAcceptanceTest(t)

	token := getAcceptanceToken(t)
	apiKey := getAcceptanceAPIKey(t)

	branchName := "test-integration-" + strings.ToLower(randStringBytes(8))
	t.Cleanup(func() { cleanupAcceptanceBranches(t, token, branchName) })

	dir := pushAcceptanceBranchWithSDK(t, token, branchName, apiKey, writeAcceptanceProjectFiles)
	writeAcceptanceUpdatedSpecWithNewOperation(t, dir)
	runAcceptanceGit(t, dir, "add", "-A")
	runAcceptanceGit(t, dir, "commit", "-m", "feat: add status endpoint")
	runAcceptanceGit(t, dir, "push", "--force", "origin", branchName)

	workspace := cloneAcceptanceBranch(t, token, branchName)
	setupAcceptanceEnvironment(t, workspace, token, branchName)
	t.Setenv("SPEAKEASY_API_KEY", apiKey)

	err := executeCIWithEnv(t, workspace, nil, "generate", "--mode", "pr", "--github-access-token", token, "--working-directory", ".", "--force", "--skip-compile", "--skip-testing", "--skip-release", "--enable-sdk-changelog", "true").Run()
	require.NoError(t, err, "ci generate should succeed")

	client := newAcceptanceGitHubClient(token)
	pr := findAcceptancePRForBranch(t, client, branchName)
	require.NotNil(t, pr, "expected a PR to be created")

	body := pr.GetBody()
	require.NotEmpty(t, body, "PR body should not be empty")
	assert.True(t, strings.Contains(strings.ToLower(body), "getstatus") || strings.Contains(strings.ToLower(body), "get_status"), "PR body should mention the added operation")
	assert.Contains(t, body, "Added", "PR body should indicate the operation was added")
}

func TestCIWorkflow_PRMode_PersistentEdits(t *testing.T) {
	requireAcceptanceTest(t)

	token := getAcceptanceToken(t)
	apiKey := getAcceptanceAPIKey(t)

	branchName := "test-integration-" + strings.ToLower(randStringBytes(8))
	t.Cleanup(func() { cleanupAcceptanceBranches(t, token, branchName) })

	dir := pushAcceptanceBranchWithSDK(t, token, branchName, apiKey, writeAcceptanceProjectFiles)
	enablePersistentEditsInRootGenYAML(t, dir)

	editedFile := findGeneratedGoFile(t, dir)
	comment := addCommentToGoFile(t, editedFile)
	editedRelPath, err := filepath.Rel(dir, editedFile)
	require.NoError(t, err)

	runAcceptanceSpeakeasy(t, dir, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console")
	contentAfterLocal, err := os.ReadFile(editedFile)
	require.NoError(t, err)
	require.Contains(t, string(contentAfterLocal), comment, "comment should survive local speakeasy run")

	runAcceptanceGit(t, dir, "add", "-A")
	runAcceptanceGit(t, dir, "commit", "-m", "ci: enable persistent edits + manual edit")
	runAcceptanceGit(t, dir, "push", "--force", "origin", branchName)

	writeAcceptanceUpdatedSpecWithNewOperation(t, dir)
	runAcceptanceGit(t, dir, "add", "-A")
	runAcceptanceGit(t, dir, "commit", "-m", "feat: add status endpoint")
	runAcceptanceGit(t, dir, "push", "--force", "origin", branchName)

	workspace := cloneAcceptanceBranch(t, token, branchName)
	setupAcceptanceEnvironment(t, workspace, token, branchName)
	t.Setenv("SPEAKEASY_API_KEY", apiKey)

	err = executeCIWithEnv(t, workspace, nil, "generate", "--mode", "pr", "--github-access-token", token, "--working-directory", ".", "--force", "--skip-compile", "--skip-testing", "--skip-release").Run()
	require.NoError(t, err, "ci generate should succeed")

	client := newAcceptanceGitHubClient(token)
	pr := findAcceptancePRForBranch(t, client, branchName)
	require.NotNil(t, pr, "expected a PR to be created")

	prFileContent := getAcceptanceFileContentFromRef(t, client, pr.GetHead().GetRef(), editedRelPath)
	assert.Contains(t, prFileContent, comment, "persistent edit comment should survive regeneration")
}

func TestCIWorkflow_PRMode_PersistentEditsConflict(t *testing.T) {
	requireAcceptanceTest(t)

	token := getAcceptanceToken(t)
	apiKey := getAcceptanceAPIKey(t)

	branchName := "test-integration-" + strings.ToLower(randStringBytes(8))
	t.Cleanup(func() { cleanupAcceptanceBranches(t, token, branchName) })

	dir := pushAcceptanceBranchWithSDK(t, token, branchName, apiKey, writeAcceptanceProjectFilesWithBothOps)
	enablePersistentEditsInRootGenYAML(t, dir)

	_, editMarker := addInlineEditToStatusField(t, dir)
	runAcceptanceSpeakeasy(t, dir, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console")

	runAcceptanceGit(t, dir, "add", "-A")
	runAcceptanceGit(t, dir, "commit", "-m", "ci: enable persistent edits + edit status field")
	runAcceptanceGit(t, dir, "push", "--force", "origin", branchName)

	writeAcceptanceSpecWithRenamedProperty(t, dir)
	runAcceptanceGit(t, dir, "add", "-A")
	runAcceptanceGit(t, dir, "commit", "-m", "feat: rename status property to serviceStatus")
	runAcceptanceGit(t, dir, "push", "--force", "origin", branchName)

	workspace := cloneAcceptanceBranch(t, token, branchName)
	setupAcceptanceEnvironment(t, workspace, token, branchName)
	t.Setenv("SPEAKEASY_API_KEY", apiKey)

	err := executeCIWithEnv(t, workspace, nil, "generate", "--mode", "pr", "--github-access-token", token, "--working-directory", ".", "--force", "--skip-compile", "--skip-testing", "--skip-release").Run()
	require.Error(t, err, "ci generate should fail when persistent edits conflict with spec changes")

	client := newAcceptanceGitHubClient(token)
	pr := findAcceptancePRForBranch(t, client, branchName)
	assert.Nil(t, pr, "no PR should be created when there is an unresolved conflict")
	assert.NotEmpty(t, editMarker)
}

func TestCIWorkflow_PRMode_Changeset(t *testing.T) {
	requireAcceptanceTest(t)

	token := getAcceptanceToken(t)
	apiKey := getAcceptanceAPIKey(t)

	branchName := "test-integration-" + strings.ToLower(randStringBytes(8))
	t.Cleanup(func() { cleanupAcceptanceBranches(t, token, branchName) })

	dir := pushAcceptanceBranchWithSDK(t, token, branchName, apiKey, writeAcceptanceProjectFiles)
	enableChangesetVersionStrategy(t, dir)
	runAcceptanceGit(t, dir, "add", "gen.yaml")
	runAcceptanceGit(t, dir, "commit", "-m", "ci: enable changeset mode")
	runAcceptanceGit(t, dir, "push", "--force", "origin", branchName)

	writeAcceptanceUpdatedSpecWithNewOperation(t, dir)
	runAcceptanceGit(t, dir, "add", "openapi.yaml")
	runAcceptanceGit(t, dir, "commit", "-m", "feat: add status endpoint")
	runAcceptanceGit(t, dir, "push", "--force", "origin", branchName)

	workspace := cloneAcceptanceBranch(t, token, branchName)
	setupAcceptanceEnvironment(t, workspace, token, branchName)
	t.Setenv("SPEAKEASY_API_KEY", apiKey)

	err := executeCIWithEnv(t, workspace, nil, "generate", "--mode", "pr", "--github-access-token", token, "--working-directory", ".", "--force", "--skip-compile", "--skip-testing", "--skip-release").Run()
	require.NoError(t, err, "ci generate should succeed in changeset mode")

	client := newAcceptanceGitHubClient(token)
	pr := findAcceptancePRForBranch(t, client, branchName)
	require.NotNil(t, pr, "expected a PR to be created")

	changesets := mustGlob(t, filepath.Join(workspace, ".speakeasy", "changesets", "*.yaml"))
	require.NotEmpty(t, changesets, "changeset mode should write a changeset file on the generated branch")

	genYAML, err := os.ReadFile(filepath.Join(workspace, "gen.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(genYAML), "versionStrategy: changeset")
	assert.Contains(t, string(genYAML), "version: 1.0.0")

	genYAMLFromPR := getAcceptanceFileContentFromRef(t, client, pr.GetHead().GetRef(), "gen.yaml")
	assert.Contains(t, genYAMLFromPR, "versionStrategy: changeset")
	assert.Contains(t, genYAMLFromPR, "version: 1.0.0")
}
