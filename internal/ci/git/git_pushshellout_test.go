package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	sharedgit "github.com/speakeasy-api/speakeasy/internal/git"
	"github.com/stretchr/testify/require"
)

// TestCommitAndPush_DefaultBranchShellsOut exercises the shell-out push
// helper call against a real bare remote.
func TestCommitAndPush_DefaultBranchShellsOut(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	remote := filepath.Join(root, "remote")
	clone := filepath.Join(root, "clone")

	require.NoError(t, exec.Command("git", "init", "--bare", "-b", "main", remote).Run())

	seed := filepath.Join(root, "seed")
	require.NoError(t, exec.Command("git", "init", "-b", "main", seed).Run())
	runGitCLI(t, seed, "config", "user.email", "t@t")
	runGitCLI(t, seed, "config", "user.name", "t")
	require.NoError(t, os.WriteFile(filepath.Join(seed, "README.md"), []byte("init\n"), 0644))
	runGitCLI(t, seed, "add", "README.md")
	runGitCLI(t, seed, "commit", "-m", "init")
	runGitCLI(t, seed, "remote", "add", "origin", remote)
	runGitCLI(t, seed, "push", "origin", "main")

	require.NoError(t, exec.Command("git", "clone", remote, clone).Run())
	runGitCLI(t, clone, "config", "user.email", "t@t")
	runGitCLI(t, clone, "config", "user.name", "t")

	branch := "speakeasy-sdk-regen-test"
	runGitCLI(t, clone, "checkout", "-b", branch)
	require.NoError(t, os.WriteFile(filepath.Join(clone, "generated.txt"), []byte("v1\n"), 0644))
	runGitCLI(t, clone, "add", ".")
	runGitCLI(t, clone, "commit", "-m", "regenerated")

	repo, err := git.PlainOpenWithOptions(clone, &git.PlainOpenOptions{DetectDotGit: true})
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)
	g := &Git{repo: repo, repoRoot: wt.Filesystem.Root()}

	gotBranch, err := g.GetCurrentBranch()
	require.NoError(t, err)
	require.Equal(t, branch, gotBranch)

	out, err := sharedgit.RunGitCommand(g.repoRoot, "push", "--force", "origin", gotBranch)
	require.NoError(t, err, "push --force: %s", out)

	localHead := strings.TrimSpace(string(gitOutput(t, "-C", clone, "rev-parse", "HEAD")))
	remoteHead := strings.TrimSpace(string(gitOutput(t, "-C", remote, "rev-parse", branch)))
	require.Equal(t, localHead, remoteHead)

	// Force-push semantics: rewind, recommit, push again, remote must move.
	runGitCLI(t, clone, "reset", "--hard", "HEAD^")
	require.NoError(t, os.WriteFile(filepath.Join(clone, "generated.txt"), []byte("v2\n"), 0644))
	runGitCLI(t, clone, "add", ".")
	runGitCLI(t, clone, "commit", "-m", "regenerated v2")

	out, err = sharedgit.RunGitCommand(g.repoRoot, "push", "--force", "origin", gotBranch)
	require.NoError(t, err, "force push: %s", out)

	newLocalHead := strings.TrimSpace(string(gitOutput(t, "-C", clone, "rev-parse", "HEAD")))
	newRemoteHead := strings.TrimSpace(string(gitOutput(t, "-C", remote, "rev-parse", branch)))
	require.Equal(t, newLocalHead, newRemoteHead)
	require.NotEqual(t, remoteHead, newRemoteHead)
}

func gitOutput(t *testing.T, args ...string) []byte {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return out
}
