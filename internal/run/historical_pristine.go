package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	cfgworkflow "github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/events"
	internalpatches "github.com/speakeasy-api/speakeasy/internal/patches"
	"github.com/speakeasy-api/speakeasy/internal/updates"
)

var (
	resolveHistoricalPristineSource  = defaultHistoricalPristineSource
	generateHistoricalPristine       = defaultGenerateHistoricalPristine
	resolveHistoricalPristineVersion = defaultHistoricalPristineVersion
	runHistoricalPristineWithVersion = defaultRunHistoricalPristineWithVersion
)

func (w *Workflow) recoverHistoricalPristineForTarget(ctx context.Context, target string) error {
	t, ok := w.workflow.Targets[target]
	if !ok {
		return nil
	}

	outDir := w.ProjectDir
	if t.Output != nil && *t.Output != "" {
		outDir = *t.Output
	}

	genConfig, err := sdkGenConfig.Load(outDir)
	if err != nil || genConfig.LockFile == nil || genConfig.LockFile.TrackedFiles == nil {
		return nil
	}

	gitRepo, err := internalpatches.OpenGitRepository(outDir)
	if err != nil || gitRepo == nil || gitRepo.IsNil() {
		return nil
	}

	missingPaths := make([]string, 0)
	for path := range genConfig.LockFile.TrackedFiles.Keys() {
		tracked, ok := genConfig.LockFile.TrackedFiles.Get(path)
		if !ok {
			continue
		}
		pristineHash := strings.TrimSpace(tracked.PristineGitObject)
		if pristineHash == "" || !gitRepo.HasObject(pristineHash) {
			missingPaths = append(missingPaths, path)
		}
	}
	if len(missingPaths) == 0 {
		return nil
	}

	if recovered, err := recoverHistoricalPristineFromGitHistory(outDir, gitRepo, genConfig.LockFile, missingPaths); err != nil {
		return err
	} else if recovered {
		missingPaths = missingHistoricalPristinePaths(genConfig.LockFile, gitRepo)
		if len(missingPaths) == 0 {
			return nil
		}
	}

	historicalSourcePath, err := resolveHistoricalPristineSource(ctx, w, target)
	if err != nil {
		return err
	}

	tempRoot, err := os.MkdirTemp("", "speakeasy-historical-pristine-*")
	if err != nil {
		return fmt.Errorf("creating temporary directory for historical pristine recovery: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	tempOutDir := filepath.Join(tempRoot, "out")
	if err := os.MkdirAll(tempOutDir, 0o755); err != nil {
		return fmt.Errorf("creating temporary output directory for historical pristine recovery: %w", err)
	}

	for _, name := range []string{"gen.yaml", ".genignore"} {
		src := filepath.Join(outDir, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("reading %s for historical pristine recovery: %w", src, err)
		}
		if err := os.WriteFile(filepath.Join(tempOutDir, name), data, 0o644); err != nil {
			return fmt.Errorf("writing %s for historical pristine recovery: %w", name, err)
		}
	}

	if err := generateHistoricalPristine(ctx, w, target, t.Target, historicalSourcePath, tempOutDir); err != nil {
		return err
	}

	updated := false
	for _, path := range missingPaths {
		content, err := os.ReadFile(filepath.Join(tempOutDir, filepath.FromSlash(path)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("reading recovered historical pristine for %s: %w", path, err)
		}

		blobHash, err := gitRepo.WriteBlob(content)
		if err != nil {
			return fmt.Errorf("writing historical pristine blob for %s: %w", path, err)
		}

		tracked, ok := genConfig.LockFile.TrackedFiles.Get(path)
		if !ok {
			continue
		}
		tracked.PristineGitObject = blobHash
		genConfig.LockFile.TrackedFiles.Set(path, tracked)
		updated = true
	}

	if !updated {
		return nil
	}

	if err := sdkGenConfig.SaveLockFile(outDir, genConfig.LockFile); err != nil {
		return fmt.Errorf("saving recovered pristine git objects: %w", err)
	}

	return nil
}

func missingHistoricalPristinePaths(lockFile *sdkGenConfig.LockFile, gitRepo *internalpatches.GitRepositoryWrapper) []string {
	if lockFile == nil || lockFile.TrackedFiles == nil || gitRepo == nil {
		return nil
	}

	missingPaths := make([]string, 0)
	for path := range lockFile.TrackedFiles.Keys() {
		tracked, ok := lockFile.TrackedFiles.Get(path)
		if !ok {
			continue
		}
		pristineHash := strings.TrimSpace(tracked.PristineGitObject)
		if pristineHash == "" || !gitRepo.HasObject(pristineHash) {
			missingPaths = append(missingPaths, path)
		}
	}

	sort.Strings(missingPaths)
	return missingPaths
}

func recoverHistoricalPristineFromGitHistory(outDir string, gitRepo *internalpatches.GitRepositoryWrapper, lockFile *sdkGenConfig.LockFile, missingPaths []string) (bool, error) {
	if gitRepo == nil || gitRepo.IsNil() || lockFile == nil || lockFile.TrackedFiles == nil {
		return false, nil
	}

	recoveredAny := false
	for _, path := range missingPaths {
		tracked, ok := lockFile.TrackedFiles.Get(path)
		if !ok {
			continue
		}

		content, err := findHistoricalPristineInGitHistory(gitRepo.Root(), path, tracked.ID)
		if err != nil {
			return recoveredAny, err
		}
		if len(content) == 0 {
			continue
		}

		blobHash, err := gitRepo.WriteBlob(content)
		if err != nil {
			return recoveredAny, fmt.Errorf("writing historical pristine blob from git history for %s: %w", path, err)
		}

		tracked.PristineGitObject = blobHash
		lockFile.TrackedFiles.Set(path, tracked)
		recoveredAny = true
	}

	if !recoveredAny {
		return false, nil
	}

	if err := sdkGenConfig.SaveLockFile(outDir, lockFile); err != nil {
		return true, fmt.Errorf("saving pristine git objects recovered from history: %w", err)
	}

	return true, nil
}

func findHistoricalPristineInGitHistory(repoRoot, path, expectedID string) ([]byte, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return nil, nil
	}

	logOutput, err := runHistoricalGitCommand(repoRoot, "log", "--format=%H", "--all", "--", path)
	if err != nil {
		return nil, nil
	}

	for _, commitHash := range strings.Split(strings.TrimSpace(logOutput), "\n") {
		commitHash = strings.TrimSpace(commitHash)
		if commitHash == "" {
			continue
		}

		content, err := runHistoricalGitCommand(repoRoot, "show", fmt.Sprintf("%s:%s", commitHash, filepath.ToSlash(path)))
		if err != nil {
			continue
		}

		contentBytes := []byte(content)
		if expectedID != "" {
			generatedID, ok := extractGeneratedID(contentBytes)
			if !ok || generatedID != expectedID {
				continue
			}
		}

		return contentBytes, nil
	}

	return nil, nil
}

func runHistoricalGitCommand(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	return string(output), nil
}

func extractGeneratedID(content []byte) (string, bool) {
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "@generated-id:") {
			continue
		}
		parts := strings.SplitN(line, "@generated-id:", 2)
		if len(parts) != 2 {
			continue
		}
		id := strings.TrimSpace(parts[1])
		if id != "" {
			return id, true
		}
	}
	return "", false
}

func defaultHistoricalPristineSource(ctx context.Context, w *Workflow, target string) (string, error) {
	t, ok := w.workflow.Targets[target]
	if !ok {
		return "", fmt.Errorf("workflow target %q not found", target)
	}

	step := w.RootStep.NewSubstep("Recover Historical Pristine Source")
	sourcePath, err := NewFrozenSource(w, step, t.Source).Do(ctx, "unused")
	if err != nil {
		step.Skip(err.Error())
		return "", err
	}
	step.Succeed()
	return sourcePath, nil
}

func defaultGenerateHistoricalPristine(ctx context.Context, w *Workflow, target, language, schemaPath, outDir string) error {
	desiredVersion, err := resolveHistoricalPristineVersion(ctx, w)
	if err != nil {
		return err
	}
	return runHistoricalPristineWithVersion(ctx, w, target, language, schemaPath, outDir, desiredVersion)
}

func defaultHistoricalPristineVersion(ctx context.Context, w *Workflow) (string, error) {
	if w != nil && w.lockfileOld != nil && strings.TrimSpace(w.lockfileOld.SpeakeasyVersion) != "" {
		return normalizeHistoricalVersion(w.lockfileOld.SpeakeasyVersion), nil
	}

	if w != nil {
		desiredVersion := strings.TrimSpace(w.workflow.SpeakeasyVersion.String())
		switch desiredVersion {
		case "", "pinned":
			return normalizeHistoricalVersion(events.GetSpeakeasyVersionFromContext(ctx)), nil
		case "latest":
			latestVersion, err := updates.GetLatestVersion(ctx, historicalPristineArtifactArch(ctx))
			if err != nil {
				return "", fmt.Errorf("resolving latest Speakeasy CLI version for historical pristine recovery: %w", err)
			}
			if latestVersion == nil {
				return "", errors.New("latest Speakeasy CLI version unavailable for historical pristine recovery")
			}
			return latestVersion.String(), nil
		default:
			return normalizeHistoricalVersion(desiredVersion), nil
		}
	}

	return normalizeHistoricalVersion(events.GetSpeakeasyVersionFromContext(ctx)), nil
}

func normalizeHistoricalVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "latest" || version == "pinned" {
		return version
	}
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

func historicalPristineArtifactArch(ctx context.Context) string {
	if artifactArch, ok := ctx.Value(updates.ArtifactArchContextKey).(string); ok && strings.TrimSpace(artifactArch) != "" {
		return artifactArch
	}
	return runtime.GOOS + "_" + runtime.GOARCH
}

func defaultRunHistoricalPristineWithVersion(ctx context.Context, w *Workflow, target, language, schemaPath, outDir, desiredVersion string) error {
	binaryPath, err := historicalPristineBinaryPath(ctx, desiredVersion)
	if err != nil {
		return err
	}

	recoveryTarget := cfgworkflow.Target{
		Target: language,
		Source: "historical-source",
	}
	if w != nil {
		if originalTarget, ok := w.workflow.Targets[target]; ok {
			recoveryTarget = originalTarget
			recoveryTarget.Source = "historical-source"
			recoveryTarget.Output = nil
		}
	}

	tempWorkflow := &cfgworkflow.Workflow{
		Version:          cfgworkflow.WorkflowVersion,
		SpeakeasyVersion: cfgworkflow.Version(desiredVersion),
		Sources: map[string]cfgworkflow.Source{
			"historical-source": {
				Inputs: []cfgworkflow.Document{{Location: cfgworkflow.LocationString(schemaPath)}},
			},
		},
		Targets: map[string]cfgworkflow.Target{
			target: recoveryTarget,
		},
	}
	if err := cfgworkflow.Save(outDir, tempWorkflow); err != nil {
		return fmt.Errorf("saving temporary workflow for historical pristine recovery: %w", err)
	}

	cmd := exec.Command(
		binaryPath,
		"run",
		"-t", target,
		"--pinned",
		"--skip-versioning",
		"--skip-testing",
		"--skip-change-report",
		"--skip-snapshot",
		"--skip-cleanup",
		"--skip-compile",
		"--auto-yes",
		"--logLevel", "error",
	)
	cmd.Dir = outDir
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("generating historical pristine baseline with Speakeasy CLI %s: %w\n%s", desiredVersion, err, strings.TrimSpace(string(output)))
	}

	return nil
}

func historicalPristineBinaryPath(ctx context.Context, desiredVersion string) (string, error) {
	currentVersion := normalizeHistoricalVersion(events.GetSpeakeasyVersionFromContext(ctx))
	if desiredVersion == "" || desiredVersion == currentVersion || desiredVersion == "pinned" {
		execPath, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("locating current Speakeasy CLI for historical pristine recovery: %w", err)
		}
		return execPath, nil
	}

	binaryPath, err := updates.InstallVersion(ctx, strings.TrimPrefix(desiredVersion, "v"), historicalPristineArtifactArch(ctx), 30)
	if err != nil {
		return "", fmt.Errorf("installing Speakeasy CLI %s for historical pristine recovery: %w", desiredVersion, err)
	}
	return binaryPath, nil
}
