package merging

import (
	"io/fs"
)

// MergeStatus represents the outcome of a file merge operation.
type MergeStatus string

const (
	MergeStatusClean       MergeStatus = "CLEAN"
	MergeStatusConflict    MergeStatus = "CONFLICT"
	MergeStatusBinary      MergeStatus = "BINARY"
	MergeStatusFastForward MergeStatus = "FAST_FORWARD" // Used when Current == Base
	MergeStatusCreated     MergeStatus = "CREATED"      // New file
	MergeStatusDeleted     MergeStatus = "DELETED"
	MergeStatusSkipped     MergeStatus = "SKIPPED"
)

// VirtualFile represents a file in memory (usually from the generator).
type VirtualFile struct {
	Path     string
	Content  []byte
	Mode     fs.FileMode
	IsBinary bool
}

// TrackedFile represents the metadata stored in gen.yaml/gen.lock for RTE.
type TrackedFile struct {
	ID                string `yaml:"id,omitempty"`
	PristineBlobHash  string `yaml:"pristine_blob_hash,omitempty"`
	LastWriteChecksum string `yaml:"last_write_checksum,omitempty"`
}

// MergeResult holds the result of merging a single file.
type MergeResult struct {
	Path         string
	Content      []byte
	Status       MergeStatus
	HasConflicts bool
	Conflicts    []Conflict
	Error        error
}

// Conflict represents a specific conflict region in a file.
type Conflict struct {
	StartLine int
	EndLine   int
	Message   string
}

// MergeStrategy defines how the merger should behave (e.g., favor new, favor current).
type MergeStrategy int

const (
	StrategyDefault MergeStrategy = iota
	StrategyOurs                  // Favor Current (User)
	StrategyTheirs                // Favor New (Generator)
)

// HistoryProvider abstracts the retrieval of historical file versions.
type HistoryProvider interface {
	// GetPristine retrieves the content of the base revision (Base).
	GetPristine(blobHash string) ([]byte, error)
}

// Merger abstracts the algorithm for 3-way merging.
type Merger interface {
	// Merge performs a 3-way merge: Base + (Current-Base) + (New-Base)
	Merge(base, current, new []byte) (*MergeResult, error)
}

// GenConfigAccessor abstracts access to the generation configuration (gen.yaml).
type GenConfigAccessor interface {
	GetTrackedFile(path string) *TrackedFile
	UpdateTrackedFile(path, blobHash, checksum string)
}
