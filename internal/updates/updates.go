package updates

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	"github.com/google/go-github/v63/github"
	"github.com/hashicorp/go-version"
)

type contextKey string

const (
	ArtifactArchContextKey         contextKey = "cli-artifact-arch"
	GitHubReleaseRateLimitingLimit            = time.Second * 60
	fallbackBaseURL                           = "https://cli-releases.speakeasy.com"
)

type fallbackDownloadResponse struct {
	URL string `json:"url"`
}

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
	defer func() { _ = mutex.Unlock() }()

	v, err := version.NewVersion(desiredVersion)
	if err != nil {
		return "", err
	}

	currentVersion := events.GetSpeakeasyVersionFromContext(ctx)
	curVer, err := version.NewVersion(currentVersion)
	// If the current version is the same as the desired version, just return the current executable location
	if err == nil && curVer.Equal(v) {
		return os.Executable()
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

	// Release asset URLs are deterministic, so a known version can be
	// downloaded without first resolving release metadata through the GitHub
	// API, whose unauthenticated 60 requests/hour rate limit is easily
	// exhausted on shared CI runner IPs. Asset downloads are not subject to
	// that limit. If this fails for any reason, fall back to the API lookup.
	directErr := install(artifactArch, directAssetURL(v, artifactArch), dst, timeout)
	if directErr == nil {
		return dst, nil
	}
	log.From(ctx).Debug(fmt.Sprintf("direct download of version %s failed, falling back to GitHub release lookup: %s", v.String(), directErr.Error()))

	release, asset, err := getReleaseForVersion(ctx, *v, artifactArch, 30*time.Second)
	if err != nil || release == nil {
		return "", fmt.Errorf("failed to find release for version %s: %w", v.String(), err)
	}

	return dst, install(artifactArch, asset.GetBrowserDownloadURL(), dst, timeout)
}

// directAssetURL returns the conventional GitHub release asset URL for a CLI
// version. Asset names follow the goreleaser convention speakeasy_{os}_{arch}.zip.
func directAssetURL(v *version.Version, artifactArch string) string {
	return fmt.Sprintf("https://github.com/speakeasy-api/speakeasy/releases/download/v%s/speakeasy_%s.zip", v.String(), strings.ToLower(artifactArch))
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

	defer func() { _ = os.RemoveAll(dirName) }()

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

// githubToken returns the first GitHub token set in the environment, or "".
func githubToken() string {
	for _, key := range []string{"SPEAKEASY_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"} {
		if token := os.Getenv(key); token != "" {
			return token
		}
	}
	return ""
}

// githubClient returns a GitHub API client, authenticated when a token is
// available in the environment. Unauthenticated requests share a 60/hour rate
// limit per IP, which shared CI runners exhaust quickly.
func githubClient(timeout time.Duration) *github.Client {
	client := github.NewClient(&http.Client{
		Timeout: timeout,
	})
	if token := githubToken(); token != "" {
		return client.WithAuthToken(token)
	}
	return client
}

// describeGitHubError surfaces rate limiting explicitly: without this, a
// rate-limited lookup for a pinned version bubbles up as "release not found",
// which misleadingly points at the version pin rather than the real cause.
func describeGitHubError(err error) error {
	var rateLimitErr *github.RateLimitError
	if errors.As(err, &rateLimitErr) {
		remedy := "retry after " + rateLimitErr.Rate.Reset.Format(time.RFC1123)
		if rateLimitErr.Rate.Limit <= 60 {
			// The anonymous per-IP limit; authenticating raises it substantially.
			remedy = "set GITHUB_TOKEN to authenticate, or " + remedy
		}
		return fmt.Errorf("GitHub API rate limit exceeded (%d requests/hour; %s): %w", rateLimitErr.Rate.Limit, remedy, err)
	}
	var abuseErr *github.AbuseRateLimitError
	if errors.As(err, &abuseErr) {
		// Secondary limits can hit authenticated clients too, so only suggest a
		// token when the request was actually anonymous.
		var remedies []string
		if githubToken() == "" {
			remedies = append(remedies, "set GITHUB_TOKEN to authenticate")
		}
		if abuseErr.RetryAfter != nil {
			remedies = append(remedies, "retry after "+abuseErr.RetryAfter.String())
		}
		if len(remedies) > 0 {
			return fmt.Errorf("GitHub API rate limit exceeded (secondary limit; %s): %w", strings.Join(remedies, ", or "), err)
		}
		return fmt.Errorf("GitHub API rate limit exceeded (secondary limit): %w", err)
	}
	return err
}

func getLatestRelease(ctx context.Context, artifactArch string, timeout time.Duration) (*github.RepositoryRelease, *github.ReleaseAsset, error) {
	client := githubClient(timeout)

	releaseCache, _ := cache.NewFileCache[ReleaseCache](ctx, cache.CacheSettings{
		Key:               artifactArch,
		Namespace:         "getLatestReleaseGitHub",
		ClearOnNewVersion: true,
		Duration:          GitHubReleaseRateLimitingLimit,
	})

	cached, err := releaseCache.Get()
	if err == nil {
		return cached.Repo, cached.Release, nil
	}

	releases, _, err := client.Repositories.ListReleases(context.Background(), "speakeasy-api", "speakeasy", nil)
	if err != nil {
		var fallbackErr error
		releases, fallbackErr = fetchReleasesFromFallback(timeout)
		if fallbackErr != nil {
			return nil, nil, describeGitHubError(err) // return original error
		}
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
	client := githubClient(timeout)

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
			// Fall back to the caching proxy and filter by tag.
			releases, fallbackErr := fetchReleasesFromFallback(timeout)
			if fallbackErr != nil {
				return nil, nil, describeGitHubError(err) // return original error
			}
			for _, r := range releases {
				if r.GetTagName() == tag {
					release = r
					break
				}
			}
			if release == nil {
				// The fallback only serves recent releases, so an older tag
				// missing from it is not evidence the release doesn't exist.
				// Preserve the original GitHub error rather than discarding it.
				return nil, nil, fmt.Errorf("release %s not found in fallback cache (which only holds recent releases); GitHub API lookup failed: %w", tag, describeGitHubError(err))
			}
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
	downloadURL := link

	c := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	resp, err := c.Get(downloadURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		// Try fallback: resolve a signed URL via the caching proxy.
		fallbackURL, fallbackErr := getFallbackDownloadURL(link, time.Duration(timeout)*time.Second)
		if fallbackErr != nil {
			if err != nil {
				return "", err // return original error
			}
			return "", fmt.Errorf("failed to download artifact: %s", resp.Status)
		}
		downloadURL = fallbackURL
		resp, err = c.Get(downloadURL)
		if err != nil {
			return "", err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download artifact: %s", resp.Status)
	}

	download, err := os.Create(filepath.Join(dest, filepath.Base(link)))
	if err != nil {
		return "", err
	}
	defer download.Close()

	if _, err := io.Copy(download, resp.Body); err != nil {
		return "", err
	}

	return download.Name(), nil
}

// fetchReleasesFromFallback calls the caching proxy's list endpoint and
// unmarshals the response into GitHub RepositoryRelease objects.
func fetchReleasesFromFallback(timeout time.Duration) ([]*github.RepositoryRelease, error) {
	c := &http.Client{Timeout: timeout}
	resp, err := c.Get(fallbackBaseURL + "?action=list")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fallback list failed: %s", resp.Status)
	}

	var releases []*github.RepositoryRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("fallback list decode: %w", err)
	}

	return releases, nil
}

// getFallbackDownloadURL parses a GitHub release asset URL to extract the tag
// and asset name, then asks the caching proxy for a signed download URL.
func getFallbackDownloadURL(link string, timeout time.Duration) (string, error) {
	// GitHub asset URLs look like:
	//   https://github.com/speakeasy-api/speakeasy/releases/download/v1.2.3/speakeasy_linux_amd64.zip
	u, err := url.Parse(link)
	if err != nil {
		return "", err
	}

	parts := strings.Split(u.Path, "/")
	// Expected: ["", "speakeasy-api", "speakeasy", "releases", "download", "v1.2.3", "asset.zip"]
	if len(parts) < 7 {
		return "", fmt.Errorf("unexpected GitHub asset URL format: %s", link)
	}
	tag := parts[len(parts)-2]
	asset := parts[len(parts)-1]

	c := &http.Client{Timeout: timeout}
	reqURL := fmt.Sprintf("%s?action=download&tag=%s&asset=%s", fallbackBaseURL, url.QueryEscape(tag), url.QueryEscape(asset))
	resp, err := c.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fallback download failed: %s", resp.Status)
	}

	var dr fallbackDownloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return "", fmt.Errorf("fallback download decode: %w", err)
	}

	if dr.URL == "" {
		return "", fmt.Errorf("fallback returned empty download URL")
	}

	return dr.URL, nil
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

// securePath joins name onto dest and ensures the result stays inside dest,
// rejecting entries like "../../evil" that would escape via path traversal
// (zip-slip).
func securePath(dest, name string) (string, error) {
	path := filepath.Join(dest, name)
	if path != filepath.Clean(dest) && !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
		return "", fmt.Errorf("illegal archive entry path escaping destination: %s", name)
	}
	return path, nil
}

func extractZip(archive, dest string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer func() { _ = z.Close() }()

	for _, file := range z.File {
		filePath, err := securePath(dest, file.Name)
		if err != nil {
			return err
		}

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

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return err
		}

		headerPath, err := securePath(dest, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(headerPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(headerPath), 0o755); err != nil {
				return err
			}
			outFile, err := os.Create(headerPath)
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
