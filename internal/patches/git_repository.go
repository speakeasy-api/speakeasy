package patches

import (
	"github.com/speakeasy-api/speakeasy/internal/git"
)

// GitRepositoryWrapper wraps git.Repository to implement GitRepository interface.
// This handles the TreeEntry type conversion between packages.
type GitRepositoryWrapper struct {
	repo *git.Repository
}

var _ GitRepository = (*GitRepositoryWrapper)(nil)

// WrapGitRepository wraps a git.Repository to implement the GitRepository interface.
func WrapGitRepository(repo *git.Repository) *GitRepositoryWrapper {
	return &GitRepositoryWrapper{repo: repo}
}

func (w *GitRepositoryWrapper) IsNil() bool {
	return w.repo.IsNil()
}

func (w *GitRepositoryWrapper) Root() string {
	return w.repo.Root()
}

func (w *GitRepositoryWrapper) HasObject(hash string) bool {
	return w.repo.HasObject(hash)
}

func (w *GitRepositoryWrapper) GetBlob(hash string) ([]byte, error) {
	return w.repo.GetBlob(hash)
}

func (w *GitRepositoryWrapper) WriteBlob(content []byte) (string, error) {
	return w.repo.WriteBlob(content)
}

func (w *GitRepositoryWrapper) WriteTree(entries []TreeEntry) (string, error) {
	// Convert patches.TreeEntry to git.TreeEntry
	gitEntries := make([]git.TreeEntry, len(entries))
	for i, e := range entries {
		gitEntries[i] = git.TreeEntry{
			Name: e.Name,
			Mode: e.Mode,
			Hash: e.Hash,
		}
	}
	return w.repo.WriteTree(gitEntries)
}

func (w *GitRepositoryWrapper) CommitTree(treeHash, parentHash, message string) (string, error) {
	return w.repo.CommitTree(treeHash, parentHash, message)
}

func (w *GitRepositoryWrapper) GetRef(refName string) (string, error) {
	return w.repo.GetRef(refName)
}

func (w *GitRepositoryWrapper) UpdateRef(refName, newHash, oldHash string) error {
	return w.repo.UpdateRef(refName, newHash, oldHash)
}

func (w *GitRepositoryWrapper) FetchRef(refSpec string) error {
	return w.repo.FetchRef(refSpec)
}

func (w *GitRepositoryWrapper) PushRef(refSpec string) error {
	return w.repo.PushRef(refSpec)
}

func (w *GitRepositoryWrapper) SetConflictState(path string, base, ours, theirs []byte, isExecutable bool) error {
	return w.repo.SetConflictState(path, base, ours, theirs, isExecutable)
}
