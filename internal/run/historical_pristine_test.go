package run

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/lockfile"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	internalpatches "github.com/speakeasy-api/speakeasy/internal/patches"
	"github.com/stretchr/testify/require"
)

func TestRecoverHistoricalPristineForTarget_SeedsMissingPristineBlobs(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	initHistoricalPristineTestRepo(t, tempDir)

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "gen.yaml"), []byte(`configVersion: 2.0.0
generation:
  persistentEdits:
    enabled: true
go:
  version: 1.0.0
  packageName: testsdk
`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".speakeasy"), 0o755))

	lf := sdkGenConfig.NewLockFile()
	lf.TrackedFiles.Set("sdk.go", lockfile.TrackedFile{
		ID:                "sdk-generated-id",
		LastWriteChecksum: "sha1:test",
	})
	require.NoError(t, sdkGenConfig.SaveLockFile(tempDir, lf))

	originalResolve := resolveHistoricalPristineSource
	originalGenerate := generateHistoricalPristine
	t.Cleanup(func() {
		resolveHistoricalPristineSource = originalResolve
		generateHistoricalPristine = originalGenerate
	})

	resolveHistoricalPristineSource = func(_ context.Context, _ *Workflow, _ string) (string, error) {
		return filepath.Join(tempDir, "historical-spec.yaml"), nil
	}
	generateHistoricalPristine = func(_ context.Context, _ *Workflow, _ string, _ string, _ string, outDir string) error {
		return os.WriteFile(filepath.Join(outDir, "sdk.go"), []byte("package testsdk\n"), 0o644)
	}

	w := &Workflow{
		ProjectDir: tempDir,
		workflow: workflow.Workflow{
			Targets: map[string]workflow.Target{
				"test-target": {
					Target: "go",
					Source: "test-source",
				},
			},
		},
		RepoSubDirs: map[string]string{},
	}

	require.NoError(t, w.recoverHistoricalPristineForTarget(context.Background(), "test-target"))

	cfg, err := sdkGenConfig.Load(tempDir)
	require.NoError(t, err)
	tracked, ok := cfg.LockFile.TrackedFiles.Get("sdk.go")
	require.True(t, ok)
	require.NotEmpty(t, tracked.PristineGitObject)

	gitRepo, err := internalpatches.OpenGitRepository(tempDir)
	require.NoError(t, err)
	content, err := gitRepo.GetBlob(tracked.PristineGitObject)
	require.NoError(t, err)
	require.Equal(t, "package testsdk\n", string(content))
}

func TestRecoverHistoricalPristineForTarget_UsesGitHistoryBeforeRegeneration(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	initHistoricalPristineTestRepo(t, tempDir)

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "gen.yaml"), []byte(`configVersion: 2.0.0
generation:
  persistentEdits:
    enabled: true
go:
  version: 1.0.0
  packageName: testsdk
`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".speakeasy"), 0o755))

	historicalContent := []byte("package testsdk\n\n// @generated-id: sdk-generated-id\n")
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "sdk.go"), historicalContent, 0o644))
	commitAllHistoricalPristineTestRepo(t, tempDir, "seed historical pristine")

	lf := sdkGenConfig.NewLockFile()
	lf.TrackedFiles.Set("sdk.go", lockfile.TrackedFile{
		ID:                "sdk-generated-id",
		LastWriteChecksum: "sha1:test",
	})
	require.NoError(t, sdkGenConfig.SaveLockFile(tempDir, lf))

	originalResolve := resolveHistoricalPristineSource
	originalGenerate := generateHistoricalPristine
	t.Cleanup(func() {
		resolveHistoricalPristineSource = originalResolve
		generateHistoricalPristine = originalGenerate
	})

	resolveHistoricalPristineSource = func(_ context.Context, _ *Workflow, _ string) (string, error) {
		t.Fatalf("historical source recovery should not run when git history already has the pristine")
		return "", nil
	}
	generateHistoricalPristine = func(_ context.Context, _ *Workflow, _ string, _ string, _ string, _ string) error {
		t.Fatalf("historical regeneration should not run when git history already has the pristine")
		return nil
	}

	w := &Workflow{
		ProjectDir: tempDir,
		workflow: workflow.Workflow{
			Targets: map[string]workflow.Target{
				"test-target": {
					Target: "go",
					Source: "test-source",
				},
			},
		},
		RepoSubDirs: map[string]string{},
	}

	require.NoError(t, w.recoverHistoricalPristineForTarget(context.Background(), "test-target"))

	cfg, err := sdkGenConfig.Load(tempDir)
	require.NoError(t, err)
	tracked, ok := cfg.LockFile.TrackedFiles.Get("sdk.go")
	require.True(t, ok)
	require.NotEmpty(t, tracked.PristineGitObject)

	gitRepo, err := internalpatches.OpenGitRepository(tempDir)
	require.NoError(t, err)
	content, err := gitRepo.GetBlob(tracked.PristineGitObject)
	require.NoError(t, err)
	require.Equal(t, string(historicalContent), string(content))
}

func TestDefaultHistoricalPristineVersion_PrefersOldWorkflowLockVersion(t *testing.T) {
	t.Parallel()

	w := &Workflow{
		lockfileOld: &workflow.LockFile{
			SpeakeasyVersion: "0.2.3",
		},
		workflow: workflow.Workflow{
			SpeakeasyVersion: "1.2.3",
		},
	}

	version, err := defaultHistoricalPristineVersion(context.Background(), w)
	require.NoError(t, err)
	require.Equal(t, "v0.2.3", version)
}

func TestDefaultGenerateHistoricalPristine_UsesResolvedVersionRunner(t *testing.T) {
	t.Parallel()

	originalResolveVersion := resolveHistoricalPristineVersion
	originalRunWithVersion := runHistoricalPristineWithVersion
	t.Cleanup(func() {
		resolveHistoricalPristineVersion = originalResolveVersion
		runHistoricalPristineWithVersion = originalRunWithVersion
	})

	resolveHistoricalPristineVersion = func(_ context.Context, _ *Workflow) (string, error) {
		return "v9.9.9", nil
	}

	var gotTarget, gotLanguage, gotSchemaPath, gotOutDir, gotVersion string
	runHistoricalPristineWithVersion = func(_ context.Context, _ *Workflow, target, language, schemaPath, outDir, desiredVersion string) error {
		gotTarget = target
		gotLanguage = language
		gotSchemaPath = schemaPath
		gotOutDir = outDir
		gotVersion = desiredVersion
		return nil
	}

	err := defaultGenerateHistoricalPristine(context.Background(), &Workflow{}, "sdk", "go", "/tmp/schema.yaml", "/tmp/out")
	require.NoError(t, err)
	require.Equal(t, "sdk", gotTarget)
	require.Equal(t, "go", gotLanguage)
	require.Equal(t, "/tmp/schema.yaml", gotSchemaPath)
	require.Equal(t, "/tmp/out", gotOutDir)
	require.Equal(t, "v9.9.9", gotVersion)
}

func initHistoricalPristineTestRepo(t *testing.T, dir string) {
	t.Helper()

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git init failed: %s", string(output))

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git config user.email failed: %s", string(output))

	cmd = exec.Command("git", "config", "user.name", "test")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git config user.name failed: %s", string(output))
}

func commitAllHistoricalPristineTestRepo(t *testing.T, dir, message string) {
	t.Helper()

	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git add failed: %s", string(output))

	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git commit failed: %s", string(output))
}
