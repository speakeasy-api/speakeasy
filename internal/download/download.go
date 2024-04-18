package download

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/loader"
	"github.com/speakeasy-api/speakeasy-core/ocicommon"
	"github.com/speakeasy-api/speakeasy/internal/config"
)

const (
	maxAttempts = 3
	baseDelay   = 1 * time.Second
)

func DownloadFile(url, outPath, header, token string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if header != "" {
		if token == "" {
			return fmt.Errorf("token required for header")
		}
		req.Header.Add(header, token)
	}

	var res *http.Response
	for i := 0; i < maxAttempts; i++ {
		res, err = http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to download file: %w", err)
		}

		// retry for any 5xx status code
		if res.StatusCode < 500 || res.StatusCode > 599 || i >= maxAttempts-1 {
			break
		}

		res.Body.Close()
		jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
		time.Sleep(baseDelay*time.Duration(i+1) + jitter)
	}

	defer res.Body.Close()

	switch res.StatusCode {
	case 204:
		fallthrough
	case 404:
		return fmt.Errorf("file not found")
	case 401:
		return fmt.Errorf("unauthorized, please ensure auth_header and auth_token inputs are set correctly and a valid token has been provided")
	default:
		if res.StatusCode/100 != 2 {
			return fmt.Errorf("failed to download file: %s", res.Status)
		}
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create file for download: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, res.Body); err != nil {
		return fmt.Errorf("failed to copy file to location: %w", err)
	}

	return nil
}

// DownloadRegistryBundle Returns a file path within the downloaded bundle or error
func DownloadRegistryBundle(ctx context.Context, namespaceID, reference, outPath string) (string, error) {
	serverURL := auth.GetServerURL()
	insecurePublish := false
	if strings.HasPrefix(serverURL, "http://") {
		insecurePublish = true
	}
	reg := strings.TrimPrefix(serverURL, "http://")
	reg = strings.TrimPrefix(reg, "https://")

	bundleLoader := loader.NewLoader(loader.OCILoaderOptions{
		Registry: reg,
		Access: ocicommon.NewRepositoryAccess(config.GetSpeakeasyAPIKey(), namespaceID, ocicommon.RepositoryAccessOptions{
			Insecure: insecurePublish,
		}),
	})

	bundleResult, err := bundleLoader.LoadOpenAPIBundle(ctx, reference)
	if err != nil {
		return "", err
	}

	buf, err := io.ReadAll(bundleResult.Body)
	if err != nil {
		return "", err
	}
	defer bundleResult.Body.Close()

	reader := bytes.NewReader(buf)
	zipReader, err := zip.NewReader(reader, int64(len(buf)))
	if err != nil {
		return "", err
	}

	// Iterate through the files in the archive,
	for _, file := range zipReader.File {
		fmt.Printf("File: %s\n", file.Name)
	}

	return "", errors.New("not finished implementing")
}
