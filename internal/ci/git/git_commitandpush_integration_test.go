package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/speakeasy-api/versioning-reports/versioning"
	"github.com/stretchr/testify/require"
)

// TestCommitAndPush_Integration_UnsignedDefaultBranch drives CommitAndPush
// end-to-end through the unsigned default-branch path against a real bare
// remote. Confirms the full Add → Commit → push chain lands on the remote.
func TestCommitAndPush_Integration_UnsignedDefaultBranch(t *testing.T) {
	// Uses t.Setenv, so not parallel.
	t.Setenv("INPUT_SIGNED_COMMITS", "false")
	t.Setenv("INPUT_BRANCH_NAME", "")
	t.Setenv("INPUT_MODE", "pr")
	t.Setenv("INPUT_WORKING_DIRECTORY", "")
	t.Setenv("INPUT_FEATURE_BRANCH", "")
	t.Setenv("INPUT_ENABLE_SDK_CHANGELOG", "false")

	root := t.TempDir()
	remote := filepath.Join(root, "remote")
	clone := filepath.Join(root, "clone")

	require.NoError(t, exec.Command("git", "init", "--bare", "-b", "main", remote).Run())

	seed := filepath.Join(root, "seed")
	require.NoError(t, exec.Command("git", "init", "-b", "main", seed).Run())
	runGitCLI(t, seed, "config", "user.email", "speakeasybot@speakeasyapi.dev")
	runGitCLI(t, seed, "config", "user.name", "speakeasybot")
	require.NoError(t, os.WriteFile(filepath.Join(seed, "sdk.ts"), []byte("// v0\n"), 0644))
	runGitCLI(t, seed, "add", "sdk.ts")
	runGitCLI(t, seed, "commit", "-m", "initial")
	runGitCLI(t, seed, "remote", "add", "origin", remote)
	runGitCLI(t, seed, "push", "origin", "main")

	require.NoError(t, exec.Command("git", "clone", remote, clone).Run())
	runGitCLI(t, clone, "config", "user.email", "speakeasybot@speakeasyapi.dev")
	runGitCLI(t, clone, "config", "user.name", "speakeasybot")

	branch := "speakeasy-sdk-regen-1779200000"
	runGitCLI(t, clone, "checkout", "-b", branch)

	require.NoError(t, os.WriteFile(filepath.Join(clone, "sdk.ts"), []byte("// v1 regenerated\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(clone, "new_generated.ts"), []byte("// new file\n"), 0644))

	t.Chdir(clone)
	g := New("test-token")
	require.NoError(t, g.OpenRepo())

	headBefore := strings.TrimSpace(string(gitOutput(t, "-C", clone, "rev-parse", "HEAD")))
	require.False(t, remoteHasBranch(t, remote, branch))

	commitHash, err := g.CommitAndPush(
		"openapi-doc-v1",
		"1.500.0",
		"",
		"run-workflow",
		false,
		(*versioning.MergedVersionReport)(nil),
	)
	require.NoError(t, err)
	require.NotEmpty(t, commitHash)

	remoteHead := strings.TrimSpace(string(gitOutput(t, "-C", remote, "rev-parse", branch)))
	localHead := strings.TrimSpace(string(gitOutput(t, "-C", clone, "rev-parse", "HEAD")))
	require.Equal(t, localHead, remoteHead)
	require.NotEqual(t, headBefore, localHead)

	listOut := gitOutput(t, "-C", remote, "show", "--name-only", "--pretty=format:", branch)
	files := strings.Fields(string(listOut))
	require.Contains(t, files, "sdk.ts")
	require.Contains(t, files, "new_generated.ts")
}

func remoteHasBranch(t *testing.T, bareDir, branch string) bool {
	t.Helper()
	out, err := exec.Command("git", "-C", bareDir, "rev-parse", "--verify", branch).CombinedOutput()
	return err == nil && len(out) > 0
}
