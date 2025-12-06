package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

// Root returns the root directory of the repository (where .git is located).
// Returns empty string if the repository is not initialized.
func (r *Repository) Root() string {
	if r.repo == nil {
		return ""
	}

	// Get the worktree to find the filesystem root
	wt, err := r.repo.Worktree()
	if err != nil {
		return ""
	}

	return wt.Filesystem.Root()
}

// HasObject checks if a blob or commit exists in the local object database.
// Uses native git commands to ensure we see objects fetched by native git fetch.
func (r *Repository) HasObject(hash string) bool {
	if r.repo == nil {
		return false
	}

	// Strip "sha1:" prefix if present
	hash = strings.TrimPrefix(hash, "sha1:")

	// Use native git to check object existence.
	// This is necessary because go-git's storer may not see objects
	// that were fetched via native git fetch commands.
	repoRoot := r.Root()
	if repoRoot == "" {
		return false
	}

	cmd := exec.Command("git", "cat-file", "-e", hash)
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}

// FetchRef fetches a specific ref from origin.
// refSpec format: "+refs/speakeasy/gen/uuid:refs/speakeasy/gen/uuid"
func (r *Repository) FetchRef(refSpec string) error {
	if r.repo == nil {
		return nil
	}

	if _, err := r.repo.Remote("origin"); err != nil {
		return fmt.Errorf("remote 'origin' not found: %w", err)
	}

	cmd := exec.Command("git", "fetch", "--force", "origin", refSpec)
	cmd.Dir = r.Root()
	cmd.Env = os.Environ()

	return cmd.Run()
}

// PushRef pushes a ref to origin.
// refSpec format: "refs/speakeasy/gen/uuid:refs/speakeasy/gen/uuid"
func (r *Repository) PushRef(refSpec string) error {
	if r.repo == nil {
		return nil
	}

	if _, err := r.repo.Remote("origin"); err != nil {
		return fmt.Errorf("remote 'origin' not found: %w", err)
	}

	cmd := exec.Command("git", "push", "origin", refSpec)
	cmd.Dir = r.Root()
	cmd.Env = os.Environ()

	return cmd.Run()
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
	defaultBranch string = "main"
)

// Retrieves the default branch from the user's global git config
// e.g
// git config --get init.defaultbranch
// To set:
// git config --global init.defaultbranch main
func getDefaultGitBranch() string {
	if cfg, _ := config.LoadConfig(config.GlobalScope); cfg != nil {
		if branch := cfg.Init.DefaultBranch; branch != "" {
			return branch
		}
	}
	return defaultBranch
}
