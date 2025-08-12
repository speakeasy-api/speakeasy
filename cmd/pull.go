package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"archive/zip"
	"bytes"

	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/loader"
	"github.com/speakeasy-api/speakeasy-core/ocicommon"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type pullFlags struct {
	Spec      string `json:"spec"`
	Revision  string `json:"revision"`
	OutputDir string `json:"output-dir"`
}

var pullCmd = &model.ExecutableCommand[pullFlags]{
	Usage:        "pull",
	Short:        "pull",
	Run:          runPull,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "spec",
			Description: "The name of the spec to want to pull",
			Required:    true,
		},
		flag.StringFlag{
			Name:         "revision",
			Description:  "The revision to pull",
			DefaultValue: "latest",
			Required:     true,
		},
		flag.StringFlag{
			Name:         "output-dir",
			Description:  "The directory to output the image to",
			DefaultValue: getCurrentWorkingDirectory(),
		},
	},
}

func runPull(ctx context.Context, flags pullFlags) error {
	logger := log.From(ctx)

	logger.Infof("Pulling from spec: %s", flags.Spec)

	// Get server URL and determine if insecure
	serverURL := auth.GetServerURL()
	insecurePublish := false
	if strings.HasPrefix(serverURL, "http://") {
		insecurePublish = true
	}
	reg := strings.TrimPrefix(serverURL, "http://")
	reg = strings.TrimPrefix(reg, "https://")

	// Get API key
	apiKey := config.GetSpeakeasyAPIKey()
	if apiKey == "" {
		return fmt.Errorf("no API key available, please run 'speakeasy auth' to authenticate")
	}

	// Create repository access
	access := ocicommon.NewRepositoryAccess(apiKey, flags.Spec, ocicommon.RepositoryAccessOptions{
		Insecure: insecurePublish,
	})

	// Create bundle loader
	bundleLoader := loader.NewLoader(loader.OCILoaderOptions{
		Registry: reg,
		Access:   access,
	})

	// Load the OpenAPI bundle
	bundleResult, err := bundleLoader.LoadOpenAPIBundle(ctx, flags.Revision)
	if err != nil {
		logger.Errorf("Error loading OCI image by revision %s: %v", flags.Revision, err)
		return fmt.Errorf("failed to load bundle: %w", err)
	}

	defer bundleResult.Body.Close()

	// Create output directory
	if err := os.MkdirAll(flags.OutputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Extract bundle to output directory
	if err := extractBundle(bundleResult, flags.OutputDir); err != nil {
		return fmt.Errorf("failed to extract bundle: %w", err)
	}

	logger.Infof("Successfully pulled bundle to %s", flags.OutputDir)
	return nil
}

func extractBundle(bundleResult *loader.OpenAPIBundleResult, outputDir string) error {
	buf, err := io.ReadAll(bundleResult.Body)
	if err != nil {
		return fmt.Errorf("failed to read bundle content: %w", err)
	}

	// Create zip reader
	reader := bytes.NewReader(buf)
	zipReader, err := zip.NewReader(reader, int64(len(buf)))
	if err != nil {
		return fmt.Errorf("failed to create zip reader: %w", err)
	}

	// Extract files
	for _, file := range zipReader.File {
		cleanName := filepath.Clean(file.Name)
		filePath := filepath.Join(outputDir, cleanName)

		// Security check to prevent path traversal
		if !strings.HasPrefix(filePath, filepath.Clean(outputDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", filePath)
		}

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Skip if it's a directory
		if file.FileInfo().IsDir() {
			continue
		}

		// Create file
		dst, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer dst.Close()

		// Open source file
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}
		defer src.Close()

		// Copy content
		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("failed to copy file content: %w", err)
		}
	}

	return nil
}

func getCurrentWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	return cwd
}
