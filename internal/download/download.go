package download

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/download"
	"github.com/speakeasy-api/speakeasy/internal/cache"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/log"

	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/loader"
	"github.com/speakeasy-api/speakeasy-core/ocicommon"
	"github.com/speakeasy-api/speakeasy/internal/config"
)

const (
	maxAttempts     = 3
	baseDelay       = 1 * time.Second
	bundleCacheTime = time.Hour * 24 * 7
)

var allowedDocumentExtensions = []string{".yaml", ".yml", ".json"}
var ErrUnknownDocumentType = fmt.Errorf("unrecognized document extension")

type DownloadedRegistryOpenAPIBundle struct {
	LocalFilePath     string
	NamespaceName     string
	ManifestReference string
	ManifestDigest    string
	BlobDigest        string
}

func Fetch(url, header, token string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if header != "" {
		if token == "" {
			return nil, fmt.Errorf("token required for header")
		}
		req.Header.Add(header, token)
	}

	var res *http.Response
	for i := 0; i < maxAttempts; i++ {
		res, err = http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to download file: %w", err)
		}

		// retry for any 5xx status code
		if res.StatusCode < 500 || res.StatusCode > 599 || i >= maxAttempts-1 {
			break
		}

		res.Body.Close()
		jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
		time.Sleep(baseDelay*time.Duration(i+1) + jitter)
	}

	var resErr error
	switch res.StatusCode {
	case 204:
		fallthrough
	case 404:
		resErr = fmt.Errorf("file not found")
	case 401:
		resErr = fmt.Errorf("unauthorized, please ensure auth_header and auth_token inputs are set correctly and a valid token has been provided")
	default:
		if res.StatusCode/100 != 2 {
			resErr = fmt.Errorf("failed to download file: %s", res.Status)
		}
	}

	if resErr != nil {
		defer res.Body.Close()
		return nil, resErr
	}

	return res, nil
}

func DownloadFile(url, outPath, header, token string) error {
	res, err := Fetch(url, header, token)
	if err != nil {
		return err
	}
	defer res.Body.Close()

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

func documentKey(document workflow.SpeakeasyRegistryDocument) string {
	hashKey := strings.Builder{}
	hashKey.WriteString(document.WorkspaceSlug)
	hashKey.WriteString(":")
	hashKey.WriteString(document.OrganizationSlug)
	hashKey.WriteString(":")
	hashKey.WriteString(document.NamespaceName)
	hashKey.WriteString(":")
	hashKey.WriteString(document.NamespaceID)
	hashKey.WriteString(":")
	hashKey.WriteString(document.Reference)
	return hashKey.String()
}

func canCache(document workflow.SpeakeasyRegistryDocument) bool {
	return strings.HasPrefix(document.Reference, "sha256:")
}

type BundleResultCache struct {
	// Body holds the bundle data
	Body string
	// MediaType represents the bundle's data
	MediaType string
	// Annotations alongside manifest data
	BundleAnnotations map[string]string
	// BlobDigest is the digest of the blob in the layer of the manifest
	BlobDigest string
	// ManifestDigest is the digest of the manifest
	ManifestDigest string
}

// DownloadRegistryOpenAPIBundle Returns a file path within the downloaded bundle or error
func DownloadRegistryOpenAPIBundle(ctx context.Context, document workflow.SpeakeasyRegistryDocument, outPath string) (*DownloadedRegistryOpenAPIBundle, error) {
	var fileCache *cache.FileCache[BundleResultCache]
	if canCache(document) {
		fileCache, _ = cache.NewFileCache[BundleResultCache](ctx, cache.CacheSettings{
			Key:               documentKey(document),
			Namespace:         "oasbundle",
			ClearOnNewVersion: false,
			Duration:          bundleCacheTime,
		})
	}
	bundleCache, err := fileCache.Get()
	if err != nil {
		serverURL := auth.GetServerURL()
		insecurePublish := false
		if strings.HasPrefix(serverURL, "http://") {
			insecurePublish = true
		}
		reg := strings.TrimPrefix(serverURL, "http://")
		reg = strings.TrimPrefix(reg, "https://")

		apiKey := config.GetWorkspaceAPIKey(document.OrganizationSlug, document.WorkspaceSlug)
		if apiKey == "" {
			apiKey = config.GetSpeakeasyAPIKey()
		}

		workspaceID, err := auth.GetWorkspaceIDFromContext(ctx)
		if err != nil {
			return nil, err
		}

		access := ocicommon.NewRepositoryAccess(apiKey, document.NamespaceName, ocicommon.RepositoryAccessOptions{
			Insecure: insecurePublish,
		})
		if (document.WorkspaceSlug != auth.GetWorkspaceSlugFromContext(ctx) || document.OrganizationSlug != auth.GetOrgSlugFromContext(ctx)) && workspaceID == "self" {
			access = ocicommon.NewRepositoryAccessAdmin(apiKey, document.NamespaceID, document.NamespaceName, false, ocicommon.RepositoryAccessOptions{
				Insecure: insecurePublish,
			})
		}

		bundleLoader := loader.NewLoader(loader.OCILoaderOptions{
			Registry: reg,
			Access:   access,
		})

		bundleResult, err := bundleLoader.LoadOpenAPIBundle(ctx, document.Reference)
		if err != nil {
			return nil, err
		}

		defer bundleResult.Body.Close()

		buf, err := io.ReadAll(bundleResult.Body)
		if err != nil {
			return nil, err
		}
		bodyEncoded := base64.StdEncoding.EncodeToString(buf)

		bundleCache = &BundleResultCache{
			Body:              bodyEncoded,
			MediaType:         bundleResult.MediaType,
			BundleAnnotations: bundleResult.BundleAnnotations,
			BlobDigest:        bundleResult.BlobDigest,
			ManifestDigest:    bundleResult.ManifestDigest,
		}

		err = fileCache.Store(bundleCache)
		if err == nil {
			log.From(ctx).Infof("Stored bundle into global cache")
		}
	} else {
		log.From(ctx).Infof("Loading bundle from cache")
	}
	buf, err := base64.StdEncoding.DecodeString(bundleCache.Body)
	if err != nil {
		if err = fileCache.Delete(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("error loading bundle")
	}

	reader := bytes.NewReader(buf)
	zipReader, err := zip.NewReader(reader, int64(len(buf)))
	if err != nil {
		return nil, err
	}

	shortDigest := bundleCache.BlobDigest[8:14]
	outPath = filepath.Join(outPath, shortDigest)

	var outputFileName string
	if fileName, _ := bundleCache.BundleAnnotations[ocicommon.AnnotationBundleRoot]; fileName != "" {
		cleanName := filepath.Clean(fileName)
		outputFileName = filepath.Join(outPath, cleanName)
		err = os.MkdirAll(filepath.Dir(outputFileName), os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no root openapi file found in bundle")
	}

	if err := copyZipToOutDir(zipReader, outPath); err != nil {
		return nil, fmt.Errorf("failed to copy zip contents to outdir: %w", err)
	}

	return &DownloadedRegistryOpenAPIBundle{
		LocalFilePath:     outputFileName,
		NamespaceName:     document.NamespaceName,
		ManifestReference: document.Reference,
		ManifestDigest:    bundleCache.ManifestDigest,
		BlobDigest:        bundleCache.BlobDigest,
	}, nil
}

func copyZipToOutDir(zipReader *zip.Reader, outDir string) error {
	for _, file := range zipReader.File {
		cleanName := filepath.Clean(file.Name)
		filePath := filepath.Join(outDir, cleanName)

		if !strings.HasPrefix(filePath, filepath.Clean(outDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", filePath)
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return err
		}

		if file.FileInfo().IsDir() {
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()

		// If the target file exists and has the same content, skip it
		if existingFile, err := os.Open(filePath); err == nil {
			defer existingFile.Close()
			existingContent, err := io.ReadAll(existingFile)
			if err != nil {
				return err
			}

			newContent, err := io.ReadAll(fileReader)
			if err != nil {
				return err
			}

			if bytes.Equal(existingContent, newContent) {
				continue // Skip this file as it's unchanged
			}

			if err := existingFile.Close(); err != nil {
				return err
			}

			// Else (can happen if we had a partial extraction), given the folder name is based on a checksum of the zip, we are safe to just delete the file
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := fileReader.Close(); err != nil {
				return err
			}

			if fileReader, err = file.Open(); err != nil {
				return err
			}
			defer fileReader.Close()
		}

		targetFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o660)
		if err != nil {
			return err
		}
		defer targetFile.Close()

		if _, err := io.Copy(targetFile, fileReader); err != nil {
			return err
		}
	}

	return nil
}

func SniffDocumentExtension(res *http.Response) (string, error) {
	ext := path.Ext(res.Request.URL.Path)
	if slices.Contains(allowedDocumentExtensions, ext) {
		return ext, nil
	}

	contentType := res.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", fmt.Errorf("failed to parse content type: %w", err)
	}

	switch {
	case strings.HasSuffix(mediaType, "yaml") || strings.HasSuffix(mediaType, "yml"):
		return ".yaml", nil
	case strings.HasSuffix(mediaType, "json"):
		return ".json", nil
	default:
		return "", fmt.Errorf("%w: unsupported media type: %s", ErrUnknownDocumentType, mediaType)
	}
}

func ResolveRemoteDocument(ctx context.Context, d workflow.Document, outPath string) (string, error) {
	var token, header string
	if d.Auth != nil {
		header = d.Auth.Header
		envVar := strings.TrimPrefix(d.Auth.Secret, "$")

		// GitHub action secrets are prefixed with INPUT_
		if env.IsGithubAction() {
			envVar = "INPUT_" + envVar
		}
		token = os.Getenv(strings.ToUpper(envVar))
	}

	var fileResponse *http.Response
	var err error
	fileResponse, err = download.Fetch(d.Location.Resolve(), header, token)
	if err != nil {
		// Retry with bearer in case this is a github PAT and the user forgot
		if env.IsGithubAction() && !strings.HasPrefix(token, "bearer") {
			fileResponse, err = download.Fetch(d.Location.Resolve(), header, fmt.Sprintf("bearer %s", token))
			if err != nil {
				return "", err
			}
		}

		return "", err
	}
	defer fileResponse.Body.Close()

	ext := filepath.Ext(outPath)
	if !slices.Contains([]string{".yaml", ".yml", ".json"}, ext) {
		ext, err := download.SniffDocumentExtension(fileResponse)
		if errors.Is(err, download.ErrUnknownDocumentType) {
			ext = ".yaml"
		} else if err != nil {
			return "", err
		}

		outPath += ext
	}

	if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
		return "", err
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, fileResponse.Body); err != nil {
		return "", fmt.Errorf("failed to save response to location: %w", err)
	}

	log.From(ctx).Infof("Downloaded %s to %s\n", d.Location, outPath)

	return outPath, nil
}
