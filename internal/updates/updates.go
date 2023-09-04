package updates

import (
	"archive/tar"
	"archive/zip"
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

func Update(currentVersion, artifactArch string, timeout int) (string, error) {
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

	downloadedPath, err := downloadCLI(dirName, asset.GetBrowserDownloadURL(), timeout)
	if err != nil {
		return "", fmt.Errorf("failed to download artifact: %w", err)
	}

	extractDest := filepath.Join(dirName, "extracted")
	if err := os.MkdirAll(extractDest, 0o755); err != nil {
		return "", err
	}

	if err := extract(downloadedPath, extractDest); err != nil {
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
			if strings.Contains(strings.ToLower(asset.GetName()), strings.ToLower(artifactArch)) {
				return release, asset, nil
			}
		}
	}

	return nil, nil, nil
}

func downloadCLI(dest, link string, timeout int) (string, error) {
	download, err := os.Create(filepath.Join(dest, path.Base(link)))
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
		filePath := path.Join(dest, file.Name)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(path.Dir(filePath), 0o755); err != nil {
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
			if err := os.MkdirAll(path.Join(dest, header.Name), 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(path.Join(dest, header.Name))
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
