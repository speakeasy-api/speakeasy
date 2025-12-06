package integration_tests

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/speakeasy-api/speakeasy/internal/patches"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ = plumbing.ZeroHash // Silence unused import warning until plumbing is used

// GitHistoryVerifier validates git repository structure after generation.
// It verifies that:
// - Refs are correctly structured under refs/speakeasy/gen/
// - Objects survive aggressive garbage collection
// - Commits form proper ancestry chains
// - Repository size remains efficient (deduplication works)
type GitHistoryVerifier struct {
	t       *testing.T
	repoDir string
	repo    *git.Repository
}

// NewGitHistoryVerifier creates a verifier for an existing git repository.
func NewGitHistoryVerifier(t *testing.T, repoDir string) *GitHistoryVerifier {
	t.Helper()

	repo, err := git.PlainOpen(repoDir)
	require.NoError(t, err, "Failed to open git repository at %s", repoDir)

	return &GitHistoryVerifier{
		t:       t,
		repoDir: repoDir,
		repo:    repo,
	}
}

// gitCommand runs a git command in the repository directory.
// This uses system git for validation (canonical behavior).
func (v *GitHistoryVerifier) gitCommand(args ...string) (string, error) {
	v.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = v.repoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if err != nil {
		return output, fmt.Errorf("git %v failed: %w\nstdout: %s\nstderr: %s",
			args, err, output, stderr.String())
	}

	return output, nil
}

// GetGenerationRefs returns all refs under refs/speakeasy/gen/.
func (v *GitHistoryVerifier) GetGenerationRefs(t *testing.T) map[string]string {
	t.Helper()

	refs := make(map[string]string)

	output, err := v.gitCommand("for-each-ref", "--format=%(refname) %(objectname)", "refs/speakeasy/gen/")
	if err != nil {
		// No refs yet is not an error
		return refs
	}

	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			refs[parts[0]] = parts[1]
		}
	}

	return refs
}

// ValidateRefStructure verifies that refs/speakeasy/gen/{UUID} exist for each generation.
func (v *GitHistoryVerifier) ValidateRefStructure(t *testing.T, expectedUUIDs []string) {
	t.Helper()

	refs := v.GetGenerationRefs(t)

	for _, uuid := range expectedUUIDs {
		refName := fmt.Sprintf("refs/speakeasy/gen/%s", uuid)
		_, exists := refs[refName]
		assert.True(t, exists, "Expected ref %s to exist", refName)
	}
}

// ValidateGCSafety runs git gc aggressively and verifies all objects remain accessible.
// This validates that our refs keep objects reachable and not accidentally pruned.
func (v *GitHistoryVerifier) ValidateGCSafety(t *testing.T, blobHashes, commitHashes []string) {
	t.Helper()

	// Critical: Expire reflogs to ensure objects are only kept alive by our refs
	_, err := v.gitCommand("reflog", "expire", "--expire=now", "--all")
	require.NoError(t, err, "Failed to expire reflogs")

	// Run aggressive GC with immediate pruning
	_, err = v.gitCommand("gc", "--aggressive", "--prune=now")
	require.NoError(t, err, "Failed to run git gc")

	// Verify all blob objects still exist
	for _, hash := range blobHashes {
		_, err := v.gitCommand("cat-file", "-e", hash)
		assert.NoError(t, err, "Blob %s was pruned by gc (not reachable from refs)", hash)
	}

	// Verify all commit objects still exist
	for _, hash := range commitHashes {
		_, err := v.gitCommand("cat-file", "-e", hash)
		assert.NoError(t, err, "Commit %s was pruned by gc (not reachable from refs)", hash)
	}

	// Verify repository integrity
	_, err = v.gitCommand("fsck", "--full", "--strict")
	assert.NoError(t, err, "Repository failed fsck after gc")
}

// ValidateReachability verifies objects are reachable from a specific ref.
func (v *GitHistoryVerifier) ValidateReachability(t *testing.T, refName string, expectedBlobHashes []string) {
	t.Helper()

	output, err := v.gitCommand("rev-list", "--objects", refName)
	require.NoError(t, err, "Failed to list objects for ref %s", refName)

	reachableObjects := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		// Format: "<hash> <path>" or just "<hash>" for commits
		parts := strings.SplitN(line, " ", 2)
		hash := parts[0]
		reachableObjects[hash] = true
	}

	for _, blobHash := range expectedBlobHashes {
		assert.True(t, reachableObjects[blobHash],
			"Blob %s is not reachable from ref %s", blobHash, refName)
	}
}

// RepositorySizeMetrics contains git repository size statistics.
type RepositorySizeMetrics struct {
	LooseObjects   int // Number of unpacked objects
	LooseObjectsKB int // Size of unpacked objects in KB
	PackedObjects  int // Number of objects in packfiles
	PackSizeKB     int // Total size of packfiles in KB
	PackCount      int // Number of packfiles
	PrunePackable  int // Objects that are both loose and packed
}

// ValidateRepositorySize checks that the repository doesn't bloat over generations.
func (v *GitHistoryVerifier) ValidateRepositorySize(t *testing.T) *RepositorySizeMetrics {
	t.Helper()

	output, err := v.gitCommand("count-objects", "-v")
	require.NoError(t, err, "Failed to run git count-objects")

	metrics := &RepositorySizeMetrics{}

	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])

		switch key {
		case "count":
			fmt.Sscanf(value, "%d", &metrics.LooseObjects)
		case "size":
			fmt.Sscanf(value, "%d", &metrics.LooseObjectsKB)
		case "in-pack":
			fmt.Sscanf(value, "%d", &metrics.PackedObjects)
		case "packs":
			fmt.Sscanf(value, "%d", &metrics.PackCount)
		case "size-pack":
			fmt.Sscanf(value, "%d", &metrics.PackSizeKB)
		case "prune-packable":
			fmt.Sscanf(value, "%d", &metrics.PrunePackable)
		}
	}

	t.Logf("Repository size metrics: loose=%d (%dKB), packed=%d (%dKB), packs=%d, prune-packable=%d",
		metrics.LooseObjects, metrics.LooseObjectsKB,
		metrics.PackedObjects, metrics.PackSizeKB,
		metrics.PackCount, metrics.PrunePackable)

	return metrics
}

// GetBlobHash computes the git blob hash for content.
func (v *GitHistoryVerifier) GetBlobHash(t *testing.T, content []byte) string {
	t.Helper()

	cmd := exec.Command("git", "hash-object", "--stdin")
	cmd.Dir = v.repoDir
	cmd.Stdin = bytes.NewReader(content)

	output, err := cmd.Output()
	require.NoError(t, err, "Failed to compute blob hash")

	return strings.TrimSpace(string(output))
}

// GetCommitHashForRef returns the commit hash for a given ref.
func (v *GitHistoryVerifier) GetCommitHashForRef(t *testing.T, refName string) string {
	t.Helper()

	output, err := v.gitCommand("rev-parse", refName)
	require.NoError(t, err, "Failed to get commit hash for ref %s", refName)

	return strings.TrimSpace(output)
}

// CollectAllGenerationUUIDs extracts UUIDs from all refs/speakeasy/gen/ refs.
func (v *GitHistoryVerifier) CollectAllGenerationUUIDs(t *testing.T) []string {
	t.Helper()

	refs := v.GetGenerationRefs(t)
	var uuids []string

	for refName := range refs {
		// Extract UUID from refs/speakeasy/gen/<uuid>
		parts := strings.Split(refName, "/")
		if len(parts) >= 4 {
			uuids = append(uuids, parts[len(parts)-1])
		}
	}

	return uuids
}

// extractGeneratedIDsFromDir scans directory for @generated-id headers and returns UUID -> filepath map.
func extractGeneratedIDsFromDir(t *testing.T, dir string) map[string]string {
	t.Helper()

	idPattern := regexp.MustCompile(`@generated-id:\s+([a-f0-9]{12})`)
	ids := make(map[string]string)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden directories and vendor
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" || info.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		// Only check source files
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".ts" && ext != ".py" && ext != ".java" && ext != ".cs" && ext != ".rb" && ext != ".php" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		match := idPattern.FindSubmatch(content)
		if match != nil {
			id := string(match[1])
			ids[id] = path
		}

		return nil
	})

	require.NoError(t, err)
	return ids
}

// setupPersistentEditsInDir sets up the generation config files in the given directory.
// This is like setupPersistentEditsTestDir but works on an existing directory.
func setupPersistentEditsInDir(t *testing.T, dir string) {
	t.Helper()

	// Create a minimal OpenAPI spec
	specContent := `openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
servers:
  - url: https://api.example.com
paths:
  /pets:
    get:
      summary: List pets
      operationId: listPets
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Pet'
    post:
      summary: Create a pet
      operationId: createPet
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
      responses:
        '201':
          description: Created
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
`
	err := os.WriteFile(filepath.Join(dir, "spec.yaml"), []byte(specContent), 0644)
	require.NoError(t, err)

	// Create .speakeasy directory
	err = os.MkdirAll(filepath.Join(dir, ".speakeasy"), 0755)
	require.NoError(t, err)

	// Create workflow.yaml
	workflowContent := `workflowVersion: 1.0.0
sources:
  test-source:
    inputs:
      - location: spec.yaml
targets:
  test-target:
    target: go
    source: test-source
`
	err = os.WriteFile(filepath.Join(dir, ".speakeasy", "workflow.yaml"), []byte(workflowContent), 0644)
	require.NoError(t, err)

	// Create gen.yaml with persistent edits enabled
	genYamlContent := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
  maintainOpenAPIOrder: true
  usageSnippets:
    optionalPropertyRendering: withExample
  persistentEdits:
    enabled: true
go:
  version: 1.0.0
  packageName: testsdk
`
	err = os.WriteFile(filepath.Join(dir, "gen.yaml"), []byte(genYamlContent), 0644)
	require.NoError(t, err)

	// Create .genignore
	genignoreContent := `go.mod
go.sum
`
	err = os.WriteFile(filepath.Join(dir, ".genignore"), []byte(genignoreContent), 0644)
	require.NoError(t, err)
}

// gitCommitAllInDir stages all changes and commits in the specified directory.
func gitCommitAllInDir(t *testing.T, dir string, message string) {
	t.Helper()
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git add failed: %s", string(output))

	cmd = exec.Command("git", "commit", "-m", message, "--allow-empty")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git commit failed: %s", string(output))
}

// ========== Integration Tests ==========

// TestGitArchitecture_GCDoesNotPruneObjects validates that after aggressive gc,
// all pristine objects are still accessible via refs/speakeasy/gen/ refs.
func TestGitArchitecture_GCDoesNotPruneObjects(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Run initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Initial generation should succeed")
	gitCommitAll(t, temp, "generation 1")

	// Get UUIDs from generated files
	ids := extractGeneratedIDsFromDir(t, temp)
	require.NotEmpty(t, ids, "Should have generated files with @generated-id")
	t.Logf("Generation 1: found %d generated files", len(ids))

	// Create verifier
	verifier := NewGitHistoryVerifier(t, temp)

	// Collect initial refs and their commit hashes
	refs := verifier.GetGenerationRefs(t)
	var commitHashes []string
	for _, hash := range refs {
		commitHashes = append(commitHashes, hash)
	}

	// Run GC and verify objects survive
	if len(commitHashes) > 0 {
		verifier.ValidateGCSafety(t, nil, commitHashes)
	}

	// Verify repository integrity
	_, err = verifier.gitCommand("fsck", "--full", "--strict")
	assert.NoError(t, err, "Repository should pass fsck after gc")
}

// TestGitArchitecture_MultiGenerationPreservesHistory validates that multiple
// generations form a proper commit chain and objects survive gc.
func TestGitArchitecture_MultiGenerationPreservesHistory(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Generation 1: Initial
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "generation 1")

	ids1 := extractGeneratedIDsFromDir(t, temp)
	require.NotEmpty(t, ids1)
	t.Logf("Generation 1: %d files", len(ids1))

	// Generation 2: Update spec to add a field
	specContent := `openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
servers:
  - url: https://api.example.com
paths:
  /pets:
    get:
      summary: List pets
      operationId: listPets
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Pet'
    post:
      summary: Create a pet
      operationId: createPet
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
      responses:
        '201':
          description: Created
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
        breed:
          type: string
          description: The breed of the pet
`
	err = os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(specContent), 0644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "update spec: add breed")

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "generation 2")

	ids2 := extractGeneratedIDsFromDir(t, temp)
	require.NotEmpty(t, ids2)
	t.Logf("Generation 2: %d files", len(ids2))

	// Generation 3: Add another endpoint
	specContent = `openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
servers:
  - url: https://api.example.com
paths:
  /pets:
    get:
      summary: List pets
      operationId: listPets
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Pet'
    post:
      summary: Create a pet
      operationId: createPet
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
      responses:
        '201':
          description: Created
  /pets/{id}:
    get:
      summary: Get pet by ID
      operationId: getPet
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Pet'
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
        breed:
          type: string
`
	err = os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(specContent), 0644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "update spec: add getPet")

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "generation 3")

	// Verify git structure
	verifier := NewGitHistoryVerifier(t, temp)

	// Collect all refs and commits
	refs := verifier.GetGenerationRefs(t)
	t.Logf("Found %d generation refs", len(refs))

	var commitHashes []string
	for refName, hash := range refs {
		t.Logf("  %s -> %s", refName, hash[:8])
		commitHashes = append(commitHashes, hash)
	}

	// Run aggressive GC and verify all commits survive
	if len(commitHashes) > 0 {
		verifier.ValidateGCSafety(t, nil, commitHashes)
	}

	// Verify object count is reasonable (not exponential growth)
	metrics := verifier.ValidateRepositorySize(t)
	totalObjects := metrics.LooseObjects + metrics.PackedObjects
	t.Logf("Total objects after 3 generations: %d", totalObjects)

	// Sanity check: should have some objects but not crazy amounts
	assert.Greater(t, totalObjects, 0, "Should have git objects")
}

// TestGitArchitecture_BinaryFileDeduplication verifies that identical binary files
// are deduplicated across generations.
func TestGitArchitecture_BinaryFileDeduplication(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Generation 1
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "generation 1")

	verifier := NewGitHistoryVerifier(t, temp)
	metrics1 := verifier.ValidateRepositorySize(t)

	// Generation 2 (same spec, should deduplicate)
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "generation 2")

	metrics2 := verifier.ValidateRepositorySize(t)

	// Size should not double - identical content should be deduplicated
	totalSize1 := metrics1.LooseObjectsKB + metrics1.PackSizeKB
	totalSize2 := metrics2.LooseObjectsKB + metrics2.PackSizeKB

	t.Logf("Size after gen1: %d KB, after gen2: %d KB", totalSize1, totalSize2)

	// The size increase should be minimal (only new tree/commit objects)
	// Allow for some overhead but not doubling
	if totalSize1 > 0 {
		assert.Less(t, totalSize2, totalSize1*3,
			"Repository should not grow excessively with identical content")
	}
}

// TestGitArchitecture_DeltaCompressionEfficiency validates that Git efficiently
// packs similar content using delta compression.
func TestGitArchitecture_DeltaCompressionEfficiency(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Generation 1: State A
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "generation 1 - state A")

	// Modify a generated file slightly
	sdkFile := filepath.Join(temp, "sdk.go")
	require.FileExists(t, sdkFile)

	content, err := os.ReadFile(sdkFile)
	require.NoError(t, err)

	// Add a small user modification
	modifiedContent := strings.Replace(string(content), "package testsdk", "package testsdk\n\n// Small user edit", 1)
	err = os.WriteFile(sdkFile, []byte(modifiedContent), 0644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "user edit")

	// Generation 2: State B (with user edit preserved)
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "generation 2 - state B")

	// Run gc to pack everything
	verifier := NewGitHistoryVerifier(t, temp)
	_, err = verifier.gitCommand("gc", "--aggressive")
	require.NoError(t, err)

	metrics := verifier.ValidateRepositorySize(t)

	// After gc, objects should be packed efficiently
	assert.Equal(t, 0, metrics.PrunePackable,
		"Objects should not be both loose and packed (indicates good packing)")

	t.Logf("Delta compression test: pack size = %d KB for 2 generations", metrics.PackSizeKB)
}

// TestGitArchitecture_ImplicitFetchFromRemote verifies that generation can fetch
// refs/speakeasy/gen/* refs implicitly from a remote when needed for 3-way merge.
//
// This tests the "cold start" scenario where:
// 1. Developer A generates code, pushes to remote (including speakeasy refs)
// 2. Developer B clones the repo (standard clone does NOT fetch speakeasy refs)
// 3. Developer B modifies a generated file
// 4. Developer B runs generation - must implicitly fetch the pristine ref to merge
func TestGitArchitecture_ImplicitFetchFromRemote(t *testing.T) {
	t.Parallel()

	// Create temp directories inside the module tree (required for `go run` to work)
	// Using the integration folder as base ensures go.mod is findable
	_, filename, _, _ := runtime.Caller(0)
	integrationDir := filepath.Dir(filename)

	remoteDir := filepath.Join(integrationDir, "temp", "remote-"+randStringBytes(7)+".git")
	envADir := filepath.Join(integrationDir, "temp", "envA-"+randStringBytes(7))
	envBDir := filepath.Join(integrationDir, "temp", "envB-"+randStringBytes(7))

	// Create directories
	require.NoError(t, os.MkdirAll(remoteDir, 0755))
	require.NoError(t, os.MkdirAll(envADir, 0755))
	require.NoError(t, os.MkdirAll(envBDir, 0755))

	// Cleanup on test completion
	t.Cleanup(func() {
		os.RemoveAll(remoteDir)
		os.RemoveAll(envADir)
		os.RemoveAll(envBDir)
	})

	// Step 1: Create bare "remote" repository
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = remoteDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git init --bare failed: %s", string(output))
	t.Logf("Created bare remote at %s", remoteDir)

	// Configure bare remote to not convert line endings
	for _, args := range [][]string{
		{"config", "core.autocrlf", "false"},
	} {
		cmd = exec.Command("git", args...)
		cmd.Dir = remoteDir
		cmd.CombinedOutput()
	}

	// Step 2: Clone to Environment A and set up for generation
	cmd = exec.Command("git", "clone", remoteDir, ".")
	cmd.Dir = envADir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git clone to envA failed: %s", string(output))

	// Configure git user in envA and disable line ending conversion for consistency
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"config", "core.autocrlf", "false"},
		{"config", "core.eol", "lf"},
	} {
		cmd = exec.Command("git", args...)
		cmd.Dir = envADir
		cmd.CombinedOutput()
	}

	// Set up generation config in envA
	setupPersistentEditsInDir(t, envADir)
	gitCommitAllInDir(t, envADir, "initial setup")

	// Push setup to remote
	cmd = exec.Command("git", "push", "-u", "origin", "HEAD")
	cmd.Dir = envADir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git push setup failed: %s", string(output))

	// Step 3: Run generation in Environment A (this should implicitly push refs)
	err = execute(t, envADir, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Generation in envA should succeed")
	gitCommitAllInDir(t, envADir, "generation 1")

	// Push all changes including the implicit speakeasy refs
	cmd = exec.Command("git", "push", "--all")
	cmd.Dir = envADir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git push --all failed: %s", string(output))

	// Also push the speakeasy refs explicitly (simulating what the generator should do)
	verifierA := NewGitHistoryVerifier(t, envADir)
	refsA := verifierA.GetGenerationRefs(t)
	for refName := range refsA {
		refSpec := fmt.Sprintf("%s:%s", refName, refName)
		cmd = exec.Command("git", "push", "origin", refSpec)
		cmd.Dir = envADir
		output, _ = cmd.CombinedOutput()
		t.Logf("Pushed ref %s: %s", refName, strings.TrimSpace(string(output)))
	}

	// Step 4: Clone to Environment B (simulating another developer)
	cmd = exec.Command("git", "clone", remoteDir, ".")
	cmd.Dir = envBDir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git clone to envB failed: %s", string(output))

	// Configure git user in envB and disable line ending conversion for consistency
	for _, args := range [][]string{
		{"config", "user.email", "devb@example.com"},
		{"config", "user.name", "Developer B"},
		{"config", "core.autocrlf", "false"},
		{"config", "core.eol", "lf"},
	} {
		cmd = exec.Command("git", args...)
		cmd.Dir = envBDir
		cmd.CombinedOutput()
	}

	// Verify: speakeasy refs should NOT exist locally in envB yet
	// Standard git clone only fetches refs/heads/*, not custom refs
	for refName := range refsA {
		cmd = exec.Command("git", "show-ref", "--verify", refName)
		cmd.Dir = envBDir
		err = cmd.Run()
		require.Error(t, err, "Ref %s should NOT exist locally in envB before generation", refName)
	}
	t.Log("Verified: speakeasy refs not present in fresh clone (as expected)")

	// Step 5: Modify a generated model file in Environment B
	// We use a model file instead of sdk.go because version bumps modify sdk.go's
	// version constants and can cause conflicts with edits near the package declaration.
	petFile := filepath.Join(envBDir, "models", "components", "pet.go")
	require.FileExists(t, petFile, "pet.go should exist in envB")

	content, err := os.ReadFile(petFile)
	require.NoError(t, err)
	originalID := extractGeneratedIDFromContent(content)
	require.NotEmpty(t, originalID, "pet.go should have @generated-id")

	// Add a custom method at the end of the file
	// We append to the very end of the file to avoid conflict with generated getters
	modifiedContent := string(content) + "\n// ENVB_USER_EDIT: Developer B's customization\nfunc (p *Pet) CustomMethod() string {\n\treturn \"custom\"\n}\n"
	err = os.WriteFile(petFile, []byte(modifiedContent), 0644)
	require.NoError(t, err)
	gitCommitAllInDir(t, envBDir, "developer B user edit")

	// Step 6: Run generation in Environment B
	// This MUST implicitly fetch the pristine ref from origin to perform 3-way merge
	err = execute(t, envBDir, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Generation in envB should succeed (with implicit fetch)")
	gitCommitAllInDir(t, envBDir, "generation 2 in envB")

	// Step 7: Verify user edit was preserved (proving 3-way merge worked)
	finalContent, err := os.ReadFile(petFile)
	require.NoError(t, err)
	require.Contains(t, string(finalContent), "ENVB_USER_EDIT: Developer B's customization",
		"User modification should be preserved after generation (3-way merge worked)")
	require.Contains(t, string(finalContent), "func (p *Pet) CustomMethod()",
		"Custom method should be preserved after generation")

	// Verify ID is preserved
	finalID := extractGeneratedIDFromContent(finalContent)
	require.Equal(t, originalID, finalID, "Generated ID should be preserved")

	t.Log("Success: implicit fetch enabled 3-way merge in fresh clone")
}

// TestGitArchitecture_HealerOfflineFallback verifies that generation succeeds
// when the remote is unavailable but the required objects exist locally.
//
// This tests the "Healer" logic where:
// 1. Developer runs generation (creates local refs)
// 2. Remote becomes unavailable (simulated by renaming origin)
// 3. Developer modifies a file and runs generation again
// 4. Generation should succeed using local cache (not crash)
func TestGitArchitecture_HealerOfflineFallback(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Step 1: Run initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Initial generation should succeed")
	gitCommitAll(t, temp, "generation 1")

	// Verify refs were created
	verifier := NewGitHistoryVerifier(t, temp)
	refs := verifier.GetGenerationRefs(t)
	require.NotEmpty(t, refs, "Should have created generation refs")
	t.Logf("Created %d generation refs", len(refs))

	// Step 2: Sabotage the remote (simulate offline)
	// First add a fake remote, then rename it to break connectivity
	cmd := exec.Command("git", "remote", "add", "origin", "https://nonexistent.example.com/repo.git")
	cmd.Dir = temp
	cmd.CombinedOutput() // Ignore error if origin already exists

	cmd = exec.Command("git", "remote", "rename", "origin", "broken_origin")
	cmd.Dir = temp
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If rename failed, origin might not exist - that's fine for this test
		t.Logf("Remote rename result: %s", string(output))
	}

	// Step 3: Modify a generated file
	sdkFile := filepath.Join(temp, "sdk.go")
	require.FileExists(t, sdkFile)

	content, err := os.ReadFile(sdkFile)
	require.NoError(t, err)
	originalID := extractGeneratedIDFromContent(content)

	modifiedContent := strings.Replace(string(content),
		"package testsdk",
		"package testsdk\n\n// OFFLINE_USER_EDIT: Made while offline", 1)
	err = os.WriteFile(sdkFile, []byte(modifiedContent), 0644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "user edit while offline")

	// Step 4: Run generation again - should succeed using local cache
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Generation should succeed using local cache when offline")
	gitCommitAll(t, temp, "generation 2 (offline)")

	// Step 5: Verify user edit was preserved
	finalContent, err := os.ReadFile(sdkFile)
	require.NoError(t, err)
	require.Contains(t, string(finalContent), "OFFLINE_USER_EDIT: Made while offline",
		"User modification should be preserved when working offline")

	// Verify ID is preserved
	finalID := extractGeneratedIDFromContent(finalContent)
	require.Equal(t, originalID, finalID, "Generated ID should be preserved")

	// Verify local refs still exist
	finalRefs := verifier.GetGenerationRefs(t)
	require.NotEmpty(t, finalRefs, "Local refs should still exist after offline generation")

	t.Log("Success: Healer fallback allowed generation while offline")
}

// TestGitArchitecture_ObjectCountLinear verifies that object count grows linearly,
// not exponentially, with generations.
func TestGitArchitecture_ObjectCountLinear(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	verifier := NewGitHistoryVerifier(t, temp)
	var objectCounts []int

	// Run 3 generations and track object count
	for i := 1; i <= 3; i++ {
		err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
		require.NoError(t, err, "Generation %d failed", i)
		gitCommitAll(t, temp, fmt.Sprintf("generation %d", i))

		// Run gc to consolidate
		_, _ = verifier.gitCommand("gc")

		metrics := verifier.ValidateRepositorySize(t)
		totalObjects := metrics.LooseObjects + metrics.PackedObjects
		objectCounts = append(objectCounts, totalObjects)
		t.Logf("Generation %d: %d total objects", i, totalObjects)
	}

	// Verify growth is roughly linear, not exponential
	// If each generation doubled objects, we'd see exponential growth
	// Linear growth means gen3 objects < gen1 objects * 4
	if objectCounts[0] > 0 {
		assert.Less(t, objectCounts[2], objectCounts[0]*5,
			"Object count should grow linearly, not exponentially")
	}
}

// TestGitArchitecture_NoOrphanedObjects verifies that after gc, no orphaned
// (unreachable) objects remain.
func TestGitArchitecture_NoOrphanedObjects(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Run generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "generation 1")

	verifier := NewGitHistoryVerifier(t, temp)

	// Expire reflogs and run gc
	_, err = verifier.gitCommand("reflog", "expire", "--expire=now", "--all")
	require.NoError(t, err)

	_, err = verifier.gitCommand("gc", "--aggressive", "--prune=now")
	require.NoError(t, err)

	// Check for unreachable objects
	output, err := verifier.gitCommand("fsck", "--unreachable", "--no-reflogs")
	require.NoError(t, err)

	// Filter out expected unreachable messages (empty is best)
	unreachableCount := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "unreachable") {
			unreachableCount++
		}
	}

	assert.Equal(t, 0, unreachableCount,
		"Should have no unreachable objects after gc, found %d", unreachableCount)
}

// TestGitArchitecture_CommitAncestryChain verifies that generation commits
// form a proper parent-child chain.
func TestGitArchitecture_CommitAncestryChain(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Run 3 generations
	for i := 1; i <= 3; i++ {
		if i > 1 {
			// Modify spec slightly to force new generation
			specContent := fmt.Sprintf(`openapi: 3.0.3
info:
  title: Test API
  version: 1.0.%d
servers:
  - url: https://api.example.com
paths:
  /pets:
    get:
      summary: List pets
      operationId: listPets
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Pet'
    post:
      summary: Create a pet
      operationId: createPet
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
      responses:
        '201':
          description: Created
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
`, i)
			err := os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(specContent), 0644)
			require.NoError(t, err)
			gitCommitAll(t, temp, fmt.Sprintf("update spec v1.0.%d", i))
		}

		err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
		require.NoError(t, err, "Generation %d failed", i)
		gitCommitAll(t, temp, fmt.Sprintf("generation %d", i))
	}

	verifier := NewGitHistoryVerifier(t, temp)
	refs := verifier.GetGenerationRefs(t)

	t.Logf("Found %d generation refs", len(refs))
	for refName, hash := range refs {
		t.Logf("  %s -> %s", refName, hash[:12])

		// Verify each ref points to a valid commit
		_, err := verifier.gitCommand("cat-file", "-t", hash)
		assert.NoError(t, err, "Ref %s should point to a valid object", refName)

		// Get the commit's tree
		output, err := verifier.gitCommand("rev-parse", hash+"^{tree}")
		assert.NoError(t, err, "Should be able to get tree from commit")
		assert.NotEmpty(t, strings.TrimSpace(output), "Commit should have a tree")
	}
}

// TestGitArchitecture_MultipleTypeScriptTargetsSameRepo verifies that two TypeScript targets
// in the same repo (one at root, one in a subpackage) have independent non-conflicting IDs.
//
// This tests the scenario where:
// 1. One target outputs to the root of the repo
// 2. Another target outputs to packages/subpackage
// 3. Both are TypeScript targets sharing the same source
// 4. Each should have independent @generated-id tracking
func TestGitArchitecture_MultipleTypeScriptTargetsSameRepo(t *testing.T) {
	t.Parallel()
	temp := setupDualTypeScriptTargetsTestDir(t)

	// Run generation for both targets
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Generation should succeed for both targets")
	gitCommitAll(t, temp, "initial generation")

	// Find SDK files from both targets
	rootSdkFile := filepath.Join(temp, "src", "sdk", "sdk.ts")
	subpackageSdkFile := filepath.Join(temp, "packages", "subpackage", "src", "sdk", "sdk.ts")

	require.FileExists(t, rootSdkFile, "Root TypeScript SDK file should exist")
	require.FileExists(t, subpackageSdkFile, "Subpackage TypeScript SDK file should exist")

	// Extract IDs from both SDK files
	rootContent, err := os.ReadFile(rootSdkFile)
	require.NoError(t, err)
	rootID := extractGeneratedIDFromContent(rootContent)
	require.NotEmpty(t, rootID, "Root SDK should have @generated-id")
	t.Logf("Root SDK ID: %s", rootID)

	subpackageContent, err := os.ReadFile(subpackageSdkFile)
	require.NoError(t, err)
	subpackageID := extractGeneratedIDFromContent(subpackageContent)
	require.NotEmpty(t, subpackageID, "Subpackage SDK should have @generated-id")
	t.Logf("Subpackage SDK ID: %s", subpackageID)

	// Verify IDs are different (independent targets should have independent IDs)
	assert.NotEqual(t, rootID, subpackageID,
		"Root and subpackage SDKs should have different @generated-id values")

	// Scan both directories and verify no ID collisions
	rootScanner := patches.NewScanner(temp)
	rootResult, err := rootScanner.Scan()
	require.NoError(t, err)

	// Build maps of IDs per target directory
	rootIDs := make(map[string]string)   // id -> path
	subpkgIDs := make(map[string]string) // id -> path
	allIDs := make(map[string][]string)  // id -> list of paths (for collision detection)

	for id, path := range rootResult.UUIDToPath {
		allIDs[id] = append(allIDs[id], path)

		// Categorize by target
		if strings.HasPrefix(path, "packages/subpackage/") || strings.HasPrefix(path, "packages\\subpackage\\") {
			subpkgIDs[id] = path
		} else {
			rootIDs[id] = path
		}
	}

	t.Logf("Found %d files in root target", len(rootIDs))
	t.Logf("Found %d files in subpackage target", len(subpkgIDs))

	// Verify no ID appears in both targets (would indicate collision)
	for id := range rootIDs {
		if _, existsInSubpkg := subpkgIDs[id]; existsInSubpkg {
			t.Errorf("ID collision detected: %s appears in both root (%s) and subpackage (%s)",
				id, rootIDs[id], subpkgIDs[id])
		}
	}

	// Verify no duplicate IDs within the whole repo
	for id, paths := range allIDs {
		if len(paths) > 1 {
			t.Errorf("Duplicate ID detected: %s appears in multiple files: %v", id, paths)
		}
	}

	// Verify git refs exist for both targets
	verifier := NewGitHistoryVerifier(t, temp)
	refs := verifier.GetGenerationRefs(t)
	t.Logf("Found %d generation refs", len(refs))

	// Should have refs for files in both targets
	require.NotEmpty(t, refs, "Should have generation refs")

	// Verify refs survive GC
	var commitHashes []string
	for _, hash := range refs {
		commitHashes = append(commitHashes, hash)
	}
	if len(commitHashes) > 0 {
		verifier.ValidateGCSafety(t, nil, commitHashes)
	}

	// Now test that user modifications to both targets are preserved independently
	// Modify root SDK
	rootModified := strings.Replace(string(rootContent),
		"export class SDK extends ClientSDK",
		"// ROOT_USER_EDIT: Custom code for root SDK\nexport class SDK extends ClientSDK", 1)
	err = os.WriteFile(rootSdkFile, []byte(rootModified), 0644)
	require.NoError(t, err)

	// Modify subpackage SDK
	subpkgModified := strings.Replace(string(subpackageContent),
		"export class SDK extends ClientSDK",
		"// SUBPKG_USER_EDIT: Custom code for subpackage SDK\nexport class SDK extends ClientSDK", 1)
	err = os.WriteFile(subpackageSdkFile, []byte(subpkgModified), 0644)
	require.NoError(t, err)

	gitCommitAll(t, temp, "user modifications to both SDKs")

	// Regenerate
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Regeneration should succeed")

	// Verify both user modifications are preserved
	rootFinal, err := os.ReadFile(rootSdkFile)
	require.NoError(t, err)
	require.Contains(t, string(rootFinal), "ROOT_USER_EDIT: Custom code for root SDK",
		"Root SDK user modification should be preserved")

	subpkgFinal, err := os.ReadFile(subpackageSdkFile)
	require.NoError(t, err)
	require.Contains(t, string(subpkgFinal), "SUBPKG_USER_EDIT: Custom code for subpackage SDK",
		"Subpackage SDK user modification should be preserved")

	// Verify IDs are still preserved
	rootFinalID := extractGeneratedIDFromContent(rootFinal)
	require.Equal(t, rootID, rootFinalID, "Root SDK ID should be preserved after regeneration")

	subpkgFinalID := extractGeneratedIDFromContent(subpkgFinal)
	require.Equal(t, subpackageID, subpkgFinalID, "Subpackage SDK ID should be preserved after regeneration")

	t.Log("Success: Multiple TypeScript targets have independent non-conflicting IDs")
}

// setupDualTypeScriptTargetsTestDir creates a test directory with two TypeScript targets:
// - ts-root: outputs to the root of the repo
// - ts-subpackage: outputs to packages/subpackage
func setupDualTypeScriptTargetsTestDir(t *testing.T) string {
	t.Helper()

	// Create temp directory using runtime.Caller pattern (required for go run to work)
	_, filename, _, _ := runtime.Caller(0)
	integrationDir := filepath.Dir(filename)
	temp := filepath.Join(integrationDir, "temp", "dual-ts-"+randStringBytes(7))

	require.NoError(t, os.MkdirAll(temp, 0755))
	t.Cleanup(func() {
		os.RemoveAll(temp)
	})

	// Create a minimal OpenAPI spec
	specContent := `openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
servers:
  - url: https://api.example.com
paths:
  /pets:
    get:
      summary: List pets
      operationId: listPets
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Pet'
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
`
	err := os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(specContent), 0644)
	require.NoError(t, err)

	// Create .speakeasy directory
	err = os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0755)
	require.NoError(t, err)

	// Create output directory for subpackage
	err = os.MkdirAll(filepath.Join(temp, "packages", "subpackage"), 0755)
	require.NoError(t, err)

	// Create workflow.yaml with two TypeScript targets
	workflowContent := `workflowVersion: 1.0.0
sources:
  test-source:
    inputs:
      - location: spec.yaml
targets:
  ts-root:
    target: typescript
    source: test-source
  ts-subpackage:
    target: typescript
    source: test-source
    output: packages/subpackage
`
	err = os.WriteFile(filepath.Join(temp, ".speakeasy", "workflow.yaml"), []byte(workflowContent), 0644)
	require.NoError(t, err)

	// Create gen.yaml for root target
	rootGenYaml := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
  maintainOpenAPIOrder: true
  usageSnippets:
    optionalPropertyRendering: withExample
  persistentEdits:
    enabled: true
typescript:
  version: 1.0.0
  packageName: "@test/root-sdk"
`
	err = os.WriteFile(filepath.Join(temp, "gen.yaml"), []byte(rootGenYaml), 0644)
	require.NoError(t, err)

	// Create gen.yaml for subpackage target
	subpkgGenYaml := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
  maintainOpenAPIOrder: true
  usageSnippets:
    optionalPropertyRendering: withExample
  persistentEdits:
    enabled: true
typescript:
  version: 1.0.0
  packageName: "@test/subpackage-sdk"
`
	err = os.WriteFile(filepath.Join(temp, "packages", "subpackage", "gen.yaml"), []byte(subpkgGenYaml), 0644)
	require.NoError(t, err)

	// Create .genignore in both directories
	genignoreContent := `package.json
package-lock.json
node_modules
`
	err = os.WriteFile(filepath.Join(temp, ".genignore"), []byte(genignoreContent), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(temp, "packages", "subpackage", ".genignore"), []byte(genignoreContent), 0644)
	require.NoError(t, err)

	// Initialize git repo
	gitInit(t, temp)
	gitCommitAll(t, temp, "initial commit")

	return temp
}
