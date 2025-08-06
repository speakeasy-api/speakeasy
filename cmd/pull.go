package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"archive/zip"
	"bytes"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/loader"
	"github.com/speakeasy-api/speakeasy-core/ocicommon"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
	"oras.land/oras-go/v2/registry/remote"
	orasauth "oras.land/oras-go/v2/registry/remote/auth"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
)

type pullFlags struct {
	Namespace string `json:"namespace"`
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
			Name:        "namespace",
			Description: "The namespace to pull from",
			Required:    true,
			SuggestionsFunc: func() ([]string, error) {
				return getNamespaces()
			},
		},
		flag.StringFlag{
			Name:         "revision",
			Description:  "The revision to pull",
			DefaultValue: "latest",
			SuggestionsFunc: func() ([]string, error) {
				return getTags()
			},
		},
		flag.StringFlag{
			Name:         "output-dir",
			Description:  "The directory to output the image to",
			DefaultValue: "/tmp",
		},
	},
}

func runPull(ctx context.Context, flags pullFlags) error {
	logger := log.From(ctx)

	logger.Infof("Pulling from namespace: %s", flags.Namespace)

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
	access := ocicommon.NewRepositoryAccess(apiKey, flags.Namespace, ocicommon.RepositoryAccessOptions{
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
	// Read the bundle content
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

// getTags connects to a remote OCI registry and retrieves all tags for a given repository.
// It takes a context and the repository name (e.g., "ghcr.io/oras-project/oras-go-demo") as input.
// It returns a slice of strings containing all the tags, or an error if one occurred.
func getTags() ([]string, error) {
	namespace := "first-source"
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
		return nil, fmt.Errorf("no API key available, please run 'speakeasy auth' to authenticate")
	}

	// Create repository access
	access := ocicommon.NewRepositoryAccess(apiKey, namespace, ocicommon.RepositoryAccessOptions{
		Insecure: insecurePublish,
	})
	accessResult, err := access.Acquire(context.Background(), reg)
	if err != nil {
		return nil, fmt.Errorf("error acquiring oci access: %w", err)
	}

	repositoryURL := path.Join(reg, accessResult.Repository)

	// Create a new instance of a remote repository client.
	repo, err := remote.NewRepository(repositoryURL)
	if err != nil {
		return nil, fmt.Errorf("error creating remote repository client: %w", err)
	}

	rh := retryablehttp.NewClient()

	// TODO: remove this once we have a logger
	rh.Logger = nil
	repo.Client = access.WrapClient(&orasauth.Client{
		Client:     rh.StandardClient(),
		Header:     orasauth.DefaultClient.Header,
		Cache:      orasauth.NewCache(),
		Credential: accessResult.CredentialFunc,
	})

	var allTags []string
	// The `Tags` method paginates through the tags from the registry.
	// We provide a callback function that appends each page of tags to our slice.
	err = repo.Tags(context.Background(), "", func(tags []string) error {
		allTags = append(allTags, tags...)
		// Return nil to continue fetching subsequent pages.
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list tags for repository %s: %w", repositoryURL, err)
	}

	return allTags, nil
}

func getNamespaces() ([]string, error) {
	// Initialize speakeasy client
	client, err := sdk.InitSDK()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize speakeasy client: %w", err)
	}

	// Get targets from the events API
	res, err := client.Events.GetTargets(context.Background(), operations.GetWorkspaceTargetsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get targets: %w", err)
	}

	// Extract unique namespaces from targets
	seenNamespaces := make(map[string]bool)
	var namespaces []string

	for _, target := range res.TargetSDKList {
		if target.SourceNamespaceName != nil && !seenNamespaces[*target.SourceNamespaceName] {
			seenNamespaces[*target.SourceNamespaceName] = true
			namespaces = append(namespaces, *target.SourceNamespaceName)
		}
	}

	return namespaces, nil
}
