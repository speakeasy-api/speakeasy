package git

import (
	"errors"
	"fmt"

	gitc "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

type Repository struct {
	repo *gitc.Repository
}

// NewLocalRepository will attempt to open a pre-existing git repository in the given directory
// If no repository is found, it will return an empty Repository
func NewLocalRepository(dir string) (*Repository, error) {
	repo, err := gitc.PlainOpenWithOptions(dir, &gitc.PlainOpenOptions{
		DetectDotGit: true,
	})
	if errors.Is(err, gitc.ErrRepositoryNotExists) {
		return &Repository{}, nil
	} else if err != nil {
		return &Repository{}, fmt.Errorf("git: %w", err)
	}

	return &Repository{repo: repo}, nil
}

// InitLocalRepository will initialize a new git repository in the given directory
func InitLocalRepository(dir string) (*Repository, error) {
	// Try to retrieve the default branch from the global git config
	// if the user has an explicit default branch set. Otherwise it
	// will default to master.
	branch := getDefaultGitBranch()
	reference := plumbing.NewBranchReferenceName(branch)

	repo, err := gitc.PlainInitWithOptions(dir, &gitc.PlainInitOptions{
		Bare: false,
		InitOptions: gitc.InitOptions{
			DefaultBranch: reference,
		},
	})

	if err != nil {
		return &Repository{}, err
	}

	return &Repository{repo: repo}, nil
}

func (r *Repository) IsNil() bool {
	return r.repo == nil
}

func (r *Repository) HeadHash() (string, error) {
	if r.IsNil() {
		return "", nil
	}

	head, err := r.repo.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("git: %w", err)
	}

	return head.Hash().String(), nil
}

const (
	defaultBranch string = "master"
)

// Retrieves the default branch from the user's global git config
// e.g
// git config --get init.defaultbranch
func getDefaultGitBranch() string {
	if cfg, _ := config.LoadConfig(config.GlobalScope); cfg != nil {
		if branch := cfg.Raw.Section("init").Options.Get("defaultBranch"); branch != "" {
			return branch
		}
	}
	return defaultBranch
}
