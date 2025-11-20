package merging

import (
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/git"
)

// GitHistoryProvider implements HistoryProvider using the local git repository.
type GitHistoryProvider struct {
	repo *git.Repository
}

// NewGitHistoryProvider creates a new provider backed by the given git repository.
func NewGitHistoryProvider(repo *git.Repository) *GitHistoryProvider {
	return &GitHistoryProvider{
		repo: repo,
	}
}

// GetPristine retrieves the blob content for the given hash.
func (p *GitHistoryProvider) GetPristine(blobHash string) ([]byte, error) {
	if blobHash == "" {
		return nil, fmt.Errorf("no pristine hash provided")
	}

	content, err := p.repo.GetBlob(blobHash)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve pristine blob %s: %w", blobHash, err)
	}

	return content, nil
}
