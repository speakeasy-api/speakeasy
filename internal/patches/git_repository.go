package patches

import (
	"github.com/speakeasy-api/speakeasy/internal/git"
)

// gitRepositoryWrapper wraps git.Repository to implement GitRepository interface.
// This handles the TreeEntry type conversion between packages.
type gitRepositoryWrapper struct {
	repo *git.Repository
}

var _ GitRepository = (*gitRepositoryWrapper)(nil)

// WrapGitRepository wraps a git.Repository to implement the GitRepository interface.
func WrapGitRepository(repo *git.Repository) GitRepository {
	return &gitRepositoryWrapper{repo: repo}
}

func (w *gitRepositoryWrapper) IsNil() bool {
	return w.repo.IsNil()
}

func (w *gitRepositoryWrapper) HasObject(hash string) bool {
	return w.repo.HasObject(hash)
}

func (w *gitRepositoryWrapper) GetBlob(hash string) ([]byte, error) {
	return w.repo.GetBlob(hash)
}

func (w *gitRepositoryWrapper) WriteBlob(content []byte) (string, error) {
	return w.repo.WriteBlob(content)
}

func (w *gitRepositoryWrapper) WriteTree(entries []TreeEntry) (string, error) {
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

func (w *gitRepositoryWrapper) CommitTree(treeHash, parentHash, message string) (string, error) {
	return w.repo.CommitTree(treeHash, parentHash, message)
}

func (w *gitRepositoryWrapper) GetRef(refName string) (string, error) {
	return w.repo.GetRef(refName)
}

func (w *gitRepositoryWrapper) UpdateRef(refName, newHash, oldHash string) error {
	return w.repo.UpdateRef(refName, newHash, oldHash)
}

func (w *gitRepositoryWrapper) FetchRef(refSpec string) error {
	return w.repo.FetchRef(refSpec)
}

func (w *gitRepositoryWrapper) PushRef(refSpec string) error {
	return w.repo.PushRef(refSpec)
}

func (w *gitRepositoryWrapper) SetConflictState(path string, base, ours, theirs []byte, isExecutable bool) error {
	return w.repo.SetConflictState(path, base, ours, theirs, isExecutable)
}
