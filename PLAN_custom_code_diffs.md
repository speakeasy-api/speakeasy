# Plan: Modified Files with Diffs in FileChangeSummary

## Summary

Enable `GetFileChangeSummary()` to include modified files with pretty diffs, and ensure pristine git objects are always stored locally.

## Current State Analysis

### What's Already Implemented
1. `TrackedFile.PristineGitObject` field exists in sdk-gen-config
2. `merge.go:80` calls `ctx.Git.WriteObject(vFile.Content)` - blobs ARE written
3. `updateTracking()` always sets `tracked.PristineGitObject = newBlobHash`
4. `GitAdapter.ReadBlob()` in speakeasy can retrieve pristine content
5. `FileChangeSummary.Modified []string` field exists but is never populated

### The Gap
**`subsystem.go:115-117`**: The Patches subsystem is only initialized when `persistentEdits.IsEnabled()`:
```go
if opts.Config != nil &&
    opts.Config.Generation.PersistentEdits.IsEnabled() {
    s.Patches = &PatchesSubsystem{...}
}
```

This means pristine objects are NOT stored when persistentEdits is disabled.

### What Needs to Change

| Repo | Change |
|------|--------|
| openapi-generation | Always initialize patches subsystem (or blob-writing portion), just skip push when disabled |
| speakeasy | Return modified paths from `DetectFileChanges()` |
| speakeasy | Add `go-difflib` dependency for unified diff formatting |
| speakeasy | Implement `ComputeFileDiff()` function |
| speakeasy | Update `GetFileChangeSummary()` to include modified files with diffs |

---

## Implementation

### Phase 1: Always Store Pristine Objects (openapi-generation)

**File: `internal/subsystem/subsystem.go`**

Option A: Always initialize patches subsystem, add flag for push behavior
```go
// Initialize patches subsystem ALWAYS for local blob storage
// Push behavior controlled by persistentEdits.IsEnabled()
if opts.Config != nil {
    s.Patches = &PatchesSubsystem{
        config:      opts.Config,
        lockFile:    opts.LockFile,
        git:         opts.Git,
        fileSystem:  opts.FileSystem,
        pushEnabled: opts.Config.Generation.PersistentEdits.IsEnabled(), // NEW
    }
}
```

Option B: Separate blob storage from full merge (simpler)
- Move blob writing to always happen in generator
- Keep full merge/commit/push only when enabled

**Recommendation**: Option A is cleaner - single code path, push controlled by flag.

### Phase 2: Track Modified Files (speakeasy)

**File: `internal/patches/pregeneration.go`**

Change `DetectFileChanges` signature:
```go
// Before
func DetectFileChanges(outDir string, lockFile *config.LockFile) (bool, error)

// After
func DetectFileChanges(outDir string, lockFile *config.LockFile) (isDirty bool, modifiedPaths []string, err error)
```

Update the checksum comparison block:
```go
// Check for content modification via checksum
if tracked.LastWriteChecksum != "" && fileExists {
    currentChecksum, err := lockfile.ComputeFileChecksum(os.DirFS(outDir), path)
    if err == nil && currentChecksum != tracked.LastWriteChecksum {
        isDirty = true
        modifiedPaths = append(modifiedPaths, path)  // NEW
    }
}
```

### Phase 3: Add Diff Computation (speakeasy)

**Add dependency:**
```bash
go get github.com/pmezard/go-difflib
```

**File: `internal/patches/diff.go` (new file)**

```go
package patches

import (
    "os"
    "path/filepath"

    "github.com/pmezard/go-difflib/difflib"
)

// FileDiff represents a modified file with its diff
type FileDiff struct {
    Path         string
    PristineHash string
    DiffText     string
    Stats        DiffStats
}

type DiffStats struct {
    Added   int
    Removed int
}

// ComputeFileDiff generates a unified diff between pristine and current content
func ComputeFileDiff(outDir, path, pristineHash string, gitRepo GitRepository) (FileDiff, error) {
    fd := FileDiff{Path: path, PristineHash: pristineHash}

    // Handle missing pristine (first generation or legacy lockfile)
    if pristineHash == "" {
        fd.DiffText = "(no pristine base available)"
        return fd, nil
    }

    // Get pristine content from git
    pristine, err := gitRepo.GetBlob(pristineHash)
    if err != nil {
        fd.DiffText = "(pristine object not found in git)"
        return fd, nil
    }

    // Get current content from disk
    current, err := os.ReadFile(filepath.Join(outDir, path))
    if err != nil {
        fd.DiffText = "(file not found on disk)"
        return fd, nil
    }

    // Skip binary files
    if isBinary(pristine) || isBinary(current) {
        fd.DiffText = "(binary file)"
        return fd, nil
    }

    // Normalize line endings
    pristineStr := normalizeLineEndings(string(pristine))
    currentStr := normalizeLineEndings(string(current))

    // Compute unified diff
    diff := difflib.UnifiedDiff{
        A:        difflib.SplitLines(pristineStr),
        B:        difflib.SplitLines(currentStr),
        FromFile: "generated",
        ToFile:   "current",
        Context:  3,
    }

    diffText, err := difflib.GetUnifiedDiffString(diff)
    if err != nil {
        fd.DiffText = "(diff computation failed)"
        return fd, nil
    }

    fd.DiffText = diffText
    fd.Stats = countDiffStats(diffText)
    return fd, nil
}

func isBinary(content []byte) bool {
    // Check first 512 bytes for null byte
    checkLen := 512
    if len(content) < checkLen {
        checkLen = len(content)
    }
    for i := 0; i < checkLen; i++ {
        if content[i] == 0 {
            return true
        }
    }
    return false
}

func normalizeLineEndings(s string) string {
    s = strings.ReplaceAll(s, "\r\n", "\n")
    s = strings.ReplaceAll(s, "\r", "\n")
    return s
}

func countDiffStats(diffText string) DiffStats {
    var stats DiffStats
    for _, line := range strings.Split(diffText, "\n") {
        if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
            stats.Added++
        } else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
            stats.Removed++
        }
    }
    return stats
}
```

### Phase 4: Update FileChangeSummary (speakeasy)

**File: `internal/patches/pregeneration.go`**

Update struct:
```go
type FileChangeSummary struct {
    Deleted  []string
    Moved    map[string]string
    Modified []FileDiff  // Changed from []string
}
```

Update `GetFileChangeSummary`:
```go
// GetFileChangeSummaryWithDiffs extracts summary and computes diffs for modified files
func GetFileChangeSummaryWithDiffs(
    outDir string,
    lockFile *config.LockFile,
    modifiedPaths []string,
    gitRepo GitRepository,
) FileChangeSummary {
    summary := FileChangeSummary{
        Moved:    make(map[string]string),
        Modified: make([]FileDiff, 0, len(modifiedPaths)),
    }

    if lockFile == nil || lockFile.TrackedFiles == nil {
        return summary
    }

    // Handle deleted and moved (existing logic)
    for path := range lockFile.TrackedFiles.Keys() {
        tracked, ok := lockFile.TrackedFiles.Get(path)
        if !ok {
            continue
        }
        if tracked.Deleted {
            summary.Deleted = append(summary.Deleted, path)
        } else if tracked.MovedTo != "" {
            summary.Moved[path] = tracked.MovedTo
        }
    }

    // Handle modified (NEW)
    for _, path := range modifiedPaths {
        tracked, ok := lockFile.TrackedFiles.Get(path)
        if !ok {
            continue
        }
        fd, _ := ComputeFileDiff(outDir, path, tracked.PristineGitObject, gitRepo)
        summary.Modified = append(summary.Modified, fd)
    }

    return summary
}
```

### Phase 5: Update FormatSummary (speakeasy)

```go
func (s FileChangeSummary) FormatSummary(maxLines int, showDiffs bool) string {
    var lines []string

    for _, path := range s.Deleted {
        lines = append(lines, fmt.Sprintf("  D %s", path))
    }
    for from, to := range s.Moved {
        lines = append(lines, fmt.Sprintf("  R %s -> %s", from, to))
    }
    for _, fd := range s.Modified {
        if showDiffs && fd.Stats.Added+fd.Stats.Removed > 0 {
            lines = append(lines, fmt.Sprintf("  M %s (+%d/-%d)", fd.Path, fd.Stats.Added, fd.Stats.Removed))
            // Add truncated diff (max 10 lines per file)
            diffLines := strings.Split(strings.TrimSpace(fd.DiffText), "\n")
            maxDiffLines := 10
            for i, dl := range diffLines {
                if i >= maxDiffLines {
                    lines = append(lines, fmt.Sprintf("      ... (%d more lines)", len(diffLines)-maxDiffLines))
                    break
                }
                lines = append(lines, "      "+dl)
            }
        } else {
            lines = append(lines, fmt.Sprintf("  M %s", fd.Path))
        }
    }

    // Truncate total
    total := len(lines)
    if total > maxLines && maxLines > 0 {
        lines = lines[:maxLines]
        lines = append(lines, fmt.Sprintf("  ... and %d more", total-maxLines))
    }

    return strings.Join(lines, "\n")
}
```

### Phase 6: Update Callers

**File: `internal/patches/pregeneration.go`**

Update `PrepareForGeneration`:
```go
func PrepareForGeneration(outDir string, autoYes bool, promptFunc PromptFunc, warnFunc func(format string, args ...any)) error {
    // ... load config ...

    isDirty, modifiedPaths, err := DetectFileChanges(outDir, cfg.LockFile)
    if err != nil {
        warnFunc("Failed to detect file changes: %v", err)
    } else if isDirty && !autoYes {
        // Initialize git repo for reading blobs
        gitRepo, _ := git.NewRepository(outDir)
        var adapter GitRepository
        if gitRepo != nil && !gitRepo.IsNil() {
            adapter = WrapGitRepository(gitRepo)
        }

        summary := GetFileChangeSummaryWithDiffs(outDir, cfg.LockFile, modifiedPaths, adapter)
        summaryText := summary.FormatSummary(20, adapter != nil /* showDiffs only if git available */)

        choice, err := promptFunc(summaryText)
        // ... handle choice ...
    }
}
```

---

## Testing

1. **Unit tests** for `ComputeFileDiff`, `isBinary`, `normalizeLineEndings`
2. **Update existing tests** for new `DetectFileChanges` signature
3. **Integration test**: Generate, modify file, run again, verify diff shown

---

## Example Output

```
Changes detected in generated files:
  D src/deprecated_model.go
  R src/old_client.go -> src/api/client.go
  M src/sdk.go (+5/-2)
      @@ -15,7 +15,10 @@
       func Init() {
      -    config.Default()
      +    config.Default()
      +    // Custom initialization
      +    setupLogging()
       }
  M src/types.go (+12/-0)
      ... (8 more lines)
  ... and 3 more
```

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Git not available | Graceful degradation - show "modified" without diff |
| No pristine hash (first run) | Skip diff, show "(no pristine base)" |
| Large diffs | Truncate to 10 lines per file |
| Binary files | Detect and skip |
| CRLF/LF mismatch | Normalize before diff |
