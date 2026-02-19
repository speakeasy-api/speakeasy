package git

import (
	"os"
	"path/filepath"
	"testing"

	gitc "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTestRepo creates a temporary git repository with an initial commit on "main".
func initTestRepo(t *testing.T) *Repository {
	t.Helper()

	dir := t.TempDir()

	repo, err := gitc.PlainInitWithOptions(dir, &gitc.PlainInitOptions{
		InitOptions: gitc.InitOptions{
			DefaultBranch: plumbing.NewBranchReferenceName("main"),
		},
	})
	require.NoError(t, err)

	// Create an initial commit so HEAD exists
	wt, err := repo.Worktree()
	require.NoError(t, err)

	f, err := os.Create(filepath.Join(dir, "README.md"))
	require.NoError(t, err)
	_, err = f.WriteString("# test")
	require.NoError(t, err)
	f.Close()

	_, err = wt.Add("README.md")
	require.NoError(t, err)

	_, err = wt.Commit("initial commit", &gitc.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
		},
	})
	require.NoError(t, err)

	return &Repository{repo: repo}
}

func TestHeadBranch_NilRepo(t *testing.T) {
	t.Parallel()

	r := &Repository{}
	branch, err := r.HeadBranch()
	require.NoError(t, err)
	assert.Empty(t, branch)
}

func TestHeadBranch_OnBranch(t *testing.T) {
	t.Parallel()

	r := initTestRepo(t)

	branch, err := r.HeadBranch()
	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestHeadBranch_DetachedHEAD(t *testing.T) {
	t.Parallel()

	r := initTestRepo(t)

	// Detach HEAD by checking out a specific commit
	head, err := r.repo.Head()
	require.NoError(t, err)

	wt, err := r.repo.Worktree()
	require.NoError(t, err)

	err = wt.Checkout(&gitc.CheckoutOptions{
		Hash: head.Hash(),
	})
	require.NoError(t, err)

	branch, err := r.HeadBranch()
	require.NoError(t, err)
	assert.Empty(t, branch)
}

func TestCheckoutBranch_NilRepo(t *testing.T) {
	t.Parallel()

	r := &Repository{}
	err := r.CheckoutBranch("main")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository not initialized")
}

func TestCheckoutBranch_ExistingBranch(t *testing.T) {
	t.Parallel()

	r := initTestRepo(t)

	// Create a second branch, then switch back to main
	err := r.CreateBranch("feature")
	require.NoError(t, err)

	branch, err := r.HeadBranch()
	require.NoError(t, err)
	assert.Equal(t, "feature", branch)

	err = r.CheckoutBranch("main")
	require.NoError(t, err)

	branch, err = r.HeadBranch()
	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestCheckoutBranch_NonExistent(t *testing.T) {
	t.Parallel()

	r := initTestRepo(t)

	err := r.CheckoutBranch("nonexistent")
	require.Error(t, err)
}

func TestCreateBranch_NilRepo(t *testing.T) {
	t.Parallel()

	r := &Repository{}
	err := r.CreateBranch("feature")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository not initialized")
}

func TestCreateBranch_NewBranch(t *testing.T) {
	t.Parallel()

	r := initTestRepo(t)

	err := r.CreateBranch("feature-x")
	require.NoError(t, err)

	branch, err := r.HeadBranch()
	require.NoError(t, err)
	assert.Equal(t, "feature-x", branch)
}

func TestCreateBranch_AlreadyExists(t *testing.T) {
	t.Parallel()

	r := initTestRepo(t)

	err := r.CreateBranch("main")
	require.Error(t, err) // "main" already exists
}
