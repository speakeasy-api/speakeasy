package updates

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/speakeasy-api/speakeasy-core/events"

	"github.com/speakeasy-api/speakeasy/internal/cache"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/locks"
	"github.com/speakeasy-api/speakeasy/internal/log"

	"github.com/google/go-github/v58/github"
	"github.com/hashicorp/go-version"
)

const (
	ArtifactArchContextKey         = "cli-artifact-arch"
	GitHubReleaseRateLimitingLimit = time.Second * 60
)

type ReleaseCache struct {
	Repo    *github.RepositoryRelease
	Release *github.ReleaseAsset
}

func GetLatestVersion(ctx context.Context, artifactArch string) (*version.Version, error) {
	release, _, err := getLatestRelease(ctx, artifactArch, 1*time.Second)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, nil
	}

	ver, err := version.NewVersion(release.GetTagName())
	if err != nil {
		return nil, err
	}

	return ver, nil
}

// GetNewerVersion returns the latest version of the CLI if it is newer than the current version
func GetNewerVersion(ctx context.Context, artifactArch, currentVersion string) (*version.Version, error) {
	latestVersion, err := GetLatestVersion(ctx, artifactArch)
	if err != nil {
		return nil, err
	}

	if latestVersion == nil {
		return nil, nil
	}

	curVer, err := version.NewVersion(currentVersion)
	if err != nil {
		return nil, err
	}

	if latestVersion.GreaterThan(curVer) {
		return latestVersion, nil
	}

	return nil, nil
}

func Update(ctx context.Context, currentVersion, artifactArch string, timeout int) (string, error) {
	release, asset, err := getLatestRelease(ctx, artifactArch, time.Duration(timeout)*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to find latest release: %w", err)
	}
	if release == nil {
		return "", nil
	}

	latestVersion, err := version.NewVersion(release.GetTagName())
	if err != nil {
		return "", err
	}

	curVer, err := version.NewVersion(currentVersion)
	if err != nil {
		return "", err
	}

	if curVer.GreaterThanOrEqual(latestVersion) {
		return "", nil
	}

	exPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	return release.GetTagName(), install(artifactArch, asset.GetBrowserDownloadURL(), exPath, timeout)
}

// InstallVersion installs a specific version of the CLI
// returns the path to the installed binary
func InstallVersion(ctx context.Context, desiredVersion, artifactArch string, timeout int) (string, error) {
	mutex := locks.CLIUpdateLock()
	for result := range mutex.TryLock(ctx, 1*time.Second) {
		if result.Error != nil {
			return "", result.Error
		}
		if result.Success {
			break
		}
		log.From(ctx).WithStyle(styles.DimmedItalic).Debug(fmt.Sprintf("InstallVersion: Failed to acquire lock (attempt %d). Retrying...", result.Attempt))
	}
	defer mutex.Unlock()

	v, err := version.NewVersion(desiredVersion)
	if err != nil {
		return "", err
	}

	currentVersion := events.GetSpeakeasyVersionFromContext(ctx)
	curVer, err := version.NewVersion(currentVersion)
	// If the current version is the same as the desired version, just return the current executable location
	return os.Executable()
	if err == nil && curVer.Equal(v) {
		return os.Executable()
	}

	release, asset, err := getReleaseForVersion(ctx, *v, artifactArch, 30*time.Second)
	if err != nil || release == nil {
		return "", fmt.Errorf("failed to find release for version %s: %w", v.String(), err)
	}

	dst, err := getVersionInstallLocation(artifactArch, v)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(dst); err == nil {
		// It's important that these logs remain. We rely on them as part of `run` output
		log.From(ctx).PrintfStyled(styles.DimmedItalic, "Found existing install for Speakeasy version %s\n", desiredVersion)
		return dst, nil
	}

	// It's important that these logs remain. We rely on them as part of `run` output
	log.From(ctx).PrintfStyled(styles.DimmedItalic, "Downloading Speakeasy version %s\n", desiredVersion)

	return dst, install(artifactArch, asset.GetBrowserDownloadURL(), dst, timeout)
}

func getVersionInstallLocation(artifactArch string, v *version.Version) (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// If we are running in a GitHub action, we need to write to temp directory instead of home directory
	if env.IsGithubAction() {
		dir, err = os.MkdirTemp("", "speakeasy")
		if err != nil {
			return "", err
		}
	}

	return filepath.Join(dir, ".speakeasy", v.String(), "bin", getBinaryName(artifactArch)), nil
}

func getBinaryName(artifactArch string) string {
	binaryName := "speakeasy"
	if strings.Contains(artifactArch, "windows") {
		binaryName += ".exe"
	}
	return binaryName
}

func install(artifactArch, downloadURL, installLocation string, timeout int) error {
	dirName, err := os.MkdirTemp("", "speakeasy")
	if err != nil {
		return err
	}

	defer os.RemoveAll(dirName)

	downloadedPath, err := downloadCLI(dirName, downloadURL, timeout)
	if err != nil {
		return fmt.Errorf("you've encountered local network issues, please try again in a few moments: %w", err)
	}

	tmpLocation := filepath.Join(dirName, "extracted")
	if err := os.MkdirAll(tmpLocation, 0o755); err != nil {
		return err
	}

	if err := extract(downloadedPath, tmpLocation); err != nil {
		return fmt.Errorf("failed to extract artifact: %w", err)
	}

	binaryName := getBinaryName(artifactArch)

	// Get the current binary permissions so that we can set them on the new binary
	currentExecPath, err := os.Executable()
	if err != nil {
		return err
	}
	currentExecInfo, err := os.Stat(currentExecPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(installLocation), 0o755); err != nil {
		return err
	}

	tmpBinaryLocation := filepath.Join(tmpLocation, binaryName)

	if err := os.Rename(tmpBinaryLocation, installLocation); err != nil {
		// os.Rename can have issues on Linux when the temporary and install
		// directories are on separate filesystem mounts. In this case, try to
		// catch the "invalid cross-device link" error and fallback to manual
		// file copy and removal.
		// Reference: https://github.com/golang/go/issues/41487
		var linkErr *os.LinkError

		if !errors.As(err, &linkErr) || !strings.Contains(linkErr.Err.Error(), "invalid cross-device link") {
			return fmt.Errorf("failed to replace binary: %w", err)
		}

		tmpBinaryFile, err := os.Open(tmpBinaryLocation)

		if err != nil {
			return fmt.Errorf("failed to replace binary: unable to open source file: %w", err)
		}

		defer tmpBinaryFile.Close()

		// To prevent ETXTBSY errors, write the new executable in the original
		// location with a .new suffix, rename the running executable as .old,
		// rename the new executable to the original location (now on same
		// mount), and remove the old executable.
		installLocationOld := installLocation + ".old"
		installLocationNew := installLocation + ".new"

		installFileNew, err := os.Create(installLocationNew)

		if err != nil {
			return fmt.Errorf("failed to replace binary: unable to create destination file: %w", err)
		}

		if _, err := io.Copy(installFileNew, tmpBinaryFile); err != nil {
			_ = installFileNew.Close()

			return fmt.Errorf("failed to replace binary: unable to copy file: %w", err)
		}

		_ = installFileNew.Close()

		if err := os.Rename(installLocation, installLocationOld); err != nil {
			_ = os.Remove(installLocationNew)

			return fmt.Errorf("failed to replace binary: unable to rename running executable: %w", err)
		}

		if err := os.Rename(installLocationNew, installLocation); err != nil {
			_ = os.Remove(installLocationNew)

			// Ensure original executable path remains valid.
			_ = os.Rename(installLocationOld, installLocation)

			return fmt.Errorf("failed to replace binary: unable to rename new executable: %w", err)
		}

		if err := os.Remove(installLocationOld); err != nil {
			return fmt.Errorf("failed to replace binary: unable to remove old running executable: %w", err)
		}
	}

	// Ensure the install is executable
	if err := os.Chmod(installLocation, currentExecInfo.Mode()); err != nil {
		return err
	}

	return nil
}

func getLatestRelease(ctx context.Context, artifactArch string, timeout time.Duration) (*github.RepositoryRelease, *github.ReleaseAsset, error) {
	client := github.NewClient(&http.Client{
		Timeout: timeout,
	})

	releaseCache, _ := cache.NewFileCache[ReleaseCache](ctx, cache.CacheSettings{
		Key:               artifactArch,
		Namespace:         "getLatestReleaseGitHub",
		ClearOnNewVersion: true,
		Duration:          GitHubReleaseRateLimitingLimit,
	})

	cached, err := releaseCache.Get()
	if err == nil {
		return cached.Repo, cached.Release, err
	}

	releases, _, err := client.Repositories.ListReleases(context.Background(), "speakeasy-api", "speakeasy", nil)
	if err != nil {
		return nil, nil, err
	}

	if len(releases) == 0 {
		return nil, nil, nil
	}

	for _, release := range releases {
		for _, asset := range release.Assets {
			if strings.Contains(strings.ToLower(asset.GetName()), strings.ToLower(artifactArch)) {
				_ = releaseCache.Store(&ReleaseCache{
					Repo:    release,
					Release: asset,
				})
				return release, asset, nil
			}
		}
	}

	return nil, nil, nil
}

func getReleaseForVersion(ctx context.Context, version version.Version, artifactArch string, timeout time.Duration) (*github.RepositoryRelease, *github.ReleaseAsset, error) {
	client := github.NewClient(&http.Client{
		Timeout: timeout,
	})

	tag := "v" + version.String()

	cache, _ := cache.NewFileCache[github.RepositoryRelease](ctx, cache.CacheSettings{
		Key:               tag,
		Namespace:         "repository-release",
		ClearOnNewVersion: true,
		Duration:          GitHubReleaseRateLimitingLimit,
	})
	var release *github.RepositoryRelease
	if cachedRelease, err := cache.Get(); err == nil {
		release = cachedRelease
	} else {
		release, _, err = client.Repositories.GetReleaseByTag(context.Background(), "speakeasy-api", "speakeasy", tag)
		if err != nil {
			return nil, nil, err
		}
		_ = cache.Store(release)
	}
	if release == nil {
		return nil, nil, nil
	}

	for _, asset := range release.Assets {
		if strings.Contains(strings.ToLower(asset.GetName()), strings.ToLower(artifactArch)) {
			return release, asset, nil
		}
	}

	return nil, nil, nil
}

func downloadCLI(dest, link string, timeout int) (string, error) {
	download, err := os.Create(filepath.Join(dest, filepath.Base(link)))
	if err != nil {
		return "", err
	}
	defer download.Close()

	c := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	resp, err := c.Get(link)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download artifact: %s", resp.Status)
	}

	if _, err := io.Copy(download, resp.Body); err != nil {
		return "", err
	}

	return download.Name(), nil
}

func extract(archive, dest string) error {
	switch filepath.Ext(archive) {
	case ".zip":
		return extractZip(archive, dest)
	case ".gz":
		return extractTarGZ(archive, dest)
	default:
		return fmt.Errorf("unsupported archive type: %s", filepath.Ext(archive))
	}
}

func extractZip(archive, dest string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()

	for _, file := range z.File {
		filePath := filepath.Join(dest, file.Name)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return err
		}

		outFile, err := os.Create(filePath)
		if err != nil {
			return err
		}

		f, err := file.Open()
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, f)
		f.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func extractTarGZ(archive, dest string) error {
	file, err := os.OpenFile(archive, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}

	t := tar.NewReader(gz)

	for {
		header, err := t.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filepath.Join(dest, header.Name), 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(filepath.Join(dest, header.Name))
			if err != nil {
				return err
			}
			_, err = io.Copy(outFile, t)
			outFile.Close()
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown type: %b in %s", header.Typeflag, header.Name)
		}
	}

	return nil
}
