package document

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/speakeasy-api/speakeasy/internal/ci/download"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/pkg/merge"
	"github.com/speakeasy-api/speakeasy/pkg/overlay"
	"gopkg.in/yaml.v3"
)

type file struct {
	Location string `yaml:"location"`
	Header   string `yaml:"auth_header"`
	Token    string `yaml:"auth_token"`
}

func GetOpenAPIFileInfo(ctx context.Context) (string, string, error) {
	// TODO OPENAPI_DOC_LOCATION is deprecated and should be removed in the future
	openapiFiles, err := getFiles(environment.GetOpenAPIDocs(), environment.GetOpenAPIDocLocation())
	if err != nil {
		return "", "", err
	}

	resolvedOpenAPIFiles, err := resolveFiles(openapiFiles, "openapi")
	if err != nil {
		return "", "", err
	}

	filePath := ""

	if len(resolvedOpenAPIFiles) == 1 {
		filePath = resolvedOpenAPIFiles[0]
	} else {
		filePath, err = mergeFiles(ctx, resolvedOpenAPIFiles)
		if err != nil {
			return "", "", err
		}
	}

	overlayFiles, err := getFiles(environment.GetOverlayDocs(), "")
	if err != nil {
		return "", "", err
	}

	resolvedOverlayFiles, err := resolveFiles(overlayFiles, "overlay")
	if err != nil {
		return "", "", err
	}

	if len(resolvedOverlayFiles) > 0 {
		filePath, err = applyOverlayFiles(filePath, resolvedOverlayFiles)
		if err != nil {
			return "", "", err
		}
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read openapi file: %w", err)
	}

	var doc struct {
		Info struct {
			Version string `yaml:"version"`
		} `yaml:"info"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", "", fmt.Errorf("failed to parse openapi file: %w", err)
	}

	version := "0.0.0"
	if doc.Info.Version != "" {
		version = doc.Info.Version
	}

	return filePath, version, nil
}

func mergeFiles(ctx context.Context, files []string) (string, error) {
	outPath := filepath.Join(environment.GetWorkspace(), ".openapi", "openapi_merged")

	if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create openapi directory: %w", err)
	}

	absOutPath, err := filepath.Abs(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for openapi file: %w", err)
	}

	if err := merge.MergeOpenAPIDocuments(ctx, files, absOutPath, "", "", false); err != nil {
		return "", fmt.Errorf("failed to merge openapi files: %w", err)
	}

	return absOutPath, nil
}

func applyOverlayFiles(filePath string, overlayFiles []string) (string, error) {
	for i, overlayFile := range overlayFiles {
		outPath := filepath.Join(environment.GetWorkspace(), "openapi", fmt.Sprintf("openapi_overlay_%v", i))

		if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
			return "", fmt.Errorf("failed to create openapi directory: %w", err)
		}

		outPathAbs, err := filepath.Abs(outPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path for openapi overlay file: %w", err)
		}

		outFile, err := os.Create(outPathAbs)
		if err != nil {
			return "", fmt.Errorf("failed to create overlay output file: %w", err)
		}

		yamlOut := utils.HasYAMLExt(outPathAbs)
		if _, err := overlay.Apply(filePath, overlayFile, yamlOut, outFile, false, false); err != nil {
			outFile.Close()
			return "", fmt.Errorf("failed to apply overlay: %w", err)
		}

		outFile.Close()
		filePath = outPathAbs
	}

	return filePath, nil
}

func resolveFiles(files []file, typ string) ([]string, error) {
	workspace := environment.GetWorkspace()

	outFiles := []string{}

	for i, file := range files {
		localPath := filepath.Join(workspace, "repo", file.Location)

		if _, err := os.Stat(localPath); err == nil {
			fmt.Printf("Found local %s file: %s\n", typ, localPath)
			absPath, err := filepath.Abs(localPath)
			if err != nil {
				return nil, fmt.Errorf("failed to get absolute path for %s file: %w", localPath, err)
			}

			outFiles = append(outFiles, absPath)
		} else {
			u, err := url.Parse(file.Location)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s url: %w", typ, err)
			}

			fmt.Printf("Downloading %s file from: %s\n", typ, u.String())

			filePath := filepath.Join(environment.GetWorkspace(), typ, fmt.Sprintf("%s_%d", typ, i))

			if environment.GetAction() == environment.ActionValidate {
				if extension := path.Ext(u.Path); extension != "" {
					filePath = filePath + extension
				}
			}

			if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
				return nil, fmt.Errorf("failed to create %s directory: %w", typ, err)
			}

			absPath, err := filepath.Abs(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to get absolute path for %s file: %w", filePath, err)
			}

			if err := download.DownloadFile(u.String(), absPath, file.Header, file.Token); err != nil {
				return nil, fmt.Errorf("failed to download %s file: %w", typ, err)
			}

			outFiles = append(outFiles, absPath)
		}
	}

	return outFiles, nil
}

func getFiles(filesYaml string, defaultFile string) ([]file, error) {
	var fileLocations []string
	if err := yaml.Unmarshal([]byte(filesYaml), &fileLocations); err != nil {
		return nil, fmt.Errorf("failed to parse openapi_docs input: %w", err)
	}

	if len(fileLocations) == 0 && defaultFile != "" {
		fileLocations = append(fileLocations, defaultFile)
	}

	files := []file{}

	for _, fileLoc := range fileLocations {
		files = append(files, file{
			Location: fileLoc,
			Header:   environment.GetOpenAPIDocAuthHeader(),
			Token:    environment.GetOpenAPIDocAuthToken(),
		})
	}

	return files, nil
}
