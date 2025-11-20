package git

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// WriteBlob writes content to the git object database and returns the SHA-1 hash.
func (r *Repository) WriteBlob(content []byte) (string, error) {
	obj := r.repo.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	obj.SetSize(int64(len(content)))

	writer, err := obj.Writer()
	if err != nil {
		return "", fmt.Errorf("failed to create object writer: %w", err)
	}

	if _, err := writer.Write(content); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to write blob content: %w", err)
	}
	writer.Close()

	hash, err := r.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return "", fmt.Errorf("failed to store blob: %w", err)
	}

	return hash.String(), nil
}

// GetBlob retrieves the content of a blob by its SHA-1 hash.
func (r *Repository) GetBlob(hash string) ([]byte, error) {
	// Strip "sha1:" prefix if present (common in some systems)
	hash = strings.TrimPrefix(hash, "sha1:")

	h := plumbing.NewHash(hash)
	blob, err := r.repo.BlobObject(h)
	if err != nil {
		return nil, fmt.Errorf("failed to find blob %s: %w", hash, err)
	}

	reader, err := blob.Reader()
	if err != nil {
		return nil, fmt.Errorf("failed to open blob reader: %w", err)
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to read blob content: %w", err)
	}

	return buf.Bytes(), nil
}

// TreeEntry represents a file or directory in a git tree.
type TreeEntry struct {
	Name string
	Mode string // "100644", "100755", "040000"
	Hash string
}

// WriteTree creates a tree object from the provided entries and returns its hash.
// Note: This creates a single tree object (flat). For deep trees, hashes must be pre-calculated.
func (r *Repository) WriteTree(entries []TreeEntry) (string, error) {
	var treeEntries []object.TreeEntry

	for _, e := range entries {
		h := plumbing.NewHash(e.Hash)

		// Parse mode
		var mode filemode.FileMode
		switch e.Mode {
		case "100644":
			mode = filemode.Regular
		case "100755":
			mode = filemode.Executable
		case "040000":
			mode = filemode.Dir
		case "120000":
			mode = filemode.Symlink
		default:
			// Default to regular file if unsure
			mode = filemode.Regular
		}

		treeEntries = append(treeEntries, object.TreeEntry{
			Name: e.Name,
			Mode: mode,
			Hash: h,
		})
	}

	tree := object.Tree{
		Entries: treeEntries,
	}

	obj := r.repo.Storer.NewEncodedObject()
	if err := tree.Encode(obj); err != nil {
		return "", fmt.Errorf("failed to encode tree: %w", err)
	}

	hash, err := r.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return "", fmt.Errorf("failed to store tree: %w", err)
	}

	return hash.String(), nil
}

// CommitTree creates a commit object pointing to a tree and parent(s).
func (r *Repository) CommitTree(treeHash, parent, message string) (string, error) {
	tHash := plumbing.NewHash(treeHash)

	var parents []plumbing.Hash
	if parent != "" {
		parents = append(parents, plumbing.NewHash(parent))
	}

	now := time.Now()
	commit := object.Commit{
		Author: object.Signature{
			Name:  "Speakeasy Bot",
			Email: "bot@speakeasyapi.dev",
			When:  now,
		},
		Committer: object.Signature{
			Name:  "Speakeasy Bot",
			Email: "bot@speakeasyapi.dev",
			When:  now,
		},
		Message:      message,
		TreeHash:     tHash,
		ParentHashes: parents,
	}

	obj := r.repo.Storer.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return "", fmt.Errorf("failed to encode commit: %w", err)
	}

	hash, err := r.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return "", fmt.Errorf("failed to store commit: %w", err)
	}

	return hash.String(), nil
}

// UpdateRef updates a git reference to point to a specific commit hash.
// If oldHash is provided, it performs a compare-and-swap (optimistic locking).
// Pass "" as oldHash to force update.
func (r *Repository) UpdateRef(refName, newHash, oldHash string) error {
	ref := plumbing.ReferenceName(refName)
	h := plumbing.NewHash(newHash)

	var oldH plumbing.Hash
	if oldHash != "" {
		oldH = plumbing.NewHash(oldHash)
	}

	newRef := plumbing.NewHashReference(ref, h)

	if oldHash == "" {
		return r.repo.Storer.SetReference(newRef)
	}

	return r.repo.Storer.CheckAndSetReference(newRef, plumbing.NewHashReference(ref, oldH))
}

// GetRef returns the current hash of a reference, or error if not found.
func (r *Repository) GetRef(refName string) (string, error) {
	ref, err := r.repo.Reference(plumbing.ReferenceName(refName), true)
	if err != nil {
		return "", err
	}
	return ref.Hash().String(), nil
}

// ResolveRevision resolves a revision (like "HEAD", "branchname") to a hash.
func (r *Repository) ResolveRevision(revision string) (string, error) {
	h, err := r.repo.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return "", err
	}
	return h.String(), nil
}

// SetConflictState sets up git's index to show a file as conflicted.
// This writes the base (stage 1), ours (stage 2), and theirs (stage 3) versions
// as index entries, enabling standard git conflict resolution tools:
//   - git status shows "both modified"
//   - git mergetool can resolve conflicts
//   - git checkout --ours/--theirs works
//   - git add marks as resolved
//
// If base is nil, only stages 2 and 3 are written (new file conflict).
func (r *Repository) SetConflictState(path string, base, ours, theirs []byte, isExecutable bool) error {
	if r.repo == nil {
		return fmt.Errorf("git repository not initialized")
	}

	// Read the current index
	idx, err := r.repo.Storer.Index()
	if err != nil {
		return fmt.Errorf("failed to read index: %w", err)
	}

	// Determine file mode
	mode := filemode.Regular
	if isExecutable {
		mode = filemode.Executable
	}

	// Remove any existing stage-0 entry for this path
	newEntries := make([]*index.Entry, 0, len(idx.Entries))
	for _, e := range idx.Entries {
		if e.Name != path {
			newEntries = append(newEntries, e)
		}
	}
	idx.Entries = newEntries

	now := time.Now()

	// Write blobs and add stage entries
	// Stage 1: Base/ancestor (if exists)
	if base != nil {
		baseHash, err := r.WriteBlob(base)
		if err != nil {
			return fmt.Errorf("failed to write base blob: %w", err)
		}
		idx.Entries = append(idx.Entries, &index.Entry{
			Name:       path,
			Hash:       plumbing.NewHash(baseHash),
			Mode:       mode,
			Stage:      index.AncestorMode, // Stage 1
			CreatedAt:  now,
			ModifiedAt: now,
			Size:       uint32(len(base)),
		})
	}

	// Stage 2: Ours
	oursHash, err := r.WriteBlob(ours)
	if err != nil {
		return fmt.Errorf("failed to write ours blob: %w", err)
	}
	idx.Entries = append(idx.Entries, &index.Entry{
		Name:       path,
		Hash:       plumbing.NewHash(oursHash),
		Mode:       mode,
		Stage:      index.OurMode, // Stage 2
		CreatedAt:  now,
		ModifiedAt: now,
		Size:       uint32(len(ours)),
	})

	// Stage 3: Theirs
	theirsHash, err := r.WriteBlob(theirs)
	if err != nil {
		return fmt.Errorf("failed to write theirs blob: %w", err)
	}
	idx.Entries = append(idx.Entries, &index.Entry{
		Name:       path,
		Hash:       plumbing.NewHash(theirsHash),
		Mode:       mode,
		Stage:      index.TheirMode, // Stage 3
		CreatedAt:  now,
		ModifiedAt: now,
		Size:       uint32(len(theirs)),
	})

	// Sort entries by (Name, Stage) as required by git index format
	sort.Slice(idx.Entries, func(i, j int) bool {
		if idx.Entries[i].Name != idx.Entries[j].Name {
			return idx.Entries[i].Name < idx.Entries[j].Name
		}
		return idx.Entries[i].Stage < idx.Entries[j].Stage
	})

	// Write the index back
	if err := r.repo.Storer.SetIndex(idx); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	return nil
}
