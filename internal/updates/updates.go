package updates

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v52/github"
	"github.com/hashicorp/go-version"
)

func GetLatestVersion(artifactArch string) (*version.Version, error) {
	release, _, err := getLatestRelease(artifactArch, 1*time.Second)
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

func Update(currentVersion, artifactArch string) (string, error) {
	release, asset, err := getLatestRelease(artifactArch, 30*time.Second)
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

	dirName, err := os.MkdirTemp("", "speakeasy")
	if err != nil {
		return "", err
	}

	downloadedPath, err := downloadCLI(dirName, asset.GetBrowserDownloadURL())
	if err != nil {
		return "", fmt.Errorf("failed to download artifact: %w", err)
	}

	extractDest := filepath.Join(dirName, "extracted")
	if err := os.MkdirAll(extractDest, 0o755); err != nil {
		return "", err
	}

	if err := extractTarGZ(downloadedPath, extractDest); err != nil {
		return "", fmt.Errorf("failed to extract artifact: %w", err)
	}

	exPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	binaryName := "speakeasy"
	if strings.Contains(artifactArch, "windows") {
		binaryName += ".exe"
	}

	info, err := os.Stat(exPath)
	if err != nil {
		return "", err
	}

	if err := os.Rename(path.Join(extractDest, binaryName), exPath); err != nil {
		return "", fmt.Errorf("failed to replace binary: %w", err)
	}

	if err := os.Chmod(exPath, info.Mode()); err != nil {
		return "", err
	}

	return release.GetTagName(), nil
}

func getLatestRelease(artifactArch string, timeout time.Duration) (*github.RepositoryRelease, *github.ReleaseAsset, error) {
	client := github.NewClient(&http.Client{
		Timeout: timeout,
	})

	releases, _, err := client.Repositories.ListReleases(context.Background(), "speakeasy-api", "speakeasy", nil)
	if err != nil {
		return nil, nil, err
	}

	if len(releases) == 0 {
		return nil, nil, nil
	}

	for _, release := range releases {
		for _, asset := range release.Assets {
			if strings.HasSuffix(strings.ToLower(asset.GetName()), strings.ToLower(artifactArch)+".tar.gz") {
				return release, asset, nil
			}
		}
	}

	return nil, nil, nil
}

func downloadCLI(dest, link string) (string, error) {
	download, err := os.Create(filepath.Join(dest, path.Base(link)))
	if err != nil {
		return "", err
	}
	defer download.Close()

	c := &http.Client{
		Timeout: 30 * time.Second,
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

func extractTarGZ(archive, dest string) error {
	file, err := os.OpenFile(archive, os.O_RDONLY, 0)
	if err != nil {
		return err
	}

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
			if err := os.MkdirAll(path.Join(dest, header.Name), 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(path.Join(dest, header.Name))
			if err != nil {
				fmt.Println("here1")
				return err
			}
			_, err = io.Copy(outFile, t)
			outFile.Close()
			if err != nil {
				fmt.Println("here2")
				return err
			}
		default:
			return fmt.Errorf("unknown type: %b in %s", header.Typeflag, header.Name)
		}
	}

	return nil
}
