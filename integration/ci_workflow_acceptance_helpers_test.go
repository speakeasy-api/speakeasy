//go:build integration

package integration_tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v63/github"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/changeset"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

const defaultAcceptanceRepo = "speakeasy-api/sdk-generation-action-test-repo"

func requireAcceptanceTest(t *testing.T) {
	t.Helper()
	if os.Getenv("SPEAKEASY_ACCEPTANCE") != "1" {
		t.Skip("skipping: set SPEAKEASY_ACCEPTANCE=1 to run GitHub-backed ci acceptance tests")
	}
}

func acceptanceRepo(t *testing.T) string {
	t.Helper()

	if repo := os.Getenv("SPEAKEASY_CI_TEST_REPO"); repo != "" {
		return repo
	}

	return defaultAcceptanceRepo
}

func acceptanceRepoParts(t *testing.T) (string, string) {
	t.Helper()

	owner, name, ok := strings.Cut(acceptanceRepo(t), "/")
	if !ok || owner == "" || name == "" {
		t.Fatalf("invalid SPEAKEASY_CI_TEST_REPO value %q", acceptanceRepo(t))
	}

	return owner, name
}

func acceptanceRemoteURL(t *testing.T, token string) string {
	t.Helper()
	return fmt.Sprintf("https://gen:%s@github.com/%s.git", token, acceptanceRepo(t))
}

func getAcceptanceToken(t *testing.T) string {
	t.Helper()

	for _, key := range []string{"SPEAKEASY_CI_TEST_GITHUB_TOKEN", "GITHUB_TOKEN", "INPUT_GITHUB_ACCESS_TOKEN"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}

	t.Skip("skipping: no SPEAKEASY_CI_TEST_GITHUB_TOKEN, GITHUB_TOKEN, or INPUT_GITHUB_ACCESS_TOKEN set")
	return ""
}

func getAcceptanceAPIKey(t *testing.T) string {
	t.Helper()

	if value := os.Getenv("SPEAKEASY_API_KEY"); value != "" {
		return value
	}

	t.Skip("skipping: SPEAKEASY_API_KEY not set")
	return ""
}

func newAcceptanceGitHubClient(token string) *github.Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return github.NewClient(oauth2.NewClient(ctx, ts))
}

func setupAcceptanceEnvironment(t *testing.T, workspace, token, branchName string) {
	t.Helper()

	owner, _ := acceptanceRepoParts(t)

	t.Setenv("GITHUB_WORKSPACE", workspace)
	t.Setenv("INPUT_GITHUB_ACCESS_TOKEN", token)
	t.Setenv("GITHUB_SERVER_URL", "https://github.com")
	t.Setenv("GITHUB_REPOSITORY", acceptanceRepo(t))
	t.Setenv("GITHUB_REPOSITORY_OWNER", owner)
	t.Setenv("GITHUB_REF", "refs/heads/"+branchName)
	t.Setenv("GITHUB_OUTPUT", filepath.Join(workspace, "github-output.txt"))
	t.Setenv("GITHUB_WORKFLOW", "integration-test")
	t.Setenv("GITHUB_RUN_ID", "1")
	t.Setenv("GITHUB_RUN_ATTEMPT", "1")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("INPUT_DEBUG", "true")
	t.Setenv("INPUT_WORKING_DIRECTORY", ".")
	t.Setenv("GITHUB_HEAD_REF", "")
	t.Setenv("GITHUB_BASE_REF", "")
	t.Setenv("GITHUB_EVENT_NAME", "push")
	t.Setenv("GITHUB_EVENT_PATH", "")
	t.Setenv("INPUT_FEATURE_BRANCH", "")
	t.Setenv("INPUT_OPENAPI_DOC_LOCATION", "")
	t.Setenv("INPUT_SPEAKEASY_VERSION", "")
	t.Setenv("INPUT_DOCS_GENERATION", "")
	t.Setenv("INPUT_TARGET", "")
	t.Setenv("INPUT_SIGNED_COMMITS", "")
	t.Setenv("INPUT_ENABLE_SDK_CHANGELOG", "")
	t.Setenv("INPUT_SKIP_COMPILE", "")
	t.Setenv("INPUT_SKIP_RELEASE", "")
	t.Setenv("INPUT_SKIP_TESTING", "")
	t.Setenv("INPUT_PUSH_CODE_SAMPLES_ONLY", "")
	t.Setenv("PR_CREATION_PAT", "")
	t.Setenv("INPUT_NPM_TAG", "")
}

func cleanupAcceptanceBranches(t *testing.T, token, branchName string) {
	t.Helper()

	client := newAcceptanceGitHubClient(token)
	ctx := context.Background()
	owner, repo := acceptanceRepoParts(t)

	prs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{State: "open"})
	if err != nil {
		t.Logf("cleanup: failed to list PRs: %v", err)
	} else {
		for _, pr := range prs {
			headRef := pr.GetHead().GetRef()
			if strings.Contains(headRef, branchName) || headRef == branchName {
				t.Logf("cleanup: closing PR #%d (%s)", pr.GetNumber(), headRef)
				_, _, err := client.PullRequests.Edit(ctx, owner, repo, pr.GetNumber(), &github.PullRequest{
					State: github.String("closed"),
				})
				if err != nil {
					t.Logf("cleanup: failed to close PR #%d: %v", pr.GetNumber(), err)
				}

				_, err = client.Git.DeleteRef(ctx, owner, repo, "heads/"+headRef)
				if err != nil {
					t.Logf("cleanup: failed to delete PR branch %s: %v", headRef, err)
				}
			}
		}
	}

	_, err = client.Git.DeleteRef(ctx, owner, repo, "heads/"+branchName)
	if err != nil {
		t.Logf("cleanup: failed to delete branch %s: %v", branchName, err)
	}
}

func findAcceptancePRForBranch(t *testing.T, client *github.Client, branchName string) *github.PullRequest {
	t.Helper()

	ctx := context.Background()
	owner, repo := acceptanceRepoParts(t)

	prs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{State: "open"})
	if err != nil {
		t.Fatalf("findAcceptancePRForBranch: failed to list PRs: %v", err)
	}

	for _, pr := range prs {
		if strings.Contains(pr.GetHead().GetRef(), branchName) {
			return pr
		}
	}

	return nil
}

func findAcceptancePRByHeadRef(t *testing.T, client *github.Client, headRef string) *github.PullRequest {
	t.Helper()

	ctx := context.Background()
	owner, repo := acceptanceRepoParts(t)

	prs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{State: "open"})
	if err != nil {
		t.Fatalf("findAcceptancePRByHeadRef: failed to list PRs: %v", err)
	}

	for _, pr := range prs {
		if pr.GetHead().GetRef() == headRef {
			return pr
		}
	}

	return nil
}

func listAcceptancePRFiles(t *testing.T, client *github.Client, prNumber int) []string {
	t.Helper()

	ctx := context.Background()
	owner, repo := acceptanceRepoParts(t)

	files, _, err := client.PullRequests.ListFiles(ctx, owner, repo, prNumber, &github.ListOptions{PerPage: 100})
	if err != nil {
		t.Fatalf("list PR files for #%d: %v", prNumber, err)
	}

	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.GetFilename())
	}

	return paths
}

func filterAcceptancePatchFiles(paths []string) []string {
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.HasPrefix(path, ".speakeasy/patches/") {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func getAcceptanceFileContentFromRef(t *testing.T, client *github.Client, ref, path string) string {
	t.Helper()

	ctx := context.Background()
	owner, repo := acceptanceRepoParts(t)

	fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
		Ref: ref,
	})
	if err != nil {
		t.Fatalf("get file content from %s:%s: %v", ref, path, err)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		t.Fatalf("decode file content: %v", err)
	}

	return content
}

func getAcceptanceChangesetFromRef(t *testing.T, client *github.Client, ref, path string) *changeset.Changeset {
	t.Helper()

	content := getAcceptanceFileContentFromRef(t, client, ref, path)

	var cs changeset.Changeset
	err := yaml.Unmarshal([]byte(content), &cs)
	require.NoError(t, err, "parse changeset from %s:%s", ref, path)
	return &cs
}

func waitForAcceptanceFileOnRef(t *testing.T, client *github.Client, ref, path string) {
	t.Helper()

	ctx := context.Background()
	owner, repo := acceptanceRepoParts(t)

	var lastErr error
	for range 15 {
		_, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
			Ref: ref,
		})
		if err == nil {
			return
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("wait for %s on %s: %v", path, ref, lastErr)
}

func mergeAcceptancePR(t *testing.T, client *github.Client, pr *github.PullRequest) {
	t.Helper()

	ctx := context.Background()
	owner, repo := acceptanceRepoParts(t)

	var lastErr error
	for range 12 {
		refreshedPR, _, err := client.PullRequests.Get(ctx, owner, repo, pr.GetNumber())
		if err != nil {
			t.Fatalf("refresh PR #%d: %v", pr.GetNumber(), err)
		}

		if refreshedPR.GetMerged() {
			return
		}

		if refreshedPR.Mergeable != nil && !refreshedPR.GetMergeable() {
			lastErr = fmt.Errorf("PR #%d is currently not mergeable", pr.GetNumber())
			time.Sleep(2 * time.Second)
			continue
		}

		_, _, err = client.PullRequests.Merge(ctx, owner, repo, pr.GetNumber(), "", &github.PullRequestOptions{
			SHA:         refreshedPR.GetHead().GetSHA(),
			MergeMethod: "squash",
		})
		if err == nil {
			return
		}

		lastErr = err
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("merge PR #%d: %v", pr.GetNumber(), lastErr)
}

func listAcceptanceReleasesByTagPrefix(t *testing.T, client *github.Client, tagPrefix string) []*github.RepositoryRelease {
	t.Helper()

	ctx := context.Background()
	owner, repo := acceptanceRepoParts(t)

	var releases []*github.RepositoryRelease
	opts := &github.ListOptions{PerPage: 100}
	for {
		page, resp, err := client.Repositories.ListReleases(ctx, owner, repo, opts)
		if err != nil {
			t.Fatalf("list releases: %v", err)
		}

		for _, release := range page {
			if strings.HasPrefix(release.GetTagName(), tagPrefix) {
				releases = append(releases, release)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return releases
}

func cleanupAcceptanceReleases(t *testing.T, token, tagPrefix string) {
	t.Helper()

	client := newAcceptanceGitHubClient(token)
	ctx := context.Background()
	owner, repo := acceptanceRepoParts(t)

	for _, release := range listAcceptanceReleasesByTagPrefix(t, client, tagPrefix) {
		if release.ID != nil {
			if _, err := client.Repositories.DeleteRelease(ctx, owner, repo, *release.ID); err != nil {
				t.Logf("cleanup: failed to delete release %s: %v", release.GetTagName(), err)
			}
		}
		if _, err := client.Git.DeleteRef(ctx, owner, repo, "tags/"+release.GetTagName()); err != nil {
			t.Logf("cleanup: failed to delete tag %s: %v", release.GetTagName(), err)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeAcceptanceProjectFiles(t *testing.T, dir string) {
	t.Helper()

	specContent := `openapi: "3.0.0"
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
`
	writeFile(t, filepath.Join(dir, "openapi.yaml"), specContent)

	speakeasyDir := filepath.Join(dir, ".speakeasy")
	if err := os.MkdirAll(speakeasyDir, 0o755); err != nil {
		t.Fatalf("mkdir .speakeasy: %v", err)
	}

	workflowContent := `workflowVersion: 1.0.0
speakeasyVersion: latest
sources:
  test-source:
    inputs:
      - location: openapi.yaml
targets:
  go:
    target: go
    source: test-source
`
	writeFile(t, filepath.Join(speakeasyDir, "workflow.yaml"), workflowContent)

	genContent := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
go:
  version: 1.0.0
  packageName: testsdk
`
	writeFile(t, filepath.Join(dir, "gen.yaml"), genContent)
}

func setAcceptanceGenVersion(t *testing.T, dir, version string) {
	t.Helper()

	genYAMLPath := filepath.Join(dir, "gen.yaml")
	content, err := os.ReadFile(genYAMLPath)
	if err != nil {
		t.Fatalf("read gen.yaml: %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		t.Fatalf("unmarshal gen.yaml: %v", err)
	}

	goCfg, ok := cfg["go"].(map[string]any)
	if !ok {
		t.Fatalf("gen.yaml missing go section")
	}
	goCfg["version"] = version
	cfg["go"] = goCfg

	updated, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal gen.yaml: %v", err)
	}

	if err := os.WriteFile(genYAMLPath, updated, 0o644); err != nil {
		t.Fatalf("write gen.yaml: %v", err)
	}
}

func enableAcceptanceChangesetVersionStrategy(t *testing.T, dir string) {
	t.Helper()

	genYAMLPath := filepath.Join(dir, "gen.yaml")
	content, err := os.ReadFile(genYAMLPath)
	if err != nil {
		t.Fatalf("read gen.yaml: %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		t.Fatalf("unmarshal gen.yaml: %v", err)
	}

	generation, ok := cfg["generation"].(map[string]any)
	if !ok {
		t.Fatalf("gen.yaml missing generation section")
	}
	generation["versionStrategy"] = "changeset"
	cfg["generation"] = generation

	updated, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal gen.yaml: %v", err)
	}

	if err := os.WriteFile(genYAMLPath, updated, 0o644); err != nil {
		t.Fatalf("write gen.yaml: %v", err)
	}
}

func uniqueAcceptanceVersionPrefix(t *testing.T) (string, string) {
	t.Helper()

	seed := int(time.Now().UnixNano() % 1000000)
	if seed < 0 {
		seed = -seed
	}

	minor := strconv.Itoa(seed)
	return "0." + minor + ".0", "v0." + minor + "."
}

func writeAcceptanceProjectFilesWithBothOps(t *testing.T, dir string) {
	t.Helper()

	writeAcceptanceProjectFiles(t, dir)
	writeAcceptanceSpecWithStatusField(t, dir, "status")
}

func writeAcceptanceSpecWithStatusField(t *testing.T, dir, fieldName string) {
	t.Helper()

	specContent := fmt.Sprintf(`openapi: "3.0.0"
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
      operationId: getStatus
      summary: Get service status
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  %s:
                    type: string
`, fieldName)

	writeFile(t, filepath.Join(dir, "openapi.yaml"), specContent)
}

func writeAcceptanceUpdatedSpecWithNewOperation(t *testing.T, dir string) {
	t.Helper()
	writeAcceptanceSpecWithStatusField(t, dir, "status")
}

func writeAcceptanceSpecWithRenamedProperty(t *testing.T, dir string) {
	t.Helper()
	writeAcceptanceSpecWithStatusField(t, dir, "serviceStatus")
}

func runAcceptanceGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	if len(args) > 0 && args[0] == "commit" {
		args = append([]string{"-c", "commit.gpgsign=false"}, args...)
	}

	output, err := executeSystemCommand(dir, "git", args...)
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}

	return output
}

func executeSystemCommand(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Integration Test",
		"GIT_AUTHOR_EMAIL=test@speakeasy.com",
		"GIT_COMMITTER_NAME=Integration Test",
		"GIT_COMMITTER_EMAIL=test@speakeasy.com",
		"GIT_TERMINAL_PROMPT=0",
	)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func pushAcceptanceOrphanBranch(t *testing.T, token, branchName string, writeFiles func(*testing.T, string)) string {
	t.Helper()

	dir := t.TempDir()
	writeFiles(t, dir)

	runAcceptanceGit(t, dir, "init")
	runAcceptanceGit(t, dir, "config", "user.name", "Integration Test")
	runAcceptanceGit(t, dir, "config", "user.email", "test@speakeasy.com")
	runAcceptanceGit(t, dir, "checkout", "--orphan", branchName)
	runAcceptanceGit(t, dir, "add", "-A")
	runAcceptanceGit(t, dir, "commit", "-m", "ci: integration test setup")
	runAcceptanceGit(t, dir, "remote", "add", "origin", acceptanceRemoteURL(t, token))
	runAcceptanceGit(t, dir, "push", "--force", "origin", branchName)

	return dir
}

func cloneAcceptanceBranch(t *testing.T, token, branchName string) string {
	t.Helper()

	workspace := t.TempDir()
	runAcceptanceGit(t, filepath.Dir(workspace), "clone", "--branch", branchName, "--single-branch", acceptanceRemoteURL(t, token), workspace)
	runAcceptanceGit(t, workspace, "config", "user.name", "Integration Test")
	runAcceptanceGit(t, workspace, "config", "user.email", "test@speakeasy.com")
	return workspace
}

func cloneAcceptanceBranchAs(t *testing.T, token, baseBranch, branchName string) string {
	t.Helper()

	workspace := cloneAcceptanceBranch(t, token, baseBranch)
	runAcceptanceGit(t, workspace, "checkout", "-b", branchName)
	return workspace
}

func createAcceptancePullRequest(t *testing.T, client *github.Client, headBranch, baseBranch, title, body string) *github.PullRequest {
	t.Helper()

	ctx := context.Background()
	owner, repo := acceptanceRepoParts(t)

	pr, _, err := client.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title:               github.String(title),
		Body:                github.String(body),
		Head:                github.String(headBranch),
		Base:                github.String(baseBranch),
		MaintainerCanModify: github.Bool(true),
	})
	if err != nil {
		t.Fatalf("create PR %s -> %s: %v", headBranch, baseBranch, err)
	}

	return pr
}

func runAcceptanceSpeakeasy(t *testing.T, dir, apiKey string, args ...string) {
	t.Helper()

	env := map[string]string{
		"SPEAKEASY_API_KEY":      apiKey,
		"SPEAKEASY_RUN_LOCATION": "action",
		"SPEAKEASY_ENVIRONMENT":  "github",
	}

	if err := executeWithEnv(t, dir, env, args...).Run(); err != nil {
		t.Fatalf("speakeasy %v failed: %v", args, err)
	}
}

func pushAcceptanceBranchWithSDK(t *testing.T, token, branchName, apiKey string, writeFiles func(*testing.T, string)) string {
	t.Helper()

	dir := pushAcceptanceOrphanBranch(t, token, branchName, writeFiles)
	runAcceptanceSpeakeasy(t, dir, apiKey, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console")
	runAcceptanceGit(t, dir, "add", "-A")
	runAcceptanceGit(t, dir, "commit", "-m", "ci: generated SDK baseline")
	runAcceptanceGit(t, dir, "push", "--force", "origin", branchName)
	return dir
}

func enablePersistentEditsInRootGenYAML(t *testing.T, dir string) {
	t.Helper()

	genYAMLPath := filepath.Join(dir, "gen.yaml")
	content, err := os.ReadFile(genYAMLPath)
	if err != nil {
		t.Fatalf("read gen.yaml: %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		t.Fatalf("unmarshal gen.yaml: %v", err)
	}

	generation, ok := cfg["generation"].(map[string]any)
	if !ok {
		t.Fatalf("gen.yaml missing generation section")
	}

	if persistentEdits, ok := generation["persistentEdits"].(map[string]any); ok {
		persistentEdits["enabled"] = true
		generation["persistentEdits"] = persistentEdits
	} else {
		generation["persistentEdits"] = map[string]any{
			"enabled": true,
		}
	}
	cfg["generation"] = generation

	updated, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal gen.yaml: %v", err)
	}

	if err := os.WriteFile(genYAMLPath, updated, 0o644); err != nil {
		t.Fatalf("write gen.yaml: %v", err)
	}
}

func enableGoCustomCodeRegionsInRootGenYAML(t *testing.T, dir string) {
	t.Helper()

	genYAMLPath := filepath.Join(dir, "gen.yaml")
	content, err := os.ReadFile(genYAMLPath)
	if err != nil {
		t.Fatalf("read gen.yaml: %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		t.Fatalf("unmarshal gen.yaml: %v", err)
	}

	goCfg, ok := cfg["go"].(map[string]any)
	if !ok {
		t.Fatalf("gen.yaml missing go section")
	}

	goCfg["enableCustomCodeRegions"] = true
	cfg["go"] = goCfg

	updated, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal gen.yaml: %v", err)
	}

	if err := os.WriteFile(genYAMLPath, updated, 0o644); err != nil {
		t.Fatalf("write gen.yaml: %v", err)
	}
}

func enableAcceptancePatchFilesInRootGenYAML(t *testing.T, dir string) {
	t.Helper()

	genYAMLPath := filepath.Join(dir, "gen.yaml")
	content, err := os.ReadFile(genYAMLPath)
	if err != nil {
		t.Fatalf("read gen.yaml: %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		t.Fatalf("unmarshal gen.yaml: %v", err)
	}

	generation, ok := cfg["generation"].(map[string]any)
	if !ok {
		t.Fatalf("gen.yaml missing generation section")
	}

	persistentEdits, ok := generation["persistentEdits"].(map[string]any)
	if !ok {
		t.Fatalf("gen.yaml missing generation.persistentEdits section")
	}
	persistentEdits["patchFiles"] = true
	generation["persistentEdits"] = persistentEdits
	cfg["generation"] = generation

	updated, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal gen.yaml: %v", err)
	}

	if err := os.WriteFile(genYAMLPath, updated, 0o644); err != nil {
		t.Fatalf("write gen.yaml: %v", err)
	}
}

func findGeneratedGoFile(t *testing.T, dir string) string {
	t.Helper()

	patterns := []string{
		filepath.Join(dir, "models", "operations", "*.go"),
		filepath.Join(dir, "*", "*.go"),
		filepath.Join(dir, "*", "*", "*.go"),
	}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return matches[0]
		}
	}

	t.Fatal("no generated .go files found")
	return ""
}

func appendGoDeclaration(t *testing.T, filePath, declaration string) {
	t.Helper()

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	updated := strings.TrimRight(string(content), "\n") + "\n\n" + strings.TrimSpace(declaration) + "\n"
	require.NoError(t, os.WriteFile(filePath, []byte(updated), 0o644))
}

func scrubAcceptanceLockPersistentState(t *testing.T, dir string) {
	t.Helper()

	cfg, err := config.Load(dir)
	require.NoError(t, err)
	require.NotNil(t, cfg.LockFile)

	cfg.LockFile.PersistentEdits = nil
	cfg.LockFile.TrackedFiles = config.NewLockFile().TrackedFiles

	require.NoError(t, config.SaveLockFile(dir, cfg.LockFile))
	require.NoError(t, os.RemoveAll(filepath.Join(dir, ".git", "refs", "speakeasy")))
}

func addCommentToGoFile(t *testing.T, filePath string) string {
	t.Helper()

	comment := "// PERSISTENT-EDIT-TEST: this comment should survive SDK regeneration"
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read %s: %v", filePath, err)
	}

	lines := strings.Split(string(content), "\n")
	var result []string
	inserted := false
	for _, line := range lines {
		result = append(result, line)
		if !inserted && strings.HasPrefix(line, "package ") {
			result = append(result, "", comment)
			inserted = true
		}
	}

	if !inserted {
		t.Fatalf("could not find package line in %s", filePath)
	}

	if err := os.WriteFile(filePath, []byte(strings.Join(result, "\n")), 0o644); err != nil {
		t.Fatalf("write %s: %v", filePath, err)
	}

	return comment
}

func addInlineEditToStatusField(t *testing.T, dir string) (string, string) {
	t.Helper()

	editMarker := "// user-edit: persistent edit test marker"
	patterns := []string{
		filepath.Join(dir, "models", "operations", "*.go"),
		filepath.Join(dir, "models", "components", "*.go"),
		filepath.Join(dir, "models", "*.go"),
	}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			content, err := os.ReadFile(match)
			if err != nil || !strings.Contains(string(content), `json:"status`) {
				continue
			}

			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				if strings.Contains(line, `json:"status`) {
					lines[i] = line + " " + editMarker
					if err := os.WriteFile(match, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
						t.Fatalf("write %s: %v", match, err)
					}
					return match, editMarker
				}
			}
		}
	}

	t.Fatal("could not find generated file with Status json field")
	return "", ""
}
