# Persistent Edits Feature - Architecture Simplification

## Problem Being Solved

The original implementation had these issues:
1. **Random UUIDs stored in lockfile** - caused churn even when persistentEdits wasn't enabled
2. **Complex data flow** - IDs needed to be copied between lockfile → FileTracker → saved lockfile
3. **Race conditions** - timing issues between TrackFile (async) and UpdateTrackedFile (sync)

## New Architecture: UUID v5 Deterministic Default IDs

### Core Concept

- **ID = UUID v5(speakeasy_namespace, file_path)** as default
- ID is deterministic and can be computed on-demand before any files move
- Once persistentEdits is enabled, ID gets embedded in file header
- TrackedFile stores ID for move detection comparison

### Key Design Points

1. **ID IS stored in TrackedFile** - needed for move detection
2. **ID has a deterministic default** - UUID v5(path), computed on demand
3. **No lockfile churn before persistentEdits enabled** - deterministic IDs don't change
4. **Move detection works** - compare stored ID vs scanned ID locations

### Flow

```
Generation 1 (persistentEdits disabled):
- File created at path A
- ID = ComputeFileID(A) = UUID v5(A)
- TrackedFile stores: { ID: UUID v5(A), LastWriteChecksum, PristineGitObject }
- ID NOT embedded in file (persistentEdits disabled)

Generation 2 (persistentEdits enabled):
- File still at path A
- ID embedded in file header
- TrackedFile stores same ID

User moves file A → B:
- Embedded ID stays UUID v5(A) in file content
- Next generation: scan finds ID=UUID v5(A) at path B
- Expected ID for path A = UUID v5(A)
- Scanned ID at path B = UUID v5(A)
- Detected as move: A → B
```

### Code Changes

#### 1. ComputeFileID(path) - Deterministic Short ID
```go
import (
    "crypto/sha1"
    "fmt"
    "path/filepath"
)

func ComputeFileID(path string) string {
    // CRITICAL: Normalize path separators to forward slashes
    // This prevents Windows/Linux path drift from changing the ID
    normalized := filepath.ToSlash(path)

    // Use first 12 hex chars of SHA1 (6 bytes = 48 bits)
    // Collision-safe for 100k+ files per SDK
    hash := sha1.Sum([]byte(normalized))
    return fmt.Sprintf("%x", hash[:6])  // 12 hex chars
}
```

**Why short IDs instead of UUIDs?**
- Deterministic = no need for UUID's global uniqueness guarantees
- 12 chars is readable in file headers: `// @generated-id: a1b2c3d4e5f6`
- 48 bits = collision probability ~0.00000001% at 10k files
- No external UUID library needed

#### 2. GetOrCreateID(path) - Returns embedded ID or computed default
```go
func (p *PatchesSubsystem) GetOrCreateID(path string) string {
    p.ensureScanned()

    // If file has embedded ID on disk, use it (stability for moved files)
    for id, scannedPath := range p.scannedIDs {
        if scannedPath == path {
            return id
        }
    }

    // No embedded ID - compute deterministic default
    return ComputeFileID(path)
}
```

#### 3. Move Detection - Compare computed vs scanned
```go
func (p *PatchesSubsystem) ensureScanned() {
    // Scan files for embedded IDs
    scannedIDs, _ := p.fileSystem.ScanForGeneratedIDs()

    // Build reverse map: ID → actualPath
    idToActualPath := make(map[string]string)
    for id, path := range scannedIDs {
        idToActualPath[id] = path
    }

    // For each tracked path, check if its computed ID is at different location
    for computedPath := range p.lockFile.TrackedFiles.Keys() {
        expectedID := ComputeFileID(computedPath)
        if actualPath, found := idToActualPath[expectedID]; found {
            if actualPath != computedPath {
                // File was moved
                p.pathRemapping[computedPath] = actualPath
            }
        }
    }
}
```

#### 4. UpdateTrackedFile - Populates FileTracker with ID and PristineGitObject
```go
// In PerformMerge, after merge completes:
for path := range p.lockFile.TrackedFiles.Keys() {
    if tracked, ok := p.lockFile.TrackedFiles.Get(path); ok {
        id := tracked.ID
        if id == "" {
            id = ComputeFileID(path)  // Default if not yet stored
        }
        p.fileTracker.UpdateTrackedFile(path, id, tracked.PristineGitObject)
    }
}
```

### Benefits

1. **No lockfile churn** - UUID v5 is deterministic, same path = same ID
2. **Simpler mental model** - ID has predictable default, only varies after moves
3. **Move detection still works** - embedded ID differs from ComputeFileID(new_path)
4. **Backwards compatible** - existing random IDs in lockfiles still work

### Edge Cases (from Gemini 3 Pro Preview review)

#### 1. Path Normalization (CRITICAL)
`UUIDv5("src/Client.ts")` ≠ `UUIDv5("src\Client.ts")` ≠ `UUIDv5("src/client.ts")`

**Risk:** Windows/macOS path differences can cause false "new file" detection.
**Fix:** Always normalize to forward slashes in `ComputeFileID` before hashing.

#### 2. Duplicate IDs (Copy-Paste Problem)
If user copies file A to B (instead of moving), both have embedded `ID_A`.

**Risk:** Scan finds `ID_A` at both paths. Map overwrites to last scanned path.
         System incorrectly detects "A moved to B" while A still exists.
**Fix:** When duplicate ID detected, prefer the path matching lockfile expectation,
         or treat as invalid and log warning.

#### 3. Move Before persistentEdits Enabled
User moves A→B before persistentEdits is enabled (no embedded ID).

**Result:** History broken - treated as "Deleted A, Created B".
**Verdict:** Acceptable trade-off. Cannot track moves without embedded markers.

### TrackedFile Structure
```yaml
trackedFiles:
  sdk.go:
    id: "computed-or-stored-uuid"           # For move detection
    last_write_checksum: "sha1:abc123"      # Dirty check optimization
    pristine_git_object: "def456"           # Base for 3-way merge
```

## Test Commands

```bash
# Run persistent edits tests
go test -v -run "TestPersistentEdits" ./integration 2>&1 | grep -E "^(--- FAIL|--- PASS)"

# Check debug log
cat /tmp/speakeasy_debug.log | tail -50
```

## Repository Structure

### speakeasy (CLI) - `/Users/thomasrooney/Code/speakeasy-api/speakeasy`
- `internal/run/workflow.go` - Orchestrates SDK generation workflow
- `internal/run/target.go` - Target configuration and execution
- `internal/sdkgen/sdkgen.go` - Calls into openapi-generation
- `internal/fs/fs.go` - FileSystem abstraction with rootDir support
- `internal/git/repository.go` - Git operations interface
- `internal/patches/pregeneration.go` - Pre-generation setup for persistent edits
- `integration/patches_test.go` - Integration tests for persistent edits

### openapi-generation (Generator) - `/Users/thomasrooney/Code/speakeasy-api/openapi-generation`
- `pkg/generate/generator.go` - Main generator, orchestrates file generation
- `pkg/generate/io.go` - File I/O, calls GetOrCreateID for file headers
- `pkg/filetracking/filetracking.go` - Tracks generated files, checksums
- `pkg/patches/merge.go` - 3-way merge implementation
- `internal/subsystem/subsystem.go` - PatchesSubsystem with ComputeFileID, GetOrCreateID

### sdk-gen-config - `/Users/thomasrooney/Code/speakeasy-api/sdk-gen-config`
- `lockfile/lockfile.go` - TrackedFile struct definition (ID, LastWriteChecksum, PristineGitObject)
- `lockfile/io.go` - Lockfile serialization

## Files Modified

- `openapi-generation/internal/subsystem/subsystem.go` - ComputeFileID, GetOrCreateID, ensureScanned
- `openapi-generation/pkg/filetracking/filetracking.go` - UpdateTrackedFile
- `openapi-generation/pkg/patches/merge.go` - Debug logging

## Gemini 3 Pro Preview Review

### Review Steps
1. Share architecture overview with repository context
2. Ask for validation of UUID v5 approach
3. Identify edge cases and potential issues
4. Get recommendations for improvements

### Review Results (Round 1)
Gemini identified these critical points:

1. **Path Normalization** - CRITICAL
   - Different path separators produce different IDs
   - Fix: `filepath.ToSlash(path)` before hashing
   - Status: ✅ Implemented

2. **Duplicate IDs (Copy-Paste Problem)**
   - User copies file A→B, both have embedded ID_A
   - Scan map overwrites, causing false move detection
   - Fix: Prefer path matching lockfile expectation, or log warning
   - Status: ⚠️ Future improvement

3. **Move Before Enabled**
   - Moving files before persistentEdits enabled loses history
   - Verdict: Acceptable trade-off
   - Status: ✅ Documented

### Review Results (Round 2 - with repo context)
Additional findings:

4. **Close vs Update Race** - CRITICAL
   - `PerformMerge` calls `UpdateTrackedFile` after merge
   - If `FileTracker.GetResult()` already called, updates are lost
   - Fix: Ensure PerformMerge runs BEFORE FileTracker finalizes gen.lock
   - Status: ⚠️ Need to verify execution order

5. **Thread Safety on newFiles** - CRITICAL
   - `TrackFile` feeds async channel → goroutine writes to `newFiles`
   - `UpdateTrackedFile` writes to `newFiles` synchronously
   - Race condition if both run concurrently
   - Fix: Add mutex or ensure channel drained before UpdateTrackedFile
   - Status: ⚠️ Need to verify

6. **Case Sensitivity**
   - `ComputeFileID("Files.go")` ≠ `ComputeFileID("files.go")`
   - Case-only renames regenerate ID if no embedded ID
   - Verdict: Acceptable - generator controls canonical casing
   - Status: ✅ Documented

7. **Relative Paths Only**
   - `ComputeFileID` must receive paths relative to generation root
   - Absolute paths would break across different machines
   - Status: ⚠️ Need to verify virtualFiles keys are relative

### Short ID Discussion
Since IDs are deterministic, we can use shorter hashes instead of UUIDs.

**Collision Math (Birthday Paradox):**
- 7 hex chars (28 bits): ~0.18% collision at 1000 files - **NOT SAFE**
- 12 hex chars (48 bits): ~0.00000001% collision at 10k files - **SAFE**

**Note:** Git's 7-char display is NOT a valid comparison - Git uses full SHA1 internally
and can ask for disambiguation. Our system cannot disambiguate at runtime.

**Recommendation from Gemini: 12 hex chars**
```go
func ComputeFileID(path string) string {
    // Normalize separators (and optionally case for cross-OS stability)
    normalized := filepath.ToSlash(path)
    // normalized = strings.ToLower(normalized)  // Optional: case normalization

    hash := sha1.Sum([]byte(normalized))
    return fmt.Sprintf("%x", hash[:6])  // 12 hex chars = 48 bits
}
```

**Benefits:**
- Readable: `// @generated-id: a1b2c3d4e5f6`
- Safe for 100k+ files
- Standard hex, easy to grep
- No custom encoding needed

**Status**: ✅ Use 12 hex chars
