package git

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-github/v63/github"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/releases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRepo(t *testing.T) (*git.Repository, billy.Filesystem) {
	t.Helper()

	mfs := memfs.New()

	err := filepath.WalkDir("./fixtures", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		fixture, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fixture.Close()

		f, err := mfs.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(f, fixture)
		if err != nil {
			return err
		}

		return nil
	})
	require.NoError(t, err, "expected to walk the fixture directory")

	storage := memory.NewStorage()
	repo, err := git.Init(storage, mfs)
	require.NoError(t, err, "expected empty repo to be initialized")

	wt, err := repo.Worktree()
	require.NoError(t, err, "expected to get worktree")

	_, err = wt.Add(".")
	require.NoError(t, err, "expected to add all files")

	_, err = wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Unix(0, 0),
		},
	})
	require.NoError(t, err, "expected to commit all files")

	return repo, mfs
}

func runGitCLI(t *testing.T, dir string, args ...string) string {
	return runGitCLIWithEnv(t, dir, nil, args...)
}

func runGitCLIWithEnv(t *testing.T, dir string, extraEnv map[string]string, args ...string) string {
	t.Helper()
	if len(args) > 0 && args[0] == "commit" {
		args = append([]string{"-c", "commit.gpgsign=false"}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	env := append([]string{}, os.Environ()...)
	baseEnv := map[string]string{
		"GIT_AUTHOR_NAME":     "Test User",
		"GIT_AUTHOR_EMAIL":    "test@example.com",
		"GIT_COMMITTER_NAME":  "Test User",
		"GIT_COMMITTER_EMAIL": "test@example.com",
	}
	for k, v := range extraEnv {
		baseEnv[k] = v
	}
	for k, v := range baseEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, output)
	}

	return string(output)
}

func TestGit_CheckDirDirty(t *testing.T) {
	repo, mfs := newTestRepo(t)

	f, err := mfs.Create("dirty-file")
	require.NoError(t, err, "expected to create a dirty file")
	defer f.Close()
	fmt.Fprintln(f, "sample content")

	g := Git{repo: repo}
	dirty, str, err := g.CheckDirDirty(".", map[string]string{})
	require.NoError(t, err, "expected to check the directory")

	require.Equal(t, `new file found: []string{"dirty-file"}`, str)
	require.True(t, dirty, "expected the directory to be dirty")
}

func TestGit_CheckDirDirty_IgnoredFiles(t *testing.T) {
	repo, mfs := newTestRepo(t)

	f, err := mfs.Create("workflow.lock")
	require.NoError(t, err, "expected to create a dirty file")
	defer f.Close()
	fmt.Fprintln(f, "sample content")

	g := Git{repo: repo}
	dirty, str, err := g.CheckDirDirty(".", map[string]string{})
	require.NoError(t, err, "expected to check the directory")

	require.Equal(t, "", str, "expected no dirty files reported")
	require.False(t, dirty, "expected the directory to be clean")
}

func TestArtifactMatchesRelease(t *testing.T) {
	tests := []struct {
		name      string
		assetName string
		goos      string
		goarch    string
		want      bool
	}{
		{
			name:      "Linux amd64",
			assetName: "speakeasy_linux_amd64.zip",
			goos:      "linux",
			goarch:    "amd64",
			want:      true,
		},
		{
			name:      "Linux 386",
			assetName: "speakeasy_linux_386.zip",
			goos:      "linux",
			goarch:    "386",
			want:      true,
		},
		{
			name:      "Linux arm64",
			assetName: "speakeasy_linux_arm64.zip",
			goos:      "linux",
			goarch:    "arm64",
			want:      true,
		},
		{
			name:      "macOS amd64",
			assetName: "speakeasy_darwin_amd64.zip",
			goos:      "darwin",
			goarch:    "amd64",
			want:      true,
		},
		{
			name:      "Linux arm64/v8",
			assetName: "speakeasy_linux_arm64.zip",
			goos:      "linux",
			goarch:    "arm64/v8",
			want:      true,
		},
		{
			name:      "macOS arm64",
			assetName: "speakeasy_darwin_arm64.zip",
			goos:      "darwin",
			goarch:    "arm64",
			want:      true,
		},
		{
			name:      "Windows amd64",
			assetName: "speakeasy_windows_amd64.zip",
			goos:      "windows",
			goarch:    "amd64",
			want:      true,
		},
		{
			name:      "Windows 386",
			assetName: "speakeasy_windows_386.zip",
			goos:      "windows",
			goarch:    "386",
			want:      true,
		},
		{
			name:      "Windows arm64",
			assetName: "speakeasy_windows_arm64.zip",
			goos:      "windows",
			goarch:    "arm64",
			want:      true,
		},
		{
			name:      "Mismatched OS",
			assetName: "speakeasy_linux_amd64.zip",
			goos:      "darwin",
			goarch:    "amd64",
			want:      false,
		},
		{
			name:      "Mismatched arch",
			assetName: "speakeasy_linux_amd64.zip",
			goos:      "linux",
			goarch:    "arm64",
			want:      false,
		},
		{
			name:      "Checksums file",
			assetName: "checksums.txt",
			goos:      "linux",
			goarch:    "amd64",
			want:      false,
		},
		{
			name:      "Source code zip",
			assetName: "Source code (zip)",
			goos:      "linux",
			goarch:    "amd64",
			want:      false,
		},
		{
			name:      "Source code tar.gz",
			assetName: "Source code (tar.gz)",
			goos:      "linux",
			goarch:    "amd64",
			want:      false,
		},
		{
			name:      "Incorrect file extension",
			assetName: "speakeasy_linux_amd64.tar.gz",
			goos:      "linux",
			goarch:    "amd64",
			want:      false,
		},
		{
			name:      "Missing architecture",
			assetName: "speakeasy_linux.zip",
			goos:      "linux",
			goarch:    "amd64",
			want:      false,
		},
		{
			name:      "Wrong order of segments",
			assetName: "speakeasy_amd64_linux.zip",
			goos:      "linux",
			goarch:    "amd64",
			want:      false,
		},
		{
			name:      "Partial match in OS",
			assetName: "speakeasy_darwin_amd64.zip",
			goos:      "dar",
			goarch:    "amd64",
			want:      false,
		},
		{
			name:      "Partial match in arch",
			assetName: "speakeasy_linux_amd64.zip",
			goos:      "linux",
			goarch:    "amd",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ArtifactMatchesRelease(tt.assetName, tt.goos, tt.goarch); got != tt.want {
				t.Errorf("ArtifactMatchesRelease() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test source-branch-aware branch naming
func TestGit_FindOrCreateBranch_SourceBranchAware(t *testing.T) {
	tests := []struct {
		name           string
		sourceBranch   string
		action         environment.Action
		expectedPrefix string
	}{
		{
			name:           "main branch - SDK regen",
			sourceBranch:   "main",
			action:         environment.ActionRunWorkflow,
			expectedPrefix: "speakeasy-sdk-regen-",
		},
		{
			name:           "master branch - SDK regen",
			sourceBranch:   "master",
			action:         environment.ActionRunWorkflow,
			expectedPrefix: "speakeasy-sdk-regen-",
		},
		{
			name:           "feature branch - SDK regen",
			sourceBranch:   "feature/new-api",
			action:         environment.ActionRunWorkflow,
			expectedPrefix: "speakeasy-sdk-regen-feature-new-api-",
		},
		{
			name:           "feature branch with special chars - SDK regen",
			sourceBranch:   "feature/user-auth_v2",
			action:         environment.ActionRunWorkflow,
			expectedPrefix: "speakeasy-sdk-regen-feature-user-auth-v2-",
		},
		{
			name:           "feature branch - suggestion",
			sourceBranch:   "feature/api-changes",
			action:         environment.ActionSuggest,
			expectedPrefix: "speakeasy-openapi-suggestion-feature-api-changes-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables for the test
			originalGithubRef := os.Getenv("GITHUB_REF")
			originalGithubHeadRef := os.Getenv("GITHUB_HEAD_REF")
			originalGithubBaseRef := os.Getenv("GITHUB_BASE_REF")
			originalGithubEventName := os.Getenv("GITHUB_EVENT_NAME")

			defer func() {
				os.Setenv("GITHUB_REF", originalGithubRef)
				os.Setenv("GITHUB_HEAD_REF", originalGithubHeadRef)
				os.Setenv("GITHUB_BASE_REF", originalGithubBaseRef)
				os.Setenv("GITHUB_EVENT_NAME", originalGithubEventName)
			}()

			// Set up test environment
			if tt.sourceBranch == "main" || tt.sourceBranch == "master" {
				os.Setenv("GITHUB_REF", "refs/heads/"+tt.sourceBranch)
				os.Setenv("GITHUB_HEAD_REF", "")
				os.Setenv("GITHUB_EVENT_NAME", "push")
			} else {
				os.Setenv("GITHUB_REF", "refs/pull/123/merge")
				os.Setenv("GITHUB_HEAD_REF", tt.sourceBranch)
				os.Setenv("GITHUB_BASE_REF", "main")
				os.Setenv("GITHUB_EVENT_NAME", "pull_request")
			}

			repo, _ := newTestRepo(t)
			g := Git{repo: repo}

			branchName, err := g.FindOrCreateBranch("", tt.action)
			require.NoError(t, err)

			assert.True(t, len(branchName) > len(tt.expectedPrefix), "Branch name should be longer than prefix")
			assert.True(t, len(branchName) > 0, "Branch name should not be empty")

			// For main/master branches, should not include source branch in name
			if tt.sourceBranch == "main" || tt.sourceBranch == "master" {
				assert.Contains(t, branchName, tt.expectedPrefix)
				assert.NotContains(t, branchName, "main-")
				assert.NotContains(t, branchName, "master-")
			} else {
				// For feature branches, should include sanitized source branch name
				assert.Contains(t, branchName, tt.expectedPrefix)
			}
		})
	}
}

func TestGit_FindOrCreateBranch_FeatureBranchOverride(t *testing.T) {
	repo, _ := newTestRepo(t)
	g := Git{repo: repo}

	t.Setenv("INPUT_FEATURE_BRANCH", "feature/manual-override")
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("GITHUB_HEAD_REF", "")

	branchName, err := g.FindOrCreateBranch("", environment.ActionRunWorkflow)
	require.NoError(t, err)
	assert.Equal(t, "feature/manual-override", branchName)

	head, err := repo.Head()
	require.NoError(t, err)
	assert.Equal(t, "refs/heads/feature/manual-override", head.Name().String())
}

func TestGit_FindOrCreateBranch_NonCIPendingCommits(t *testing.T) {
	repoPath := t.TempDir()
	remotePath := filepath.Join(t.TempDir(), "remote.git")

	runGitCLI(t, t.TempDir(), "init", "--bare", remotePath)
	runGitCLI(t, repoPath, "init")
	runGitCLI(t, repoPath, "config", "user.name", "Test User")
	runGitCLI(t, repoPath, "config", "user.email", "test@example.com")

	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	runGitCLI(t, repoPath, "add", "README.md")
	runGitCLI(t, repoPath, "commit", "-m", "initial commit")

	// Create the generated.txt file on main to avoid cherry-pick conflicts
	if err := os.WriteFile(filepath.Join(repoPath, "generated.txt"), []byte("original\n"), 0o644); err != nil {
		t.Fatalf("failed to write generated.txt on main: %v", err)
	}
	runGitCLI(t, repoPath, "add", "generated.txt")
	runGitCLI(t, repoPath, "commit", "-m", "add generated file")

	runGitCLI(t, repoPath, "branch", "-M", "main")
	runGitCLI(t, repoPath, "remote", "add", "origin", remotePath)
	runGitCLI(t, repoPath, "push", "-u", "origin", "main")

	runGitCLI(t, repoPath, "checkout", "-b", "regen")
	if err := os.WriteFile(filepath.Join(repoPath, "generated.txt"), []byte("auto\n"), 0o644); err != nil {
		t.Fatalf("failed to write generated.txt: %v", err)
	}
	runGitCLI(t, repoPath, "add", "generated.txt")
	runGitCLI(t, repoPath, "commit", "-m", "ci: automated update")
	runGitCLI(t, repoPath, "push", "-u", "origin", "regen")

	// Add a different file for the manual commit to avoid conflicts
	if err := os.WriteFile(filepath.Join(repoPath, "manual.txt"), []byte("manual change\n"), 0o644); err != nil {
		t.Fatalf("failed to write manual.txt: %v", err)
	}
	runGitCLI(t, repoPath, "add", "manual.txt")
	runGitCLI(t, repoPath, "commit", "-m", "feat: manual tweak")
	runGitCLI(t, repoPath, "push", "origin", "regen")

	runGitCLI(t, repoPath, "checkout", "main")

	repo, err := git.PlainOpen(repoPath)
	require.NoError(t, err)

	g := New("test-token")
	g.repo = repo
	g.repoRoot = repoPath
	runGitCLI(t, repoPath, "config", "pull.rebase", "false")

	t.Setenv("GITHUB_WORKSPACE", repoPath)
	t.Setenv("INPUT_WORKING_DIRECTORY", "")

	_, err = g.FindOrCreateBranch("regen", environment.ActionRunWorkflow)
	require.Error(t, err)
	expectedError := "external changes detected on branch regen. The action cannot proceed because non-automated commits were pushed to this branch.\n\nPlease either:\n- Merge the associated PR for this branch\n- Close the PR and delete the branch\n\nAfter merging or closing, the action will create a new branch on the next run"
	assert.Equal(t, expectedError, err.Error())
}

func TestGit_FindOrCreateBranch_BotCommitAllowed(t *testing.T) {
	repoPath := t.TempDir()
	remotePath := filepath.Join(t.TempDir(), "remote.git")

	runGitCLI(t, t.TempDir(), "init", "--bare", remotePath)
	runGitCLI(t, repoPath, "init")
	runGitCLI(t, repoPath, "config", "user.name", "Test User")
	runGitCLI(t, repoPath, "config", "user.email", "test@example.com")

	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	runGitCLI(t, repoPath, "add", "README.md")
	runGitCLI(t, repoPath, "commit", "-m", "initial commit")
	runGitCLI(t, repoPath, "branch", "-M", "main")
	runGitCLI(t, repoPath, "remote", "add", "origin", remotePath)
	runGitCLI(t, repoPath, "push", "-u", "origin", "main")

	runGitCLI(t, repoPath, "checkout", "-b", "regen")
	if err := os.WriteFile(filepath.Join(repoPath, "generated.txt"), []byte("auto\n"), 0o644); err != nil {
		t.Fatalf("failed to write generated.txt: %v", err)
	}
	runGitCLI(t, repoPath, "add", "generated.txt")
	runGitCLI(t, repoPath, "commit", "-m", "ci: automated update")
	runGitCLI(t, repoPath, "push", "-u", "origin", "regen")

	if err := os.WriteFile(filepath.Join(repoPath, "generated.txt"), []byte("bot change\n"), 0o644); err != nil {
		t.Fatalf("failed to update generated.txt: %v", err)
	}
	runGitCLI(t, repoPath, "add", "generated.txt")
	runGitCLIWithEnv(t, repoPath, map[string]string{
		"GIT_AUTHOR_NAME":     speakeasyBotName,
		"GIT_AUTHOR_EMAIL":    "bot@speakeasyapi.dev",
		"GIT_COMMITTER_NAME":  speakeasyBotName,
		"GIT_COMMITTER_EMAIL": "bot@speakeasyapi.dev",
	}, "commit", "-m", "docs: automated")
	runGitCLI(t, repoPath, "push", "origin", "regen")

	runGitCLI(t, repoPath, "checkout", "main")

	repo, err := git.PlainOpen(repoPath)
	require.NoError(t, err)

	g := Git{repo: repo, repoRoot: repoPath}
	runGitCLI(t, repoPath, "config", "pull.rebase", "false")

	t.Setenv("GITHUB_WORKSPACE", repoPath)
	t.Setenv("INPUT_WORKING_DIRECTORY", "")

	branch, err := g.FindOrCreateBranch("regen", environment.ActionRunWorkflow)
	require.NoError(t, err)
	assert.Equal(t, "regen", branch)
}

func TestGit_FindOrCreateBranch_BotAliasCommitAllowed(t *testing.T) {
	repoPath := t.TempDir()
	remotePath := filepath.Join(t.TempDir(), "remote.git")

	runGitCLI(t, t.TempDir(), "init", "--bare", remotePath)
	runGitCLI(t, repoPath, "init")
	runGitCLI(t, repoPath, "config", "user.name", "Test User")
	runGitCLI(t, repoPath, "config", "user.email", "test@example.com")

	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	runGitCLI(t, repoPath, "add", "README.md")
	runGitCLI(t, repoPath, "commit", "-m", "initial commit")
	runGitCLI(t, repoPath, "branch", "-M", "main")
	runGitCLI(t, repoPath, "remote", "add", "origin", remotePath)
	runGitCLI(t, repoPath, "push", "-u", "origin", "main")

	runGitCLI(t, repoPath, "checkout", "-b", "regen")
	if err := os.WriteFile(filepath.Join(repoPath, "generated.txt"), []byte("auto\n"), 0o644); err != nil {
		t.Fatalf("failed to write generated.txt: %v", err)
	}
	runGitCLI(t, repoPath, "add", "generated.txt")
	runGitCLI(t, repoPath, "commit", "-m", "ci: automated update")
	runGitCLI(t, repoPath, "push", "-u", "origin", "regen")

	if err := os.WriteFile(filepath.Join(repoPath, "generated.txt"), []byte("alias bot change\n"), 0o644); err != nil {
		t.Fatalf("failed to update generated.txt: %v", err)
	}
	runGitCLI(t, repoPath, "add", "generated.txt")
	runGitCLIWithEnv(t, repoPath, map[string]string{
		"GIT_AUTHOR_NAME":     speakeasyBotAlias,
		"GIT_AUTHOR_EMAIL":    "speakeasybot@speakeasy.com",
		"GIT_COMMITTER_NAME":  speakeasyBotAlias,
		"GIT_COMMITTER_EMAIL": "speakeasybot@speakeasy.com",
	}, "commit", "-m", "docs: automated alias")
	runGitCLI(t, repoPath, "push", "origin", "regen")

	runGitCLI(t, repoPath, "checkout", "main")

	repo, err := git.PlainOpen(repoPath)
	require.NoError(t, err)

	g := Git{repo: repo, repoRoot: repoPath}
	runGitCLI(t, repoPath, "config", "pull.rebase", "false")

	t.Setenv("GITHUB_WORKSPACE", repoPath)
	t.Setenv("INPUT_WORKING_DIRECTORY", "")
	t.Setenv("INPUT_FEATURE_BRANCH", "")

	branch, err := g.FindOrCreateBranch("regen", environment.ActionRunWorkflow)
	require.NoError(t, err)
	assert.Equal(t, "regen", branch)
}

// Test source-branch-aware PR title generation
func TestGit_generatePRTitleAndBody_SourceBranchAware(t *testing.T) {
	tests := []struct {
		name               string
		sourceBranch       string
		sourceGeneration   bool
		expectedTitleParts []string
		notExpectedParts   []string
	}{
		{
			name:               "main branch - regular generation",
			sourceBranch:       "main",
			sourceGeneration:   false,
			expectedTitleParts: []string{"chore: 🐝 Update SDK"},
			notExpectedParts:   []string{"[main]"},
		},
		{
			name:               "master branch - regular generation",
			sourceBranch:       "master",
			sourceGeneration:   false,
			expectedTitleParts: []string{"chore: 🐝 Update SDK"},
			notExpectedParts:   []string{"[master]"},
		},
		{
			name:               "feature branch - regular generation",
			sourceBranch:       "feature/new-api",
			sourceGeneration:   false,
			expectedTitleParts: []string{"chore: 🐝 Update SDK", "[feature-new-api]"},
			notExpectedParts:   []string{},
		},
		{
			name:               "feature branch with special chars - regular generation",
			sourceBranch:       "feature/user-auth_v2",
			sourceGeneration:   false,
			expectedTitleParts: []string{"chore: 🐝 Update SDK", "[feature-user-auth-v2]"},
			notExpectedParts:   []string{},
		},
		{
			name:               "feature branch - source generation",
			sourceBranch:       "feature/specs-update",
			sourceGeneration:   true,
			expectedTitleParts: []string{"chore: 🐝 Update Specs", "[feature-specs-update]"},
			notExpectedParts:   []string{},
		},
		{
			name:               "main branch - source generation",
			sourceBranch:       "main",
			sourceGeneration:   true,
			expectedTitleParts: []string{"chore: 🐝 Update Specs"},
			notExpectedParts:   []string{"[main]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables for the test
			originalGithubRef := os.Getenv("GITHUB_REF")
			originalGithubHeadRef := os.Getenv("GITHUB_HEAD_REF")
			originalGithubBaseRef := os.Getenv("GITHUB_BASE_REF")
			originalGithubEventName := os.Getenv("GITHUB_EVENT_NAME")

			defer func() {
				os.Setenv("GITHUB_REF", originalGithubRef)
				os.Setenv("GITHUB_HEAD_REF", originalGithubHeadRef)
				os.Setenv("GITHUB_BASE_REF", originalGithubBaseRef)
				os.Setenv("GITHUB_EVENT_NAME", originalGithubEventName)
			}()

			// Set up test environment
			if tt.sourceBranch == "main" || tt.sourceBranch == "master" {
				os.Setenv("GITHUB_REF", "refs/heads/"+tt.sourceBranch)
				os.Setenv("GITHUB_HEAD_REF", "")
				os.Setenv("GITHUB_EVENT_NAME", "push")
			} else {
				os.Setenv("GITHUB_REF", "refs/pull/123/merge")
				os.Setenv("GITHUB_HEAD_REF", tt.sourceBranch)
				os.Setenv("GITHUB_BASE_REF", "main")
				os.Setenv("GITHUB_EVENT_NAME", "pull_request")
			}

			g := Git{}
			prInfo := PRInfo{
				SourceGeneration: tt.sourceGeneration,
				ReleaseInfo: &releases.ReleasesInfo{
					SpeakeasyVersion: "1.0.0",
				},
			}

			title, _ := g.generatePRTitleAndBody(prInfo, map[string]github.Label{}, "")

			// Check that expected parts are in the title
			for _, expectedPart := range tt.expectedTitleParts {
				assert.Contains(t, title, expectedPart, "Title should contain: %s", expectedPart)
			}

			// Check that not expected parts are NOT in the title
			for _, notExpectedPart := range tt.notExpectedParts {
				assert.NotContains(t, title, notExpectedPart, "Title should NOT contain: %s", notExpectedPart)
			}
		})
	}
}

// Test backward compatibility for main/master branches
func TestGit_BackwardCompatibility_MainBranches(t *testing.T) {
	tests := []struct {
		name         string
		sourceBranch string
	}{
		{
			name:         "main branch",
			sourceBranch: "main",
		},
		{
			name:         "master branch",
			sourceBranch: "master",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables for the test
			originalGithubRef := os.Getenv("GITHUB_REF")
			originalGithubHeadRef := os.Getenv("GITHUB_HEAD_REF")
			originalGithubEventName := os.Getenv("GITHUB_EVENT_NAME")

			defer func() {
				os.Setenv("GITHUB_REF", originalGithubRef)
				os.Setenv("GITHUB_HEAD_REF", originalGithubHeadRef)
				os.Setenv("GITHUB_EVENT_NAME", originalGithubEventName)
			}()

			// Set up test environment for main/master branch
			os.Setenv("GITHUB_REF", "refs/heads/"+tt.sourceBranch)
			os.Setenv("GITHUB_HEAD_REF", "")
			os.Setenv("GITHUB_EVENT_NAME", "push")

			repo, _ := newTestRepo(t)
			g := Git{repo: repo}

			// Test branch naming - should NOT include source branch context
			branchName, err := g.FindOrCreateBranch("", environment.ActionRunWorkflow)
			require.NoError(t, err)

			// Should follow old naming pattern without source branch context
			assert.Contains(t, branchName, "speakeasy-sdk-regen-")
			assert.NotContains(t, branchName, "main-")
			assert.NotContains(t, branchName, "master-")

			// Test PR title generation - should NOT include source branch context
			prInfo := PRInfo{
				SourceGeneration: false,
				ReleaseInfo: &releases.ReleasesInfo{
					SpeakeasyVersion: "1.0.0",
				},
			}
			title, _ := g.generatePRTitleAndBody(prInfo, map[string]github.Label{}, "")

			// Should follow old title pattern without source branch context
			assert.Contains(t, title, "chore: 🐝 Update SDK")
			assert.NotContains(t, title, "[main]")
			assert.NotContains(t, title, "[master]")
		})
	}
}

func TestCreateOrUpdateDocsPR_SourceBranchAware(t *testing.T) {
	tests := []struct {
		name                  string
		sourceBranch          string
		expectedTitleContains []string
		expectedBaseBranch    string
	}{
		{
			name:                  "main branch - no source context",
			sourceBranch:          "main",
			expectedTitleContains: []string{"chore: 🐝 Update SDK Docs"},
			expectedBaseBranch:    "main",
		},
		{
			name:                  "master branch - no source context",
			sourceBranch:          "master",
			expectedTitleContains: []string{"chore: 🐝 Update SDK Docs"},
			expectedBaseBranch:    "master",
		},
		{
			name:                  "feature branch - includes source context",
			sourceBranch:          "feature/new-api",
			expectedTitleContains: []string{"chore: 🐝 Update SDK Docs", "[feature-new-api]"},
			expectedBaseBranch:    "feature/new-api",
		},
		{
			name:                  "feature branch with special chars",
			sourceBranch:          "feature/api-v2.1_update",
			expectedTitleContains: []string{"chore: 🐝 Update SDK Docs", "[feature-api-v2.1-update]"},
			expectedBaseBranch:    "feature/api-v2.1_update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			originalGithubRef := os.Getenv("GITHUB_REF")
			originalGithubHeadRef := os.Getenv("GITHUB_HEAD_REF")
			originalGithubBaseRef := os.Getenv("GITHUB_BASE_REF")
			originalGithubEventName := os.Getenv("GITHUB_EVENT_NAME")

			defer func() {
				os.Setenv("GITHUB_REF", originalGithubRef)
				os.Setenv("GITHUB_HEAD_REF", originalGithubHeadRef)
				os.Setenv("GITHUB_BASE_REF", originalGithubBaseRef)
				os.Setenv("GITHUB_EVENT_NAME", originalGithubEventName)
			}()

			// Set up test environment
			if tt.sourceBranch == "main" || tt.sourceBranch == "master" {
				os.Setenv("GITHUB_REF", "refs/heads/"+tt.sourceBranch)
				os.Setenv("GITHUB_HEAD_REF", "")
				os.Setenv("GITHUB_EVENT_NAME", "push")
			} else {
				os.Setenv("GITHUB_REF", "refs/pull/123/merge")
				os.Setenv("GITHUB_HEAD_REF", tt.sourceBranch)
				os.Setenv("GITHUB_BASE_REF", "main")
				os.Setenv("GITHUB_EVENT_NAME", "pull_request")
			}

			// Test the title generation logic directly
			title := getDocsPRTitlePrefix()
			sourceBranch := environment.GetSourceBranch()
			isMainBranch := environment.IsMainBranch(sourceBranch)
			if !isMainBranch {
				sanitizedSourceBranch := environment.SanitizeBranchName(sourceBranch)
				title = title + " [" + sanitizedSourceBranch + "]"
			}

			targetBaseBranch := environment.GetTargetBaseBranch()
			if strings.HasPrefix(targetBaseBranch, "refs/") {
				targetBaseBranch = strings.TrimPrefix(targetBaseBranch, "refs/heads/")
			}

			// Verify title contains expected parts
			for _, expectedPart := range tt.expectedTitleContains {
				assert.Contains(t, title, expectedPart, "Title should contain: %s", expectedPart)
			}

			// Verify base branch
			assert.Equal(t, tt.expectedBaseBranch, targetBaseBranch)
		})
	}
}

func TestLegacyPRTitleWithoutBee(t *testing.T) {
	// During a bug period, PR titles were created without the bee emoji.
	// We need to ensure we can find those legacy PRs by stripping the bee.
	withoutBee := func(s string) string {
		return strings.ReplaceAll(s, "🐝 ", "")
	}

	tests := []struct {
		name     string
		current  string
		expected string
	}{
		{
			name:     "SDK PR title",
			current:  speakeasyGenPRTitle,
			expected: "chore: Update SDK - ",
		},
		{
			name:     "Specs PR title",
			current:  speakeasyGenSpecsTitle,
			expected: "chore: Update Specs - ",
		},
		{
			name:     "Docs PR title",
			current:  speakeasyDocsPRTitle,
			expected: "chore: Update SDK Docs - ",
		},
		{
			name:     "Suggest PR title",
			current:  speakeasySuggestPRTitle,
			expected: "chore: Suggest OpenAPI changes - ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, withoutBee(tt.current))
		})
	}
}

func TestConfigureSystemGitAuth_DefaultHost(t *testing.T) {
	repoDir := t.TempDir()
	runGitCLI(t, repoDir, "init")

	t.Setenv("GITHUB_SERVER_URL", "https://github.com")

	g := &Git{accessToken: "test-token-123"}
	err := g.configureSystemGitAuth(repoDir)
	require.NoError(t, err)

	output := runGitCLI(t, repoDir, "config", "--local", "--get-regexp", `url\..*\.insteadOf`)
	assert.Contains(t, output, "https://gen:test-token-123@github.com/")
	assert.Contains(t, output, "https://github.com/")
}

func TestConfigureSystemGitAuth_GHESHost(t *testing.T) {
	repoDir := t.TempDir()
	runGitCLI(t, repoDir, "init")

	t.Setenv("GITHUB_SERVER_URL", "https://github.mycompany.com")

	g := &Git{accessToken: "ghes-token-456"}
	err := g.configureSystemGitAuth(repoDir)
	require.NoError(t, err)

	output := runGitCLI(t, repoDir, "config", "--local", "--get-regexp", `url\..*\.insteadOf`)
	assert.Contains(t, output, "https://gen:ghes-token-456@github.mycompany.com/")
	assert.Contains(t, output, "https://github.mycompany.com/")
}

func TestConfigureSystemGitAuth_EmptyToken(t *testing.T) {
	repoDir := t.TempDir()
	runGitCLI(t, repoDir, "init")

	g := &Git{accessToken: ""}
	err := g.configureSystemGitAuth(repoDir)
	require.NoError(t, err)

	// Should be a no-op — no config written
	cmd := exec.Command("git", "config", "--local", "--get-regexp", `url\..*\.insteadOf`)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	// git config --get-regexp returns exit code 1 when no matches found
	assert.Error(t, err)
	assert.Empty(t, strings.TrimSpace(string(output)))
}

func TestConfigureSystemGitAuth_FallbackHost(t *testing.T) {
	repoDir := t.TempDir()
	runGitCLI(t, repoDir, "init")

	t.Setenv("GITHUB_SERVER_URL", "")

	g := &Git{accessToken: "fallback-token"}
	err := g.configureSystemGitAuth(repoDir)
	require.NoError(t, err)

	output := runGitCLI(t, repoDir, "config", "--local", "--get-regexp", `url\..*\.insteadOf`)
	assert.Contains(t, output, "https://gen:fallback-token@github.com/")
	assert.Contains(t, output, "https://github.com/")
}

func TestCreateSuggestionPR_SourceBranchAware(t *testing.T) {
	tests := []struct {
		name                  string
		sourceBranch          string
		expectedTitleContains []string
		expectedBaseBranch    string
	}{
		{
			name:                  "main branch - no source context",
			sourceBranch:          "main",
			expectedTitleContains: []string{"chore: 🐝 Suggest OpenAPI changes"},
			expectedBaseBranch:    "main",
		},
		{
			name:                  "master branch - no source context",
			sourceBranch:          "master",
			expectedTitleContains: []string{"chore: 🐝 Suggest OpenAPI changes"},
			expectedBaseBranch:    "master",
		},
		{
			name:                  "feature branch - includes source context",
			sourceBranch:          "feature/openapi-updates",
			expectedTitleContains: []string{"chore: 🐝 Suggest OpenAPI changes", "[feature-openapi-updates]"},
			expectedBaseBranch:    "feature/openapi-updates",
		},
		{
			name:                  "bugfix branch with special chars",
			sourceBranch:          "bugfix/api-v1.2_fix",
			expectedTitleContains: []string{"chore: 🐝 Suggest OpenAPI changes", "[bugfix-api-v1.2-fix]"},
			expectedBaseBranch:    "bugfix/api-v1.2_fix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			originalGithubRef := os.Getenv("GITHUB_REF")
			originalGithubHeadRef := os.Getenv("GITHUB_HEAD_REF")
			originalGithubBaseRef := os.Getenv("GITHUB_BASE_REF")
			originalGithubEventName := os.Getenv("GITHUB_EVENT_NAME")

			defer func() {
				os.Setenv("GITHUB_REF", originalGithubRef)
				os.Setenv("GITHUB_HEAD_REF", originalGithubHeadRef)
				os.Setenv("GITHUB_BASE_REF", originalGithubBaseRef)
				os.Setenv("GITHUB_EVENT_NAME", originalGithubEventName)
			}()

			// Set up test environment
			if tt.sourceBranch == "main" || tt.sourceBranch == "master" {
				os.Setenv("GITHUB_REF", "refs/heads/"+tt.sourceBranch)
				os.Setenv("GITHUB_HEAD_REF", "")
				os.Setenv("GITHUB_EVENT_NAME", "push")
			} else {
				os.Setenv("GITHUB_REF", "refs/pull/123/merge")
				os.Setenv("GITHUB_HEAD_REF", tt.sourceBranch)
				os.Setenv("GITHUB_BASE_REF", "main")
				os.Setenv("GITHUB_EVENT_NAME", "pull_request")
			}

			// Test the title generation logic directly
			title := getSuggestPRTitlePrefix()
			sourceBranch := environment.GetSourceBranch()
			isMainBranch := environment.IsMainBranch(sourceBranch)
			if !isMainBranch {
				sanitizedSourceBranch := environment.SanitizeBranchName(sourceBranch)
				title = title + " [" + sanitizedSourceBranch + "]"
			}

			targetBaseBranch := environment.GetTargetBaseBranch()
			if strings.HasPrefix(targetBaseBranch, "refs/") {
				targetBaseBranch = strings.TrimPrefix(targetBaseBranch, "refs/heads/")
			}

			// Verify title contains expected parts
			for _, expectedPart := range tt.expectedTitleContains {
				assert.Contains(t, title, expectedPart, "Title should contain: %s", expectedPart)
			}

			// Verify base branch
			assert.Equal(t, tt.expectedBaseBranch, targetBaseBranch)
		})
	}
}
