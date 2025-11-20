package patches

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/patches"
)

// GitRepository defines the low-level Git operations needed by GitAdapter.
// This interface is implemented by internal/git.Repository and abstracts
// away the go-git implementation details.
type GitRepository interface {
	// IsNil returns true if the repository is not initialized.
	IsNil() bool

	// HasObject checks if a blob or commit exists in the local object database.
	HasObject(hash string) bool

	// GetBlob retrieves the content of a blob by its SHA-1 hash.
	GetBlob(hash string) ([]byte, error)

	// WriteBlob writes content to the git object database and returns the SHA-1 hash.
	WriteBlob(content []byte) (string, error)

	// WriteTree creates a tree object from the provided entries and returns its hash.
	WriteTree(entries []TreeEntry) (string, error)

	// CommitTree creates a commit object pointing to a tree and optional parent.
	CommitTree(treeHash, parentHash, message string) (string, error)

	// GetRef returns the current hash of a reference, or error if not found.
	GetRef(refName string) (string, error)

	// UpdateRef updates a git reference to point to a specific commit hash.
	UpdateRef(refName, newHash, oldHash string) error

	// FetchRef fetches a specific ref from origin.
	// refSpec format: "+refs/speakeasy/gen/uuid:refs/speakeasy/gen/uuid"
	FetchRef(refSpec string) error

	// PushRef pushes a ref to origin.
	// refSpec format: "refs/speakeasy/gen/uuid:refs/speakeasy/gen/uuid"
	PushRef(refSpec string) error

	// SetConflictState sets up git's index to show a file as conflicted.
	// This writes stage 1 (base), 2 (ours), 3 (theirs) entries to the index.
	// If base is nil, only stages 2 and 3 are written (new file conflict).
	SetConflictState(path string, base, ours, theirs []byte, isExecutable bool) error
}

// TreeEntry represents a file or directory in a git tree.
type TreeEntry struct {
	Name string // filename or directory name
	Mode string // "100644" (file), "100755" (executable), "040000" (directory)
	Hash string // SHA-1 hash of the blob or subtree
}

// GitAdapter implements the patches.Git interface using a GitRepository.
// It provides all Git operations needed for Round-Trip Engineering.
type GitAdapter struct {
	repo GitRepository
}

var _ patches.Git = (*GitAdapter)(nil)

// NewGitAdapter creates a new GitAdapter wrapping the given GitRepository.
func NewGitAdapter(repo GitRepository) *GitAdapter {
	return &GitAdapter{repo: repo}
}

// HasObject checks if a blob or commit exists in the local Git object database.
func (g *GitAdapter) HasObject(hash string) bool {
	if g.repo.IsNil() {
		return false
	}
	return g.repo.HasObject(hash)
}

// ReadBlob returns the content of a specific blob hash.
func (g *GitAdapter) ReadBlob(hash string) ([]byte, error) {
	if g.repo.IsNil() {
		return nil, fmt.Errorf("git repository not initialized")
	}
	return g.repo.GetBlob(hash)
}

// FetchSnapshot ensures the history for a specific generation UUID exists locally.
// This fetches from the remote using refs/speakeasy/gen/<uuid>.
func (g *GitAdapter) FetchSnapshot(uuid string) error {
	if g.repo.IsNil() {
		return fmt.Errorf("git repository not initialized")
	}

	refName := fmt.Sprintf("refs/speakeasy/gen/%s", uuid)
	refSpec := fmt.Sprintf("+%s:%s", refName, refName)

	err := g.repo.FetchRef(refSpec)

	// If fetch fails, check if we already have the ref locally
	if err != nil {
		if _, refErr := g.repo.GetRef(refName); refErr == nil {
			// We have it locally, no need to fetch
			return nil
		}
		return fmt.Errorf("failed to fetch snapshot %s: %w", uuid, err)
	}

	return nil
}

// WriteObject hashes content into the Git object database (as a blob) and returns the SHA1.
func (g *GitAdapter) WriteObject(content []byte) (string, error) {
	if g.repo.IsNil() {
		return "", fmt.Errorf("git repository not initialized")
	}
	return g.repo.WriteBlob(content)
}

// CreateSnapshotTree builds a Git Tree object from a map of "path" -> "blobHash".
// It handles nested directories by creating intermediate tree objects.
func (g *GitAdapter) CreateSnapshotTree(fileHashes map[string]string) (string, error) {
	if g.repo.IsNil() {
		return "", fmt.Errorf("git repository not initialized")
	}

	// Build tree structure recursively
	return g.buildTreeRecursive(fileHashes, "")
}

// buildTreeRecursive creates a tree object for the given prefix, handling nested directories.
func (g *GitAdapter) buildTreeRecursive(fileHashes map[string]string, prefix string) (string, error) {
	// Group files by their first path component at this level
	type fileInfo struct {
		name     string
		hash     string
		isTree   bool
		children map[string]string // for directories
	}

	entries := make(map[string]*fileInfo)

	for fullPath, hash := range fileHashes {
		// Remove prefix to get relative path
		relPath := fullPath
		if prefix != "" {
			if !strings.HasPrefix(fullPath, prefix+"/") {
				continue
			}
			relPath = strings.TrimPrefix(fullPath, prefix+"/")
		}

		// Split into first component and rest
		parts := strings.SplitN(relPath, "/", 2)
		firstName := parts[0]

		// Skip empty path components (shouldn't happen with normalized paths)
		if firstName == "" {
			continue
		}

		if len(parts) == 1 {
			// This is a file at this level
			entries[firstName] = &fileInfo{
				name:   firstName,
				hash:   hash,
				isTree: false,
			}
		} else {
			// This is a directory
			if entries[firstName] == nil {
				entries[firstName] = &fileInfo{
					name:     firstName,
					isTree:   true,
					children: make(map[string]string),
				}
			}
			// Add to children
			entries[firstName].children[fullPath] = hash
		}
	}

	// Build tree entries
	var treeEntries []TreeEntry

	// Get sorted names for deterministic output
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		info := entries[name]

		if info.isTree {
			// Recursively build subtree
			childPrefix := name
			if prefix != "" {
				childPrefix = prefix + "/" + name
			}
			subtreeHash, err := g.buildTreeRecursive(fileHashes, childPrefix)
			if err != nil {
				return "", fmt.Errorf("failed to build subtree %s: %w", childPrefix, err)
			}
			treeEntries = append(treeEntries, TreeEntry{
				Name: name,
				Mode: "040000", // directory
				Hash: subtreeHash,
			})
		} else {
			// Regular file - determine mode from file extension
			mode := "100644" // regular file
			if isExecutableFile(name) {
				mode = "100755"
			}
			treeEntries = append(treeEntries, TreeEntry{
				Name: name,
				Mode: mode,
				Hash: info.hash,
			})
		}
	}

	return g.repo.WriteTree(treeEntries)
}

// isExecutableFile returns true if the file should be marked as executable.
func isExecutableFile(name string) bool {
	ext := path.Ext(name)
	switch ext {
	case ".sh", ".bash", ".zsh":
		return true
	}
	// Also check for common executable names
	switch name {
	case "gradlew", "mvnw":
		return true
	}
	return false
}

// CommitSnapshot creates a commit object linking a tree to its parent.
func (g *GitAdapter) CommitSnapshot(treeHash, parentHash, message string) (string, error) {
	if g.repo.IsNil() {
		return "", fmt.Errorf("git repository not initialized")
	}
	return g.repo.CommitTree(treeHash, parentHash, message)
}

// PushSnapshot syncs the ref to the server asynchronously.
// It pushes the commit to refs/speakeasy/gen/<uuid>.
func (g *GitAdapter) PushSnapshot(commitHash, uuid string) error {
	if g.repo.IsNil() {
		return fmt.Errorf("git repository not initialized")
	}

	refName := fmt.Sprintf("refs/speakeasy/gen/%s", uuid)

	// First, create the local ref
	if err := g.repo.UpdateRef(refName, commitHash, ""); err != nil {
		return fmt.Errorf("failed to create local ref %s: %w", refName, err)
	}

	// Push asynchronously - fire and forget
	go func() {
		refSpec := fmt.Sprintf("%s:%s", refName, refName)
		if err := g.repo.PushRef(refSpec); err != nil {
			// Log warning but don't fail - this is async
			fmt.Printf("Warning: failed to push snapshot %s: %v\n", uuid, err)
		}
	}()

	return nil
}

// SetConflictState sets up git's index to show a file as conflicted.
// This enables standard git conflict resolution tools (git status, git mergetool, etc.).
func (g *GitAdapter) SetConflictState(path string, base, ours, theirs []byte, isExecutable bool) error {
	if g.repo.IsNil() {
		return fmt.Errorf("git repository not initialized")
	}
	return g.repo.SetConflictState(path, base, ours, theirs, isExecutable)
}
