package download

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func DownloadFile(url string, outPath string, header string, token string) error {
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

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
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
